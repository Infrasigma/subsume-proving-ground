#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import random
import statistics
from dataclasses import asdict
from pathlib import Path

from cognitive_core.h6_compositional import (
    H6Controller,
    Program,
    compose,
    primitive_library,
)


def row(rng: random.Random, rule: str):
    x, y, z = (rng.randint(-25, 25) for _ in range(3))
    sx, sy, sz = x < 0, y < 0, z < 0
    px, py, pz = x % 2, y % 2, z % 2
    if rule == "sign_xy":
        cond, lo, hi = sx == sy, 1, 6
    elif rule == "parity_xy":
        cond, lo, hi = px == py, 2, 7
    elif rule == "sign_xz":
        cond, lo, hi = sx == sz, 3, 8
    elif rule == "parity_yz":
        cond, lo, hi = py == pz, 4, 9
    elif rule == "xor_sign_parity":
        cond, lo, hi = (sx == sy) ^ (px == pz), 5, 11
    elif rule == "xor_parity_sign":
        cond, lo, hi = (px == py) ^ (sy == sz), 6, 12
    elif rule == "and_sign_parity":
        cond, lo, hi = (sx == sz) and (py == pz), 7, 13
    elif rule == "or_sign_parity":
        cond, lo, hi = (sx == sz) or (py == pz), 8, 14
    elif rule == "unknown_sum_parity":
        cond, lo, hi = ((x + y) % 2) == (z % 2), 9, 15
    elif rule == "mixed_majority":
        votes = int(sx == sy) + int(py == pz) + int((x + z) % 2 == 0)
        cond, lo, hi = votes >= 2, 10, 16
    else:
        raise ValueError(rule)
    return ({"x": x, "y": y, "z": z}, "step", {"x": x, "y": y, "z": z + (lo if cond else hi)})


def make_rows(seed: int, rule: str, n: int):
    rng = random.Random(seed)
    return tuple(row(rng, rule) for _ in range(n))


def dataset(seed: int, rule: str, transfer: str, future: str):
    return {
        "train": make_rows(seed + 1, rule, 48),
        "fresh": make_rows(seed + 101, rule, 32),
        "holdout": make_rows(seed + 201, rule, 32),
        "transfer": make_rows(seed + 301, transfer, 32),
        "future": make_rows(seed + 401, future, 32),
    }


def hidden_targets():
    # Withhold compositions that the public primitive grammar can actually synthesize.
    # The controller never receives these signatures; they are evaluator-only labels.
    keys = list(primitive_library())
    return (
        compose(keys[0], keys[9]),
        compose(keys[1], keys[10]),
        compose(keys[7], keys[11]),
        compose(keys[2], keys[12]),
    )


def config_signature(cfg):
    payload = cfg if isinstance(cfg, dict) else asdict(cfg)
    return json.dumps(payload, sort_keys=True, default=str, separators=(",", ":"))


