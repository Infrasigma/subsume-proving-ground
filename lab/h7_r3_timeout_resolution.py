#!/usr/bin/env python3
"""Re-run the H7 12-primitive R2/R3 timeout fixture with an extended diagnostic ceiling.

This is a bounded synthetic diagnostic, not a qualification or an open-ended
scalability claim. The historical 15-second gate is preserved as a reported
outcome; an extended ceiling only adjudicates the prior 17-second wrapper
timeout and never converts an over-budget result into a pass.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import resource
import time
import traceback
from typing import Any, Mapping

from cognitive_core.behavioral_search import find_exact_expression
from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import (
    ArchiveRecord,
    CognitivePrimitive,
    Trace,
    ast_depth,
    ast_nodes,
)


def add_primitive(compiler: OpenEndedRecursiveCognitiveCompiler, primitive: CognitivePrimitive) -> None:
    compiler.primitives[primitive.primitive_id] = primitive
    compiler.archive[primitive.primitive_id] = ArchiveRecord(primitive)


def expanded_cost(
    expression: Mapping[str, Any],
    primitives: Mapping[str, CognitivePrimitive],
    active: tuple[str, ...] = (),
) -> tuple[int, int]:
    op = expression["op"]
    if op == "macro":
        pid = str(expression["id"])
        if pid in active or pid not in primitives:
            raise ValueError(f"invalid/cyclic macro edge: {pid}")
        return expanded_cost(primitives[pid].expression, primitives, active + (pid,))
    if op in {"get", "const"}:
        return (1, 1)
    if op in {"abs", "neg", "threshold"}:
        depth, nodes = expanded_cost(expression["arg"], primitives, active)
        return (depth + 1, nodes + 1)
    left = expanded_cost(expression["left"], primitives, active)
    right = expanded_cost(expression["right"], primitives, active)
    return (1 + max(left[0], right[0]), 1 + left[1] + right[1])


def free_keys(
    expression: Mapping[str, Any],
    primitives: Mapping[str, CognitivePrimitive],
    active: tuple[str, ...] = (),
) -> tuple[str, ...]:
    op = expression["op"]
    if op == "get":
        return (str(expression["key"]),)
    if op == "const":
        return ()
    if op == "macro":
        pid = str(expression["id"])
        if pid in active or pid not in primitives:
            raise ValueError(f"invalid/cyclic macro edge: {pid}")
        return free_keys(primitives[pid].expression, primitives, active + (pid,))
    if op in {"abs", "neg", "threshold"}:
        return free_keys(expression["arg"], primitives, active)
    return tuple(sorted(set(free_keys(expression["left"], primitives, active)) |
                        set(free_keys(expression["right"], primitives, active))))


def make_rows(xs: tuple[float, ...]) -> tuple[Trace, ...]:
    return tuple(
        Trace(
            {"x": x, "y": (-2.0 * x if x < 0.0 else 0.0)},
            (-3.0 * x if x < 0.0 else x),
            "opaque", {}, f"h7-alias-scale-{index}",
        )
        for index, x in enumerate(xs)
    )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", required=True, choices=("r2", "r3"))
    parser.add_argument("--seed", type=int, required=True)
    parser.add_argument("--pair", type=int, required=True, choices=(1, 4))
    parser.add_argument("--alias-pairs", type=int, default=5)
    parser.add_argument("--original-guard-seconds", type=float, default=15.0)
    parser.add_argument("--diagnostic-budget-seconds", type=float, default=120.0)
    args = parser.parse_args()

    compiler = OpenEndedRecursiveCognitiveCompiler(max_depth=2, seed=17)
    p_a = CognitivePrimitive(
        "prim:a",
        {"op": "neg", "arg": {"op": "neg", "arg": {"op": "neg", "arg": {"op": "neg", "arg": {"op": "get", "key": "x"}}}}},
        ("x",), "delta", 0, (), ("opaque",), 0.0, 0.0, 0.0, 5,
        {"origin": "fixed-synthetic-base"},
    )
    p_b = CognitivePrimitive(
        "prim:b",
        {
            "op": "add",
            "left": {"op": "mul", "left": {"op": "get", "key": "y"}, "right": {"op": "const", "value": 1.0}},
            "right": {"op": "mul", "left": {"op": "get", "key": "y"}, "right": {"op": "const", "value": 0.0}},
        },
        ("y",), "delta", 0, (), ("opaque",), 0.0, 0.0, 0.0, 7,
        {"origin": "fixed-synthetic-base"},
    )
    add_primitive(compiler, p_a)
    add_primitive(compiler, p_b)

    previous_a, previous_b = p_a.primitive_id, p_b.primitive_id
    for index in range(1, args.alias_pairs + 1):
        aid = f"prim:alias:a:{index:02d}"
        aexpr = {"op": "neg", "arg": {"op": "neg", "arg": {"op": "macro", "id": previous_a}}}
        add_primitive(compiler, CognitivePrimitive(
            aid, aexpr, ("x",), "delta", 0, (previous_a,), ("opaque",),
            0.0, 0.0, 0.0, ast_nodes(aexpr),
            {"origin": "bounded-redundant-executable-alias", "alias_index": index},
        ))
        previous_a = aid

        bid = f"prim:alias:b:{index:02d}"
        bexpr = {"op": "neg", "arg": {"op": "neg", "arg": {"op": "macro", "id": previous_b}}}
        add_primitive(compiler, CognitivePrimitive(
            bid, bexpr, ("y",), "delta", 0, (previous_b,), ("opaque",),
            0.0, 0.0, 0.0, ast_nodes(bexpr),
            {"origin": "bounded-redundant-executable-alias", "alias_index": index},
        ))
        previous_b = bid

    xs = (-4.0, -3.0, -2.0, -1.0, 1.0, 2.0, 3.0, 4.0)
    rows = make_rows(xs)
    target = tuple(row.target for row in rows)
    started = time.perf_counter()
    status = "SEARCH_EXCEPTION"
    error = None
    stable: dict[str, Any] = {
        "schema": "ACSIE.H7_R3.timeout_resolution.v1",
        "source": args.source,
        "source_commit": os.environ.get("ACSIE_SOURCE_COMMIT", "UNSET"),
        "hash_seed": args.seed,
        "order_pair": args.pair,
        "primitive_count": len(compiler.primitives),
        "alias_pairs": args.alias_pairs,
        "rows": len(rows),
        "max_depth": 3,
        "target_found": False,
        "selected_expression": None,
        "selected_used_primitives": [],
        "returned_alternative_count": 0,
        "returned_base_pair_profiles": [],
        "expected_profile_7_14_returned": False,
        "search_stats": {},
    }
    try:
        result = find_exact_expression(
            compiler, rows, max_depth=3, require_macro=True, max_alternatives=8,
        )
        if result is None:
            status = "TARGET_NOT_FOUND"
        else:
            alternatives = []
            base_pair_profiles = set()
            for expression, used in result.alternatives:
                cost = expanded_cost(expression, compiler.primitives)
                inputs = free_keys(expression, compiler.primitives)
                row = {
                    "expression": expression,
                    "used_primitives": list(used),
                    "expanded_cost": list(cost),
                    "ast_depth": ast_depth(expression),
                    "free_inputs": list(inputs),
                }
                alternatives.append(row)
                if set(used) == {"prim:a", "prim:b"} and inputs == ("x", "y") and ast_depth(expression) == 3:
                    base_pair_profiles.add(cost)
            stable.update({
                "target_found": True,
                "selected_expression": result.expression,
                "selected_used_primitives": list(result.used_primitives),
                "returned_alternative_count": len(result.alternatives),
                "returned_base_pair_profiles": [list(x) for x in sorted(base_pair_profiles)],
                "expected_profile_7_14_returned": (7, 14) in base_pair_profiles,
                "alternatives": alternatives,
                "search_stats": dict(result.stats),
            })
            status = "TARGET_FOUND"
    except BaseException as exc:
        error = f"{type(exc).__name__}: {exc}"
        stable["error"] = error
        stable["traceback"] = traceback.format_exc()

    elapsed_ms = (time.perf_counter() - started) * 1000.0
    rss_mb = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024.0
    original_pass = elapsed_ms <= args.original_guard_seconds * 1000.0
    diagnostic_pass = elapsed_ms <= args.diagnostic_budget_seconds * 1000.0 and rss_mb <= 2048.0
    stable["original_15s_gate_pass"] = original_pass
    stable["diagnostic_120s_gate_pass"] = diagnostic_pass
    stable["rss_gate_pass"] = rss_mb <= 2048.0
    stable["error"] = error

    signature_payload = dict(stable)
    for key in ("hash_seed", "order_pair", "original_15s_gate_pass",
                "diagnostic_120s_gate_pass", "rss_gate_pass"):
        signature_payload.pop(key, None)
    signature = hashlib.sha256(
        json.dumps(signature_payload, sort_keys=True, separators=(",", ":")).encode()
    ).hexdigest()
    record = {
        "schema": "ACSIE.H7_R3.timeout_resolution_record.v1",
        "status": status,
        "stable": stable,
        "stable_signature_sha256": signature,
        "performance": {
            "search_elapsed_ms": round(elapsed_ms, 3),
            "process_peak_rss_mb": round(rss_mb, 3),
            "instrumentation": "none within find_exact_expression",
            "original_gate_seconds": args.original_guard_seconds,
            "extended_diagnostic_budget_seconds": args.diagnostic_budget_seconds,
        },
        "interpretation_guard": (
            "12-primitive synthetic diagnostic only. The extended budget adjudicates the old "
            "17-second wrapper timeout; an overrun remains a failure of its named budget."
        ),
    }
    print(json.dumps(record, sort_keys=True, separators=(",", ":")), flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
