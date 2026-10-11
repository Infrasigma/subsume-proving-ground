#!/usr/bin/env python3
"""Profile the exact H7 R2/R3 hot path on the same bounded alias-pressure case.

Only find_exact_expression is under cProfile. The workload is the fixed
synthetic two-primitive/eight-row fixture plus three executable alias pairs
(n=8 total primitives). Profiled timings are diagnostic, not performance
qualification; result/counter identities are checked independently.
"""
from __future__ import annotations

import argparse
import cProfile
import hashlib
import json
import os
import pstats
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


def add_primitive(
    compiler: OpenEndedRecursiveCognitiveCompiler,
    primitive: CognitivePrimitive,
) -> None:
    compiler.primitives[primitive.primitive_id] = primitive
    compiler.archive[primitive.primitive_id] = ArchiveRecord(primitive)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", required=True, choices=("r2", "r3"))
    parser.add_argument("--seed", type=int, required=True)
    parser.add_argument("--alias-pairs", type=int, choices=(3,), default=3)
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
        a = CognitivePrimitive(
            aid, aexpr, ("x",), "delta", 0, (previous_a,), ("opaque",),
            0.0, 0.0, 0.0, ast_nodes(aexpr),
            {"origin": "bounded-redundant-executable-alias", "alias_index": index},
        )
        add_primitive(compiler, a)
        previous_a = aid

        bid = f"prim:alias:b:{index:02d}"
        bexpr = {"op": "neg", "arg": {"op": "neg", "arg": {"op": "macro", "id": previous_b}}}
        b = CognitivePrimitive(
            bid, bexpr, ("y",), "delta", 0, (previous_b,), ("opaque",),
            0.0, 0.0, 0.0, ast_nodes(bexpr),
            {"origin": "bounded-redundant-executable-alias", "alias_index": index},
        )
        add_primitive(compiler, b)
        previous_b = bid

    rows = tuple(
        Trace(
            {"x": x, "y": (-2.0 * x if x < 0.0 else 0.0)},
            (-3.0 * x if x < 0.0 else x),
            "opaque", {}, f"h7-profile-{i}",
        )
        for i, x in enumerate((-4.0, -3.0, -2.0, -1.0, 1.0, 2.0, 3.0, 4.0))
    )

    profiler = cProfile.Profile()
    started = time.perf_counter()
    profiler.enable()
    try:
        result = find_exact_expression(
            compiler, rows, max_depth=3, require_macro=True, max_alternatives=8,
        )
    finally:
        profiler.disable()
    wall_ms = (time.perf_counter() - started) * 1000.0
    peak_rss_mb = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024.0
    if result is None:
        raise AssertionError("exact target was not found")

    returned_profiles = sorted({
        expanded_cost(expression, compiler.primitives)
        for expression, used in result.alternatives
        if set(used) == {"prim:a", "prim:b"}
        and ast_depth(expression) == 3
    })
    expected_profile = (7, 14)
    expected_returned = list(expected_profile) in [list(x) for x in returned_profiles]
    if args.source == "r2" and expected_returned:
        raise AssertionError("R2 negative control unexpectedly retained profile (7,14)")
    if args.source == "r3" and not expected_returned:
        raise AssertionError("R3 lost the expected (7,14) alternative")

    stats = pstats.Stats(profiler)
    hot_functions = []
    for (filename, line_number, function_name), value in stats.stats.items():
        if filename.endswith("/cognitive_core/behavioral_search.py"):
            primitive_calls, total_calls, self_seconds, cumulative_seconds, _ = value
            hot_functions.append({
                "function": function_name,
                "line": line_number,
                "primitive_calls": primitive_calls,
                "total_calls": total_calls,
                "self_seconds": round(self_seconds, 6),
                "cumulative_seconds": round(cumulative_seconds, 6),
            })
    hot_functions.sort(
        key=lambda item: (item["cumulative_seconds"], item["self_seconds"]),
        reverse=True,
    )
    stable = {
        "source": args.source,
        "hash_seed": args.seed,
        "source_label": os.environ.get("ACSIE_SOURCE_LABEL", args.source),
        "primitive_count": len(compiler.primitives),
        "alias_pairs": args.alias_pairs,
        "rows": len(rows),
        "max_depth": 3,
        "target_found": True,
        "returned_alternatives": len(result.alternatives),
        "returned_base_pair_profiles": [list(x) for x in returned_profiles],
        "expected_profile_7_14_returned": expected_returned,
        "selected_used_primitives": list(result.used_primitives),
        "selected_expression": result.expression,
        "search_stats": dict(result.stats),
        "hot_functions": hot_functions,
        "profile_total_seconds": round(stats.total_tt, 6),
        "profile_total_calls": stats.total_calls,
    }
    deterministic_keys = (
        "primitive_count",
        "alias_pairs",
        "rows",
        "max_depth",
        "target_found",
        "returned_alternatives",
        "returned_base_pair_profiles",
        "expected_profile_7_14_returned",
        "selected_used_primitives",
        "selected_expression",
        "search_stats",
    )
    signature_payload = {key: stable[key] for key in deterministic_keys}
    signature = hashlib.sha256(
        json.dumps(signature_payload, sort_keys=True, separators=(",", ":")).encode()
    ).hexdigest()
    output = {
        "stable": stable,
        "signature_sha256": signature,
        "performance": {
            "unprofiled_wrapper_wall_ms_includes_cprofile": round(wall_ms, 3),
            "process_peak_rss_mb": round(peak_rss_mb, 3),
        },
        "note": "cProfile timings are diagnostic and are not a baseline performance qualification.",
    }
    print(json.dumps(output, sort_keys=True, separators=(",", ":")), flush=True)
    if wall_ms > 30000.0:
        raise TimeoutError("30-second wall guard exceeded for profiled search")
    if peak_rss_mb > 2048.0:
        raise MemoryError("2-GiB process RSS guard exceeded")


if __name__ == "__main__":
    main()
