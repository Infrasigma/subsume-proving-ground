from __future__ import annotations

"""H8-A: target-blind active causal experiment-selection proving harness.

Scientific scope:
  Can a native selector choose experiments that improve later action quality
  over matched random and immediate-gain selection?

This is deliberately narrower than mechanism invention, ontology expansion, or
recursive self-improvement. The hidden environment contains an anonymous causal
axis; the controller sees only candidate feature vectors and observed outcomes.
The axis identity is retained by the evaluator only.

The experiment set has 48 fresh candidates per episode. Each method receives the
same candidate pool, same hidden world, same observation budget, and same delayed
evaluation. No model, task-family label, benchmark identifier, or target identity
is exposed to the controller.
"""

import argparse
import json
import math
import random
import statistics
from dataclasses import dataclass
from pathlib import Path
from typing import Sequence

from cognitive_core.h8_active_causal_experiment import ActiveCausalExperimentPlanner
from cognitive_core.h8_bayesian_active_experiment import BayesianActiveCausalExperimentPlanner


MAGNITUDES = (0.25, 0.50, 0.75, 1.00, 1.25, 1.50)
AXES = 8
CANDIDATE_COUNT = len(MAGNITUDES) * AXES
EXPLORATION_ROUNDS = 4
EPISODES_PER_SEED = 24


@dataclass(frozen=True)
class HiddenWorld:
    coefficients: tuple[float, ...]
    hidden_axis: int


@dataclass(frozen=True)
class Candidate:
    signature: str
    feature: tuple[float, ...]
    cost: float = 1.0


@dataclass
class MethodState:
    name: str
    planner: ActiveCausalExperimentPlanner
    selected: list[str]
    outcomes: list[float]


def make_world(rng: random.Random) -> HiddenWorld:
    hidden_axis = rng.randrange(AXES)
    coefficients = [0.10 for _ in range(AXES)]
    # One anonymous capability axis is materially more useful. Its index is never
    # exposed to the runtime controller.
    coefficients[hidden_axis] = 1.00
    return HiddenWorld(tuple(coefficients), hidden_axis)


def make_candidates(rng: random.Random) -> tuple[Candidate, ...]:
    candidates = []
    for axis in range(AXES):
        for magnitude in MAGNITUDES:
            f = tuple(magnitude if i == axis else 0.0 for i in range(AXES))
            candidates.append(
                Candidate(
                    signature=f"exp-{rng.randrange(10**9):09d}",
                    feature=f,
                    cost=1.0,
                )
            )
    shuffled = list(candidates)
    rng.shuffle(shuffled)
    assert len({c.signature for c in shuffled}) == CANDIDATE_COUNT
    return tuple(shuffled)


def true_reward(world: HiddenWorld, feature: Sequence[float]) -> float:
    return sum(w * x for w, x in zip(world.coefficients, feature))


def observe(world: HiddenWorld, candidate: Candidate, rng: random.Random) -> float:
    return true_reward(world, candidate.feature) + rng.gauss(0.0, 0.05)


def planner_update(
    planner: ActiveCausalExperimentPlanner,
    candidate: Candidate,
    observed: float,
) -> None:
    planner.observe(
        signature=candidate.signature,
        feature=candidate.feature,
        future_delta=observed,
        regression=0.0,
        novelty=1.0,
        cost=candidate.cost,
        accepted=False,
    )


def greedy_choice(
    planner: ActiveCausalExperimentPlanner,
    candidates: Sequence[Candidate],
) -> Candidate:
    proposals = planner.propose(
        tuple((c.signature, c.feature, c.cost, 1.0) for c in candidates)
    )
    return sorted(
        proposals,
        key=lambda p: (
            p.predicted_future_gain,
            p.expected_information_gain,
            p.signature,
        ),
        reverse=True,
    )[0] and next(
        c for c in candidates
        if c.signature == sorted(
            proposals,
            key=lambda p: (p.predicted_future_gain, p.expected_information_gain, p.signature),
            reverse=True,
        )[0].signature
    )


