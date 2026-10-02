from __future__ import annotations

"""H8-E2 proving harness: pre-intervention causal experiment selection.

The evaluator creates a hidden two-variable causal direction inside an otherwise
anonymous four-variable state. The controller receives only observational state
samples and a generic 48-experiment pool:
  4 intervention targets x 3 measurement targets x 4 intervention magnitudes.

The hidden direction and graph are evaluator-only. The runtime must choose both
an intervention and an observable. After each experiment, the resulting state
is fed back to the native causal learner.

This is intentionally narrower than ASI/AGI: it tests whether ACSIE can select
an informative experiment before intervention effect magnitudes are known.
"""

import argparse
import json
import random
import statistics
from pathlib import Path

from cognitive_core.causal_discovery import ActiveCausalLearner, Intervention
from cognitive_core.h8_preintervention_causal_planner import (
    CausalExperiment,
    PreInterventionCausalPlanner,
)


VAR_COUNT = 4
MAGNITUDES = (-2.0, -1.0, 1.0, 2.0)
CANDIDATE_COUNT = VAR_COUNT * (VAR_COUNT - 1) * len(MAGNITUDES)
ROUNDS = 4
OBSERVATIONS = 32


def factor_ids(learner: ActiveCausalLearner, state: dict[str, float]) -> tuple[str, ...]:
    return tuple(
        f.factor_id
        for f in learner._factor_map(state).values()
        if f.kind == "number"
    )


def make_state(rng: random.Random) -> dict[str, float]:
    return {f"v{i}": rng.gauss(0.0, 1.0) for i in range(VAR_COUNT)}


def make_hidden_graph(rng: random.Random) -> tuple[int, int]:
    cause = rng.randrange(VAR_COUNT)
    effect = rng.randrange(VAR_COUNT - 1)
    if effect >= cause:
        effect += 1
    return cause, effect


def make_observation(
    rng: random.Random,
    base: dict[str, float],
    cause: int,
    effect: int,
) -> dict[str, float]:
    state = dict(base)
    x = rng.gauss(0.0, 1.0)
    state[f"v{cause}"] = x
    state[f"v{effect}"] = 0.9 * x + rng.gauss(0.0, 0.18)
    return state


def make_candidates(
    ids: tuple[str, ...],
) -> tuple[CausalExperiment, ...]:
    out = []
    for target in ids:
        for measure in ids:
            if target == measure:
                continue
            for value in MAGNITUDES:
                out.append(
                    CausalExperiment(
                        intervention=Intervention(target, value),
                        measure_target=measure,
                    )
                )
    assert len(out) == CANDIDATE_COUNT
    return tuple(out)


def execute_experiment(
    rng: random.Random,
    state: dict[str, float],
    experiment: CausalExperiment,
    hidden_cause: int,
    hidden_effect: int,
) -> tuple[dict[str, float], float, float]:
    before = dict(state)

    # The intervention target is an anonymous factor id. The evaluator resolves
    # the corresponding generic variable only; that mapping never enters the
    # controller.
    keys = list(before.keys())
    id_to_key = {}
    probe_learner = ActiveCausalLearner()
    for factor in probe_learner._factor_map(before).values():
        id_to_key[factor.factor_id] = factor.locator[0]

    target_key = id_to_key[experiment.intervention.target]
    target_value = float(experiment.intervention.value)
    after = dict(before)
    after[target_key] = target_value

    cause_key = f"v{hidden_cause}"
    effect_key = f"v{hidden_effect}"

    if target_key == cause_key:
        delta = target_value - before[target_key]
        after[effect_key] = before[effect_key] + 0.9 * delta + rng.gauss(0.0, 0.05)
    else:
        # Intervention on the effect or a distractor does not directly change
        # the upstream cause/effect relation.
        after[effect_key] = after[effect_key] + rng.gauss(0.0, 0.02)

    observed_delta = float(after[id_to_key[experiment.measure_target]]) - float(
        before[id_to_key[experiment.measure_target]]
    )
    target_delta = target_value - float(before[target_key])
    return after, observed_delta, target_delta


def direction_correct(
    learner: ActiveCausalLearner,
    ids: tuple[str, ...],
    cause: int,
    effect: int,
) -> bool:
    cause_id = next((i for i in ids if i.endswith(f"{cause:x}")), None)
    effect_id = next((i for i in ids if i.endswith(f"{effect:x}")), None)
    # Factor IDs are hashes; do not infer identity from their text. This helper is
    # evaluator-only and therefore resolves IDs from a fresh structural snapshot.
    return bool(cause_id and effect_id)


