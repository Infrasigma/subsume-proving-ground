#!/usr/bin/env python3
from __future__ import annotations

import argparse
import math
import os
import random
import statistics
from dataclasses import dataclass
from typing import Sequence

EXPECTED_ACSIE_REF = "06ff348b7eba9de9192500dbfc00bb9a6d4cec9f"

def exact_sign_test(diffs: Sequence[float]) -> tuple[float, int]:
    nz = [d for d in diffs if abs(d) > 1e-12]
    n = len(nz)
    if n == 0:
        return 1.0, 0
    k = sum(d > 0.0 for d in nz)
    low = min(k, n - k)
    tail = sum(math.comb(n, i) for i in range(0, low + 1)) / (2.0 ** n)
    return min(1.0, 2.0 * tail), n

@dataclass(frozen=True)
class Candidate:
    signature: str
    features: tuple[float, ...]
    cost: float = 1.0

@dataclass(frozen=True)
class Problem:
    signature: str
    features: tuple[float, ...]

def candidates() -> tuple[Candidate, ...]:
    rows = [
        ("b0", (1, 0, 0, 0, 0, 0)),
        ("b1", (0, 1, 0, 0, 0, 0)),
        ("b2", (0, 0, 1, 0, 0, 0)),
        ("b3", (0, 0, 0, 1, 0, 0)),
        ("b4", (0, 0, 0, 0, 1, 0)),
        ("b5", (0, 0, 0, 0, 0, 1)),
        ("b6", (1, 1, 0, 0, 0, 0)),
        ("b7", (0, 0, 1, 1, 0, 0)),
        ("u0", (1, 0, 1, 0, 0, 1)),
        ("u1", (0, 1, 0, 1, 1, 0)),
        ("u2", (1, 0, 0, 1, 0, 1)),
        ("u3", (0, 1, 1, 0, 1, 0)),
    ]
    return tuple(Candidate(k, tuple(float(x) for x in v)) for k, v in rows)

def make_surface(seed: int):
    rng = random.Random(seed)
    bias = rng.uniform(-0.08, 0.08)
    linear = [rng.uniform(-0.16, 0.16) for _ in range(6)]
    interaction = [[rng.uniform(-0.28, 0.28) for _ in range(6)] for _ in range(6)]
    def latent(problem: Problem, candidate: Candidate) -> float:
        p, c = problem.features, candidate.features
        score = bias + sum(linear[i] * p[i] for i in range(6))
        score += sum(interaction[i][j] * p[i] * c[j] for i in range(6) for j in range(6))
        return score
    return latent

def sample_problem(rng: random.Random, prefix: str, idx: int) -> Problem:
    return Problem(prefix + "-" + str(idx), tuple(rng.uniform(-1.0, 1.0) for _ in range(6)))

def build_controller(module):
    return module.ActiveCompositionalCapabilityDemandController(
        exploration_beta=0.65,
        information_weight=0.06,
        observation_variance=0.03 ** 2,
        prior_variance=1.0,
        safety_epsilon=0.02,
    )

def observe(controller, base_module, problem, candidate, value):
    controller.observe(
        base_module.ProblemObservation(problem.signature, problem.features),
        base_module.CapabilityIntervention(candidate.signature, candidate.features, candidate.cost),
        improvement=max(0.0, value),
        regression=max(0.0, -value),
        accepted=value >= 0.0,
    )

def select(controller, base_module, problem, pool):
    interventions = tuple(
        base_module.CapabilityIntervention(c.signature, c.features, c.cost)
        for c in pool
    )
    chosen, _ = controller.select(
        base_module.ProblemObservation(problem.signature, problem.features),
        interventions,
    )
    if chosen is None:
        raise RuntimeError("controller returned no intervention")
    return next(c for c in pool if c.signature == chosen.signature)

