#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import json
import os
import sys
import time
from collections import Counter
from pathlib import Path

import cognitive_core.recursive_cognitive_compiler as recursive_module
from cognitive_core.recursive_cognitive_compiler import RecursiveCognitiveCompiler
from lab.exact_checkpoint import load_checkpoint, restore_checkpoint
import lab.acsie_open_ended_self_extending_proving as proving

SEED = 2026100305
GENERATIONS = 6
GENERATION = 5
RUNTIME_SHA = "9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be"
PROVING_SHA = "33259fc5c69d87b99c604f0050508c433a279daf"


def file_record(module):
    path = Path(module.__file__).resolve()
    return {
        "path": str(path),
        "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
    }


class FrontierProfiler:
    """One-shot observer for the first real Gen5 process-frontier call.

    The real methods run unchanged. Instrumentation is in-memory only and stops
    the diagnostic seed immediately after the first frontier has returned.
    No sys.settrace, target-conditioned branch, ranking change, or compression.
    """

    def __init__(self, output: Path, checkpoint: Path):
        self.output = output
        self.checkpoint = checkpoint
        self.started = time.perf_counter()
        self.active = None
        self.selected = False
        self.exprs_records = []
        self.original_exprs = RecursiveCognitiveCompiler._exprs
        self.original_frontier = RecursiveCognitiveCompiler.synthesize_process_frontier

    def write(self, status: str, **extra):
        report = {
            "schema": "ACSIE.PR110.H7.process-frontier-profile.v1",
            "status": status,
            "runtime_sha": RUNTIME_SHA,
            "proving_sha": PROVING_SHA,
            "seed": SEED,
            "generation": GENERATION,
            "generations_requested": GENERATIONS,
            "observer": {
                "mode": "single-real-method-invocation-wrapper",
                "trace_function_enabled": False,
                "runtime_modified_on_disk": False,
                "method_profiled": "RecursiveCognitiveCompiler.synthesize_process_frontier",
                "enumerator_profiled": "RecursiveCognitiveCompiler._exprs",
                "stops_after_first_selected_frontier_return": True,
            },
            "python": {
                "version": sys.version,
                "pythonhashseed": os.environ.get("PYTHONHASHSEED", "unset"),
                "hash_randomization": sys.flags.hash_randomization,
                "sys_path": list(sys.path),
            },
            "modules": {
                "recursive_cognitive_compiler": file_record(recursive_module),
                "proving_evaluator": file_record(proving),
            },
            "checkpoint_sha256": hashlib.sha256(self.checkpoint.read_bytes()).hexdigest(),
            "elapsed_seconds_from_profiler_start": time.perf_counter() - self.started,
            "exprs_invocations": self.exprs_records,
        }
        if self.active is not None:
            report["frontier_call"] = self.active
        report.update(extra)
        self.output.parent.mkdir(parents=True, exist_ok=True)
        self.output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")

    def exprs(self, runtime_self, keys, primitive_ids, depth_limit):
        active = self.active
        is_target_call = (
            active is not None
            and active.get("_compiler_identity") == id(runtime_self)
            and not active.get("_frontier_finished", False)
        )
        if not is_target_call:
            return self.original_exprs(runtime_self, keys, primitive_ids, depth_limit)

        cache_key = (
            tuple(sorted(keys)),
            tuple(sorted(primitive_ids)),
            int(depth_limit),
        )
        cache = getattr(runtime_self, "_expr_cache", {})
        record = {
            "depth_limit": int(depth_limit),
            "key_count": len(set(keys)),
            "primitive_id_count": len(set(primitive_ids)),
            "expr_cache_entries_before": len(cache),
            "cache_hit_before_call": cache_key in cache,
        }
        start = time.perf_counter()
        try:
            result = self.original_exprs(runtime_self, keys, primitive_ids, depth_limit)
        except BaseException as exc:
            record.update({
                "status": "CRASHED",
                "elapsed_seconds": time.perf_counter() - start,
                "exception_type": type(exc).__name__,
                "exception": str(exc)[:1000],
                "expr_cache_entries_after": len(getattr(runtime_self, "_expr_cache", {})),
            })
            self.exprs_records.append(record)
            raise
        record.update({
            "status": "RETURNED",
            "elapsed_seconds": time.perf_counter() - start,
            "returned_expression_count": len(result),
            "expr_cache_entries_after": len(getattr(runtime_self, "_expr_cache", {})),
        })
        self.exprs_records.append(record)
        return result

    def frontier(self, runtime_self, rows, transfer, ood, *args, **kwargs):
        if self.selected or int(getattr(runtime_self, "generation", -1)) != GENERATION:
            return self.original_frontier(runtime_self, rows, transfer, ood, *args, **kwargs)
        if type(runtime_self).__name__ != "OpenEndedRecursiveCognitiveCompiler":
            return self.original_frontier(runtime_self, rows, transfer, ood, *args, **kwargs)

        self.selected = True
        caller = sys._getframe(1)
        callsite = {
            "caller_function": caller.f_code.co_name,
            "caller_line": caller.f_lineno,
        }
        for key in ("generation", "pair_idx", "probe_idx"):
            val = caller.f_locals.get(key)
            if isinstance(val, (str, int, float, bool)):
                callsite[key] = val
        fams = Counter(str(getattr(row, "family", "unknown")) for row in rows)
        self.active = {
            "callsite": callsite,
            "compiler_class": type(runtime_self).__name__,
            "generation": int(runtime_self.generation),
            "primitive_count_before": len(getattr(runtime_self, "primitives", {})),
            "process_count_before": len(getattr(runtime_self, "processes", {})),
            "discovery_rows": len(rows),
            "transfer_rows": len(transfer),
            "ood_rows": len(ood),
            "discovery_row_families": dict(sorted(fams.items())),
            "input_key_count": len(set().union(*(row.inputs.keys() for row in rows))) if rows else 0,
            "requested_max_candidates": kwargs.get("max_candidates", args[0] if args else 32),
            "expr_cache_entries_before": len(getattr(runtime_self, "_expr_cache", {})),
            "candidate_selection_summary_before": dict(getattr(runtime_self, "last_process_candidate_selection_summary", {})),
            "_compiler_identity": id(runtime_self),
            "_frontier_finished": False,
        }
        start = time.perf_counter()
        try:
            result = self.original_frontier(runtime_self, rows, transfer, ood, *args, **kwargs)
        except BaseException as exc:
            self.active["_frontier_finished"] = True
            self.active.update({
                "status": "CRASHED",
                "elapsed_seconds": time.perf_counter() - start,
                "exception_type": type(exc).__name__,
                "exception": str(exc)[:1500],
                "primitive_count_after": len(getattr(runtime_self, "primitives", {})),
                "process_count_after": len(getattr(runtime_self, "processes", {})),
                "expr_cache_entries_after": len(getattr(runtime_self, "_expr_cache", {})),
                "candidate_selection_summary_after": dict(getattr(runtime_self, "last_process_candidate_selection_summary", {})),
            })
            self.write("PROCESS_FRONTIER_CAPTURED_ERROR")
            raise
        self.active["_frontier_finished"] = True
        self.active.update({
            "status": "RETURNED",
            "elapsed_seconds": time.perf_counter() - start,
            "retained_frontier_count": len(result),
            "retained_process_ids": [str(p.process_id) for p in result],
            "retained_used_primitive_sets": [list(p.used_primitives) for p in result],
            "retained_complexities": [int(p.complexity) for p in result],
            "primitive_count_after": len(getattr(runtime_self, "primitives", {})),
            "process_count_after": len(getattr(runtime_self, "processes", {})),
            "expr_cache_entries_after": len(getattr(runtime_self, "_expr_cache", {})),
            "candidate_selection_summary_after": dict(getattr(runtime_self, "last_process_candidate_selection_summary", {})),
        })
        # The wrapper observes the method after its result has been computed.
        # The return value is deliberately not consumed by the following learner
        # code: this is a bounded capture from a fresh checkpoint-resume process.
        self.write("CAPTURED_FIRST_PROCESS_FRONTIER")
        raise SystemExit(0)

    def install(self):
        RecursiveCognitiveCompiler._exprs = self.exprs
        RecursiveCognitiveCompiler.synthesize_process_frontier = self.frontier

    def restore(self):
        RecursiveCognitiveCompiler._exprs = self.original_exprs
        RecursiveCognitiveCompiler.synthesize_process_frontier = self.original_frontier


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--checkpoint", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    checkpoint = Path(args.checkpoint)
    output = Path(args.output)

    envelope = load_checkpoint(
        checkpoint,
        expected_seed=SEED,
        expected_metadata={"proving_sha": PROVING_SHA, "acsie_ref": RUNTIME_SHA},
    )
    if int(envelope["generation_next"]) != GENERATION:
        raise ValueError(f"expected generation_next={GENERATION}, got {envelope['generation_next']}")
    restored = restore_checkpoint(
        envelope,
        expected_metadata={"proving_sha": PROVING_SHA, "acsie_ref": RUNTIME_SHA},
    )
    if int(restored["learner"].generation) != GENERATION - 1:
        raise ValueError(f"unexpected restored learner generation {restored['learner'].generation}")

    profiler = FrontierProfiler(output, checkpoint)
    profiler.install()
    exit_code = 0
    try:
        proving.run_seed(
            SEED,
            GENERATIONS,
            resume_from=str(checkpoint),
            checkpoint_metadata={"proving_sha": PROVING_SHA, "acsie_ref": RUNTIME_SHA},
        )
    except SystemExit as exc:
        exit_code = 0 if exc.code is None else int(exc.code)
    finally:
        profiler.restore()
    if not profiler.selected:
        profiler.write("NO_GEN5_PROCESS_FRONTIER_REACHED", exit_code=exit_code)
        raise RuntimeError("run_seed finished without reaching the first Gen5 process frontier")
    if exit_code != 0:
        raise RuntimeError(f"diagnostic run exited unexpectedly: {exit_code}")
    print(json.dumps({
        "status": "CAPTURED_FIRST_PROCESS_FRONTIER",
        "generation": GENERATION,
        "frontier_elapsed_seconds": profiler.active.get("elapsed_seconds"),
        "exprs_calls": len(profiler.exprs_records),
        "exprs_seconds_total": sum(float(r.get("elapsed_seconds", 0.0)) for r in profiler.exprs_records),
        "exprs_cache_hits": sum(bool(r.get("cache_hit_before_call")) for r in profiler.exprs_records),
        "expr_count_returned_by_exprs": [r.get("returned_expression_count") for r in profiler.exprs_records],
        "candidate_summary": profiler.active.get("candidate_selection_summary_after"),
        "report": str(output),
    }, sort_keys=True))


if __name__ == "__main__":
    main()
