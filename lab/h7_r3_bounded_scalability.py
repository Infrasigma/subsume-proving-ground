#!/usr/bin/env python3
"""Bounded, synthetic H7 R3 stress ladder; never uses holdout/OOD data.

This is a diagnostic, not a scientific qualification gate. It deliberately
uses integer-valued inputs and non-integer targets so the exact target cannot
be synthesized from the bounded integer operator language at depth <= 2.
Stable search counters are compared across PYTHONHASHSEED values; time and
tracemalloc peaks are reported but excluded from the determinism comparison.
"""
from __future__ import annotations

import json
import math
import sys
import time
import tracemalloc
from typing import Any

from cognitive_core.behavioral_search import find_exact_expression
from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import (
    ArchiveRecord,
    CognitivePrimitive,
    Trace,
)


def run_case(n_primitives: int) -> dict[str, Any]:
    keys = tuple(f"x{i}" for i in range(n_primitives))
    compiler = OpenEndedRecursiveCognitiveCompiler(max_depth=2, seed=17)
    primitives = {}
    for i, key in enumerate(keys):
        pid = f"prim:scale:{i:02d}"
        primitive = CognitivePrimitive(
            pid, {"op": "get", "key": key}, (key,), "synthetic", 0, (),
            ("h7-r3-bounded-scale",), 0.0, 0.0, 0.0, 1,
            {"experiment": "bounded-scalability-diagnostic"},
        )
        primitives[pid] = primitive
    compiler.primitives = primitives
    compiler.archive = {
        pid: ArchiveRecord(primitive) for pid, primitive in primitives.items()
    }

    rows = tuple(
        Trace(
            {
                key: float((row + 1) * (column + 1)
                           + ((row * row + 3 * column) % 7))
                for column, key in enumerate(keys)
            },
            math.sqrt(2.0) / 7.0 + (row + 1) * math.pi,
            "synthetic",
            {},
            f"h7-r3-scale-n{n_primitives}-row{row}",
        )
        for row in range(8)
    )

    target_code = find_exact_expression.__code__
    captured: dict[str, Any] = {}
    def profiler(frame, event, arg):
        if frame.f_code is target_code and event == "return":
            local = frame.f_locals
            captured["stats"] = dict(local["stats"])
            captured["levels"] = local["levels"]
            captured["dominance_frontier"] = local["dominance_frontier"]
            captured["indexes"] = local["_H7_online_indexes"]
        return profiler

    old_profile = sys.getprofile()
    tracemalloc.start()
    started = time.perf_counter()
    sys.setprofile(profiler)
    try:
        result = find_exact_expression(
            compiler, rows, max_depth=2, require_macro=True,
        )
    finally:
        sys.setprofile(old_profile)
        elapsed = time.perf_counter() - started
        _, peak_bytes = tracemalloc.get_traced_memory()
        tracemalloc.stop()

    if result is not None:
        raise AssertionError(
            "diagnostic target was unexpectedly synthesized; benchmark invalid"
        )
    if not captured:
        raise AssertionError("search counters were not captured")

    stats = captured["stats"]
    levels = captured["levels"]
    frontier = captured["dominance_frontier"]
    indexes = captured["indexes"]
    level_tokens = {
        state[5] for bucket in levels.values() for state in bucket.values()
    }
    frontier_tokens = {
        state[5] for states in frontier.values() for state in states
    }
    indexed_tokens = {
        token for index in indexes.values() for token in index["active"]
    }
    identity_consistent = (
        level_tokens == frontier_tokens == indexed_tokens
    )
    if not identity_consistent:
        raise AssertionError(
            "active state identity differs across levels/frontier/index"
        )

    stable = {
        "n_primitives": n_primitives,
        "n_rows": len(rows),
        "max_depth": 2,
        "target_found": False,
        "status": str(stats.get("status")),
        "generated_expressions": int(stats.get("generated_expressions", 0)),
        "unique_states": int(stats.get("unique_states", 0)),
        "duplicate_states": int(stats.get("duplicate_states", 0)),
        "dominance_pruned_states": int(stats.get("dominance_pruned_states", 0)),
        "dominance_frontier_max": int(stats.get("dominance_frontier_max", 0)),
        "generic_binary_pairs_evaluated": int(
            stats.get("generic_binary_pairs_evaluated", 0)
        ),
        "target_candidate_count": int(stats.get("target_candidate_count", 0)),
        "levels_by_depth": {
            str(depth): len(bucket)
            for depth, bucket in sorted(levels.items())
        },
        "active_frontier_states": len(frontier_tokens),
        "indexed_states": len(indexed_tokens),
        "active_identity_consistent": identity_consistent,
    }
    performance = {
        "elapsed_ms": round(elapsed * 1000.0, 3),
        "tracemalloc_peak_bytes": int(peak_bytes),
    }
    record = {"stable": stable, "performance": performance}
    print(json.dumps(record, sort_keys=True, separators=(",", ":")), flush=True)

    # Hard safety guards only; these do not claim that passing is scalable
    # enough for open-ended search.
    if elapsed > 20.0:
        raise TimeoutError(
            f"bounded case n={n_primitives} exceeded 20-second guard"
        )
    if peak_bytes > 512 * 1024 * 1024:
        raise MemoryError(
            f"bounded case n={n_primitives} exceeded 512 MiB traced-memory guard"
        )
    if stable["generic_binary_pairs_evaluated"] <= 0:
        raise AssertionError("stress case did not reach generic binary enumeration")
    return record


def main() -> None:
    for n in (2, 4, 6, 8):
        run_case(n)


if __name__ == "__main__":
    main()
