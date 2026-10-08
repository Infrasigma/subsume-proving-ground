#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import linecache
import os
import signal
import sys
import time
import traceback
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any

try:
    import resource
except ImportError:
    resource = None

from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
import lab.acsie_open_ended_self_extending_proving as proving


SEED = 2026100305
GENERATIONS = 6
SELECTED_GENERATIONS = [5, 6]


class ForensicTrace:
    def __init__(self, label: str, output_path: Path):
        self.label = label
        self.output_path = output_path
        self.started = time.perf_counter()
        self.finished = False
        self.searches: list[dict[str, Any]] = []
        self.active: dict[int, dict[str, Any]] = {}
        self.child_started: dict[int, float] = {}
        self.next_id = 0
        self.checkpoint_timings: list[dict[str, Any]] = []
        self._old_save = proving.save_checkpoint
        self._install_checkpoint_wrapper()
        self._install_signal_handlers()

    def _install_checkpoint_wrapper(self) -> None:
        def timed_save(*args, **kwargs):
            t0 = time.perf_counter()
            try:
                return self._old_save(*args, **kwargs)
            finally:
                self.checkpoint_timings.append({
                    "seconds": time.perf_counter() - t0,
                    "path": str(args[0]) if args else str(kwargs.get("path")),
                })
                self.flush()
        proving.save_checkpoint = timed_save

    def _install_signal_handlers(self) -> None:
        def handler(signum, _frame):
            self.flush()
            raise SystemExit(128 + int(signum))
        signal.signal(signal.SIGTERM, handler)
        signal.signal(signal.SIGINT, handler)

    @staticmethod
    def _source_markers(filename: str) -> dict[str, int]:
        lines = Path(filename).read_text(encoding="utf-8").splitlines()
        markers: dict[str, int] = {}
        for i, line in enumerate(lines, 1):
            s = line.strip()
            if "best_at_depth = choose_target_in_depth(depth)" in s and "if" not in s:
                markers["post_choose"] = i
            elif "behavior_key = (outputs, state_keys)" in s:
                markers["equivalence_start"] = i
            elif "for existing in tuple(frontier):" in s and "dominance" not in markers:
                markers["dominance_start"] = i
            elif "retained_frontier = []" in s:
                markers["frontier_start"] = i
            elif s == "return state":
                markers["frontier_end"] = i
            elif "candidate = register(" in s:
                markers.setdefault("candidate_submit_first", i)
        return markers

    def _find_context(self, frame) -> dict[str, Any] | None:
        compiler = frame.f_locals.get("compiler")
        if not isinstance(compiler, OpenEndedRecursiveCognitiveCompiler):
            return None
        filename = frame.f_code.co_filename
        ctx = {
            "id": self.next_id,
            "frame_id": id(frame),
            "filename": filename,
            "generation": int(getattr(compiler, "generation", -1)),
            "stage": "unknown",
            "markers": self._source_markers(filename),
            "line_last": frame.f_lineno,
            "time_last": time.perf_counter(),
            "post_choose_pending": False,
            "last_snapshot_depth": None,
            "previous_total_frontier": 0,
            "previous_max_frontier": 0,
            "last_stats": {},
            "dominance_calls": 0,
            "dominance_seconds": 0.0,
            "lineage_calls": 0,
            "lineage_seconds": 0.0,
            "lineage_signatures": Counter(),
            "provenance_calls": 0,
            "provenance_seconds": 0.0,
            "equivalence_seconds": 0.0,
            "frontier_seconds": 0.0,
            "candidate_submit_calls": 0,
            "candidate_submit_seconds": 0.0,
            "search_started": time.perf_counter(),
            "return_seconds": None,
            "snapshots": [],
        }
        parent = frame.f_back
        parents = []
        for _ in range(6):
            if parent is None:
                break
            parents.append(parent.f_code.co_name)
            parent = parent.f_back
        if "invent_primitive" in parents:
            ctx["stage"] = "invent_primitive"
        elif "synthesize_process_frontier" in parents:
            ctx["stage"] = "synthesize_process_frontier"
        elif "synthesize_process" in parents:
            ctx["stage"] = "synthesize_process"
        self.next_id += 1
        self.active[id(frame)] = ctx
        return ctx

    @staticmethod
    def _state_is_dominated(frame, state) -> bool:
        dominated = frame.f_locals.get("dominated_state_tokens", set())
        try:
            return state[5] in dominated
        except Exception:
            return False

    def _snapshot(self, frame, ctx: dict[str, Any]) -> None:
        depth = frame.f_locals.get("depth")
        if depth is None:
            return
        try:
            depth = int(depth)
        except Exception:
            return
        if ctx["last_snapshot_depth"] == depth:
            return

        levels = frame.f_locals.get("levels", {})
        frontier_map = frame.f_locals.get("dominance_frontier", {})
        structural_by_key = frame.f_locals.get("structural_lineage_by_key", {})
        compiler = frame.f_locals.get("compiler")

        live_states = [
            s for s in levels.get(depth, {}).values()
            if not self._state_is_dominated(frame, s)
        ]

        class_sizes: list[int] = []
        live_frontier: list[tuple] = []
        exec_signatures: set[tuple[str, ...]] = set()
        structural_signatures: set[tuple[str, ...]] = set()
        lineage_sizes: list[int] = []
        multi_lineage_classes = 0
        removable_if_lineage_ignored = 0
        overlap_values: list[float] = []

        for frontier in frontier_map.values():
            live = [
                s for s in frontier
                if not self._state_is_dominated(frame, s) and
                self._safe_depth(s[0]) == depth
            ]
            if not live:
                continue
            class_sizes.append(len(live))
            live_frontier.extend(live)
            sigs = []
            for s in live:
                used = tuple(s[2])
                try:
                    sig = tuple(sorted({
                        x for pid in used
                        for x in compiler.executable_lineage(pid)
                    }))
                except Exception:
                    sig = used
                sigs.append(sig)
                exec_signatures.add(sig)
                try:
                    structural_signatures.add(
                        tuple(sorted(structural_by_key.get(s[4], set())))
                    )
                except Exception:
                    pass
                lineage_sizes.append(len(sig))

            if len(set(sigs)) > 1:
                multi_lineage_classes += 1
            removable_if_lineage_ignored += max(0, len(live) - 1)
            for i in range(len(sigs)):
                a = set(sigs[i])
                for j in range(i + 1, len(sigs)):
                    b = set(sigs[j])
                    union = a | b
                    overlap_values.append(
                        (len(a & b) / len(union)) if union else 1.0
                    )

        stats = frame.f_locals.get("stats", {})
        current_total_frontier = len(live_frontier)
        current_max_frontier = max(class_sizes, default=0)
        candidate_delta = int(stats.get("generated_expressions", 0)) - int(ctx["last_stats"].get("generated_expressions", 0))
        duplicate_delta = int(stats.get("duplicate_states", 0)) - int(ctx["last_stats"].get("duplicate_states", 0))
        pruned_delta = int(stats.get("dominance_pruned_states", 0)) - int(ctx["last_stats"].get("dominance_pruned_states", 0))

        snapshot = {
            "depth": depth,
            "generated_candidates_delta": max(0, candidate_delta),
            "duplicate_state_delta": max(0, duplicate_delta),
            "surviving_candidates_delta": max(0, len(live_states) - int(ctx["last_stats"].get("_live_state_count", 0))),
            "unique_semantic_states": len(live_states),
            "unique_executable_lineage_signatures": len(exec_signatures),
            "unique_structural_lineage_signatures": len(structural_signatures),
            "behavioral_equivalence_class_count": len(class_sizes),
            "equivalence_class_size_histogram": dict(Counter(class_sizes)),
            "frontier_size_total": current_total_frontier,
            "frontier_size_max_class": current_max_frontier,
            "frontier_growth_factor_total": current_total_frontier / ctx["previous_total_frontier"] if ctx["previous_total_frontier"] else 1.0,
            "frontier_growth_factor_max_class": current_max_frontier / ctx["previous_max_frontier"] if ctx["previous_max_frontier"] else 1.0,
            "dominance_comparisons_cumulative": ctx["dominance_calls"],
            "dominance_pruned_delta": max(0, pruned_delta),
            "non_dominated_candidates": len(live_frontier),
            "behavior_classes_with_multiple_executable_lineages": multi_lineage_classes,
            "representatives_removed_if_executable_lineage_ignored": removable_if_lineage_ignored,
            "average_executable_lineage_size": (
                sum(lineage_sizes) / len(lineage_sizes) if lineage_sizes else 0.0
            ),
            "maximum_executable_lineage_size": max(lineage_sizes, default=0),
            "average_candidate_lineage_overlap_jaccard": (
                sum(overlap_values) / len(overlap_values) if overlap_values else 0.0
            ),
            "lineage_overlap_pairs": len(overlap_values),
            "duplicate_lineage_calls": sum(
                max(0, n - 1) for n in ctx["lineage_signatures"].values()
            ),
            "lineage_call_count": ctx["lineage_calls"],
            "lineage_time_seconds": ctx["lineage_seconds"],
            "provenance_lineage_call_count": ctx["provenance_calls"],
            "provenance_lineage_time_seconds": ctx["provenance_seconds"],
            "dominance_time_seconds": ctx["dominance_seconds"],
            "equivalence_time_seconds": ctx["equivalence_seconds"],
            "frontier_maintenance_time_seconds": ctx["frontier_seconds"],
            "candidate_submission_time_seconds": ctx["candidate_submit_seconds"],
            "max_rss_kb": float(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss) if resource else None,
        }
        ctx["snapshots"].append(snapshot)
        ctx["last_snapshot_depth"] = depth
        ctx["previous_total_frontier"] = current_total_frontier
        ctx["previous_max_frontier"] = current_max_frontier
        ctx["last_stats"] = dict(stats)
        ctx["last_stats"]["_live_state_count"] = len(live_states)

        record = {
            "label": self.label,
            "generation": ctx["generation"],
            "stage": ctx["stage"],
            "search_id": ctx["id"],
            "depth": depth,
            "snapshot": snapshot,
        }
        self.searches.append(record)
        self.flush()

    @staticmethod
    def _safe_depth(expr) -> int:
        try:
            op = expr.get("op")
            if op in {"get", "const"}:
                return 1
            if op in {"abs", "neg", "threshold"}:
                return 1 + ForensicTrace._safe_depth(expr["arg"])
            return 1 + max(
                ForensicTrace._safe_depth(expr["left"]),
                ForensicTrace._safe_depth(expr["right"]),
            )
        except Exception:
            return 0

    def _line_event(self, frame, ctx) -> None:
        now = time.perf_counter()
        last_lineno = ctx["line_last"]
        dt = now - ctx["time_last"]
        markers = ctx["markers"]

        if markers.get("equivalence_start", 0) <= last_lineno < markers.get("dominance_start", -1):
            ctx["equivalence_seconds"] += dt
        if markers.get("frontier_start", 0) <= last_lineno <= markers.get("frontier_end", -1):
            ctx["frontier_seconds"] += dt
        if markers.get("candidate_submit_first", 0) == last_lineno:
            ctx["candidate_submit_seconds"] += dt

        if last_lineno == markers.get("post_choose"):
            self._snapshot(frame, ctx)

        ctx["line_last"] = frame.f_lineno
        ctx["time_last"] = now

    def _return_event(self, frame, ctx, result, elapsed: float) -> None:
        name = frame.f_code.co_name
        if name == "dominates":
            ctx["dominance_calls"] += 1
            ctx["dominance_seconds"] += elapsed
        elif name == "executable_lineage_ids":
            ctx["lineage_calls"] += 1
            ctx["lineage_seconds"] += elapsed
            if result is not None:
                try:
                    sig = tuple(result)
                    ctx["lineage_signatures"][sig] += 1
                except Exception:
                    pass
        elif name == "provenance_lineage_ids":
            ctx["provenance_calls"] += 1
            ctx["provenance_seconds"] += elapsed
        elif name == "register":
            ctx["candidate_submit_calls"] += 1
            ctx["candidate_submit_seconds"] += elapsed

    def trace(self, frame, event, arg):
        code_name = frame.f_code.co_name
        if event == "call":
            if code_name == "find_exact_expression":
                self._find_context(frame)
                return self.trace
            if code_name in {"dominates", "executable_lineage_ids", "provenance_lineage_ids", "register"}:
                parent = frame.f_back
                if parent is not None and id(parent) in self.active:
                    self.child_started[id(frame)] = time.perf_counter()
                    return self._child_trace
            return None
        if event == "line":
            ctx = self.active.get(id(frame))
            if ctx is not None:
                self._line_event(frame, ctx)
            return self.trace if ctx is not None else None
        if event == "return":
            ctx = self.active.get(id(frame))
            if ctx is not None:
                if code_name == "find_exact_expression":
                    self._snapshot(frame, ctx)
                    ctx["return_seconds"] = time.perf_counter() - ctx["search_started"]
                    ctx["return_stats"] = dict(frame.f_locals.get("stats", {}))
                    ctx["search_wall_seconds"] = ctx["return_seconds"]
                    ctx["completed_depths"] = list(frame.f_locals.get("completed", []))
                    self.flush()
                    self.searches.extend(
                        {
                            "label": self.label,
                            "generation": ctx["generation"],
                            "stage": ctx["stage"],
                            "search_id": ctx["id"],
                            "terminal": True,
                            "completed_depths": ctx["completed_depths"],
                            "stats": ctx["return_stats"],
                        }
                        for _ in [0]
                    )
                    self.active.pop(id(frame), None)
                return None
            if code_name in {"dominates", "executable_lineage_ids", "provenance_lineage_ids", "register"}:
                parent = frame.f_back
                pctx = self.active.get(id(parent)) if parent is not None else None
                if pctx is not None:
                    started = self.child_started.pop(id(frame), time.perf_counter())
                    self._return_event(frame, pctx, arg, time.perf_counter() - started)
                return None
        return self.trace if id(frame) in self.active else None

    def _child_trace(self, frame, event, arg):
        if event == "return":
            parent = frame.f_back
            ctx = self.active.get(id(parent)) if parent is not None else None
            if ctx is not None:
                started = self.child_started.pop(id(frame), time.perf_counter())
                self._return_event(frame, ctx, arg, time.perf_counter() - started)
            return None
        return self._child_trace

    def flush(self) -> None:
        payload = {
            "schema": "ACSIE.PR110.phase6.scalability-forensics.v1",
            "label": self.label,
            "seed": SEED,
            "generations_requested": GENERATIONS,
            "runtime_sha": os.environ.get("RUNTIME_SHA"),
            "proving_sha": os.environ.get("PROVING_SHA"),
            "searches": self.searches,
            "checkpoint_timings": self.checkpoint_timings,
            "wall_seconds": time.perf_counter() - self.started,
        }
        self.output_path.write_text(
            json.dumps(payload, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--label", required=True)
    ap.add_argument("--output", required=True)
    ap.add_argument("--checkpoint", required=True)
    args = ap.parse_args()

    trace = ForensicTrace(args.label, Path(args.output))
    original = sys.gettrace()
    sys.settrace(trace.trace)
    exit_code = 0
    result = None
    try:
        result = proving.run_seed(
            SEED,
            GENERATIONS,
            checkpoint_path=args.checkpoint,
            checkpoint_metadata={
                "proving_sha": os.environ["PROVING_SHA"],
                "acsie_ref": os.environ["RUNTIME_SHA"],
            },
        )
    except SystemExit as exc:
        exit_code = int(exc.code or 1)
    except BaseException:
        exit_code = 1
        trace.output_path.with_suffix(".traceback.txt").write_text(
            traceback.format_exc(), encoding="utf-8"
        )
    finally:
        sys.settrace(original)
        trace.finished = True
        trace.flush()

    if result is not None:
        trace.output_path.with_suffix(".result.json").write_text(
            json.dumps(result, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