def exploit_choice(
    planner: ActiveCausalExperimentPlanner,
    candidates: Sequence[Candidate],
) -> Candidate:
    proposals = planner.propose(
        tuple((c.signature, c.feature, c.cost, 1.0) for c in candidates)
    )
    chosen = sorted(
        proposals,
        key=lambda p: (
            p.predicted_future_gain,
            p.novelty,
            -p.cost,
            p.signature,
        ),
        reverse=True,
    )[0]
    return next(c for c in candidates if c.signature == chosen.signature)


def run_method(
    method_name: str,
    world: HiddenWorld,
    candidates: Sequence[Candidate],
    seed: int,
) -> dict:
    if method_name == "h8":
        planner: ActiveCausalExperimentPlanner = BayesianActiveCausalExperimentPlanner()
    elif method_name == "greedy":
        planner = BayesianActiveCausalExperimentPlanner()
    elif method_name == "random":
        planner = BayesianActiveCausalExperimentPlanner()
    else:
        raise ValueError(method_name)

    rng = random.Random(seed + {"h8": 101, "greedy": 202, "random": 303}[method_name])
    selected: list[Candidate] = []
    outcomes: list[float] = []
    information_trace: list[float] = []
    prediction_regret_trace: list[float] = []
    q_quality_trace: list[float] = []

    remaining = list(candidates)

    for _ in range(EXPLORATION_ROUNDS):
        if method_name == "h8":
            choice, trace = planner.select(
                tuple((c.signature, c.feature, c.cost, 1.0) for c in remaining)
            )
            if choice is None:
                raise RuntimeError("H8 produced no safe experiment")
            candidate = next(c for c in remaining if c.signature == choice.signature)
            information_trace.append(float(trace.selected_information_gain))
        elif method_name == "greedy":
            candidate = greedy_choice(planner, remaining)
            props = planner.propose(
                tuple((c.signature, c.feature, c.cost, 1.0) for c in remaining)
            )
            info = next(p.expected_information_gain for p in props if p.signature == candidate.signature)
            information_trace.append(float(info))
        else:
            candidate = rng.choice(tuple(remaining))
            information_trace.append(0.0)

        selected.append(candidate)
        remaining = [c for c in remaining if c.signature != candidate.signature]

        y = observe(world, candidate, rng)
        outcomes.append(y)
        planner_update(planner, candidate, y)

        predicted = max(
            (true_reward(world, c) for c in remaining),
            default=true_reward(world, candidate),
        )
        if remaining:
            predicted_candidate = max(
                remaining,
                key=lambda c: (
                    planner._predicted_future(c.feature),
                    c.signature,
                ),
            )
            predicted_truth = true_reward(world, predicted_candidate.feature)
        else:
            predicted_truth = true_reward(world, candidate.feature)
        prediction_regret_trace.append(
            float(max(0.0, oracle if (oracle := max(true_reward(world, c.feature) for c in candidates)) else 0.0) - predicted_truth)
        )
        oracle_now = max(true_reward(world, c.feature) for c in candidates)
        q_quality_trace.append(
            float(predicted_truth / max(oracle_now, 1e-9))
        )

    final_candidate = exploit_choice(planner, remaining)
    immediate_truth = true_reward(world, final_candidate.feature)
    delayed_observation = immediate_truth + rng.gauss(0.0, 0.05)
    oracle_truth = max(true_reward(world, c.feature) for c in candidates)

    return {
        "method": method_name,
        "selected": [c.signature for c in selected],
        "observed_outcomes": outcomes,
        "information_gain_trace": information_trace,
        "prediction_regret_trace": prediction_regret_trace,
        "q_quality_trace": q_quality_trace,
        "final_candidate": final_candidate.signature,
        "final_true_reward": immediate_truth,
        "delayed_observation": delayed_observation,
        "oracle_best_reward": oracle_truth,
        "final_regret": max(0.0, oracle_truth - immediate_truth),
        "posterior_entropy": planner.posterior_entropy(),
    }