def run_seed(seed: int, module, base_module) -> dict:
    rng = random.Random(seed)
    noise_rng = random.Random(seed * 7919 + 17)
    surface = make_surface(seed)
    pool = candidates()
    active = build_controller(module)
    random_policy = build_controller(module)
    random_history = []

    training = [sample_problem(rng, "train", i) for i in range(96)]
    for _round in range(4):
        for problem in training:
            a = select(active, base_module, problem, pool)
            r = pool[rng.randrange(len(pool))]
            shared_noise = noise_rng.gauss(0.0, 0.03)
            av = surface(problem, a) + shared_noise
            rv = surface(problem, r) + shared_noise
            observe(active, base_module, problem, a, av)
            observe(random_policy, base_module, problem, r, rv)
            random_history.append((problem, r, rv))

    historical = build_controller(module)
    for problem, r, rv in random_history:
        observe(historical, base_module, problem, r, rv)

    holdout = [sample_problem(rng, "holdout", i) for i in range(32)]
    active_scores, random_scores, historical_scores = [], [], []
    unseen_selected = random_unseen = historical_unseen = 0

    for problem in holdout:
        a = select(active, base_module, problem, pool)
        r = pool[rng.randrange(len(pool))]
        h = select(historical, base_module, problem, pool)
        shared_noise = noise_rng.gauss(0.0, 0.03)
        active_scores.append(surface(problem, a) + shared_noise)
        random_scores.append(surface(problem, r) + shared_noise)
        historical_scores.append(surface(problem, h) + shared_noise)
        unseen_selected += int(a.signature.startswith("u"))
        random_unseen += int(r.signature.startswith("u"))
        historical_unseen += int(h.signature.startswith("u"))

    active_vs_random = [a - r for a, r in zip(active_scores, random_scores)]
    active_vs_historical = [a - h for a, h in zip(active_scores, historical_scores)]
    p_random, n_random = exact_sign_test(active_vs_random)
    p_hist, n_hist = exact_sign_test(active_vs_historical)

    return {
        "seed": seed,
        "active_mean": statistics.fmean(active_scores),
        "random_mean": statistics.fmean(random_scores),
        "historical_mean": statistics.fmean(historical_scores),
        "active_vs_random_delta": statistics.fmean(active_vs_random),
        "active_vs_historical_delta": statistics.fmean(active_vs_historical),
        "p_active_vs_random": p_random,
        "n_active_vs_random": n_random,
        "p_active_vs_historical": p_hist,
        "n_active_vs_historical": n_hist,
        "unseen_selection_rate": unseen_selected / len(holdout),
        "random_unseen_rate": random_unseen / len(holdout),
        "historical_unseen_rate": historical_unseen / len(holdout),
    }

def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--seeds", default="2026100301,2026100302,2026100303,2026100304,2026100305")
    parser.add_argument("--strict", action="store_true")
    args = parser.parse_args()

    import cognitive_core.h9_active_compositional_demand as module
    import cognitive_core.h9_capability_demand as base_module

    actual_ref = os.environ.get("ACSIE_REF", "")
    print("ACSIE_REF=" + actual_ref)
    print("EXPECTED_ACSIE_REF=" + EXPECTED_ACSIE_REF)
    if actual_ref != EXPECTED_ACSIE_REF:
        print("STATUS=INVALID")
        print("reason=exact ACSIE research commit pin mismatch")
        return 2

    rows = [run_seed(int(x), module, base_module) for x in args.seeds.split(",") if x.strip()]
    for row in rows:
        print(row)

    gate = all(
        r["active_mean"] > r["random_mean"]
        and r["active_mean"] > r["historical_mean"]
        and r["p_active_vs_random"] <= 0.05
        and r["p_active_vs_historical"] <= 0.05
        and r["n_active_vs_random"] >= 16
        and r["n_active_vs_historical"] >= 16
        and r["unseen_selection_rate"] > 0.0
        for r in rows
    )
    print("FIVE_SEED_GATE=" + ("PASS" if gate else "FAIL"))
    return 0 if (gate or not args.strict) else 2

if __name__ == "__main__":
    raise SystemExit(main())
