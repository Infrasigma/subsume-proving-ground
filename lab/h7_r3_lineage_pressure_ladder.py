#!/usr/bin/env python3
"""Bounded H7 lineage-frontier pressure ladder for exact R2/R3 sources.

A fixed, human-authored synthetic fixture adds 0..3 redundant but executable
macro aliases per base primitive. Every workload uses the same discovery rows
and target; aliases only vary executable ancestry/expanded-cost profile. The
script reports rather than suppresses any target-alternative truncation.
No hidden target identity or task-family routing is used by the search.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import resource
import sys
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


def make_rows(xs: tuple[float, ...]) -> tuple[Trace, ...]:
    return tuple(
        Trace(
            {"x": x, "y": (-2.0 * x if x < 0.0 else 0.0)},
            (-3.0 * x if x < 0.0 else x),
            "opaque", {}, f"h7-alias-scale-{index}",
        )
        for index, x in enumerate(xs)
    )


def run_case(alias_pairs: int, seed: int, source_label: str) -> dict[str, Any]:
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
    for index in range(1, alias_pairs + 1):
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

    xs = (-4.0, -3.0, -2.0, -1.0, 1.0, 2.0, 3.0, 4.0)
    rows = make_rows(xs)
    target = tuple(row.target for row in rows)
    source_fn = find_exact_expression.__code__
    captured: dict[str, Any] = {}

    def profiler(frame, event, arg):
        if frame.f_code is source_fn and event == "return":
            captured.update(frame.f_locals)
        return profiler

    previous_profiler = sys.getprofile()
    started = time.perf_counter()
    sys.setprofile(profiler)
    try:
        result = find_exact_expression(
            compiler, rows, max_depth=3, require_macro=True, max_alternatives=8,
        )
    finally:
        sys.setprofile(previous_profiler)
    elapsed_ms = (time.perf_counter() - started) * 1000.0
    peak_rss_mb = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024.0

    if result is None or not captured:
        raise AssertionError(f"{source_label}: no exact target result, aliases={alias_pairs}")
    levels = captured["levels"]
    frontier = captured["dominance_frontier"]
    indexes = captured["_H7_online_indexes"]
    live_states = [state for bucket in levels.values() for state in bucket.values()]
    live_tokens = {state[5] for state in live_states}
    frontier_tokens = {state[5] for states in frontier.values() for state in states}
    indexed_tokens = {token for index in indexes.values() for token in index["active"]}
    identity_consistent = live_tokens == frontier_tokens == indexed_tokens
    if not identity_consistent:
        raise AssertionError(
            f"{source_label}: level/frontier/index identities diverged, aliases={alias_pairs}"
        )

    base_pair_profile = (7, 14)
    live_base_pair_profiles = sorted({
        expanded_cost(state[0], compiler.primitives)
        for state in live_states
        if state[1] == target
        and set(state[2]) == {"prim:a", "prim:b"}
        and state[3] == ("x", "y")
        and ast_depth(state[0]) == 3
    })
    returned_base_pair_profiles = sorted({
        expanded_cost(expr, compiler.primitives)
        for expr, used in result.alternatives
        if set(used) == {"prim:a", "prim:b"}
        and tuple(captured["free_keys"](expr)) == ("x", "y")
        and ast_depth(expr) == 3
    })
    stats = dict(result.stats)
    stable = {
        "source_label": source_label,
        "hash_seed": seed,
        "n_primitives": len(compiler.primitives),
        "alias_pairs": alias_pairs,
        "max_depth": 3,
        "target_found": True,
        "target_expression": result.expression,
        "target_used_primitives": list(result.used_primitives),
        "returned_alternatives": [
            {
                "expression": expr,
                "used_primitives": list(used),
                "expanded_cost": list(expanded_cost(expr, compiler.primitives)),
                "ast_depth": ast_depth(expr),
            }
            for expr, used in result.alternatives
        ],
        "returned_alternative_count": len(result.alternatives),
        "live_base_pair_profiles": [list(x) for x in live_base_pair_profiles],
        "returned_base_pair_profiles": [list(x) for x in returned_base_pair_profiles],
        "expected_profile_live": list(base_pair_profile) in [list(x) for x in live_base_pair_profiles],
        "expected_profile_returned": list(base_pair_profile) in [list(x) for x in returned_base_pair_profiles],
        "active_level_states": len(live_tokens),
        "active_frontier_states": len(frontier_tokens),
        "indexed_states": len(indexed_tokens),
        "active_identity_consistent": identity_consistent,
        "stats": stats,
    }
    signature = hashlib.sha256(
        json.dumps(stable, sort_keys=True, separators=(",", ":")).encode()
    ).hexdigest()
    output = {
        "stable": stable,
        "signature_sha256": signature,
        "performance": {
            "elapsed_ms": round(elapsed_ms, 3),
            "process_peak_rss_mb": round(peak_rss_mb, 3),
        },
    }
    print(json.dumps(output, sort_keys=True, separators=(",", ":")), flush=True)

    if elapsed_ms > 15000:
        raise TimeoutError(f"15-second guard exceeded for n={len(compiler.primitives)}")
    if peak_rss_mb > 2048:
        raise MemoryError(f"2-GiB process RSS guard exceeded for n={len(compiler.primitives)}")
    return output


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--seed", type=int, required=True)
    parser.add_argument("--source", required=True, choices=("r2", "r3"))
    args = parser.parse_args()
    for alias_pairs in (0, 1, 2, 3):
        run_case(alias_pairs, args.seed, args.source)


if __name__ == "__main__":
    main()
