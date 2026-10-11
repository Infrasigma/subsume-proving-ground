#!/usr/bin/env python3
"""Uninstrumented, single-search H7 R2/R3 timing probe.

Unlike the profiling harness, this script does not install sys.setprofile or
cProfile. It times only find_exact_expression after constructing the fixed
synthetic 2/4/6/8-primitive alias fixture. The workflow counterbalances R2/R3
execution order and repeats each cell under two hash seeds. Timing is still a
bounded diagnostic, not a broad performance qualification.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import resource
import time
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


def add_primitive(c: OpenEndedRecursiveCognitiveCompiler, p: CognitivePrimitive) -> None:
    c.primitives[p.primitive_id] = p
    c.archive[p.primitive_id] = ArchiveRecord(p)


def make_compiler(alias_pairs: int) -> OpenEndedRecursiveCognitiveCompiler:
    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2, seed=17)
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
    add_primitive(c, p_a)
    add_primitive(c, p_b)
    previous_a, previous_b = p_a.primitive_id, p_b.primitive_id
    for index in range(1, alias_pairs + 1):
        aid = f"prim:alias:a:{index:02d}"
        aexpr = {"op": "neg", "arg": {"op": "neg", "arg": {"op": "macro", "id": previous_a}}}
        add_primitive(c, CognitivePrimitive(
            aid, aexpr, ("x",), "delta", 0, (previous_a,), ("opaque",),
            0.0, 0.0, 0.0, ast_nodes(aexpr),
            {"origin": "bounded-redundant-executable-alias", "alias_index": index},
        ))
        previous_a = aid
        bid = f"prim:alias:b:{index:02d}"
        bexpr = {"op": "neg", "arg": {"op": "neg", "arg": {"op": "macro", "id": previous_b}}}
        add_primitive(c, CognitivePrimitive(
            bid, bexpr, ("y",), "delta", 0, (previous_b,), ("opaque",),
            0.0, 0.0, 0.0, ast_nodes(bexpr),
            {"origin": "bounded-redundant-executable-alias", "alias_index": index},
        ))
        previous_b = bid
    return c


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", choices=("r2", "r3"), required=True)
    parser.add_argument("--seed", type=int, required=True)
    parser.add_argument("--alias-pairs", choices=(0, 1, 2, 3), type=int, required=True)
    parser.add_argument("--pair", type=int, required=True)
    args = parser.parse_args()

    rows = tuple(
        Trace(
            {"x": x, "y": (-2.0 * x if x < 0.0 else 0.0)},
            (-3.0 * x if x < 0.0 else x),
            "opaque", {}, f"h7-uninstrumented-{i}",
        )
        for i, x in enumerate((-4.0, -3.0, -2.0, -1.0, 1.0, 2.0, 3.0, 4.0))
    )
    compiler = make_compiler(args.alias_pairs)
    start = time.perf_counter()
    result = find_exact_expression(
        compiler, rows, max_depth=3, require_macro=True, max_alternatives=8,
    )
    elapsed_ms = (time.perf_counter() - start) * 1000.0
    peak_rss_mb = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024.0
    if result is None:
        raise AssertionError("exact target was not found")

    returned_profiles = sorted({
        expanded_cost(expr, compiler.primitives)
        for expr, used in result.alternatives
        if set(used) == {"prim:a", "prim:b"}
        and ast_depth(expr) == 3
    })
    expected_profile_returned = (7, 14) in returned_profiles
    if args.source == "r2" and expected_profile_returned:
        raise AssertionError("R2 negative control unexpectedly retained (7,14)")
    if args.source == "r3" and not expected_profile_returned:
        raise AssertionError("R3 lost preserved (7,14) profile")

    stable = {
        "source": args.source,
        "hash_seed": args.seed,
        "order_pair": args.pair,
        "alias_pairs": args.alias_pairs,
        "primitive_count": len(compiler.primitives),
        "rows": len(rows),
        "max_depth": 3,
        "target_found": True,
        "returned_alternative_count": len(result.alternatives),
        "returned_base_pair_profiles": [list(x) for x in returned_profiles],
        "expected_profile_7_14_returned": expected_profile_returned,
        "selected_expression": result.expression,
        "selected_used_primitives": list(result.used_primitives),
        "search_stats": dict(result.stats),
    }
    signature_payload = {key: value for key, value in stable.items()
                         if key not in {"source", "hash_seed", "order_pair"}}
    signature = hashlib.sha256(
        json.dumps(signature_payload, sort_keys=True, separators=(",", ":")).encode()
    ).hexdigest()
    row = {
        "stable": stable,
        "stable_signature_sha256": signature,
        "performance": {
            "search_elapsed_ms": round(elapsed_ms, 3),
            "process_peak_rss_mb": round(peak_rss_mb, 3),
        },
        "instrumentation": "none within find_exact_expression (no cProfile/sys.setprofile)",
    }
    print(json.dumps(row, sort_keys=True, separators=(",", ":")), flush=True)
    if elapsed_ms > 15000:
        raise TimeoutError("15-second per-search guard exceeded")
    if peak_rss_mb > 2048:
        raise MemoryError("2-GiB per-process RSS guard exceeded")


if __name__ == "__main__":
    main()
