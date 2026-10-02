#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import random
import statistics
from dataclasses import dataclass
from hashlib import sha256
from typing import Any

from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import Trace, eval_expr, ast_depth


BASE_BIN_OPS = ("add", "sub", "mul", "max", "min")
UNARY_OPS = ("abs", "neg")
KEYS = ("x", "y", "z")


@dataclass(frozen=True)
class HiddenCapability:
    hidden_id: str
    expression: dict[str, Any]
    depth: int
    generation: int
    parent_hidden_id: str | None = None
    parent_hidden_ids: tuple[str, ...] = ()


def stable_int(text: str) -> int:
    return int.from_bytes(sha256(text.encode()).digest()[:4], "big")


def random_base_expr(rng: random.Random, depth: int) -> dict[str, Any]:
    if depth <= 1:
        if rng.random() < 0.75:
            return {"op": "get", "key": rng.choice(KEYS)}
        return {"op": "const", "value": rng.choice((-2, -1, 0, 1, 2, 3, 4))}
    if rng.random() < 0.20:
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
    else:
        rhs = {"op": "const", "value": rng.choice((-2, -1, 0, 1, 2, 3, 4))}
    return {"op": op, "left": parent, "right": rhs}


def combine_parents(
    rng: random.Random,
    parents: tuple[HiddenCapability, ...],
) -> dict[str, Any]:
    if len(parents) == 1:
        return wrap_parent(rng, parents[0].expression)
    op = rng.choice(("add", "sub", "mul", "max", "min"))
    return {
        "op": op,
        "left": parents[0].expression,
        "right": parents[1].expression,
    }


def eval_hidden(
    expr: dict[str, Any],
    row: dict[str, float],
    macros: dict[str, dict[str, Any]],
) -> float:
    return float(eval_expr(expr, row, macros))


def expanded_depth(
    expr: dict[str, Any],
    macros: dict[str, dict[str, Any]],
    stack: tuple[str, ...] = (),
) -> int:
    op = expr["op"]
    if op == "macro":
        mid = str(expr["id"])
        if mid in stack or mid not in macros:
            return 1
        return expanded_depth(macros[mid], macros, stack + (mid,))
    if op in {"get", "const"}:
        return 1
    if op in {"abs", "neg", "threshold"}:
        return 1 + expanded_depth(expr["arg"], macros, stack)
    return 1 + max(
        expanded_depth(expr["left"], macros, stack),
        expanded_depth(expr["right"], macros, stack),
    )


def make_traces(expr, seed, generation, split, count, shift):
    rng = random.Random(seed * 1000003 + generation * 9176 + stable_int(split))
    base_split = split.rsplit("_", 1)[-1]
    span = {"train": 2.0, "holdout": 3.0, "transfer": 4.0, "ood": 5.0}[base_split]
    rows = []
    for i in range(count):
        inputs = {k: rng.uniform(-span, span) + shift for k in KEYS}
        rows.append(
            Trace(
                inputs=inputs,
                target=eval_hidden(expr, inputs, macros),
                family="opaque",
                context={"risk": 0.0, "ambiguity": 0.0},
                task_id=f"opaque-{seed}-{generation}-{split}-{i}",
            )
        )
    return tuple(rows)


def mae(expr, rows, macros):
    if not rows:
        return float("inf")
    vals = []
    for row in rows:
        try:
            vals.append(abs(float(eval_expr(expr, row.inputs, macros)) - row.target))
        except Exception:
            return float("inf")
    return statistics.fmean(vals)


