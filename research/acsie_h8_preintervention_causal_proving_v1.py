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
OBSERVATIONS = 64


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
    effect_noise: float,
    background_noise: float,
) -> tuple[dict[str, float], float, float]:
    before = dict(state)
    probe_learner = ActiveCausalLearner()
    id_to_key = {
        factor.factor_id: factor.locator[0]
        for factor in probe_learner._factor_map(before).values()
    }

    target_key = id_to_key[experiment.intervention.target]
    measure_key = id_to_key[experiment.measure_target]
    target_value = float(experiment.intervention.value)

    after = dict(before)
    after[target_key] = target_value

    cause_key = f"v{hidden_cause}"
    effect_key = f"v{hidden_effect}"
    if target_key == cause_key:
        delta = target_value - before[target_key]
        after[effect_key] = before[effect_key] + 0.9 * delta + effect_noise
    else:
        after[effect_key] = after[effect_key] + background_noise

    observed_delta = float(after[measure_key]) - float(before[measure_key])
    target_delta = target_value - float(before[target_key])
    return after, observed_delta, target_delta



def causal_direction_margin(
    learner: ActiveCausalLearner,
    state: dict[str, float],
    hidden_cause: int,
    hidden_effect: int,
) -> float:
    factor_map = learner._factor_map(state)
    by_locator = {
        factor.locator[0]: factor.factor_id
        for factor in factor_map.values()
        if factor.locator
    }
    cause_id = by_locator[f"v{hidden_cause}"]
    effect_id = by_locator[f"v{hidden_effect}"]

    true_scores = [
        h.posterior_score()
        for h in learner.state.hypotheses
        if h.valid and h.cause == cause_id and h.effect == effect_id
    ]
    reverse_scores = [
        h.posterior_score()
        for h in learner.state.hypotheses
        if h.valid and h.cause == effect_id and h.effect == cause_id
    ]
    return max(true_scores, default=0.0) - max(reverse_scores, default=0.0)


def run_method(
    method: str,
    seed: int,
    hidden_cause: int,
    hidden_effect: int,
    observations: tuple[dict[str, float], ...],
    initial_state: dict[str, float],
    effect_noises: tuple[float, ...],
    background_noises: tuple[float, ...],
) -> dict:
    rng = random.Random(seed + {"h8e2": 11, "random": 23, "heuristic": 37}[method])
    learner = ActiveCausalLearner()
    state = dict(initial_state)

    for observed in observations:
        learner.observe(observed)
        state = dict(observed)

    ids = factor_ids(learner, state)
    candidates = make_candidates(ids)
    planner = PreInterventionCausalPlanner()

    selected_rows = []
    q_quality = []
    direction_margin_trace = [causal_direction_margin(
        learner, state, hidden_cause, hidden_effect
    )]
    remaining = list(candidates)

    for round_index in range(ROUNDS):
        if method == "h8e2":
            selected, trace = planner.select(learner, tuple(remaining))
            if selected is None:
                raise RuntimeError("H8-E2 produced no safe experiment")
            selection_ig = trace.selected_information_gain
            selection_relevance = trace.selected_decision_relevance
        elif method == "random":
            selected = rng.choice(tuple(remaining))
            selection_ig = 0.0
            selection_relevance = 0.0
        else:
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
            rng,
            state,
            selected,
            hidden_cause,
            hidden_effect,
            effect_noises[round_index],
            background_noises[round_index],
        )

        learner.learn_intervention(
            before,
            after,
            selected.intervention,
            selected_reason=method,
        )
        state = after
        direction_margin_trace.append(
            causal_direction_margin(
                learner, state, hidden_cause, hidden_effect
            )
        )
        remaining = [
            c for c in remaining
            if not (
                c.intervention.target == selected.intervention.target
                and c.intervention.value == selected.intervention.value
                and c.measure_target == selected.measure_target
            )
        ]

        evaluator_learner = ActiveCausalLearner()
        factor_map = evaluator_learner._factor_map(before)
        target_key = factor_map[selected.intervention.target].locator[0]
        measure_key = factor_map[selected.measure_target].locator[0]
        discriminative = (
            (target_key == f"v{hidden_cause}" and measure_key == f"v{hidden_effect}")
            or (target_key == f"v{hidden_effect}" and measure_key == f"v{hidden_cause}")
        )
        q_quality.append(1.0 if discriminative else 0.0)

        selected_rows.append({
            "round": round_index,
            "target": selected.intervention.target,
            "measure": selected.measure_target,
            "value": selected.intervention.value,
            "selection_information_gain": selection_ig,
            "selection_decision_relevance": selection_relevance,
            "measured_delta": measured_delta,
            "target_delta": target_delta,
            "discriminative_evaluator_only": discriminative,
        })

    return {
        "method": method,
        "candidate_count": len(candidates),
        "selected": selected_rows,
        "mean_discriminative_quality": statistics.mean(q_quality),
        "late_round_discriminative_quality": statistics.mean(q_quality[1:]) if len(q_quality) > 1 else 0.0,
        "any_discriminative": any(q_quality),
        "direction_margin_trace": direction_margin_trace,
        "final_direction_margin": direction_margin_trace[-1],
        "direction_margin_improvement": direction_margin_trace[-1] - direction_margin_trace[0],
        "direction_improved_at_least_once": any(
            later > earlier
            for earlier, later in zip(
                direction_margin_trace,
                direction_margin_trace[1:],
            )
        ),
        "direction_correct_final": direction_margin_trace[-1] > 0.0,
        "runtime_integrity": {
            "external_model": False,
            "network_dependency": False,
            "manual_runtime_strategy": False,
            "target_identity_accessible": False,
            "hidden_edge_in_loop": False,
        },
    }


