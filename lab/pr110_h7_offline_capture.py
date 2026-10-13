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
    def __init__(self, output: Path, capture_role: str):
        self.output = output
        self.capture_role = capture_role
        self.started = time.perf_counter()
        self.captured = []
        self.done = False
        self.current_invocation = {}
        self.current_call_id = None
        self.current_call_role = None
        self.selected_call_id = None
        self.selected_call_index = None
        self.search_call_counter = 0
        self.status = "CAPTURE_PENDING"

    def _resolve_role(self, caller, compiler):
        name = caller.f_code.co_name
        if name == "invent_primitive":
            if type(compiler).__name__ != "OpenEndedRecursiveCognitiveCompiler":
                return None
            return "primary_primitive_invention"
        if name.startswith("synthesize_process"):
            return "secondary_process_synthesis"
        return None

    def enter(self, local_values):
        """Called once per viable search call; no sys.settrace or per-line file I/O."""
        caller = sys._getframe(2)
        compiler = local_values.get("compiler")
        if getattr(compiler, "generation", -1) != GENERATION:
            self.current_call_id = None
            self.current_call_role = None
            return
        role = self._resolve_role(caller, compiler)
        self.search_call_counter += 1
        self.current_call_id = self.search_call_counter
        self.current_call_role = role
        if role != self.capture_role:
            return
        if self.selected_call_id is not None:
            return
        self.selected_call_id = self.current_call_id
        self.selected_call_index = self.search_call_counter
        self.current_invocation = {
            "role": role,
            "caller_method": caller.f_code.co_name,
            "compiler_class": type(compiler).__name__,
            "generation": int(getattr(compiler, "generation", -1)),
            "primitive_count": len(getattr(compiler, "primitives", {})),
            "search_max_depth": local_values.get("max_depth"),
            "discovery_row_count": len(local_values.get("discovery_rows", local_values.get("rows", ()))),
            "require_macro": bool(local_values.get("require_macro", False)),
            "search_call_index": self.selected_call_index,
            "caller_stack": self._caller_stack(caller),
            "hook_elapsed_seconds_from_run_seed_start": time.perf_counter() - self.started,
        }

    @staticmethod
    def _caller_stack(caller):
        rows = []
        frame = caller
        for _ in range(10):
            if frame is None:
                break
            row = {"function": frame.f_code.co_name}
            for key in ("generation", "pair_idx", "probe_idx"):
                value = frame.f_locals.get(key)
                if isinstance(value, (str, int, float, bool)) or value is None:
                    if value is not None:
                        row[key] = value
            if "self" in frame.f_locals:
                row["self_class"] = type(frame.f_locals["self"]).__name__
            rows.append(row)
            frame = frame.f_back
        return rows

    def hook(self, depth, local_values):
        """Capture a selected call after depth-local target selection."""
        if self.current_call_id is None or self.current_call_id != self.selected_call_id:
            return
        if self.current_call_role != self.capture_role:
            return
        if not isinstance(depth, int) or depth not in TARGET_DEPTHS:
            return
        if any(item["depth"] == depth for item in self.captured):
            return
        self._snapshot(local_values, depth)
    def _snapshot(self, local_values, depth):
        hook_entered = time.perf_counter()
        frontier_map = local_values.get("dominance_frontier", {})
        compiler = local_values.get("compiler")
        classes = []
        frontiers_by_depth = {}
        for _frontier in frontier_map.values():
            for _state in _frontier:
                _d = ast_depth(_state[0])
                frontiers_by_depth[_d] = frontiers_by_depth.get(_d, 0) + 1
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
        stats = local_values.get("stats", {})
        stat_keys = (
            "schema", "status", "rows", "max_depth", "primitive_count",
            "generated_expressions", "unique_states", "duplicate_states",
            "inverse_hits", "target_matches", "max_states_in_depth",
            "max_lineage_coverage", "best_target_depth", "dominance_pruned_states",
            "dominance_frontier_max", "exact_expression_duplicates_skipped",
            "target_directed_candidates", "generic_binary_pairs_evaluated",
            "generic_binary_path_skipped_after_target", "target_cutoff_depth",
            "target_cutoff_reason", "alternative_candidate_count",
            "target_candidate_count",
        )
        payload = {
            "generation": GENERATION,
            "depth": depth,
            "class_count": len(classes),
            "representative_count": sum(len(item["representatives"]) for item in classes),
            "classes": classes,
            "frontier_live_representatives_by_ast_depth": {str(k): v for k, v in sorted(frontiers_by_depth.items())},
            "search_stats_at_capture": {k: stats.get(k) for k in stat_keys if k in stats},
            "frontier_bucket_sizes_by_ast_depth": {str(k): len(v) for k, v in sorted(local_values.get("levels", {}).items())},
            "capture_elapsed_seconds": time.perf_counter() - self.started,
            "capture_hook_entered_elapsed_seconds": hook_entered - self.started,
            "capture_snapshot_cost_seconds": time.perf_counter() - hook_entered,
            "capture_semantics": "post_choose_target_in_depth; existing live frontier only",
            "invocation": dict(self.current_invocation),
        }
        if depth == 4:
            self.done = True
            self.status = "CAPTURE_COMPLETE_DEPTH4"
        self.captured.append(payload)
        source_path = Path(behavioral_search.__file__).resolve()
        self.output.write_text(json.dumps({
            "schema": "ACSIE.PR110.H7.offline-frontier-capture.v4",
            "status": self.status,
            "runtime_sha": RUNTIME_SHA,
            "proving_sha": PROVING_SHA,
            "seed": SEED,
            "generation": GENERATION,
            "requested_depths": sorted(TARGET_DEPTHS),
            "requested_capture_role": self.capture_role,
            "selected_search_call_index": self.selected_call_index,
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
            raise SystemExit(0)

    def install(self):
        """Compile an in-memory observer-only variant; never edit checked-in code."""
        original = behavioral_search.find_exact_expression
        source_lines, _ = inspect.getsourcelines(original)
        source = "".join(source_lines)
        enter_marker = "    target = tuple(float(row.target) for row in discovery_rows)\n"
        depth_marker = "        best_at_depth = choose_target_in_depth(depth)\n"
        if source.count(enter_marker) != 1:
            raise RuntimeError("expected exactly one start-of-search instrumentation marker")
        if source.count(depth_marker) != 1:
            raise RuntimeError("expected exactly one choose_target_in_depth capture marker")
        instrumented = source.replace(
            enter_marker,
            "    _H7_CAPTURE_ENTER(locals())\n" + enter_marker,
            1,
        )
        instrumented = instrumented.replace(
            depth_marker,
            depth_marker + "        _H7_CAPTURE_HOOK(depth, locals())\n",
            1,
        )
        self.instrumented_source_sha256 = sha256(instrumented.encode("utf-8")).hexdigest()
        behavioral_search._H7_CAPTURE_ENTER = self.enter
        behavioral_search._H7_CAPTURE_HOOK = self.hook
        exec(compile(instrumented, behavioral_search.__file__, "exec"), behavioral_search.__dict__)
        selected_function = behavioral_search.find_exact_expression

        def bounded_wrapper(*args, **kwargs):
            result = None
            completed_normally = False
            caught_error = None
            try:
                result = selected_function(*args, **kwargs)
                completed_normally = True
                return result
            except BaseException as exc:
                caught_error = exc
                raise
            finally:
                # Stop immediately after the selected call returns if the search
                # terminated before depth 4. This avoids running the remaining
                # generations just to wait for a depth that this invocation
                # cannot produce. A partial capture remains explicitly partial.
                if (
                    self.selected_call_id is not None
                    and self.current_call_id == self.selected_call_id
                    and self.current_call_role == self.capture_role
                    and not self.done
                ):
                    self.done = True
                    if completed_normally:
                        self.status = (
                            "CAPTURE_PARTIAL_SEARCH_RETURNED"
                            if self.captured else "NO_TARGET_DEPTH_CAPTURE"
                        )
                    else:
                        self.status = (
                            "CAPTURE_PARTIAL_SEARCH_CRASHED"
                            if self.captured else "SELECTED_SEARCH_CRASHED"
                        )
                    source_path = Path(behavioral_search.__file__).resolve()
                    payload = {
                        "schema": "ACSIE.PR110.H7.offline-frontier-capture.v4",
                        "status": self.status,
                        "runtime_sha": RUNTIME_SHA,
                        "proving_sha": PROVING_SHA,
                        "seed": SEED,
                        "generation": GENERATION,
                        "requested_depths": sorted(TARGET_DEPTHS),
                        "requested_capture_role": self.capture_role,
                        "selected_search_call_index": self.selected_call_index,
                        "observer": {
                            "mode": "single-invocation-local-callback",
                            "trace_function_enabled": False,
                            "hook_point": "after depth-local selection; wrapper stops after the selected search returns",
                            "original_behavioral_search_file": str(source_path),
                            "original_behavioral_search_sha256": sha256(source_path.read_bytes()).hexdigest(),
                            "instrumented_function_source_sha256": self.instrumented_source_sha256,
                        },
                        "captures": self.captured,
                        "selected_search_completion": {
                            "returned_normally": completed_normally,
                            "result_is_none": result is None,
                            "exception_type": type(caught_error).__name__ if caught_error is not None else None,
                            "exception": str(caught_error)[:1000] if caught_error is not None else None,
                            "result_stats": dict(getattr(result, "stats", {})) if result is not None else {},
                            "hook_elapsed_seconds_from_run_seed_start": time.perf_counter() - self.started,
                        },
                    }
                    self.output.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\\n", encoding="utf-8")
                    if completed_normally:
                        raise SystemExit(0 if self.captured else 86)

        growth.find_exact_expression = bounded_wrapper
        return original



def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--checkpoint", required=True)
    ap.add_argument("--output", required=True)
    ap.add_argument("--capture-role", choices=("primary_primitive_invention", "secondary_process_synthesis"), default="primary_primitive_invention")
    args = ap.parse_args()

    envelope = load_checkpoint(
        args.checkpoint,
        expected_seed=SEED,
        expected_metadata={"proving_sha": PROVING_SHA, "acsie_ref": RUNTIME_SHA},
    )
    assert envelope["generation_next"] == GENERATION
    restored = restore_checkpoint(envelope, expected_metadata={"proving_sha": PROVING_SHA, "acsie_ref": RUNTIME_SHA})
    assert restored["learner"].generation is not None

    capture = Capture(Path(args.output), args.capture_role)
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
        behavioral_search.__dict__.pop("_H7_CAPTURE_ENTER", None)

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
        "capture": capture.status,
        "generation": GENERATION,
        "depths": sorted({item["depth"] for item in capture.captured}),
        "capture_status": capture.status,
        "selected_search_call_index": capture.selected_call_index,
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
        "evidence_scope": "existing non-dominated frontier only; selected call role and invocation recorded",
    }, sort_keys=True))


if __name__ == "__main__":
    main()