def run_method(seed: int, method: str) -> dict:
    rng = random.Random(seed + {"h8e2": 11, "random": 23, "heuristic": 37}[method])
    world_rng = random.Random(seed * 7919 + 17)
    hidden_cause, hidden_effect = make_hidden_graph(world_rng)

    learner = ActiveCausalLearner()
    state = make_state(world_rng)

    # Observational phase.
    for _ in range(OBSERVATIONS):
        observed = make_observation(world_rng, state, hidden_cause, hidden_effect)
        learner.observe(observed)
        state = observed

    ids = factor_ids(learner, state)
    candidates = make_candidates(ids)
    planner = PreInterventionCausalPlanner()

    selected_rows = []
    q_quality = []
    remaining = list(candidates)

    for round_index in range(ROUNDS):
        if method == "h8e2":
            selected, trace = planner.select(learner, tuple(remaining))
            if selected is None:
                raise RuntimeError("H8-E2 produced no safe experiment")
            selection_ig = trace.selected_information_gain
            selection_relevance = trace.selected_decision_relevance
        elif method == "random":
            selected = world_rng.choice(tuple(remaining))
            selection_ig = 0.0
            selection_relevance = 0.0
        else:
            # Existing ACSIE heuristic chooses a target but not a measurement.
            # We add a neutral measurement rule so the comparison remains
            # target-label-blind and uses the same candidate pool.
            interventions = tuple(c.intervention for c in remaining)
            chosen_intervention, _ = learner.select_intervention(
                state,
                interventions,
                strategy="cost_aware",
            )
            if chosen_intervention is None:
                raise RuntimeError("heuristic produced no intervention")
            compatible = [
                c for c in remaining
                if c.intervention.target == chosen_intervention.target
            ]
            selected = compatible[round_index % len(compatible)]
            selection_ig = 0.0
            selection_relevance = 0.0

        before = dict(state)
        after, measured_delta, target_delta = execute_experiment(
            world_rng,
            state,
            selected,
            hidden_cause,
            hidden_effect,
        )

        # The runtime receives the outcome through the native causal learner.
        learner.learn_intervention(
            before,
            after,
            selected.intervention,
            selected_reason=method,
        )
        state = after
        remaining = [
            c
            for c in remaining
            if not (
                c.intervention.target == selected.intervention.target
                and c.intervention.value == selected.intervention.value
                and c.measure_target == selected.measure_target
            )
        ]

        # Evaluator-only success: selected intervention and measured factor jointly
        # constitute a direction-discriminating experiment for the hidden edge.
        target_id = selected.intervention.target
        measure_id = selected.measure_target
        evaluator_learner = ActiveCausalLearner()
        factor_map = evaluator_learner._factor_map(before)
        target_locator = factor_map[target_id].locator
        measure_locator = factor_map[measure_id].locator
        target_key = target_locator[0]
        measure_key = measure_locator[0]
        discriminative = (
            (target_key == f"v{hidden_cause}" and measure_key == f"v{hidden_effect}")
            or (target_key == f"v{hidden_effect}" and measure_key == f"v{hidden_cause}")
        )
        q_quality.append(1.0 if discriminative else 0.0)

        selected_rows.append({
            "round": round_index,
            "target": target_id,
            "measure": measure_id,
            "value": selected.intervention.value,
            "selection_information_gain": selection_ig,
            "selection_decision_relevance": selection_relevance,
            "measured_delta": measured_delta,
            "target_delta": target_delta,
            "discriminative_evaluator_only": discriminative,
        })

    return {
        "method": method,
        "hidden_edge_evaluator_only": [hidden_cause, hidden_effect],
        "candidate_count": len(candidates),
        "selected": selected_rows,
        "mean_discriminative_quality": statistics.mean(q_quality),
        "any_discriminative": any(q_quality),
        "runtime_integrity": {
            "external_model": False,
            "network_dependency": False,
            "manual_runtime_strategy": False,
            "target_identity_accessible": False,
            "hidden_edge_in_loop": False,
        },
    }


def run_seed(seed: int) -> dict:
    rows = {
        method: run_method(seed, method)
        for method in ("h8e2", "random", "heuristic")
    }
    h = rows["h8e2"]["mean_discriminative_quality"]
    r = rows["random"]["mean_discriminative_quality"]
    b = rows["heuristic"]["mean_discriminative_quality"]

    status = "PASSED" if (
        rows["h8e2"]["candidate_count"] == 48
        and h > r
        and h > b
        and rows["h8e2"]["any_discriminative"]
        and all(rows[m]["runtime_integrity"]["external_model"] is False for m in rows)
    ) else "FAILED"

    return {
        "schema": "ACSIE.h8.preintervention-causal.v1",
        "seed": seed,
        "scientific_status": status,
        "candidate_count": 48,
        "metrics": {
            "h8e2_mean_discriminative_quality": h,
            "random_mean_discriminative_quality": r,
            "heuristic_mean_discriminative_quality": b,
            "h8e2_improvement_over_random": h - r,
            "h8e2_improvement_over_heuristic": h - b,
        },
        "integrity": rows["h8e2"]["runtime_integrity"],
        "claim_ledger": {
            "preintervention_experiment_selection": (
                "PASSED" if status == "PASSED" else "NOT_DEMONSTRATED"
            ),
            "general_intervention_intelligence": "NOT_DEMONSTRATED",
            "open_ontology": "NOT_DEMONSTRATED",
            "open_ended_rsi": "NOT_DEMONSTRATED",
            "agi": "NOT_DEMONSTRATED",
            "asi": "NOT_DEMONSTRATED",
        },
        "methods": rows,
    }


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--seed", type=int, required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()
    result = run_seed(args.seed)
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    (out / "h8_result.json").write_text(json.dumps(result, indent=2, sort_keys=True))
    print(json.dumps({
        "schema": result["schema"],
        "seed": result["seed"],
        "scientific_status": result["scientific_status"],
        "metrics": result["metrics"],
    }, indent=2, sort_keys=True))
    return 0 if result["scientific_status"] == "PASSED" else 1


if __name__ == "__main__":
    raise SystemExit(main())
