from __future__ import annotations

"""H9 bounded capability-demand inference proving harness.

Question:
    Can ACSIE infer which anonymous capability intervention is demanded by a
    problem structure from intervention consequences, then select the demanded
    intervention on fresh problem structures?

Protocol:
- 6-dimensional anonymous problem structure.
- 8 opaque capability interventions with fixed native feature vectors.
- hidden response surface sampled independently per seed.
- 32 problem worlds per seed.
- 4 training interventions on each problem, then one delayed decision on a fresh
  paired problem.
- random and historical-average controls.
- target/intervention identity is evaluator-only; the runtime sees only anonymous
  signatures and feature vectors.
"""

import argparse
import json
import math
import random
import statistics
from pathlib import Path

from cognitive_core.h9_capability_demand import (
    AnonymousCapabilityDemandInferencer,
    CapabilityIntervention,
    ProblemObservation,
)

PROBLEM_DIMENSIONS = 6
INTERVENTION_COUNT = 8
TRAIN_EPISODES = 64
HOLDOUT_EPISODES = 32
TRAIN_ROUNDS = 4
NOISE_STD = 0.05


def sigmoid(x: float) -> float:
    return 1.0 / (1.0 + math.exp(-max(-30.0, min(30.0, x))))


def make_interventions() -> tuple[CapabilityIntervention, ...]:
    rows = []
    for i in range(INTERVENTION_COUNT):
        features = tuple(
            1.0 if j == i % PROBLEM_DIMENSIONS else 0.0
            for j in range(PROBLEM_DIMENSIONS)
        )
        rows.append(
            CapabilityIntervention(
                signature=f"cap-{i:02d}-opaque",
                features=features,
                cost=1.0 + 0.1 * (i % 3),
            )
        )
    return tuple(rows)


def make_hidden_surface(rng: random.Random) -> tuple[tuple[float, ...], ...]:
    # Each intervention has its own sparse consequence vector. The runtime never
    # sees this matrix; it can only infer it from intervention outcomes.
    surface = []
    for _ in range(INTERVENTION_COUNT):
        row = [
            rng.uniform(-1.0, 1.0)
            for _ in range(PROBLEM_DIMENSIONS)
        ]
        surface.append(tuple(row))
    return tuple(surface)


def net_gain(
    surface: tuple[tuple[float, ...], ...],
    problem: tuple[float, ...],
    intervention_index: int,
) -> float:
    return sum(
        a * b
        for a, b in zip(surface[intervention_index], problem)
    ) / math.sqrt(PROBLEM_DIMENSIONS)


def make_problem(rng: random.Random) -> tuple[float, ...]:
    values = [rng.gauss(0.0, 1.0) for _ in range(PROBLEM_DIMENSIONS)]
    norm = math.sqrt(sum(v * v for v in values)) or 1.0
    return tuple(v / norm for v in values)


def observe_problem(
    rng: random.Random,
    surface: tuple[tuple[float, ...], ...],
    problem: tuple[float, ...],
    intervention_index: int,
) -> tuple[float, float]:
    true_gain = net_gain(surface, problem, intervention_index)
    # Regression is a separate observable. The runtime never receives the true
    # latent gain; it sees noisy downstream improvement and noisy regression.
    regression = max(0.0, rng.gauss(0.02 if true_gain < -0.25 else 0.005, 0.01))
    improvement = true_gain + rng.gauss(0.0, NOISE_STD)
    return improvement, regression


def best_historical_choice(
    history: dict[str, list[float]],
    interventions: tuple[CapabilityIntervention, ...],
) -> CapabilityIntervention:
    scored = []
    for intervention in interventions:
        values = history.get(intervention.signature, [])
        mean_value = statistics.mean(values) if values else 0.0
        scored.append((mean_value, intervention.signature, intervention))
    return sorted(scored, key=lambda x: (x[0], x[1]), reverse=True)[0][2]


