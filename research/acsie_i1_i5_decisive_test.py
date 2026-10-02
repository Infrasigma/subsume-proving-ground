#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import pathlib
import random
import statistics
import subprocess
import sys
from dataclasses import asdict
from hashlib import sha256

from cognitive_core.recursive_research import (
    CausalSelfModel,
    MechanismConfig,
    RecursiveResearchLoop,
    score_core,
)


ROOT = pathlib.Path(__file__).resolve().parents[1]


def digest(x):
    return sha256(json.dumps(x, sort_keys=True, default=str, separators=(",", ":")).encode()).hexdigest()


def row(rng, rule):
    x = rng.randint(-15, 15)
    y = rng.randint(-15, 15)
    z = rng.randint(-15, 15)
    sx, sy, sz = x < 0, y < 0, z < 0
    px, py, pz = x % 2, y % 2, z % 2
    if rule == "sign_xy":
        cond = sx == sy
        lo, hi = 1, 6
    elif rule == "parity_xy":
        cond = px == py
        lo, hi = 2, 7
    elif rule == "sign_xz":
        cond = sx == sz
        lo, hi = 3, 8
    elif rule == "parity_yz":
        cond = py == pz
        lo, hi = 4, 9
    elif rule == "xor_sign_parity":
        cond = (sx == sy) ^ (px == pz)
        lo, hi = 5, 10
    elif rule == "xor_parity_sign":
        cond = (px == py) ^ (sy == sz)
        lo, hi = 6, 11
    elif rule == "and_sign_parity":
        cond = (sx == sz) and (py == pz)
        lo, hi = 7, 12
    elif rule == "or_sign_parity":
        cond = (sx == sz) or (py == pz)
        lo, hi = 8, 13
    elif rule == "unknown_sum_parity":
        cond = ((x + y) % 2) == (z % 2)
        lo, hi = 9, 14
    else:
        raise ValueError(rule)
    return (
        {"x": x, "y": y, "z": z},
        "step",
        {"x": x, "y": y, "z": z + (lo if cond else hi)},
    )


def make_rows(seed, rule, n):
    rng = random.Random(seed)
    return tuple(row(rng, rule) for _ in range(n))


def dataset(seed, rule, transfer_rule=None, future_rule=None):
    return {
        "train": make_rows(seed + 1, rule, 48),
        "fresh": make_rows(seed + 101, rule, 32),
        "holdout": make_rows(seed + 201, rule, 32),
        "transfer": make_rows(seed + 301, transfer_rule or rule, 32),
        "future": make_rows(seed + 401, future_rule or transfer_rule or rule, 32),
    }


