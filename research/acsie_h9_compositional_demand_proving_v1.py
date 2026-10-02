from __future__ import annotations

"""H9-B compositional capability-demand transfer proving harness.

The runtime learns on eight basis intervention descriptors, then must choose among
a fresh pool containing both seen basis interventions and unseen combinations.
The hidden response is a problem x capability interaction surface.

This tests transfer of inferred demand, not intervention-signature memorization.
"""

import argparse
import json
import math
import random
import statistics
from pathlib import Path

from cognitive_core.h9_capability_demand import (
    CapabilityIntervention,
    ProblemObservation,
)
from cognitive_core.h9_capability_demand_compositional import (
    CompositionalCapabilityDemandInferencer,
)

DIM = 6
TRAIN_CAPS = 8
HOLDOUT_CAPS = 12
TRAIN_EPISODES = 96
HOLDOUT_EPISODES = 32
TRAIN_ROUNDS = 4
NOISE = 0.05


def norm(xs):
    n = math.sqrt(sum(x * x for x in xs)) or 1.0
    return tuple(x / n for x in xs)


def basis_interventions():
    return tuple(
        CapabilityIntervention(
            f"basis-{i}",
            tuple(1.0 if j == i else 0.0 for j in range(DIM)),
        )
        for i in range(TRAIN_CAPS)
    )


def holdout_interventions():
    rows = list(basis_interventions())
    combos = (
        (0, 1),
        (2, 3),
        (4, 5),
        (0, 5),
    )
    for idx, (a, b) in enumerate(combos):
        feat = [0.0] * DIM
        feat[a] = 0.7
        feat[b] = 0.7
        rows.append(
            CapabilityIntervention(
                f"unseen-{idx}",
                tuple(feat),
            )
        )
    return tuple(rows)


def hidden_surface(rng):
    return tuple(
        tuple(rng.uniform(-1.0, 1.0) for _ in range(DIM))
        for _ in range(DIM)
    )


def response(W, problem, capability):
    # Bilinear interaction: p^T W c.
    return sum(
        problem[i] * W[i][j] * capability[j]
        for i in range(DIM)
        for j in range(DIM)
    )


def run_seed(seed):
    rng = random.Random(seed * 100003 + 29)
    W = hidden_surface(rng)
    train_caps = basis_interventions()
    candidates = holdout_interventions()
    learner = CompositionalCapabilityDemandInferencer(
        observation_variance=NOISE ** 2,
    )

    training = []
    for episode in range(TRAIN_EPISODES):
        p = norm([rng.gauss(0.0, 1.0) for _ in range(DIM)])
        problem = ProblemObservation(f"train-{seed}-{episode}", p)
        chosen_train = rng.sample(range(TRAIN_CAPS), TRAIN_ROUNDS)
        rows = []
        for idx in chosen_train:
            cap = train_caps[idx]
            y = response(W, p, cap.features) + rng.gauss(0.0, NOISE)
            learner.observe(
                problem,
                cap,
                improvement=y,
                regression=0.0,
                accepted=y > 0.0,
            )
            rows.append({
                "capability": cap.signature,
                "observed": y,
            })
        training.append({
            "episode": episode,
            "rows": rows,
        })

    holdout_rows = []
    for episode in range(HOLDOUT_EPISODES):
        p = norm([rng.gauss(0.0, 1.0) for _ in range(DIM)])
        problem = ProblemObservation(f"holdout-{seed}-{episode}", p)

        selected, trace = learner.select(problem, candidates)
        if selected is None:
            raise RuntimeError("H9-B produced no candidate")
        selected_idx = next(
            i for i, c in enumerate(candidates)
            if c.signature == selected.signature
        )

        random_idx = rng.randrange(len(candidates))
        oracle_idx = max(
            range(len(candidates)),
            key=lambda i: response(W, p, candidates[i].features),
        )

        selected_gain = response(
            W, p, candidates[selected_idx].features
        )
        random_gain = response(
            W, p, candidates[random_idx].features
        )
        oracle_gain = response(
            W, p, candidates[oracle_idx].features
        )

        holdout_rows.append({
            "episode": episode,
            "selected": selected.signature,
            "selected_gain": selected_gain,
            "random_gain": random_gain,
            "oracle_gain": oracle_gain,
            "regret": oracle_gain - selected_gain,
            "selected_information_gain": trace.selected_information_gain,
            "selected_demand": trace.selected_demand,
            "unseen_selected": selected.signature.startswith("unseen-"),
            "oracle_unseen": candidates[oracle_idx].signature.startswith("unseen-"),
        })

    h9 = statistics.mean(r["selected_gain"] for r in holdout_rows)
    rnd = statistics.mean(r["random_gain"] for r in holdout_rows)
    regret = statistics.mean(r["regret"] for r in holdout_rows)
    unseen_rate = statistics.mean(
        1.0 if r["unseen_selected"] else 0.0
        for r in holdout_rows
    )

    pair = [
        r["selected_gain"] - r["random_gain"]
        for r in holdout_rows
        if abs(r["selected_gain"] - r["random_gain"]) > 1e-12
    ]
    positive = sum(x > 0 for x in pair)
    n = len(pair)
    if n and positive > n // 2:
        pvalue = sum(math.comb(n, i) for i in range(positive, n + 1)) / (2.0 ** n)
    else:
        pvalue = 1.0

    status = "PASSED" if (
        h9 > rnd
        and pvalue <= 0.05
        and unseen_rate > 0.0
    ) else "FAILED"

    return {
        "schema": "ACSIE.h9.compositional.v1",
        "seed": seed,
        "scientific_status": status,
        "train_episode_count": TRAIN_EPISODES,
        "holdout_episode_count": HOLDOUT_EPISODES,
        "candidate_count": HOLDOUT_CAPS,
        "unseen_candidate_count": HOLDOUT_CAPS - TRAIN_CAPS,
        "metrics": {
            "h9_mean_holdout_gain": h9,
            "random_mean_holdout_gain": rnd,
            "h9_improvement_over_random": h9 - rnd,
            "h9_mean_holdout_regret": regret,
            "h9_unseen_selection_rate": unseen_rate,
        },
        "paired_sign_test": {
            "positive": positive,
            "nonzero": n,
            "p": pvalue,
        },
        "integrity": {
            "external_model": False,
            "network_dependency": False,
            "manual_runtime_strategy": False,
            "hidden_surface_in_loop": False,
            "task_family_label_in_loop": False,
            "benchmark_identity_in_loop": False,
        },
        "claim_ledger": {
            "compositional_capability_demand_transfer": (
                "PASSED" if status == "PASSED" else "NOT_DEMONSTRATED"
            ),
            "general_capability_demand_inference": "NOT_DEMONSTRATED",
            "agi": "NOT_DEMONSTRATED",
            "asi": "NOT_DEMONSTRATED",
        },
        "training": training,
        "holdout": holdout_rows,
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--seed", type=int, required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

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
        "paired_sign_test": result["paired_sign_test"],
    }, indent=2, sort_keys=True))
    return 0 if result["scientific_status"] == "PASSED" else 1


if __name__ == "__main__":
    raise SystemExit(main())