def run_training_episode(
    *,
    seed: int,
    episode: int,
    surface: tuple[tuple[float, ...], ...],
    interventions: tuple[CapabilityIntervention, ...],
    learner: AnonymousCapabilityDemandInferencer,
    history: dict[str, list[float]],
) -> dict:
    rng = random.Random(seed * 100003 + episode * 9176 + 31)
    problem_vector = make_problem(rng)
    problem = ProblemObservation(f"train-{seed}-{episode}", problem_vector)
    remaining = list(range(INTERVENTION_COUNT))
    rows = []

    for round_index in range(TRAIN_ROUNDS):
        selected, trace = learner.select(
            problem,
            tuple(interventions[i] for i in remaining),
        )
        if selected is None:
            raise RuntimeError("H9 produced no training intervention")
        idx = next(
            i for i in remaining
            if interventions[i].signature == selected.signature
        )
        improvement, regression = observe_problem(
            rng, surface, problem_vector, idx
        )
        learner.observe(
            problem,
            selected,
            improvement=improvement,
            regression=regression,
            accepted=(improvement - regression) > 0.0,
        )
        history.setdefault(selected.signature, []).append(
            improvement - regression
        )
        rows.append({
            "round": round_index,
            "selected": selected.signature,
            "predicted_demand": trace.selected_demand,
            "information_gain": trace.selected_information_gain,
            "net_observed": improvement - regression,
        })
        remaining.remove(idx)

    return {"episode": episode, "rows": rows}


def run_holdout_episode(
    *,
    seed: int,
    episode: int,
    surface: tuple[tuple[float, ...], ...],
    interventions: tuple[CapabilityIntervention, ...],
    learner: AnonymousCapabilityDemandInferencer,
    history: dict[str, list[float]],
) -> dict:
    rng = random.Random(seed * 300007 + episode * 101 + 7)
    problem_vector = make_problem(rng)
    problem = ProblemObservation(f"holdout-{seed}-{episode}", problem_vector)

    h9_choice, h9_trace = learner.select(problem, interventions)
    if h9_choice is None:
        raise RuntimeError("H9 produced no holdout intervention")
    h9_idx = next(
        i for i, x in enumerate(interventions)
        if x.signature == h9_choice.signature
    )

    random_idx = rng.randrange(INTERVENTION_COUNT)
    heuristic_choice = best_historical_choice(history, interventions)
    heuristic_idx = next(
        i for i, x in enumerate(interventions)
        if x.signature == heuristic_choice.signature
    )

    h9_gain = net_gain(surface, problem_vector, h9_idx)
    random_gain = net_gain(surface, problem_vector, random_idx)
    heuristic_gain = net_gain(surface, problem_vector, heuristic_idx)
    oracle_gain = max(
        net_gain(surface, problem_vector, i)
        for i in range(INTERVENTION_COUNT)
    )

    return {
        "episode": episode,
        "h9_choice": h9_choice.signature,
        "random_choice": interventions[random_idx].signature,
        "heuristic_choice": heuristic_choice.signature,
        "h9_predicted_demand": h9_trace.selected_demand,
        "h9_information_gain": h9_trace.selected_information_gain,
        "h9_gain": h9_gain,
        "random_gain": random_gain,
        "heuristic_gain": heuristic_gain,
        "oracle_gain": oracle_gain,
        "h9_regret": oracle_gain - h9_gain,
        "random_regret": oracle_gain - random_gain,
        "heuristic_regret": oracle_gain - heuristic_gain,
    }



def sign_test_pvalue(values: list[float]) -> tuple[int, int, float]:
    nonzero = [x for x in values if abs(x) > 1e-12]
    n = len(nonzero)
    k = sum(x > 0.0 for x in nonzero)
    if n == 0:
        return 0, 0, 1.0
    if k <= n // 2:
        return k, n, 1.0
    p = sum(math.comb(n, i) for i in range(k, n + 1)) / (2.0 ** n)
    return k, n, p