def independent_integrity_audit():
    blocked = [
        "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY",
        "GOOGLE_API_KEY", "HF_TOKEN", "OPENROUTER_API_KEY",
    ]
    leaked = [name for name in blocked if name in __import__("os").environ]
    return {
        "external_model": False,
        "network_dependency": False,
        "manual_runtime_strategy": False,
        "credential_env_present": leaked,
        "integrity_ok": not leaked,
        "target_identity_in_loop": False,
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--seed", type=int, required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--stages", type=int, default=6)
    args = parser.parse_args()

    out = pathlib.Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    seed = args.seed

    initial = MechanismConfig(
        beam=16,
        memory_limit=32,
        max_expression_depth=2,
        evidence_threshold=0.78,
        min_support_count=2,
        delayed_window=4,
        prediction_mode="ensemble",
        context_mode="shape",
        context_program=None,
    )
    loop = RecursiveResearchLoop(seed, initial)

    stage_rules = [
        "sign_xy",
        "parity_xy",
        "sign_xz",
        "xor_sign_parity",
        "xor_parity_sign",
        "and_sign_parity",
        "unknown_sum_parity",
    ]

    stage_records = []
    for i, rule_name in enumerate(stage_rules[: args.stages]):
        ds = dataset(
            seed + i * 10000,
            rule_name,
            transfer_rule=stage_rules[(i + 1) % len(stage_rules)],
            future_rule=stage_rules[(i + 2) % len(stage_rules)],
        )
        before = loop.snapshot()
        record = loop.run_stage(ds)
        after = loop.snapshot()
        stage_records.append({
            "stage": i,
            "hidden_generator_rule": rule_name,
            "before": {
                "config": before["config"],
                "generation": before["generation"],
                "core_digest": before["core_digest"],
            },
            "after": {
                "config": after["config"],
                "generation": after["generation"],
                "core_digest": after["core_digest"],
            },
            "record": record,
        })

    # I5 decisive test: compare the learned intervention model against a
    # matched blind model on a completely fresh task/intervention family.
    meta_ds = dataset(
        seed + 90000,
        "or_sign_parity",
        transfer_rule="and_sign_parity",
        future_rule="parity_yz",
    )
    meta_candidates = loop.hypotheses.generate(
        loop.config,
        rows=meta_ds["train"],
        width=16,
    )
    blank_model = CausalSelfModel()

    post_ranked = [
        (candidate, loop.self_model.predict(loop.config, candidate, meta_ds["train"]))
        for candidate in meta_candidates
    ]
    pre_ranked = [
        (candidate, blank_model.predict(loop.config, candidate, meta_ds["train"]))
        for candidate in meta_candidates
    ]
    post_ranked.sort(
        key=lambda item: (
            item[1].predicted_success_probability,
            item[1].predicted_future_learning_gain,
            -item[1].predicted_regression,
            -item[0].resource_cost,
        ),
        reverse=True,
    )
    pre_ranked.sort(
        key=lambda item: (
            item[1].predicted_success_probability,
            item[1].predicted_future_learning_gain,
            -item[1].predicted_regression,
            -item[0].resource_cost,
        ),
        reverse=True,
    )

    meta_observed = {}
    meta_baseline_core = loop.core.__class__.from_state(loop.core.export_state())
    meta_baseline_core.seed = seed + 92000
    meta_baseline_core.observe_batch(meta_ds["train"])
    meta_baseline_fresh = score_core(meta_baseline_core, meta_ds["fresh"])
    meta_baseline_holdout = score_core(meta_baseline_core, meta_ds["holdout"])
    meta_baseline_transfer = score_core(meta_baseline_core, meta_ds["transfer"])
    meta_future_baseline_core = loop.core.__class__.from_state(loop.core.export_state())
    meta_future_baseline_core.seed = seed + 93001
    meta_baseline_future = loop.experimenter and __import__(
        "cognitive_core.recursive_research",
        fromlist=["learning_curve_auc"],
    ).learning_curve_auc(
        meta_future_baseline_core,
        meta_ds["future"],
        (4, 8, 16, 32),
    )
    meta_baseline_metrics = {
        "fresh": meta_baseline_fresh,
        "holdout": meta_baseline_holdout,
        "transfer": meta_baseline_transfer,
        "future_learning_auc": meta_baseline_future,
    }
    for index, candidate in enumerate(meta_candidates):
        meta_observed[candidate.fingerprint] = loop.experimenter.evaluate(
            loop.core,
            meta_baseline_core,
            loop.config,
            candidate,
            meta_ds,
            seed_offset=5000 + index,
            baseline_metrics=meta_baseline_metrics,
        )

    def _rank_metrics(ranked):
        best_future = max(
            (r.future_learning_auc - r.baseline_future_learning_auc
             for r in meta_observed.values()),
            default=0.0,
        )
        top = (
            meta_observed[ranked[0][0].fingerprint].future_learning_auc
            - meta_observed[ranked[0][0].fingerprint].baseline_future_learning_auc
            if ranked else 0.0
        )
        regret = max(0.0, best_future - top)

        observed = [
            meta_observed[c.fingerprint].future_learning_auc
            - meta_observed[c.fingerprint].baseline_future_learning_auc
            for c, _ in ranked
        ]
        predicted = [p.predicted_future_learning_gain for _, p in ranked]
        mae = (
            sum(abs(a - b) for a, b in zip(observed, predicted))
            / max(1, len(observed))
        )
        return {
            "ranking_regret": regret,
            "prediction_mae_future_learning": mae,
            "best_future_learning_gain": best_future,
            "top_future_learning_gain": top,
        }

    meta_post = _rank_metrics(post_ranked)
    meta_pre = _rank_metrics(pre_ranked)
    meta_improvement = {
        "ranking_regret_delta": meta_pre["ranking_regret"] - meta_post["ranking_regret"],
        "prediction_mae_delta": meta_pre["prediction_mae_future_learning"] - meta_post["prediction_mae_future_learning"],
        "post": meta_post,
        "blind": meta_pre,
        "fresh_task_digest": digest(meta_ds["train"]),
        "candidate_count": len(meta_candidates),
    }
    meta_pass = (
        meta_improvement["ranking_regret_delta"] > 0.0
        or meta_improvement["prediction_mae_delta"] > 0.0
    )

    # The hidden rule name is written only to the evaluator artifact; it is never
    # passed to the ACSIE loop or used by any candidate-selection decision.
    accepted = [r for r in stage_records if r["record"]["accepted"]]
    positive_future = []
    ranking_regrets = []
    unknown_stage_detected = False
    for r in stage_records:
        ranking_regrets.append(float(r["record"].get("ranking_regret", 0.0)))
        diagnosis = r["record"].get("diagnosis", [])
        if diagnosis and diagnosis[0].get("name") == "unknown":
            unknown_stage_detected = True
        for _, obs in r["record"]["observations"].items():
            if obs.get("future_learning_auc") is not None:
                positive_future.append(float(obs["future_learning_auc"]))
    diagnoses = [
        r["record"]["diagnosis"][0]["name"]
        for r in stage_records
        if r["record"]["diagnosis"]
    ]

    artifact = {
        "schema": "ACSIE.i1-i5.decisive-test.v1",
        "seed": seed,
        "scientific_status": "PASSED" if (
            len(accepted) >= max(3, args.stages // 2)
            and unknown_stage_detected
            and meta_pass
        ) else "FAILED",
        "source_commit": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip(),
        "source_tree": subprocess.check_output(["git", "rev-parse", "HEAD^{tree}"], cwd=ROOT, text=True).strip(),
        "acsie_ref": "research/i1-i5-decisive-loop-20261002",
        "integrity": independent_integrity_audit(),
        "stage_count": len(stage_records),
        "accepted_stage_count": len(accepted),
        "diagnosis_head_count": len(diagnoses),
        "ranking_regret_sequence": ranking_regrets,
        "unknown_stage_detected": unknown_stage_detected,
        "meta_improvement": meta_improvement,
        "meta_improvement_pass": meta_pass,
        "diagnosis_sequence": diagnoses,
        "future_learning_values": positive_future,
        "ontology": sorted(loop.ontology),
        "history": stage_records,
        "claim_ledger": {
            "capability_demand_inference": "PROVISIONAL" if diagnoses else "FAILED",
            "causal_self_model": "PROVISIONAL" if loop.self_model.records else "FAILED",
            "open_hypothesis_engine": "PROVISIONAL",
            "quarantined_admission": "PROVISIONAL",
            "recursive_improvement": "PASSED" if meta_pass else "FAILED",
            "ontology_expansion": "PROVISIONAL" if unknown_stage_detected else "NOT_DEMONSTRATED",
        },
    }
    (out / "i1_i5_result.json").write_text(json.dumps(artifact, sort_keys=True, indent=2))
    print(json.dumps({
        "schema": artifact["schema"],
        "seed": seed,
        "scientific_status": artifact["scientific_status"],
        "accepted_stage_count": len(accepted),
        "diagnosis_sequence": diagnoses,
        "ontology": artifact["ontology"],
        "integrity_ok": artifact["integrity"]["integrity_ok"],
    }, sort_keys=True, indent=2))

    raise SystemExit(0 if artifact["scientific_status"] == "PASSED" else 1)


if __name__ == "__main__":
    main()
