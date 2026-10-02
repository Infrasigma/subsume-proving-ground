#!/usr/bin/env python3
from __future__ import annotations

"""H8 selector experiment.

This isolates the intervention-selection question from mechanism construction.
The candidate generator is kept fixed; H8 changes only which candidate is
experimented on. The controller receives no task-family/target label.
"""

import argparse
import json
import random
import statistics
from pathlib import Path

from cognitive_core.h6_compositional import (
    H6Controller,
    MechanismConfig,
    _candidate_ir,
    compile_program,
)
from cognitive_core.h6_r2_active_acquisition import H6R2Controller
from cognitive_core.recursive_research import (
    clone_with_config,
    fit_core,
    learning_curve_auc,
    score_core,
)
from cognitive_core.h8_active_causal_experiment import ActiveCausalExperimentPlanner
from acsie_h6_open_compositional_test import dataset


def _run_one(
    controller,
    program,
    datasets,
    incumbent,
    *,
    seed_offset: int,
):
    baseline = clone_with_config(
        controller._fresh_core(),
        controller.config,
        seed=controller.seed + 41000 + seed_offset,
    )
    fit_core(baseline, datasets["train"])
    baseline_metrics = {
        "fresh": score_core(baseline, datasets["fresh"]),
        "holdout": score_core(baseline, datasets["holdout"]),
        "transfer": score_core(baseline, datasets["transfer"]),
        "future_learning_auc": learning_curve_auc(
            clone_with_config(baseline, controller.config, seed=controller.seed + 42000 + seed_offset),
            datasets["future"],
            (4, 8, 16, 32),
        ),
    }
    cfg = compile_program(controller.config, program)
    ir = _candidate_ir(program, cfg)
    result = controller.experimenter.evaluate(
        controller._fresh_core(),
        baseline,
        controller.config,
        ir,
        datasets,
        seed_offset=1000 + seed_offset,
        baseline_metrics=baseline_metrics,
    )
    retention = 0.0
    retention_ok = True
    for prior in incumbent:
        prior_result = controller.experimenter.evaluate(
            controller._fresh_core(),
            baseline,
            controller.config,
            ir,
            prior,
            seed_offset=3000 + seed_offset,
        )
        retention = max(retention, prior_result.regression)
        if prior_result.regression > controller.experimenter.epsilon:
            retention_ok = False
    integrity_ok = bool(result.integrity_ok and retention_ok)
    future_delta = result.future_learning_auc - result.baseline_future_learning_auc
    accepted = bool(
        integrity_ok
        and result.route_invariance
        and result.fresh >= result.baseline_fresh
        and result.holdout >= result.baseline_holdout
        and result.transfer >= result.baseline_transfer
        and future_delta > 0.0
        and retention <= controller.experimenter.epsilon
    )
    return result, future_delta, retention, accepted