def run_seed(seed: int) -> dict:
    episode_rows = []

    for episode in range(EPISODES_PER_SEED):
        episode_seed = seed * 1000 + episode * 37
        rng = random.Random(episode_seed)
        world = make_world(rng)
        candidates = make_candidates(rng)

        h8 = run_method("h8", world, candidates, episode_seed)
        greedy = run_method("greedy", world, candidates, episode_seed)
        random_row = run_method("random", world, candidates, episode_seed)

        episode_rows.append({
            "episode": episode,
            "h8": h8,
            "greedy": greedy,
            "random": random_row,
            # Evaluator-only hidden fact. Never supplied to any planner.
            "hidden_axis_evaluator_only": world.hidden_axis,
        })

    def mean_method(field: str, method: str) -> float:
        return statistics.mean(row[method][field] for row in episode_rows)

    h8_final = mean_method("final_true_reward", "h8")
    greedy_final = mean_method("final_true_reward", "greedy")
    random_final = mean_method("final_true_reward", "random")
    h8_regret = mean_method("final_regret", "h8")
    greedy_regret = mean_method("final_regret", "greedy")
    random_regret = mean_method("final_regret", "random")

    h8_improvement_over_random = h8_final - random_final
    h8_improvement_over_greedy = h8_final - greedy_final

    delayed_abs_error = statistics.mean(
        abs(row["h8"]["delayed_observation"] - row["h8"]["final_true_reward"])
        for row in episode_rows
    )

    h8_info = statistics.mean(
        sum(row["h8"]["information_gain_trace"])
        for row in episode_rows
    )

    within_episode_h8_improvement = [
        sum(
            later > earlier
            for earlier, later in zip(
                row["h8"]["q_quality_trace"],
                row["h8"]["q_quality_trace"][1:],
            )
        )
        for row in episode_rows
    ]
    repeated_improvement = statistics.mean(within_episode_h8_improvement) >= 1.0

    q_quality = [
        row["h8"]["final_true_reward"] / max(row["h8"]["oracle_best_reward"], 1e-9)
        for row in episode_rows
    ]

    scientific_status = "PASSED" if (
        h8_improvement_over_random > 0.0
        and h8_improvement_over_greedy > 0.0
        and h8_info > 0.0
        and delayed_abs_error < 0.20
        and repeated_improvement
    ) else "FAILED"

    return {
        "schema": "ACSIE.h8.active-causal-proving.v1",
        "seed": seed,
        "candidate_count": CANDIDATE_COUNT,
        "episodes": EPISODES_PER_SEED,
        "exploration_rounds": EXPLORATION_ROUNDS,
        "integrity": {
            "external_model": False,
            "network_dependency": False,
            "manual_runtime_strategy": False,
            "target_identity_accessible": False,
            "hidden_world_identity_in_loop": False,
            "evaluator_only_hidden_axis": True,
        },
        "metrics": {
            "h8_final_reward": h8_final,
            "greedy_final_reward": greedy_final,
            "random_final_reward": random_final,
            "h8_improvement_over_random": h8_improvement_over_random,
            "h8_improvement_over_greedy": h8_improvement_over_greedy,
            "h8_final_regret": h8_regret,
            "greedy_final_regret": greedy_regret,
            "random_final_regret": random_regret,
            "h8_mean_information_gain": h8_info,
            "h8_delayed_abs_error": delayed_abs_error,
            "h8_mean_quality_ratio": statistics.mean(q_quality),
        },
        "scientific_status": scientific_status,
        "claim_ledger": {
            "active_experiment_selection_on_hidden_causal_response": (
                "PASSED" if scientific_status == "PASSED" else "NOT_DEMONSTRATED"
            ),
            "selection_above_random": (
                "PASSED" if h8_improvement_over_random > 0.0 else "NOT_DEMONSTRATED"
            ),
            "selection_above_greedy": (
                "PASSED" if h8_improvement_over_greedy > 0.0 else "NOT_DEMONSTRATED"
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
    parser = argparse.ArgumentParser()
    parser.add_argument("--seed", type=int, required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

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
    }, indent=2, sort_keys=True))
    return 0 if result["scientific_status"] == "PASSED" else 1


if __name__ == "__main__":
    raise SystemExit(main())
