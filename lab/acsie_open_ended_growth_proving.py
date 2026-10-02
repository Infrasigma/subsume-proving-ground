#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
import random
import statistics
from dataclasses import dataclass
from typing import Any

from cognitive_core.recursive_cognitive_compiler import (
    RecursiveCognitiveCompiler,
    Trace,
    eval_expr,
    ast_depth,
)


BASE_BIN_OPS = ("add", "sub", "mul", "max", "min")
UNARY_OPS = ("abs", "neg")
KEYS = ("x", "y", "z")


@dataclass(frozen=True)
class HiddenCapability:
    capability_id: str
    expression: dict[str, Any]
    depth: int
    generation: int
    parent_capabilities: tuple[str, ...]


def _const(rng: random.Random) -> int:
    return rng.choice((-2, -1, 0, 1, 2, 3, 4))


def random_base_expr(rng: random.Random, depth: int) -> dict[str, Any]:
    if depth <= 1:
        if rng.random() < 0.75:
            return {"op": "get", "key": rng.choice(KEYS)}
        return {"op": "const", "value": _const(rng)}
    if rng.random() < 0.2:
        return {"op": rng.choice(UNARY_OPS), "arg": random_base_expr(rng, depth - 1)}
    return {
        "op": rng.choice(BASE_BIN_OPS),
        "left": random_base_expr(rng, depth - 1),
        "right": random_base_expr(rng, depth - 1),
    }


def wrap_parent(rng: random.Random, parent: dict[str, Any]) -> dict[str, Any]:
    op = rng.choice(BASE_BIN_OPS + UNARY_OPS)
    if op in UNARY_OPS:
        return {"op": op, "arg": parent}
    if rng.random() < 0.5:
        rhs = {"op": "get", "key": rng.choice(KEYS)}
        return {"op": op, "left": parent, "right": rhs}
    rhs = {"op": "const", "value": _const(rng)}
    return {"op": op, "left": parent, "right": rhs}


def eval_hidden(expr: dict[str, Any], inputs: dict[str, float]) -> float:
    return float(eval_expr(expr, inputs, {}))


def traces_for(
    expr: dict[str, Any],
    seed: int,
    generation: int,
    split: str,
    count: int,
    *,
    key_shift: float = 0.0,
) -> tuple[Trace, ...]:
    rng = random.Random(seed * 1000003 + generation * 9176 + hash(split) % 997)
    rows = []
    for i in range(count):
        span = 2.0 if split == "train" else 3.5 if split == "holdout" else 4.5
        inputs = {k: rng.uniform(-span, span) + key_shift for k in KEYS}
        target = eval_hidden(expr, inputs)
        rows.append(
            Trace(
                inputs=inputs,
                target=target,
                family="opaque",
                context={"risk": 0.0, "ambiguity": 0.0},
                task_id=f"opaque-{seed}-{generation}-{split}-{i}",
            )
        )
    return tuple(rows)


def evaluate_expr(expr: dict[str, Any], rows: tuple[Trace, ...], macros: dict[str, dict[str, Any]]) -> float:
    if not rows:
        return math.inf
    vals = []
    for row in rows:
        try:
            vals.append(abs(float(eval_expr(expr, row.inputs, macros)) - row.target))
        except Exception:
            return math.inf
    return statistics.fmean(vals)


def capability_library_exprs(
    hidden: list[HiddenCapability],
    learned: RecursiveCognitiveCompiler,
) -> list[HiddenCapability]:
    learned_ids = set(learned.primitives)
    return [h for h in hidden if h.capability_id in learned_ids]


def make_probe(
    rng: random.Random,
    retained: list[HiddenCapability],
    *,
    extra_wraps: int,
) -> dict[str, Any]:
    if not retained:
        return random_base_expr(rng, 2)
    parent = rng.choice(retained).expression
    expr = parent
    for _ in range(max(1, extra_wraps)):
        expr = wrap_parent(rng, expr)
    return expr


