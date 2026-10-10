#!/usr/bin/env python3
"""Reference-vs-memoized exact-snapshot search equivalence smoke test.

This test checks only execution-level lineage memoization on a small deterministic
discovery-only task. It is not a scientific capability qualification.
"""
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path


_RUN = r"""
import json
from cognitive_core.behavioral_search import find_exact_expression
from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import CognitivePrimitive, Trace, eval_expr

compiler = OpenEndedRecursiveCognitiveCompiler(max_depth=2, population=24, seed=2026100305)
pid = "prim:lineage-cache-smoke"
compiler.primitives[pid] = CognitivePrimitive(
    primitive_id=pid,
    expression={"op": "get", "key": "x"},
    input_roles=("x",),
    output_semantics="delta",
    generation=0,
    parent_ids=(),
    source_families=("synthetic-lineage-cache-smoke",),
    train_error=0.0,
    holdout_error=0.0,
    transfer_error=0.0,
    complexity=1,
    provenance={"origin": "synthetic_test", "external_model": False},
)
rows = tuple(
    Trace(
        inputs={"x": float(x), "y": float(-x)},
        target=2.0 * float(x),
        family="synthetic-lineage-cache-smoke",
        context={},
        task_id=f"smoke:{i}",
    )
    for i, x in enumerate((-7, -5, -3, -2, -1, 1, 2, 3, 5, 7, 9, 11))
)
calls = {"primitive": {}, "executable": {}}
original_primitive = compiler.primitive_lineage
def counted_primitive(primitive_id):
    key = str(primitive_id)
    calls["primitive"][key] = calls["primitive"].get(key, 0) + 1
    return original_primitive(key)
compiler.primitive_lineage = counted_primitive
if hasattr(compiler, "executable_lineage"):
    original_executable = compiler.executable_lineage
    def counted_executable(primitive_id):
        key = str(primitive_id)
        calls["executable"][key] = calls["executable"].get(key, 0) + 1
        return original_executable(key)
    compiler.executable_lineage = counted_executable
result = find_exact_expression(compiler, rows, 2, require_macro=True, max_alternatives=8)
assert result is not None, "exact synthetic composition not discovered"
assert pid in result.used_primitives, result.used_primitives
pmap = compiler._pmap()
assert all(abs(eval_expr(result.expression, row.inputs, pmap) - row.target) <= 1e-9 for row in rows)
stats = dict(result.stats)
cache_counters = stats.pop("lineage_cache_counters", {})
print(json.dumps({
    "expression": result.expression,
    "used_primitives": list(result.used_primitives),
    "alternatives": [{"expression": e, "used": list(u)} for e, u in result.alternatives],
    "stats": stats,
    "lineage_cache_counters": cache_counters,
    "underlying_graph_walk_calls": calls,
}, sort_keys=True, separators=(",", ":")))
"""


def run(root: Path, arm: str) -> dict:
    env = dict(os.environ)
    env["PYTHONPATH"] = str(root / arm)
    env["PYTHONHASHSEED"] = "0"
    proc = subprocess.run(
        [sys.executable, "-c", _RUN],
        env=env,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        timeout=120,
        check=False,
    )
    if proc.returncode:
        raise RuntimeError(
            f"{arm} run at {root} failed with exit {proc.returncode}:\n{proc.stderr}\n{proc.stdout}"
        )
    lines = [line for line in proc.stdout.splitlines() if line.strip()]
    if len(lines) != 1:
        raise RuntimeError(f"{arm} produced unexpected stdout: {proc.stdout!r}")
    return json.loads(lines[0])


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--reference-root", type=Path, required=True)
    parser.add_argument("--optimized-root", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    results = {}
    for arm in ("A", "B"):
        reference = run(args.reference_root, arm)
        optimized = run(args.optimized_root, arm)
        if reference["expression"] != optimized["expression"]:
            raise AssertionError(f"{arm}: selected expression changed")
        if reference["used_primitives"] != optimized["used_primitives"]:
            raise AssertionError(f"{arm}: selected executable primitives changed")
        if reference["alternatives"] != optimized["alternatives"]:
            raise AssertionError(f"{arm}: alternative frontier changed")
        if reference["stats"] != optimized["stats"]:
            differences = [
                key for key in set(reference["stats"]) | set(optimized["stats"])
                if reference["stats"].get(key) != optimized["stats"].get(key)
            ]
            raise AssertionError(f"{arm}: search stats changed outside cache counters: {differences}")
        optimized_calls = optimized["underlying_graph_walk_calls"]
        for kind, by_id in optimized_calls.items():
            if by_id and max(by_id.values()) != 1:
                raise AssertionError(f"{arm}: {kind} graph walks were repeated: {by_id}")
        counters = optimized["lineage_cache_counters"]
        if not counters or not any(int(value) > 0 for value in counters.values()):
            raise AssertionError(f"{arm}: memoization counters show no cache activity: {counters}")
        results[arm] = {
            "selected_expression_identical": True,
            "used_primitives_identical": True,
            "alternatives_identical": True,
            "all_non_cache_search_stats_identical": True,
            "reference_graph_walk_calls": reference["underlying_graph_walk_calls"],
            "memoized_graph_walk_calls": optimized_calls,
            "memoization_counters": counters,
            "baseline_expression": reference["expression"],
        }

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps({
        "schema": "ACSIE.PR110.LineageMemoizationTwinCheck.v1",
        "classification": "SEMANTICS_NEUTRAL_SYNTHETIC_EQUIVALENCE_ONLY",
        "arms": results,
        "scientific_gate": "NOT_RUN",
        "promotion_status": "DO_NOT_PROMOTE",
    }, indent=2, sort_keys=True) + "\n")
    print(json.dumps(results, indent=2, sort_keys=True))
    print("PR110_LINEAGE_MEMOIZATION_TWIN_CHECK=PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
