#!/usr/bin/env python3
from __future__ import annotations

import argparse
import inspect
import json
import os
import sys
import time
from hashlib import sha256
from pathlib import Path

from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import ast_depth, ast_nodes
from lab.exact_checkpoint import load_checkpoint, restore_checkpoint
import lab.acsie_open_ended_self_extending_proving as proving


SEED = 2026100305
GENERATIONS = 6
GENERATION = 5
TARGET_DEPTHS = {2, 3, 4}
RUNTIME_SHA = "9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be"
PROVING_SHA = "33259fc5c69d87b99c604f0050508c433a279daf"


def lineage_ids(compiler, primitive_ids):
    out = set()
    method = getattr(compiler, "executable_lineage", None)
    if method is not None:
        for pid in primitive_ids:
            out.update(str(x) for x in method(pid))
        return tuple(sorted(out))
    stack = [str(pid) for pid in primitive_ids]
    seen = set()
    while stack:
        pid = stack.pop()
        if pid in seen:
            continue
        seen.add(pid)
        out.add(pid)
        primitive = compiler.primitives.get(pid)
        if primitive is None:
            continue
        for node in compiler._walk(primitive.expression):
            if node.get("op") == "macro":
                stack.append(str(node["id"]))
    return tuple(sorted(out))


def execution_cost(compiler, expr, active=()):
    op = str(expr["op"])
    if op in {"get", "const"}:
        return (1, 1)
    if op == "macro":
        mid = str(expr["id"])
        if mid in active or mid not in compiler.primitives:
            return (1, 1)
        return execution_cost(compiler, compiler.primitives[mid].expression, active + (mid,))
    if op in {"abs", "neg", "threshold"}:
        depth, nodes = execution_cost(compiler, expr["arg"], active)
        return (depth + 1, nodes + 1)
    left = execution_cost(compiler, expr["left"], active)
    right = execution_cost(compiler, expr["right"], active)
    return (1 + max(left[0], right[0]), 1 + left[1] + right[1])


def canonical(expr):
    return json.dumps(expr, sort_keys=True, separators=(",", ":"))


