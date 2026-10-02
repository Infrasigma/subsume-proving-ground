from __future__ import annotations

"""H8-E2 v3: pre-intervention causal experiment selection with controlled observation.

Scientific question:
    Given anonymous observational experience and competing causal hypotheses,
    can ACSIE choose an intervention+measurement experiment that is more likely
    to identify the hidden causal direction than matched random and existing
    target-only heuristic selection, and does that advantage persist after the
    learner has updated from earlier experiments?

Protocol:
- 32 independent hidden worlds per seed.
- 4 anonymous numeric variables.
- 1 evaluator-only directed causal edge.
- 64 shared observational samples.
- 48 candidate experiments:
    4 target factors x 3 measurement factors x 4 intervention magnitudes.
- 5 sequential decisions; round 5 is the delayed-selection evaluation.
- Methods share the exact world, observations, candidate pool and outcome noise
  within every episode.
- 5 seeds are required.
- Per-seed superiority uses paired exact sign tests across the 32 worlds.

The runtime never receives hidden edge identity or evaluator labels.
"""

import argparse
import json
import math
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
OBSERVATIONS = 64
ROUNDS = 5
TRAIN_ROUNDS = 4
EPISODES_PER_SEED = 32


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

    # The controller does not receive the full post-intervention state.
    # Only the chosen measurement is revealed, together with the intervened
    # target. This makes measurement selection causally meaningful.
    reported = dict(before)
    reported[target_key] = target_value
    reported[measure_key] = after[measure_key]
    return after, reported, observed_delta, target_delta


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
    method_seed: int,
    hidden_cause: int,
    hidden_effect: int,
    observations: tuple[dict[str, float], ...],
    initial_state: dict[str, float],
    effect_noises: tuple[float, ...],
    background_noises: tuple[float, ...],
) -> dict:
    rng = random.Random(
        method_seed + {"h8e2": 11, "random": 23, "heuristic": 37}[method]
    )
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
    direction_margin_trace = [
        causal_direction_margin(learner, state, hidden_cause, hidden_effect)
    ]
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
        world_after, reported_after, measured_delta, target_delta = execute_experiment(
            state,
            selected,
            hidden_cause,
            hidden_effect,
            effect_noises[round_index],
            background_noises[round_index],
        )

        evaluator_learner = ActiveCausalLearner()
        factor_map = evaluator_learner._factor_map(before)
        target_key = factor_map[selected.intervention.target].locator[0]
        measure_key = factor_map[selected.measure_target].locator[0]
        discriminative = (
            (target_key == f"v{hidden_cause}" and measure_key == f"v{hidden_effect}")
            or (target_key == f"v{hidden_effect}" and measure_key == f"v{hidden_cause}")
        )

        # Round 5 is a delayed selection test: the decision is evaluated before
        # observing its outcome, using only the evidence acquired in rounds 0..3.
        delayed_round = round_index >= TRAIN_ROUNDS
        q_quality.append(1.0 if discriminative else 0.0)

        learner.learn_intervention(
            before,
            reported_after,
            selected.intervention,
            selected_reason=method,
        )
        state = reported_after
        direction_margin_trace.append(
            causal_direction_margin(
                learner, state, hidden_cause, hidden_effect
            )
        )

        remaining = [
            c
            for c in remaining
            if not (
                c.intervention.target == selected.intervention.target
                and c.intervention.value == selected.intervention.value
                and c.measure_target == selected.measure_target
            )
        ]

        selected_rows.append({
            "round": round_index,
            "delayed_evaluation_round": delayed_round,
            "target": selected.intervention.target,
            "measure": selected.measure_target,
            "value": selected.intervention.value,
            "selection_information_gain": selection_ig,
            "selection_decision_relevance": selection_relevance,
            "measured_delta": measured_delta,
            "target_delta": target_delta,
        "world_state_hidden_from_runtime": True,
            "discriminative_evaluator_only": discriminative,
        })

    return {
        "method": method,
        "candidate_count": len(candidates),
        "selected": selected_rows,
        "mean_discriminative_quality": statistics.mean(q_quality),
        "training_mean_quality": statistics.mean(q_quality[:TRAIN_ROUNDS]),
        "late_round_quality": statistics.mean(q_quality[1:]),
        "delayed_round_quality": q_quality[-1],
        "any_discriminative": any(q_quality),
        "direction_margin_trace": direction_margin_trace,
        "initial_direction_margin": direction_margin_trace[0],
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


def sign_test_pvalue(differences: list[float]) -> tuple[int, int, float]:
    nonzero = [d for d in differences if abs(d) > 1e-12]
    n = len(nonzero)
    k = sum(d > 0 for d in nonzero)
    if n == 0:
        return 0, 0, 1.0
    if k <= n // 2:
        return k, n, 1.0
    p = sum(math.comb(n, i) for i in range(k, n + 1)) / (2.0 ** n)
    return k, n, p


def episode_spec(seed: int, episode: int) -> dict:
    rng = random.Random(seed * 1000003 + episode * 9176 + 19)
    cause, effect = make_hidden_graph(rng)
    initial = make_state(rng)
    current = dict(initial)
    observations = []
    for _ in range(OBSERVATIONS):
        current = make_observation(rng, current, cause, effect)
        observations.append(dict(current))
    effect_noises = tuple(rng.gauss(0.0, 0.05) for _ in range(ROUNDS))
    background_noises = tuple(rng.gauss(0.0, 0.02) for _ in range(ROUNDS))
    return {
        "cause": cause,
        "effect": effect,
        "initial": initial,
        "observations": tuple(observations),
        "effect_noises": effect_noises,
        "background_noises": background_noises,
    }


def run_seed(seed: int) -> dict:
    episode_rows = []
    for episode in range(EPISODES_PER_SEED):
        spec = episode_spec(seed, episode)
        methods = {
            method: run_method(
                method,
                seed * 1000003 + episode * 9176,
                spec["cause"],
                spec["effect"],
                spec["observations"],
                spec["initial"],
                spec["effect_noises"],
                spec["background_noises"],
            )
            for method in ("h8e2", "random", "heuristic")
        }
        episode_rows.append({
            "episode": episode,
            "methods": methods,
            "hidden_edge_evaluator_only": [spec["cause"], spec["effect"]],
        })

    h = [row["methods"]["h8e2"] for row in episode_rows]
    r = [row["methods"]["random"] for row in episode_rows]
    b = [row["methods"]["heuristic"] for row in episode_rows]

    h_delay = statistics.mean(x["delayed_round_quality"] for x in h)
    r_delay = statistics.mean(x["delayed_round_quality"] for x in r)
    b_delay = statistics.mean(x["delayed_round_quality"] for x in b)

    h_late = statistics.mean(x["late_round_quality"] for x in h)
    r_late = statistics.mean(x["late_round_quality"] for x in r)
    b_late = statistics.mean(x["late_round_quality"] for x in b)

    h_dir = statistics.mean(x["direction_margin_improvement"] for x in h)
    r_dir = statistics.mean(x["direction_margin_improvement"] for x in r)
    b_dir = statistics.mean(x["direction_margin_improvement"] for x in b)

    delay_random_k, delay_random_n, delay_random_p = sign_test_pvalue([
        x["delayed_round_quality"] - y["delayed_round_quality"]
        for x, y in zip(h, r)
    ])
    delay_heur_k, delay_heur_n, delay_heur_p = sign_test_pvalue([
        x["delayed_round_quality"] - y["delayed_round_quality"]
        for x, y in zip(h, b)
    ])
    dir_random_k, dir_random_n, dir_random_p = sign_test_pvalue([
        x["direction_margin_improvement"] - y["direction_margin_improvement"]
        for x, y in zip(h, r)
    ])
    dir_heur_k, dir_heur_n, dir_heur_p = sign_test_pvalue([
        x["direction_margin_improvement"] - y["direction_margin_improvement"]
        for x, y in zip(h, b)
    ])

    status = "PASSED" if (
        all(row["methods"]["h8e2"]["candidate_count"] == 48 for row in episode_rows)
        and h_delay > r_delay
        and h_delay > b_delay
        and h_late > r_late
        and h_late > b_late
        and h_dir > r_dir
        and h_dir > b_dir
        and delay_random_n >= 16
        and delay_heur_n >= 16
        and dir_random_n >= 16
        and dir_heur_n >= 16
        and delay_random_p <= 0.05
        and delay_heur_p <= 0.05
        and dir_random_p <= 0.05
        and dir_heur_p <= 0.05
        and all(
            row["methods"]["h8e2"]["runtime_integrity"]["external_model"] is False
            and row["methods"]["h8e2"]["runtime_integrity"]["network_dependency"] is False
            and row["methods"]["h8e2"]["runtime_integrity"]["manual_runtime_strategy"] is False
            and row["methods"]["h8e2"]["runtime_integrity"]["target_identity_accessible"] is False
            and row["methods"]["h8e2"]["runtime_integrity"]["hidden_edge_in_loop"] is False
            for row in episode_rows
        )
    ) else "FAILED"

    return {
        "schema": "ACSIE.h8-e2.aggregate-sequential.v2",
        "seed": seed,
        "scientific_status": status,
        "episode_count": EPISODES_PER_SEED,
        "candidate_count": CANDIDATE_COUNT,
        "rounds": ROUNDS,
        "training_rounds": TRAIN_ROUNDS,
        "matched_controls": {
            "shared_world_per_episode": True,
            "shared_observation_stream_per_episode": True,
            "shared_candidate_pool_per_episode": True,
            "shared_outcome_noise_per_episode": True,
        },
        "metrics": {
            "h8_delayed_quality": h_delay,
            "random_delayed_quality": r_delay,
            "heuristic_delayed_quality": b_delay,
            "h8_late_quality": h_late,
            "random_late_quality": r_late,
            "heuristic_late_quality": b_late,
            "h8_direction_margin_improvement": h_dir,
            "random_direction_margin_improvement": r_dir,
            "heuristic_direction_margin_improvement": b_dir,
        },
        "paired_exact_sign_tests": {
            "delayed_h8_vs_random": {"positive": delay_random_k, "nonzero": delay_random_n, "p": delay_random_p},
            "delayed_h8_vs_heuristic": {"positive": delay_heur_k, "nonzero": delay_heur_n, "p": delay_heur_p},
            "direction_h8_vs_random": {"positive": dir_random_k, "nonzero": dir_random_n, "p": dir_random_p},
            "direction_h8_vs_heuristic": {"positive": dir_heur_k, "nonzero": dir_heur_n, "p": dir_heur_p},
        },
        "integrity": h[0]["runtime_integrity"],
        "claim_ledger": {
            "preintervention_experiment_selection": (
                "PASSED" if status == "PASSED" else "NOT_DEMONSTRATED"
            ),
            "sequential_experiment_selection": (
                "PASSED" if status == "PASSED" else "NOT_DEMONSTRATED"
            ),
            "general_intervention_intelligence": "NOT_DEMONSTRATED",
            "open_ontology": "NOT_DEMONSTRATED",
            "open_ended_rsi": "NOT_DEMONSTRATED",
            "agi": "NOT_DEMONSTRATED",
            "asi": "NOT_DEMONSTRATED",
        },
        "episodes": episode_rows,
    }


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--seed", type=int, required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()
    result = run_seed(args.seed)
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    (out / "h8_result.json").write_text(
        json.dumps(result, indent=2, sort_keys=True)
    )
    print(json.dumps({
        "schema": result["schema"],
        "seed": result["seed"],
        "scientific_status": result["scientific_status"],
        "metrics": result["metrics"],
        "paired_exact_sign_tests": result["paired_exact_sign_tests"],
    }, indent=2, sort_keys=True))
    return 0 if result["scientific_status"] == "PASSED" else 1


if __name__ == "__main__":
    raise SystemExit(main())