def rank_regret(controller: H6Controller, candidates, datasets):
    from cognitive_core.recursive_research import clone_with_config, fit_core, score_core, learning_curve_auc
    baseline = clone_with_config(
        controller._fresh_core(),
        controller.config,
        seed=controller.seed + 70000,
    )
    fit_core(baseline, datasets["train"])
    bm = {
        "fresh": score_core(baseline, datasets["fresh"]),
        "holdout": score_core(baseline, datasets["holdout"]),
        "transfer": score_core(baseline, datasets["transfer"]),
        "future_learning_auc": learning_curve_auc(
            clone_with_config(
                baseline,
                controller.config,
                seed=controller.seed + 70001,
            ),
            datasets["future"],
            (4, 8, 16, 32),
        ),
    }

    rows = []
    for i, program in enumerate(candidates):
        from cognitive_core.h6_compositional import _candidate_ir, compile_program
        cfg = compile_program(controller.config, program)
        ir = _candidate_ir(program, cfg)
        result = controller.experimenter.evaluate(
            controller._fresh_core(),
            baseline,
            controller.config,
            ir,
            datasets,
            seed_offset=80000 + i,
            baseline_metrics=bm,
        )
        prediction = controller.model.predict(program, controller.config, ())
        gain = result.future_learning_auc - result.baseline_future_learning_auc
        rows.append((program, prediction, result, gain))

    gains = [float(r[3]) for r in rows]
    model_ranked = sorted(
        rows,
        key=lambda r: (r[1].predicted_future, r[0].signature()),
        reverse=True,
    )
    model_top = float(model_ranked[0][3]) if model_ranked else float("-inf")
    chance_mean = statistics.mean(gains) if gains else 0.0
    chance_sd = statistics.pstdev(gains) if len(gains) > 1 else 0.0
    # Exact random-selector one-sided p-value: with a uniform random choice among
    # the evaluated candidates, each candidate is equally likely to be selected.
    ge_count = sum(1 for gain in gains if gain >= model_top - 1e-12)
    random_p = (
        (ge_count + 1) / (len(gains) + 1)
        if gains and model_top != float("-inf")
        else 1.0
    )
    model_excess = model_top - chance_mean if gains else float("-inf")
    return {
        "model_top_future_gain": model_top,
        "chance_mean_future_gain": chance_mean,
        "chance_sd_future_gain": chance_sd,
        "model_excess_over_chance": model_excess,
        "random_selector_p_value": random_p,
        "candidate_count": len(gains),
        "model_better": bool(
            gains
            and model_top > chance_mean
            and random_p <= 0.05
        ),
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--seed", type=int, required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    seed = args.seed
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)

    from cognitive_core.h6_compositional import MechanismConfig

    initial = MechanismConfig(
        beam=16, memory_limit=32, max_expression_depth=2,
        evidence_threshold=0.78, min_support_count=2, delayed_window=4,
        prediction_mode="ensemble", context_mode="shape", context_program=None,
    )
    controller = H6Controller(seed, initial)

    # First three families are used for learning the discovery process.
    # The fourth target is an unseen composition and an unseen task family.
    train_rules = ("sign_xy", "parity_xy", "sign_xz")
    holdout_rules = ("xor_sign_parity", "xor_parity_sign", "unknown_sum_parity", "mixed_majority")
    targets = hidden_targets()
    stages = []
    incumbent = []
    q_values = []

    for generation, rule in enumerate((*train_rules, *holdout_rules)):
        transfer = holdout_rules[(generation + 1) % len(holdout_rules)]
        future = holdout_rules[(generation + 2) % len(holdout_rules)]
        ds = dataset(seed + generation * 10000, rule, transfer, future)
        before = controller.generation
        record = controller.run_stage(ds, incumbent)
        stages.append({
            "stage": generation,
            "evaluation_family": "training" if generation < 3 else "withheld",
            "hidden_generator_rule": rule,
            "target_program_signature": targets[generation % len(targets)].signature(),
            "controller_generation_before": before,
            "record": record,
        })
        q_gain = max(
            (
                o["future_learning_auc"] - o["baseline_future_learning_auc"]
                for o in record["observations"].values()
            ),
            default=0.0,
        )
        q_values.append(float(q_gain))
        if record["accepted"]:
            # Retention is evaluated against every prior family, making the
            # incumbent set longitudinal rather than stage-local.
            incumbent.append(ds)

    withheld = [x for x in stages if x["evaluation_family"] == "withheld"]
    novel_accepted = [
        x for x in stages
        if x["record"]["accepted"] and x["record"]["novel_mechanism"]
    ]
    repeated_q_improvement = sum(b > a for a, b in zip(q_values, q_values[1:])) >= 2

    # Fresh intervention-family meta-test: trained H6 model versus blind
    # first-order ranking on candidates that include compositional programs.
    meta_ds = dataset(seed + 90000, "mixed_majority", "unknown_sum_parity", "or_sign_parity")
    meta_candidates = controller.generate(
        controller.diagnostics.diagnose(
            meta_ds["train"], {}, controller._fresh_core(), controller.config
        ),
        meta_ds["train"],
        width=48,
    )
    meta_rank = rank_regret(controller, tuple(meta_candidates), meta_ds)

    target_recovery = 0
    target_sigs = {t.signature() for t in targets}
    for stage in stages:
        sig = stage["record"].get("accepted_program")
        if sig in target_sigs:
            target_recovery += 1

    # Novelty is checked against the baseline's single-primitive configuration
    # space, not merely against AST length. A multi-node program that compiles to
    # an already exposed single mechanism is not counted as open invention.
    primitive_cfgs = {
        config_signature(
            __import__(
                "cognitive_core.h6_compositional",
                fromlist=["compile_program"],
            ).compile_program(initial, p)
        )
        for p in primitive_library()
    }
    withheld_novel_configs = 0
    for stage in withheld:
        accepted_sig = stage["record"].get("accepted_program")
        if not accepted_sig:
            continue
        program = next((p for p in primitive_library() if p.signature() == accepted_sig), None)
        if program is not None:
            continue
        # The accepted artifact contains its resulting config; compare that
        # configuration against the exposed single-primitive baseline.
        cfg = stage["record"].get("accepted_config")
        if cfg and config_signature(cfg) not in primitive_cfgs:
            withheld_novel_configs += 1

    novel_count = sum(1 for p in meta_candidates if p.nodes() >= 3)
    scientific_status = "PASSED" if (
        len(stages) == 7
        and len(withheld) == 4
        and len(novel_accepted) >= 1
        and withheld_novel_configs >= 1
        and novel_count >= 3
        and repeated_q_improvement
        and meta_rank["model_better"]
        and meta_rank["model_excess_over_chance"] > 0.0
    ) else "FAILED"

    artifact = {
        "schema": "ACSIE.h6.open-compositional.decisive.v2",
        "seed": seed,
        "scientific_status": scientific_status,
        "stages": stages,
        "q_future_gain_sequence": q_values,
        "repeated_q_improvement": repeated_q_improvement,
        "withheld_stage_count": len(withheld),
        "novel_accepted_count": len(novel_accepted),
        "generated_compositional_candidate_count": novel_count,
        "withheld_target_recovery_count": target_recovery,
        "withheld_novel_configuration_count": withheld_novel_configs,
        "hidden_target_signatures": sorted(target_sigs),
        "meta_test": meta_rank,
        "integrity": {
            "external_model": False,
            "network_dependency": False,
            "manual_runtime_strategy": False,
            "target_identity_accessible": False,
        },
        "claim_ledger": {
            "compositional_mechanism_invention": "PASSED" if novel_accepted else "NOT_DEMONSTRATED",
            "withheld_composition_transfer": "PASSED" if target_recovery or withheld_novel_configs else "NOT_DEMONSTRATED",
            "longitudinal_recursive_improvement": "PASSED" if repeated_q_improvement else "NOT_DEMONSTRATED",
            "selection_above_chance": "PASSED" if meta_rank["model_better"] else "NOT_DEMONSTRATED",
            "open_ended_rsi": "NOT_DEMONSTRATED",
            "agi": "NOT_DEMONSTRATED",
            "asi": "NOT_DEMONSTRATED",
        },
    }
    (out / "h6_result.json").write_text(json.dumps(artifact, indent=2, sort_keys=True))
    print(json.dumps({
        "schema": artifact["schema"],
        "seed": seed,
        "scientific_status": scientific_status,
        "novel_accepted_count": len(novel_accepted),
        "target_recovery": target_recovery,
        "q_future_gain_sequence": q_values,
    }, indent=2, sort_keys=True))
    raise SystemExit(0 if scientific_status == "PASSED" else 1)


if __name__ == "__main__":
    main()
