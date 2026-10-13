#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import sys
import time

from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive, Trace, ast_nodes
from cognitive_core.behavioral_search import find_exact_expression


def primitive(pid: str, expr: dict, generation: int = 0) -> CognitivePrimitive:
    def keys(e):
        op = e["op"]
        if op == "get":
            return {str(e["key"])}
        if op == "const":
            return set()
        if op == "macro":
            return set()
        if op in {"abs", "neg", "threshold"}:
            return keys(e["arg"])
        return keys(e["left"]) | keys(e["right"])

    return CognitivePrimitive(
        pid, expr, tuple(sorted(keys(expr))), "synthetic", generation,
        (), ("h7-benchmark",), 0.0, 0.0, 0.0, ast_nodes(expr), {},
    )


def make_compiler(n: int) -> OpenEndedRecursiveCognitiveCompiler:
    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2, seed=20261013)
    exprs = [
        {"op": "get", "key": "x"},
        {"op": "neg", "arg": {"op": "get", "key": "x"}},
        {"op": "abs", "arg": {"op": "get", "key": "x"}},
        {"op": "add", "left": {"op": "get", "key": "x"}, "right": {"op": "const", "value": 1.0}},
        {"op": "sub", "left": {"op": "get", "key": "x"}, "right": {"op": "const", "value": 1.0}},
        {"op": "max", "left": {"op": "get", "key": "x"}, "right": {"op": "const", "value": 0.0}},
        {"op": "min", "left": {"op": "get", "key": "x"}, "right": {"op": "const", "value": 10.0}},
        {"op": "const", "value": 1.0},
        {"op": "add", "left": {"op": "get", "key": "x"}, "right": {"op": "const", "value": 2.0}},
        {"op": "sub", "left": {"op": "get", "key": "x"}, "right": {"op": "const", "value": 2.0}},
    ]
    primitives = {}
    for i in range(n):
        pid = f"prim:{i:03d}"
        p = primitive(pid, exprs[i % len(exprs)])
        primitives[pid] = p
    c.primitives = primitives
    c.archive = {pid: ArchiveRecord(p) for pid, p in primitives.items()}
    return c


def run_once(n: int, task: str) -> tuple[dict, float]:
    c = make_compiler(n)
    rows = []
    for i in range(1, 11):
        x = float(i)
        target = 2.0 * x if task == "reachable_add" else x * x + 7.0
        rows.append(Trace({"x": x}, target, "h7-benchmark", {}, f"{task}-{i}"))
    rows = tuple(rows)

    stats_box = {}
    code = find_exact_expression.__code__

    def local_trace(frame, event, arg):
        if event == "return":
            stats = frame.f_locals.get("stats")
            if isinstance(stats, dict):
                stats_box["stats"] = dict(stats)
        return local_trace

    def global_trace(frame, event, arg):
        if event == "call" and frame.f_code is code:
            return local_trace
        return None

    old_trace = sys.gettrace()
    start = time.perf_counter()
    sys.settrace(global_trace)
    try:
        result = find_exact_expression(c, rows, max_depth=2, require_macro=True)
    finally:
        sys.settrace(old_trace)
    elapsed = time.perf_counter() - start
    stats = stats_box.get("stats")
    if stats is None:
        raise RuntimeError("could not capture invocation-local search stats")
    result_payload = None if result is None else {
        "expression": result.expression,
        "used_primitives": list(result.used_primitives),
        "alternatives": [
            {"expression": expression, "used_primitives": list(used)}
            for expression, used in result.alternatives
        ],
    }
    return {
        "result": result_payload,
        "stats": stats,
        "primitive_count": len(c.primitives),
        "rows": len(rows),
        "depth": 2,
        "require_macro": True,
    }, elapsed


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--n", type=int, required=True)
    parser.add_argument("--task", choices=("reachable_add", "unreachable"), required=True)
    parser.add_argument("--trials", type=int, default=3)
    args = parser.parse_args()
    if args.n not in (16, 24, 32) or args.trials < 1:
        raise SystemExit("invalid bounded benchmark settings")

    records = []
    first = None
    for trial in range(args.trials):
        payload, elapsed = run_once(args.n, args.task)
        if first is None:
            first = payload
        elif payload != first:
            raise AssertionError(f"nondeterministic full result/stats at trial {trial + 1}")
        records.append(elapsed)
    print(json.dumps({
        "schema": "ACSIE.H7.PR112.independent-parity-worker.v1",
        "variant": __import__("cognitive_core.behavioral_search", fromlist=["find_exact_expression"]).__file__,
        "task": args.task,
        "n": args.n,
        "trials_seconds": records,
        "median_seconds": sorted(records)[len(records) // 2],
        "full_result_and_stats": first,
    }, sort_keys=True))


if __name__ == "__main__":
    main()