def run_seed(seed: int, generations: int) -> dict[str, Any]:
    rng = random.Random(seed)
    learner = RecursiveCognitiveCompiler(max_depth=2, population=24, seed=seed)
    baseline = RecursiveCognitiveCompiler(max_depth=2, population=24, seed=seed + 9109)

    hidden: list[HiddenCapability] = []
    retained: list[HiddenCapability] = []
    generation_records = []

    # Generation 0 bootstraps one capability from the generic substrate.
    current_expr = random_base_expr(rng, 3)
    hidden.append(HiddenCapability("h0", current_expr, ast_depth(current_expr), 0, ()))
    previous_success = True

    for generation in range(generations):
        learner.generation = generation
        baseline.generation = 0

        if generation == 0:
            target = current_expr
            parent_ids: tuple[str, ...] = ()
        else:
            # The next challenge is generated from an actually retained capability.
            if not retained:
                parent = hidden[0]
            else:
                parent = rng.choice(retained)
            target = wrap_parent(rng, parent.expression)
            parent_ids = (parent.capability_id,)
            hidden.append(
                HiddenCapability(
                    f"h{len(hidden)}",
                    target,
                    ast_depth(target),
                    generation,
                    parent_ids,
                )
            )

        train = traces_for(target, seed, generation, "train", 12)
        hold = traces_for(target, seed, generation, "holdout", 6, key_shift=0.37)
        transfer = traces_for(target, seed, generation, "transfer", 8, key_shift=-0.61)
        ood = traces_for(target, seed, generation, "ood", 8, key_shift=1.11)

        prim = learner.invent_primitive(train, hold, transfer)
        learned_ok = False
        used_parents: tuple[str, ...] = ()
        primitive_transfer_error = math.inf
        primitive_ood_error = math.inf
        if prim is not None:
            used_parents = tuple(prim.parent_ids)
            primitive_transfer_error = float(prim.transfer_error)
            primitive_ood_error = evaluate_expr(prim.expression, ood, learner._pmap())
            learned_ok = primitive_transfer_error <= 1e-9 and primitive_ood_error <= 1e-9
            if not learned_ok:
                learner.primitives.pop(prim.primitive_id, None)
                learner.archive.pop(prim.primitive_id, None)
        else:
            learner.events.append({"event": "OPEN_ENDED_DISCOVERY_FAILURE", "generation": generation})

        if learned_ok:
            learner.archive[prim.primitive_id].utility_history.append(1.0)
            retained.append(
                HiddenCapability(
                    f"h-retained-{generation}",
                    target,
                    ast_depth(target),
                    generation,
                    parent_ids,
                )
            )
            learner.recursion_depth = max(
                learner.recursion_depth,
                generation + 1,
            )

        # Compare with a fresh non-recursive baseline on the same target.
        baseline.generation = 0
        bprim = baseline.invent_primitive(train, hold, transfer)
        baseline_ok = False
        if bprim is not None:
            b_ood = evaluate_expr(bprim.expression, ood, baseline._pmap())
            baseline_ok = bprim.transfer_error <= 1e-9 and b_ood <= 1e-9

        # Fresh capability probes use only the currently retained archive.
        probe_successes = 0
        probe_total = 0
        baseline_probe_successes = 0
        for probe_index in range(4):
            probe = make_probe(rng, retained, extra_wraps=2)
            ptrain = traces_for(probe, seed + 101 + probe_index, generation, "probe-train", 8)
            ptransfer = traces_for(probe, seed + 101 + probe_index, generation, "probe-transfer", 5, key_shift=0.53)
            pood = traces_for(probe, seed + 101 + probe_index, generation, "probe-ood", 5, key_shift=-0.47)
            proc = learner.synthesize_process(ptrain, ptransfer, pood)
            if proc is not None:
                probe_eval = learner.evaluate(proc, ptrain, ptransfer, ptransfer, pood, ())
                probe_successes += int(probe_eval.accepted)
            bproc = baseline.synthesize_process(ptrain, ptransfer, pood)
            baseline_probe_successes += int(bproc is not None)
            probe_total += 1

        growth = {
            "generation": generation,
            "target_depth": ast_depth(target),
            "parent_capabilities": parent_ids,
            "learned": learned_ok,
            "used_parent_ids": used_parents,
            "primitive_transfer_error": primitive_transfer_error,
            "primitive_ood_error": primitive_ood_error,
            "baseline_solved": baseline_ok,
            "retained_count": len(retained),
            "recursive_depth": learner.recursion_depth,
            "probe_success_rate": probe_successes / probe_total,
            "baseline_probe_success_rate": baseline_probe_successes / probe_total,
            "event_count": len(learner.events),
        }
        generation_records.append(growth)

    learned_generations = sum(r["learned"] for r in generation_records[1:])
    recursive_generations = sum(bool(r["used_parent_ids"]) for r in generation_records[1:])
    probe_gain = statistics.fmean(
        r["probe_success_rate"] - r["baseline_probe_success_rate"]
        for r in generation_records
    )
    final = generation_records[-1]
    gate = (
        learned_generations >= max(5, generations - 2)
        and recursive_generations >= max(5, generations - 2)
        and final["retained_count"] >= max(5, generations - 2)
        and final["target_depth"] > generation_records[0]["target_depth"]
        and final["probe_success_rate"] > final["baseline_probe_success_rate"]
        and probe_gain > 0.20
        and final["baseline_solved"] is False
    )

    return {
        "seed": seed,
        "generations": generations,
        "generation_records": generation_records,
        "retained_count": len(retained),
        "final_target_depth": final["target_depth"],
        "final_probe_success_rate": final["probe_success_rate"],
        "final_baseline_probe_success_rate": final["baseline_probe_success_rate"],
        "mean_probe_gain": probe_gain,
        "learned_generations_after_bootstrap": learned_generations,
        "recursive_generations": recursive_generations,
        "finite_open_ended_gate": gate,
        "external_model": False,
        "target_identity_available_to_runtime": False,
        "task_family_route": False,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--seeds",
        default="2026100301,2026100302,2026100303,2026100304,2026100305",
    )
    parser.add_argument("--generations", type=int, default=8)
    parser.add_argument("--strict", action="store_true")
    args = parser.parse_args()

    rows = [
        run_seed(int(s), args.generations)
        for s in args.seeds.split(",")
        if s.strip()
    ]
    for row in rows:
        print(json.dumps(row, sort_keys=True))

    gate = (
        len(rows) == 5
        and all(r["finite_open_ended_gate"] for r in rows)
        and all(r["external_model"] is False for r in rows)
        and all(r["target_identity_available_to_runtime"] is False for r in rows)
        and all(r["task_family_route"] is False for r in rows)
    )
    print("OPEN_ENDED_GROWTH_GATE=" + ("PASS" if gate else "FAIL"))
    return 0 if (gate or not args.strict) else 2


if __name__ == "__main__":
    raise SystemExit(main())
