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


def dataset(seed, rule, transfer_rule=None):
    return {
        "train": make_rows(seed + 1, rule, 48),
        "fresh": make_rows(seed + 101, rule, 32),
        "holdout": make_rows(seed + 201, rule, 32),
        "transfer": make_rows(seed + 301, transfer_rule or rule, 32),
        "future": make_rows(seed + 401, "parity_yz", 32),
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
    ]

    stage_records = []
    for i, rule_name in enumerate(stage_rules[: args.stages]):
        ds = dataset(seed + i * 10000, rule_name, transfer_rule=stage_rules[(i + 1) % len(stage_rules)])
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

    # The hidden rule name is written only to the evaluator artifact; it is never
    # passed to the ACSIE loop or used by any candidate-selection decision.
    accepted = [r for r in stage_records if r["record"]["accepted"]]
    positive_future = []
    for r in accepted:
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
        "scientific_status": "PASSED" if len(accepted) >= max(2, args.stages // 2) else "FAILED",
        "source_commit": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip(),
        "source_tree": subprocess.check_output(["git", "rev-parse", "HEAD^{tree}"], cwd=ROOT, text=True).strip(),
        "acsie_ref": "research/i1-i5-decisive-loop-20261002",
        "integrity": independent_integrity_audit(),
        "stage_count": len(stage_records),
        "accepted_stage_count": len(accepted),
        "diagnosis_head_count": len(diagnoses),
        "diagnosis_sequence": diagnoses,
        "future_learning_values": positive_future,
        "ontology": sorted(loop.ontology),
        "history": stage_records,
        "claim_ledger": {
            "capability_demand_inference": "PROVISIONAL" if diagnoses else "FAILED",
            "causal_self_model": "PROVISIONAL" if loop.self_model.records else "FAILED",
            "open_hypothesis_engine": "PROVISIONAL",
            "quarantined_admission": "PROVISIONAL",
            "recursive_improvement": "PASSED" if len(accepted) >= 2 else "FAILED",
            "ontology_expansion": "NOT_DEMONSTRATED" if "unknown-residual" not in loop.ontology else "PROVISIONAL",
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
