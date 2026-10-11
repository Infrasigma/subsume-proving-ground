#!/usr/bin/env python3
"""Bounded H7 R3 representative-count benchmark on the exact pinned source.

Synthetic-only, depth-2 workloads: 16/24/32 primitive aliases, ten rows,
one reachable exact composition and one guaranteed unreachable target.
Three timing trials under hash seed 0 and one repeat under seed 731. This is
not Gen5/Gen6 or frozen qualification.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import math
import random
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
)

TARGET_FN = find_exact_expression.__code__


def run_case(n: int, task: str, trial: int) -> dict[str, Any]:
    keys = ("x", "y", "z")
    compiler = OpenEndedRecursiveCognitiveCompiler(max_depth=2, seed=17)
    primitives = {}
    for i in range(n):
        pid = f"prim:scale:{i:02d}"
        key = keys[i % len(keys)]
        primitive = CognitivePrimitive(
            pid, {"op": "get", "key": key}, (key,), "synthetic", 0, (),
            ("h7-r3-representative-scale",), 0.0, 0.0, 0.0, 1,
            {"experiment": "bounded-representative-count"},
        )
        primitives[pid] = primitive
    compiler.primitives = primitives
    compiler.archive = {
        pid: ArchiveRecord(primitive) for pid, primitive in primitives.items()
    }

    rng = random.Random(99173 + n)
    rows = []
    for i in range(10):
        inputs = {key: rng.uniform(-3.0, 3.0) for key in keys}
        if task == "reachable_add":
            target = inputs["x"] + inputs["y"]
        else:
            # At max_depth=2 the language combines only leaves/constants:
            # with |inputs| <= 3 and constants in [-2,4], magnitude is bounded
            # far below these targets. The miss is a designed negative control.
            target = 1000.0 + 100.0 * i
        rows.append(
            Trace(inputs, float(target), "synthetic", {}, f"scale-{n}-{task}-{i}")
        )
    discovery_rows = tuple(rows)

    captured: dict[str, Any] = {}
    def profiler(frame, event, arg):
        if frame.f_code is TARGET_FN and event == "return":
            local = frame.f_locals
            captured["stats"] = dict(local["stats"])
            captured["levels"] = local["levels"]
            captured["frontier"] = local["dominance_frontier"]
            captured["indexes"] = local["_H7_online_indexes"]
        return profiler

    old_profile = sys.getprofile()
    started = time.perf_counter()
    sys.setprofile(profiler)
    try:
        result = find_exact_expression(
            compiler, discovery_rows, max_depth=2, require_macro=True,
        )
    finally:
        sys.setprofile(old_profile)
        elapsed_ms = (time.perf_counter() - started) * 1000.0

    if not captured:
        raise AssertionError("failed to capture exact search counters")
    if task == "reachable_add" and result is None:
        raise AssertionError(f"reachable control not found at n={n}")
    if task == "unreachable" and result is not None:
        raise AssertionError(f"unreachable negative control unexpectedly found at n={n}")

    stats = result.stats if result is not None else captured["stats"]
    levels = captured["levels"]
    frontier = captured["frontier"]
    indexes = captured["indexes"]
    level_tokens = {s[5] for bucket in levels.values() for s in bucket.values()}
    frontier_tokens = {s[5] for states in frontier.values() for s in states}
    index_tokens = {token for index in indexes.values() for token in index["active"]}
    identity_ok = level_tokens == frontier_tokens == index_tokens
    if not identity_ok:
        raise AssertionError(f"active frontier identity mismatch at n={n}, task={task}")

    payload = {
        "result": None if result is None else {
            "expression": result.expression,
            "used_primitives": list(result.used_primitives),
            "alternatives": [
                {"expression": expression, "used_primitives": list(used)}
                for expression, used in result.alternatives
            ],
        },
        "stats": stats,
        "levels_by_depth": {str(d): len(b) for d, b in sorted(levels.items())},
        "level_state_count": len(level_tokens),
        "frontier_state_count": len(frontier_tokens),
        "indexed_state_count": len(index_tokens),
        "active_identity_consistent": identity_ok,
    }
    serialized = json.dumps(payload, sort_keys=True, separators=(",", ":"))
    signature = hashlib.sha256(serialized.encode()).hexdigest()
    rss_mb = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024.0
    stable = {
        "n_primitives": n,
        "task": task,
        "trial": trial,
        "result_found": result is not None,
        "signature_sha256": signature,
        "status": str(stats.get("status")),
        "generated_expressions": int(stats.get("generated_expressions", 0)),
        "unique_states": int(stats.get("unique_states", 0)),
        "duplicate_states": int(stats.get("duplicate_states", 0)),
        "dominance_pruned_states": int(stats.get("dominance_pruned_states", 0)),
        "dominance_frontier_max": int(stats.get("dominance_frontier_max", 0)),
        "generic_binary_pairs_evaluated": int(stats.get("generic_binary_pairs_evaluated", 0)),
        "levels_by_depth": payload["levels_by_depth"],
        "active_frontier_states": len(frontier_tokens),
        "indexed_states": len(index_tokens),
        "active_identity_consistent": identity_ok,
    }
    record = {
        "stable": stable,
        "performance": {
            "elapsed_ms": round(elapsed_ms, 3),
            "process_peak_rss_mb": round(rss_mb, 3),
        },
    }
    print(json.dumps(record, sort_keys=True, separators=(",", ":")), flush=True)
    if elapsed_ms > 15000.0:
        raise TimeoutError(f"15-second search guard exceeded at n={n}, task={task}")
    if rss_mb > 2048.0:
        raise MemoryError(f"2-GiB RSS guard exceeded at n={n}, task={task}")
    return record


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repeats", type=int, default=3)
    args = parser.parse_args()
    if args.repeats not in (1, 3):
        raise SystemExit("--repeats must be 1 or 3")
    for n in (16, 24, 32):
        for task in ("reachable_add", "unreachable"):
            for trial in range(1, args.repeats + 1):
                run_case(n, task, trial)


if __name__ == "__main__":
    main()