def run_seed(seed: int) -> dict:
    rng = random.Random(seed * 7919 + 13)
    surface = make_hidden_surface(rng)
    interventions = make_interventions()

    learner = AnonymousCapabilityDemandInferencer(
        observation_variance=NOISE_STD ** 2,
    )
    history: dict[str, list[float]] = {}

    training_rows = []
    for episode in range(TRAIN_EPISODES):
        training_rows.append(
            run_training_episode(
                seed=seed,
                episode=episode,
                surface=surface,
                interventions=interventions,
                learner=learner,
                history=history,
            )
        )

    holdout_rows = [
        run_holdout_episode(
            seed=seed,
            episode=episode,
            surface=surface,
            interventions=interventions,
            learner=learner,
            history=history,
        )
        for episode in range(HOLDOUT_EPISODES)
    ]

    h9 = [r["h9_gain"] for r in holdout_rows]
    rnd = [r["random_gain"] for r in holdout_rows]
    heu = [r["heuristic_gain"] for r in holdout_rows]
    h9_vs_random = [a - b for a, b in zip(h9, rnd)]
    h9_vs_heuristic = [a - b for a, b in zip(h9, heu)]
    p_hr = sign_test_pvalue(h9_vs_random)
    p_hh = sign_test_pvalue(h9_vs_heuristic)

    h9_mean = statistics.mean(h9)
    random_mean = statistics.mean(rnd)
    heuristic_mean = statistics.mean(heu)

    status = "PASSED" if (
        h9_mean > random_mean
        and h9_mean > heuristic_mean
        and p_hr[2] <= 0.05
        and p_hh[2] <= 0.05
    ) else "FAILED"

    return {
        "schema": "ACSIE.h9.capability-demand.v2",
        "seed": seed,
        "train_episode_count": TRAIN_EPISODES,
        "holdout_episode_count": HOLDOUT_EPISODES,
        "intervention_count": INTERVENTION_COUNT,
        "problem_dimensions": PROBLEM_DIMENSIONS,
        "train_rounds": TRAIN_ROUNDS,
        "scientific_status": status,
        "integrity": {
            "external_model": False,
            "network_dependency": False,
            "manual_runtime_strategy": False,
            "hidden_surface_in_loop": False,
            "task_family_label_in_loop": False,
            "benchmark_identity_in_loop": False,
        },
        "metrics": {
            "h9_mean_holdout_gain": h9_mean,
            "random_mean_holdout_gain": random_mean,
            "heuristic_mean_holdout_gain": heuristic_mean,
            "h9_improvement_over_random": h9_mean - random_mean,
            "h9_improvement_over_heuristic": h9_mean - heuristic_mean,
            "h9_mean_holdout_regret": statistics.mean(
                r["h9_regret"] for r in holdout_rows
            ),
            "random_mean_holdout_regret": statistics.mean(
                r["random_regret"] for r in holdout_rows
            ),
            "heuristic_mean_holdout_regret": statistics.mean(
                r["heuristic_regret"] for r in holdout_rows
            ),
        },
        "paired_sign_tests": {
            "h9_vs_random": {
                "positive": p_hr[0], "nonzero": p_hr[1], "p": p_hr[2]
            },
            "h9_vs_heuristic": {
                "positive": p_hh[0], "nonzero": p_hh[1], "p": p_hh[2]
            },
        },
        "claim_ledger": {
            "anonymous_capability_demand_inference": (
                "PASSED" if status == "PASSED" else "NOT_DEMONSTRATED"
            ),
            "cross_problem_transfer_of_demand_inference": (
                "PASSED" if status == "PASSED" else "NOT_DEMONSTRATED"
            ),
            "general_capability_demand_inference": "NOT_DEMONSTRATED",
            "open_ontology": "NOT_DEMONSTRATED",
            "open_ended_rsi": "NOT_DEMONSTRATED",
            "agi": "NOT_DEMONSTRATED",
            "asi": "NOT_DEMONSTRATED",
        },
        "training": training_rows,
        "holdout": holdout_rows,
    }



def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--seed", type=int, required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    result = run_seed(args.seed)
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    (out / "h9_result.json").write_text(
        json.dumps(result, indent=2, sort_keys=True)
    )
    print(json.dumps({
        "schema": result["schema"],
        "seed": result["seed"],
        "scientific_status": result["scientific_status"],
        "metrics": result["metrics"],
        "paired_sign_tests": result["paired_sign_tests"],
    }, indent=2, sort_keys=True))
    return 0 if result["scientific_status"] == "PASSED" else 1


if __name__ == "__main__":
    raise SystemExit(main())