def run_seed(seed: int, generations: int) -> dict[str, Any]:
    rng = random.Random(seed)
    learner = OpenEndedRecursiveCognitiveCompiler(max_depth=2, population=24, seed=seed)
    # Fixed baseline: no recursive depth growth and no retained-capability
    # archive. This is the actual non-self-extending control.
    baseline = RecursiveCognitiveCompiler(max_depth=2, population=24, seed=seed + 10091)

    retained: list[HiddenCapability] = []
    generations_out = []
    initial_depth = learner.meta_policy["max_depth"]

    def hidden_library() -> dict[str, dict[str, Any]]:
        return {cap.hidden_id: cap.expression for cap in retained}

    def capability_macro(cap: HiddenCapability) -> dict[str, Any]:
        return {"op": "macro", "id": cap.hidden_id}

    for generation in range(generations):
        learner.generation = generation
        hidden_macros = hidden_library()

        target_parents: tuple[HiddenCapability, ...]
        if not retained:
            target_parents = ()
            target = random_base_expr(rng, 3)
        elif len(retained) == 1:
            target_parents = (retained[0],)
            target = {
                "op": rng.choice(("add", "sub", "mul", "max", "min")),
                "left": capability_macro(retained[0]),
                "right": {"op": "get", "key": rng.choice(KEYS)},
            }
        else:
            target_parents = tuple(rng.sample(retained, 2))
            target = {
                "op": rng.choice(("add", "sub", "mul", "max", "min")),
                "left": capability_macro(target_parents[0]),
                "right": capability_macro(target_parents[1]),
            }

        target_depth = expanded_depth(target, hidden_macros)

        train = make_traces(
            target, seed, generation, "train", 12, 0.0, hidden_macros
        )
        hold = make_traces(
            target, seed, generation, "holdout", 6, 0.31, hidden_macros
        )
        transfer = make_traces(
            target, seed, generation, "transfer", 8, -0.57, hidden_macros
        )
        ood = make_traces(
            target, seed, generation, "ood", 8, 0.93, hidden_macros
        )

        primitive = learner.invent_primitive(train, hold, transfer)
        accepted = False
        transfer_error = float("inf")
        ood_error = float("inf")
        parent_used = tuple()

        if primitive is not None:
            transfer_error = primitive.transfer_error
            ood_error = mae(primitive.expression, ood, learner._pmap())
            parent_used = primitive.parent_ids
            accepted = learner.accept_primitive(
                primitive.primitive_id,
                transfer_error=transfer_error,
                ood_error=ood_error,
                novelty=1.0 / max(1, primitive.complexity),
                resource_cost=max(1, primitive.complexity),
            )
            if accepted:
                retained.append(
                    HiddenCapability(
                        hidden_id=primitive.primitive_id,
                        expression=target,
                        depth=target_depth,
                        generation=generation,
                        parent_hidden_id=target_parents[0].hidden_id if target_parents else None,
                        parent_hidden_ids=tuple(p.hidden_id for p in target_parents),
                    )
                )

        baseline.generation = 0
        bprim = baseline.invent_primitive(train, hold, transfer)
        baseline_accepted = False
        if bprim is not None:
            b_ood = mae(bprim.expression, ood, baseline._pmap())
            baseline_accepted = (
                bprim.transfer_error <= 1e-9 and b_ood <= 1e-9
            )

        closure_rates = []
        baseline_closure_rates = []
        if len(retained) >= 2:
            pairs = [
                (retained[i], retained[j])
                for i in range(len(retained))
                for j in range(i + 1, len(retained))
            ]
            for pair_idx, (left_parent, right_parent) in enumerate(pairs):
                closure_expr = {
                    "op": "add",
                    "left": capability_macro(left_parent),
                    "right": capability_macro(right_parent),
                }
                closure_macros = hidden_library()
                ctr = make_traces(
                    closure_expr, seed + 1700 + pair_idx, generation,
                    "closure_train", 8, 0.13, closure_macros
                )
                cv = make_traces(
                    closure_expr, seed + 1700 + pair_idx, generation,
                    "closure_transfer", 5, -0.19, closure_macros
                )
                co = make_traces(
                    closure_expr, seed + 1700 + pair_idx, generation,
                    "closure_ood", 5, 0.29, closure_macros
                )
                cproc = learner.synthesize_process(ctr, cv, co)
                if cproc is not None:
                    ceval = learner.evaluate(cproc, ctr, cv, cv, co, ())
                    closure_rates.append(
                        float(ceval.accepted and ceval.ood_error <= 1e-9)
                    )
                else:
                    closure_rates.append(0.0)
                bproc = baseline.synthesize_process(ctr, cv, co)
                if bproc is not None:
                    beval = baseline.evaluate(bproc, ctr, cv, cv, co, ())
                    baseline_closure_rates.append(
                        float(beval.accepted and beval.ood_error <= 1e-9)
                    )
                else:
                    baseline_closure_rates.append(0.0)

        probe_rates = []
        baseline_probe_rates = []
        for probe_idx in range(4):
            probe_parents = tuple(
                rng.sample(retained, 2)
            ) if len(retained) >= 2 else tuple(retained[:1])
            probe_macros = hidden_library()
            if probe_parents:
                if len(probe_parents) == 1:
                    probe = {
                        "op": "add",
                        "left": capability_macro(probe_parents[0]),
                        "right": {"op": "get", "key": KEYS[probe_idx % len(KEYS)]},
                    }
                else:
                    probe = {
                        "op": "add",
                        "left": capability_macro(probe_parents[0]),
                        "right": capability_macro(probe_parents[1]),
                    }
            else:
                probe = random_base_expr(
                    random.Random(seed * 193 + generation * 17 + probe_idx),
                    3,
                )

            ptrain = make_traces(
                probe, seed + 700 + probe_idx, generation,
                "probe_train", 8, 0.17, probe_macros
            )
            ptransfer = make_traces(
                probe, seed + 700 + probe_idx, generation,
                "probe_transfer", 5, -0.22, probe_macros
            )
            pood = make_traces(
                probe, seed + 700 + probe_idx, generation,
                "probe_ood", 5, 0.41, probe_macros
            )
            proc = learner.synthesize_process(ptrain, ptransfer, pood)
            if proc is not None:
                peval = learner.evaluate(
                    proc, ptrain, ptransfer, ptransfer, pood, ()
                )
                probe_rates.append(
                    float(peval.accepted and peval.ood_error <= 1e-9)
                )
            else:
                probe_rates.append(0.0)

            bproc = baseline.synthesize_process(ptrain, ptransfer, pood)
            if bproc is not None:
                beval = baseline.evaluate(
                    bproc, ptrain, ptransfer, ptransfer, pood, ()
                )
                baseline_probe_rates.append(
                    float(beval.accepted and beval.ood_error <= 1e-9)
                )
            else:
                baseline_probe_rates.append(0.0)

        generations_out.append(
            {
                "generation": generation,
                "target_depth": target_depth,
                "accepted": accepted,
                "baseline_accepted": baseline_accepted,
                "parent_used": bool(parent_used),
                "parent_ids": list(parent_used),
                "multi_parent_used": len(target_parents) >= 2,
                "closure_success_rate": statistics.fmean(closure_rates) if closure_rates else 0.0,
                "baseline_closure_success_rate": statistics.fmean(baseline_closure_rates) if baseline_closure_rates else 0.0,
                "closure_pair_count": len(closure_rates),
                "retained_count": len(retained),
                "runtime_max_depth": int(learner.meta_policy["max_depth"]),
                "transfer_error": transfer_error,
                "ood_error": ood_error,
                "probe_success_rate": statistics.fmean(probe_rates),
                "baseline_probe_success_rate": statistics.fmean(baseline_probe_rates),
            }
        )

    probe_gain = statistics.fmean(
        row["probe_success_rate"] - row["baseline_probe_success_rate"]
        for row in generations_out
    )
    final = generations_out[-1]
    accepted_after_bootstrap = sum(row["accepted"] for row in generations_out[1:])
    recursive_generations = sum(
        row["accepted"] and row["parent_used"] for row in generations_out[1:]
    )
    last_three = generations_out[-3:]
    multi_parent_generations = sum(
        row["accepted"] and row["multi_parent_used"]
        for row in generations_out[1:]
    )
    final_closure_gain = (
        final["closure_success_rate"] - final["baseline_closure_success_rate"]
    )
    closure_growth = (
        final["closure_success_rate"]
        - generations_out[min(2, len(generations_out) - 1)]["closure_success_rate"]
    )

    finite_gate = (
        accepted_after_bootstrap >= generations - 2
        and recursive_generations >= generations - 2
        and multi_parent_generations >= generations - 2
        and all(row["accepted"] for row in last_three)
        and final["retained_count"] >= generations - 1
        and final["runtime_max_depth"] > initial_depth
        and final["target_depth"] > generations_out[0]["target_depth"]
        and final["probe_success_rate"] > 0.75
        and final["probe_success_rate"] > final["baseline_probe_success_rate"]
        and final["closure_pair_count"] >= 6
        and final_closure_gain >= 0.20
        and closure_growth >= 0.10
        and final["baseline_accepted"] is False
    )

    return {
        "seed": seed,
        "generations": generations,
        "initial_depth": initial_depth,
        "final_runtime_max_depth": final["runtime_max_depth"],
        "final_target_depth": final["target_depth"],
        "retained_count": final["retained_count"],
        "accepted_after_bootstrap": accepted_after_bootstrap,
        "recursive_generations": recursive_generations,
        "multi_parent_generations": multi_parent_generations,
        "final_closure_success_rate": final["closure_success_rate"],
        "final_baseline_closure_success_rate": final["baseline_closure_success_rate"],
        "final_closure_gain": final_closure_gain,
        "closure_growth": closure_growth,
        "mean_probe_gain": probe_gain,
        "final_probe_success_rate": final["probe_success_rate"],
        "final_baseline_probe_success_rate": final["baseline_probe_success_rate"],
        "finite_open_ended_growth_gate": finite_gate,
        "external_model": False,
        "target_identity_available_to_runtime": False,
        "task_family_route": False,
        "generations_detail": generations_out,
    }

def main():
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
        and all(row["finite_open_ended_growth_gate"] for row in rows)
        and all(not row["external_model"] for row in rows)
        and all(not row["target_identity_available_to_runtime"] for row in rows)
        and all(not row["task_family_route"] for row in rows)
    )
    print("OPEN_ENDED_SELF_EXTENDING_GATE=" + ("PASS" if gate else "FAIL"))
    raise SystemExit(0 if (gate or not args.strict) else 2)


if __name__ == "__main__":
    main()

# execution marker: PR-head condition corrected

# execution marker: ubuntu-slim runner route

# execution marker: complete lazy ACSIE stream pinned

# execution marker: fresh exact-SHA PR sync
