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
import cognitive_core.open_ended_growth as growth
import cognitive_core.behavioral_search as behavioral_search
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
        self.captured = []
        self.done = False

    def hook(self, depth, local_values):
        """Single callback after a search-depth selection; no line tracing."""
        caller = sys._getframe(2)
        if caller.f_code.co_name != "synthesize_process":
            return
        compiler = local_values.get("compiler")
        if getattr(compiler, "generation", -1) != GENERATION:
            return
        if not isinstance(depth, int) or depth not in TARGET_DEPTHS:
            return
        if any(item["depth"] == depth for item in self.captured):
            return
        self._snapshot(local_values, depth)

    def _snapshot(self, local_values, depth):
        frontier_map = local_values.get("dominance_frontier", {})
        compiler = local_values.get("compiler")
        classes = []
        dominated = local_values.get("dominated_state_tokens", set())
        for behavior_key, frontier in frontier_map.items():
            reps = []
            for state in frontier:
                expr = state[0]
                if ast_depth(expr) != depth:
                    continue
                token = int(state[5])
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
            if reps:
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
            "capture_semantics": "post_choose_target_in_depth; existing live frontier only",
        }
        self.captured.append(payload)
        source_path = Path(behavioral_search.__file__).resolve()
        self.output.write_text(json.dumps({
            "schema": "ACSIE.PR110.H7.offline-frontier-capture.v2",
            "runtime_sha": RUNTIME_SHA,
            "proving_sha": PROVING_SHA,
            "seed": SEED,
            "generation": GENERATION,
            "requested_depths": sorted(TARGET_DEPTHS),
            "observer": {
                "mode": "single-invocation-local-callback",
                "trace_function_enabled": False,
                "hook_point": "immediately after best_at_depth = choose_target_in_depth(depth)",
                "original_behavioral_search_file": str(source_path),
                "original_behavioral_search_sha256": sha256(source_path.read_bytes()).hexdigest(),
                "instrumented_function_source_sha256": self.instrumented_source_sha256,
            },
            "captures": self.captured,
        }, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        if depth == 4:
            self.done = True
            raise SystemExit(0)

    def install(self):
        """Compile an in-memory observer-only function; never edit checked-in code."""
        original = behavioral_search.find_exact_expression
        source_lines, _ = inspect.getsourcelines(original)
        source = "".join(source_lines)
        marker = "        best_at_depth = choose_target_in_depth(depth)\n"
        if source.count(marker) != 1:
            raise RuntimeError("expected exactly one choose_target_in_depth capture marker")
        instrumented = source.replace(
            marker,
            marker + "        _H7_CAPTURE_HOOK(depth, locals())\n",
            1,
        )
        self.instrumented_source_sha256 = sha256(instrumented.encode("utf-8")).hexdigest()
        behavioral_search._H7_CAPTURE_HOOK = self.hook
        exec(compile(instrumented, behavioral_search.__file__, "exec"), behavioral_search.__dict__)
        growth.find_exact_expression = behavioral_search.find_exact_expression
        return original



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
    original_search = capture.install()
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
        behavioral_search.find_exact_expression = original_search
        growth.find_exact_expression = original_search
        behavioral_search.__dict__.pop("_H7_CAPTURE_HOOK", None)

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