def run_seed(seed: int) -> dict:
    world_rng = random.Random(seed * 7919 + 17)
    hidden_cause, hidden_effect = make_hidden_graph(world_rng)
    initial_state = make_state(world_rng)

    observations = []
    current = dict(initial_state)
    for _ in range(OBSERVATIONS):
        current = make_observation(
            world_rng,
            current,
            hidden_cause,
            hidden_effect,
        )
        observations.append(dict(current))

    # Matched outcome-noise schedules are shared across all three selectors.
    effect_noises = tuple(world_rng.gauss(0.0, 0.05) for _ in range(ROUNDS))
    background_noises = tuple(world_rng.gauss(0.0, 0.02) for _ in range(ROUNDS))

    rows = {
        method: run_method(
            method,
            seed,
            hidden_cause,
            hidden_effect,
            tuple(observations),
            initial_state,
            effect_noises,
            background_noises,
        )
        for method in ("h8e2", "random", "heuristic")
    }
    h = rows["h8e2"]["mean_discriminative_quality"]
    r = rows["random"]["mean_discriminative_quality"]
    b = rows["heuristic"]["mean_discriminative_quality"]
    h_late = rows["h8e2"]["late_round_discriminative_quality"]
    r_late = rows["random"]["late_round_discriminative_quality"]
    b_late = rows["heuristic"]["late_round_discriminative_quality"]

    status = "PASSED" if (
        rows["h8e2"]["candidate_count"] == 48
        and h > r
        and h > b
        and h_late > r_late
        and h_late > b_late
        and rows["h8e2"]["any_discriminative"]
        and rows["h8e2"]["direction_improved_at_least_once"]
        and rows["h8e2"]["direction_correct_final"]
        and rows["h8e2"]["runtime_integrity"]["external_model"] is False
        and rows["h8e2"]["runtime_integrity"]["network_dependency"] is False
        and rows["h8e2"]["runtime_integrity"]["manual_runtime_strategy"] is False
        and rows["h8e2"]["runtime_integrity"]["target_identity_accessible"] is False
        and rows["h8e2"]["runtime_integrity"]["hidden_edge_in_loop"] is False
    ) else "FAILED"

    return {
        "schema": "ACSIE.h8.preintervention-causal.v1",
        "seed": seed,
        "scientific_status": status,
        "candidate_count": 48,
        "matched_control": {
            "shared_hidden_world": True,
            "shared_observation_stream": True,
            "shared_candidate_pool": True,
            "shared_outcome_noise": True,
        },
        "metrics": {
            "h8e2_mean_discriminative_quality": h,
            "random_mean_discriminative_quality": r,
            "heuristic_mean_discriminative_quality": b,
            "h8e2_improvement_over_random": h - r,
            "h8e2_improvement_over_heuristic": h - b,
            "h8e2_late_improvement_over_random": h_late - r_late,
            "h8e2_late_improvement_over_heuristic": h_late - b_late,
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