def _accept(controller, program):
    controller.accepted_programs.append(program)
    controller.config = compile_program(controller.config, program)
    controller.generation += 1


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--seed", type=int, required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()
    out = Path(args.out)
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

    h8 = H6Controller(seed, initial)
    h6r2 = H6R2Controller(seed + 900000, initial)
    planner = ActiveCausalExperimentPlanner(epsilon=0.02)

    train_rules = ("sign_xy", "parity_xy", "sign_xz")
    withheld_rules = ("xor_sign_parity", "xor_parity_sign", "unknown_sum_parity", "mixed_majority")

    incumbent_h8 = []
    h8_trace = []
    q_values = []

    for stage, rule in enumerate((*train_rules, *withheld_rules)):
        transfer = withheld_rules[(stage + 1) % len(withheld_rules)]
        future = withheld_rules[(stage + 2) % len(withheld_rules)]
        ds = dataset(seed + stage * 10000, rule, transfer, future)

        hypotheses = h8.diagnostics.diagnose(
            ds["train"], {}, h8._fresh_core(), h8.config
        )
        candidates = h8.generate(hypotheses, ds["train"], width=48)
        novelty = {p.signature(): 1.0 for p in candidates}

        chosen, trace = planner.select_programs(
            candidates,
            h8.config,
            novelty_by_signature=novelty,
        )
        if chosen is None:
            raise SystemExit(f"no safe H8 candidate at stage {stage}")

        selected = next(p for p in candidates if p.signature() == chosen.signature)
        result, future_delta, retention, accepted = _run_one(
            h8,
            selected,
            ds,
            incumbent_h8,
            seed_offset=stage,
        )
        planner.observe(
            signature=selected.signature(),
            feature=chosen.feature,
            future_delta=future_delta,
            regression=retention,
            novelty=1.0,
            cost=float(selected.nodes()),
            accepted=accepted,
        )

        if accepted:
            _accept(h8, selected)
            incumbent_h8.append(ds)

        q_values.append(float(future_delta))
        h8_trace.append({
            "stage": stage,
            "evaluation_family": "training" if stage < 3 else "withheld",
            "selected": selected.signature(),
            "selection": {
                "information_gain": chosen.expected_information_gain,
                "predicted_future_gain": chosen.predicted_future_gain,
                "predicted_regression": chosen.predicted_regression,
                "posterior_entropy": chosen.posterior_entropy,
                "safe": chosen.safe,
            },
            "observed": {
                "future_delta": future_delta,
                "retention_regression": retention,
                "accepted": accepted,
            },
        })

        # Feed the same observed sequence to H6-R2's selector for a matched
        # evidence-budget comparison at the fresh meta-test.
        h6r2.model.update(
            selected,
            h6r2.config,
            {"future_delta": future_delta, "regression": retention, "accepted": float(accepted)},
        )

    # Fresh candidate family, with evaluator-only task identity.
    meta = dataset(seed + 90000, "mixed_majority", "unknown_sum_parity", "or_sign_parity")
    hypotheses = h8.diagnostics.diagnose(
        meta["train"], {}, h8._fresh_core(), h8.config
    )
    meta_candidates = h8.generate(hypotheses, meta["train"], width=48)

    h8_selected, h8_meta_trace = planner.select_programs(
        meta_candidates, h8.config, novelty_by_signature={p.signature(): 1.0 for p in meta_candidates}
    )
    if h8_selected is None:
        raise SystemExit("H8 had no safe meta-test candidate")

    r2_ranked = sorted(
        (
            h6r2.model.predict(p, h6r2.config, ())
            .predicted_future,
            p,
        )
        for p in meta_candidates
    )
    r2_top = r2_ranked[-1][1]

    rng = random.Random(seed + 123456)
    random_top = rng.choice(tuple(meta_candidates))

    h8_result, h8_gain, h8_reg, _ = _run_one(h8, next(p for p in meta_candidates if p.signature() == h8_selected.signature), meta, incumbent_h8, seed_offset=90000)
    r2_result, r2_gain, r2_reg, _ = _run_one(h8, r2_top, meta, incumbent_h8, seed_offset=90001)
    rnd_result, rnd_gain, rnd_reg, _ = _run_one(h8, random_top, meta, incumbent_h8, seed_offset=90002)

    # Full post-hoc matched evaluation is diagnostic only; selection decisions above
    # were made before these outcomes were observed.
    posthoc = []
    for idx, program in enumerate(meta_candidates):
        result, gain, reg, _ = _run_one(h8, program, meta, incumbent_h8, seed_offset=91000 + idx)
        posthoc.append({"signature": program.signature(), "future_delta": gain, "regression": reg})

    gains = [x["future_delta"] for x in posthoc]
    random_mean = statistics.mean(gains) if gains else 0.0
    h8_rank = next(x for x in posthoc if x["signature"] == h8_selected.signature)
    r2_rank = next(x for x in posthoc if x["signature"] == r2_top.signature)
    rnd_rank = next(x for x in posthoc if x["signature"] == random_top.signature)

    artifact = {
        "schema": "ACSIE.h8.active-causal-selector.v1",
        "seed": seed,
        "scientific_status": "PASSED" if (
            h8_gain > 0.0
            and h8_rank["future_delta"] > random_mean
            and h8_meta_trace.selected_signature == h8_selected.signature
        ) else "FAILED",
        "integrity": {
            "external_model": False,
            "network_dependency": False,
            "manual_runtime_strategy": False,
            "target_identity_accessible": False,
        },
        "stages": h8_trace,
        "q_future_gain_sequence": q_values,
        "repeated_q_improvement": sum(
            b > a for a, b in zip(q_values, q_values[1:])
        ) >= 2,
        "meta_test": {
            "candidate_count": len(meta_candidates),
            "h8": h8_rank,
            "h6r2_matched": r2_rank,
            "random_matched": rnd_rank,
            "chance_mean": random_mean,
            "planner_trace": {
                "selected_signature": h8_meta_trace.selected_signature,
                "expected_information_gain": h8_meta_trace.selected_information_gain,
                "predicted_future_gain": h8_meta_trace.selected_future_gain,
            },
        },
        "claim_ledger": {
            "active_causal_selection": "PASSED" if h8_gain > 0.0 and h8_rank["future_delta"] > random_mean else "NOT_DEMONSTRATED",
            "selection_above_random": "PASSED" if h8_rank["future_delta"] > random_mean else "NOT_DEMONSTRATED",
            "general_intervention_intelligence": "NOT_DEMONSTRATED",
            "open_ontology": "NOT_DEMONSTRATED",
            "open_ended_rsi": "NOT_DEMONSTRATED",
            "agi": "NOT_DEMONSTRATED",
            "asi": "NOT_DEMONSTRATED",
        },
    }
    (out / "h8_result.json").write_text(json.dumps(artifact, indent=2, sort_keys=True))
    print(json.dumps({
        "schema": artifact["schema"],
        "seed": seed,
        "scientific_status": artifact["scientific_status"],
        "h8_future_delta": h8_gain,
        "h8_vs_chance": h8_rank["future_delta"] - random_mean,
        "h6r2_future_delta": r2_gain,
        "random_future_delta": rnd_gain,
    }, indent=2, sort_keys=True))
    raise SystemExit(0 if artifact["scientific_status"] == "PASSED" else 1)


if __name__ == "__main__":
    main()