class Capture:
    def __init__(self, output: Path):
        self.output = output
        self.started = time.perf_counter()
        self.active = {}
        self.next_id = 0
        self.captured = []
        self.done = False
        # Resolve the post-assignment marker once. Reading/splitting the source
        # file on every traced line would make this observer dominate the search.
        self.search_code = find_exact_expression.__code__
        source_lines, first_line = inspect.getsourcelines(find_exact_expression)
        marker_lines = [
            first_line + offset
            for offset, source_line in enumerate(source_lines)
            if source_line.strip() == "if best_at_depth is not None:"
        ]
        if len(marker_lines) != 1:
            raise RuntimeError(
                f"expected one post-selection marker, found {len(marker_lines)}"
            )
        self.post_choose_line = marker_lines[0]

    def _parent_stage(self, frame):
        parent = frame.f_back
        names = []
        for _ in range(8):
            if parent is None:
                break
            names.append(parent.f_code.co_name)
            parent = parent.f_back
        if "synthesize_process" in names:
            return "synthesize_process"
        return None

    def _snapshot(self, frame, ctx, depth):
        levels = frame.f_locals.get("levels", {})
        frontier_map = frame.f_locals.get("dominance_frontier", {})
        compiler = frame.f_locals.get("compiler")
        classes = []
        for behavior_key, frontier in frontier_map.items():
            reps = []
            for state in frontier:
                expr = state[0]
                if ast_depth(expr) != depth:
                    continue
                token = int(state[5])
                dominated = frame.f_locals.get("dominated_state_tokens", set())
                if token in dominated:
                    continue
                used = tuple(state[2])
                lin = lineage_ids(compiler, used)
                cost = execution_cost(compiler, expr)
                reps.append({
                    "state_token": token,
                    "expression": expr,
                    "expression_key": canonical(expr),
                    "used_primitives": list(used),
                    "executable_lineage_ids": list(lin),
                    "expanded_execution_depth": cost[0],
                    "expanded_execution_nodes": cost[1],
                    "structural_key": state[4],
                })
            if len(reps) > 1 or reps:
                outputs, free_keys = behavior_key
                classes.append({
                    "behavior_key": {
                        "outputs": list(outputs),
                        "free_keys": list(free_keys),
                    },
                    "representatives": sorted(reps, key=lambda x: x["state_token"]),
                })
        payload = {
            "generation": GENERATION,
            "depth": depth,
            "class_count": len(classes),
            "classes": classes,
            "capture_elapsed_seconds": time.perf_counter() - self.started,
        }
        self.captured.append(payload)
        self.output.write_text(json.dumps({
            "schema": "ACSIE.PR110.H7.offline-frontier-capture.v1",
            "runtime_sha": RUNTIME_SHA,
            "proving_sha": PROVING_SHA,
            "seed": SEED,
            "generation": GENERATION,
            "requested_depths": sorted(TARGET_DEPTHS),
            "captures": self.captured,
        }, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        if depth == 4:
            self.done = True
            raise SystemExit(0)

    def trace(self, frame, event, arg):
        name = frame.f_code.co_name
        if event == "call" and name == "find_exact_expression":
            if self._parent_stage(frame) == "synthesize_process":
                ctx = {"generation": int(getattr(frame.f_locals.get("compiler"), "generation", -1))}
                if ctx["generation"] == GENERATION:
                    self.active[id(frame)] = ctx
                    return self.trace
            return None
        if event == "line":
            ctx = self.active.get(id(frame))
            if ctx is None:
                return self.trace
            depth = frame.f_locals.get("depth")
            marker = self._is_post_choose(frame)
            if marker and isinstance(depth, int) and depth in TARGET_DEPTHS:
                if all(c["depth"] != depth for c in self.captured):
                    self._snapshot(frame, ctx, depth)
            return self.trace
        if event == "return":
            self.active.pop(id(frame), None)
            return self.trace
        return self.trace if id(frame) in self.active else None

    def _is_post_choose(self, frame):
        # Python line events fire before the current line executes. This marker
        # is the line after the assignment, so best_at_depth is already bound.
        # Compare code object + line number; do not do file I/O in the trace loop.
        return (
            frame.f_code is self.search_code
            and frame.f_lineno == self.post_choose_line
            and "best_at_depth" in frame.f_locals
        )


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--checkpoint", required=True)
    ap.add_argument("--output", required=True)
    args = ap.parse_args()

    envelope = load_checkpoint(
        args.checkpoint,
        expected_seed=SEED,
        expected_metadata={"proving_sha": PROVING_SHA, "acsie_ref": RUNTIME_SHA},
    )
    assert envelope["generation_next"] == GENERATION
    restored = restore_checkpoint(envelope, expected_metadata={"proving_sha": PROVING_SHA, "acsie_ref": RUNTIME_SHA})
    assert restored["learner"].generation is not None

    capture = Capture(Path(args.output))
    old = sys.gettrace()
    sys.settrace(capture.trace)
    exit_code = 0
    try:
        proving.run_seed(
            SEED,
            GENERATIONS,
            resume_from=args.checkpoint,
            checkpoint_metadata={"proving_sha": PROVING_SHA, "acsie_ref": RUNTIME_SHA},
        )
    except SystemExit as exc:
        exit_code = 0 if exc.code is None else int(exc.code)
    finally:
        sys.settrace(old)

    if not capture.done:
        raise RuntimeError("capture did not reach Gen5 depth4")
    if exit_code != 0:
        raise RuntimeError(f"instrumented resume exited unexpectedly: {exit_code}")
    # Bind provenance to the actual loaded files, not just the requested Git refs.
    def file_record(module):
        path = Path(module.__file__).resolve()
        return {
            "path": str(path),
            "sha256": sha256(path.read_bytes()).hexdigest(),
        }
    print(json.dumps({
        "capture": "PASSED",
        "generation": GENERATION,
        "depths": sorted({item["depth"] for item in capture.captured}),
        "exit_code": exit_code,
        "python_version": sys.version,
        "pythonhashseed": os.environ.get("PYTHONHASHSEED", "unset"),
        "hash_randomization": sys.flags.hash_randomization,
        "modules": {
            "behavioral_search": file_record(sys.modules["cognitive_core.behavioral_search"]),
            "recursive_cognitive_compiler": file_record(sys.modules["cognitive_core.recursive_cognitive_compiler"]),
            "open_ended_growth": file_record(sys.modules["cognitive_core.open_ended_growth"]),
            "proving_evaluator": file_record(proving),
        },
        "checkpoint_sha256": sha256(Path(args.checkpoint).read_bytes()).hexdigest(),
        "sys_path": sys.path,
        "evidence_scope": "existing non-dominated frontier only; second-stage offline compression",
    }, sort_keys=True))


if __name__ == "__main__":
    main()
