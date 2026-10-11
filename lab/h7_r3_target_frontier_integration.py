#!/usr/bin/env python3
"""Exercise H7 R2/R3 Pareto-frontier behavior through natural target reconstruction.

Reuses the exact tiny synthetic fixture from the R3 regression test; unlike the
unit negative control, this asserts on the alternatives returned by the normal
find_exact_expression call and does not mutate/register extra states after the
call. No hidden target identity reaches the runtime beyond the explicit
synthetic discovery rows.
"""
from __future__ import annotations

import hashlib
import json
import os
import resource
import sys
import time
from typing import Any

from cognitive_core.behavioral_search import find_exact_expression
from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import (
    ArchiveRecord,
    CognitivePrimitive,
    Trace,
    ast_depth,
)


def main() -> None:
    seed = int(os.environ.get("PYTHONHASHSEED", "0"))
    source_label = os.environ.get("ACSIE_SOURCE_LABEL", "unspecified")
    c = OpenEndedRecursiveCognitiveCompiler(max_depth=3, seed=17)
    p_a = CognitivePrimitive(
        "prim:a",
        {"op": "neg", "arg": {"op": "neg", "arg": {"op": "neg", "arg": {"op": "neg", "arg": {"op": "get", "key": "x"}}}}},
        ("x",), "delta", 0, (), ("opaque",), 0.0, 0.0, 0.0, 5, {},
    )
    p_b = CognitivePrimitive(
        "prim:b",
        {
            "op": "add",
            "left": {
                "op": "mul",
                "left": {"op": "get", "key": "y"},
                "right": {"op": "const", "value": 1.0},
            },
            "right": {
                "op": "mul",
                "left": {"op": "get", "key": "y"},
                "right": {"op": "const", "value": 0.0},
            },
        },
        ("y",), "delta", 0, (), ("opaque",), 0.0, 0.0, 0.0, 7, {},
    )
    c.primitives = {p.primitive_id: p for p in (p_a, p_b)}
    c.archive = {pid: ArchiveRecord(p) for pid, p in c.primitives.items()}
    xs = (-4.0, -3.0, -2.0, -1.0, 1.0, 2.0, 3.0, 4.0)
    rows = tuple(
        Trace(
            {"x": x, "y": (-2.0 * x if x < 0.0 else 0.0)},
            (-3.0 * x if x < 0.0 else x),
            "opaque", {}, f"cost-tradeoff-{i}",
        )
        for i, x in enumerate(xs)
    )
    target = tuple(row.target for row in rows)
    target_fn = find_exact_expression.__code__
    captured: dict[str, Any] = {}

    def profiler(frame, event, arg):
        if frame.f_code is target_fn and event == "return":
            captured.update(frame.f_locals)
        return profiler

    previous_profiler = sys.getprofile()
    start = time.perf_counter()
    sys.setprofile(profiler)
    try:
        result = find_exact_expression(c, rows, max_depth=3, require_macro=True)
    finally:
        sys.setprofile(previous_profiler)
    elapsed_ms = (time.perf_counter() - start) * 1000.0
    rss_mb = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024.0

    if result is None or not captured:
        raise AssertionError("synthetic target search did not return a result")
    cost = captured["executable_cost"]
    free_keys = captured["free_keys"]
    levels = captured["levels"]

    def is_pair_profile(expr, used) -> bool:
        return (
            set(used) == {"prim:a", "prim:b"}
            and tuple(free_keys(expr)) == ("x", "y")
            and ast_depth(expr) == 3
        )

    natural_level_profiles = sorted({
        tuple(cost(state[0]))
        for bucket in levels.values()
        for state in bucket.values()
        if state[1] == target
        and set(state[2]) == {"prim:a", "prim:b"}
        and state[3] == ("x", "y")
        and ast_depth(state[0]) == 3
    })
    returned_profiles = sorted({
        tuple(cost(expr))
        for expr, used in result.alternatives
        if is_pair_profile(expr, used)
    })
    alternatives_serialized = sorted(
        json.dumps(
            {"expr": expr, "used": list(used), "cost": list(cost(expr))},
            sort_keys=True, separators=(",", ":"),
        )
        for expr, used in result.alternatives
    )
    payload = {
        "source_label": source_label,
        "seed": seed,
        "target_found": True,
        "status": str(result.stats.get("status")),
        "natural_level_profiles": [list(x) for x in natural_level_profiles],
        "returned_alternative_profiles": [list(x) for x in returned_profiles],
        "returned_alternative_count": len(result.alternatives),
        "stats": dict(result.stats),
        "selected_cost": list(cost(result.expression)),
        "selected_used_primitives": list(result.used_primitives),
        "alternatives": alternatives_serialized,
    }
    encoded = json.dumps(payload, sort_keys=True, separators=(",", ":"))
    output = {
        **payload,
        "result_stats_sha256": hashlib.sha256(encoded.encode()).hexdigest(),
        "performance": {
            "elapsed_ms": round(elapsed_ms, 3),
            "process_peak_rss_mb": round(rss_mb, 3),
        },
    }
    print(json.dumps(output, sort_keys=True, separators=(",", ":")), flush=True)


if __name__ == "__main__":
    main()
