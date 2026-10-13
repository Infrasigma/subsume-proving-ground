#!/usr/bin/env python3
"""Replay Gen5 with only baseline evaluator memoization, stopping before probe 1.

The baseline selector's candidate enumeration/order/ranking and returned
ProcessCandidate metadata are kept unchanged. Only repeated pure scalar
expression evaluations are memoized per discovery-row context.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import sys
import time
from pathlib import Path

from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
import cognitive_core.behavioral_search as behavioral_search
import cognitive_core.open_ended_growth as growth
import cognitive_core.recursive_cognitive_compiler as recursive_module
from lab.exact_checkpoint import load_checkpoint, restore_checkpoint
from lab.h7_cached_baseline_eval import synthesize_discovery_only_cached
import lab.acsie_open_ended_self_extending_proving as proving

SEED = 2026100305
GENERATIONS = 6
GENERATION = 5
STOP_PROBE_INDEX = 1
RUNTIME_SHA = "9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be"
PROVING_SHA = "33259fc5c69d87b99c604f0050508c433a279daf"


class StopAtTargetBoundary(BaseException):
    pass


def file_sha(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def loop_context():
    frame = sys._getframe(1)
    while frame is not None:
        if frame.f_code.co_name == "run_seed":
            loc = frame.f_locals
            return {
                "generation": loc.get("generation"),
                "pair_idx": loc.get("pair_idx"),
                "probe_idx": loc.get("probe_idx"),
                "target_depth": loc.get("target_depth"),
            }
        frame = frame.f_back
    return {}


class Replay:
    def __init__(self, report_path: Path, event_path: Path, checkpoint: Path):
        self.report_path = report_path
        self.event_path = event_path
        self.checkpoint = checkpoint
        self.started = time.perf_counter()
        self.events = []
        self.stream = None
        self.select_calls = 0
        self.boundary_reached = False
        self.patches = []

    def emit(self, event, **fields):
        row = {"elapsed_seconds": round(time.perf_counter() - self.started, 6),
               "event": event, **fields}
        self.events.append(row)
        if self.stream is not None:
            self.stream.write(json.dumps(row, sort_keys=True, default=str) + "\n")
            self.stream.flush()
        print(json.dumps(row, sort_keys=True, default=str), flush=True)

    def install(self):
        replay = self
        original_select = proving.synthesize_discovery_only
        original_process = OpenEndedRecursiveCognitiveCompiler.synthesize_process

        def cached_select(compiler, rows, transfer):
            replay.select_calls += 1
            context = loop_context()
            start = time.perf_counter()
            metrics = {}
            replay.emit(
                "BASELINE_SELECT_START",
                invocation_index=replay.select_calls,
                context=context,
                generation=getattr(compiler, "generation", None),
                discovery_rows=len(rows),
                transfer_rows=len(transfer),
                primitive_count=len(getattr(compiler, "primitives", {})),
            )
            try:
                result = synthesize_discovery_only_cached(
                    compiler, rows, transfer, metrics_out=metrics
                )
            except BaseException as exc:
                replay.emit(
                    "BASELINE_SELECT_EXCEPTION",
                    invocation_index=replay.select_calls,
                    context=context,
                    duration_seconds=round(time.perf_counter() - start, 6),
                    exception_type=type(exc).__name__,
                    exception=str(exc)[:500],
                )
                raise
            replay.emit(
                "BASELINE_SELECT_END",
                invocation_index=replay.select_calls,
                context=context,
                duration_seconds=round(time.perf_counter() - start, 6),
                result_process_id=getattr(result, "process_id", None),
                result_expression_sha256=metrics.get("selected_expression_sha256"),
                result_policy=metrics.get("selected_policy"),
                result_used_primitives=metrics.get("selected_used_primitives"),
                metrics=metrics,
            )
            return result

        def stop_before_probe1(self, *args, **kwargs):
            context = loop_context()
            if (
                type(self).__name__ == "OpenEndedRecursiveCognitiveCompiler"
                and getattr(self, "generation", -1) == GENERATION
                and context.get("generation") == GENERATION
                and context.get("probe_idx") == STOP_PROBE_INDEX
            ):
                replay.boundary_reached = True
                replay.emit(
                    "TARGET_BOUNDARY_BEFORE_LEARNER_PROBE1_SEARCH",
                    context=context,
                    learner_generation=getattr(self, "generation", None),
                    baseline_select_calls_completed=replay.select_calls,
                )
                raise StopAtTargetBoundary()
            return original_process(self, *args, **kwargs)

        proving.synthesize_discovery_only = cached_select
        OpenEndedRecursiveCognitiveCompiler.synthesize_process = stop_before_probe1
        self.patches.extend([
            (proving, "synthesize_discovery_only", original_select),
            (OpenEndedRecursiveCognitiveCompiler, "synthesize_process", original_process),
        ])

    def restore(self):
        for owner, name, original in reversed(self.patches):
            setattr(owner, name, original)
        self.patches.clear()

    def finish(self, status, **details):
        self.report_path.parent.mkdir(parents=True, exist_ok=True)
        report = {
            "schema": "ACSIE.H7.cached-baseline-replay.v1",
            "status": status,
            "runtime_sha": RUNTIME_SHA,
            "proving_sha": PROVING_SHA,
            "seed": SEED,
            "generation": GENERATION,
            "stop_probe_index": STOP_PROBE_INDEX,
            "python": {
                "version": sys.version,
                "pythonhashseed": os.environ.get("PYTHONHASHSEED", "unset"),
                "hash_randomization": sys.flags.hash_randomization,
            },
            "modules": {
                "behavioral_search": {"path": str(Path(behavioral_search.__file__).resolve()),
                                      "sha256": file_sha(Path(behavioral_search.__file__).resolve())},
                "open_ended_growth": {"path": str(Path(growth.__file__).resolve()),
                                      "sha256": file_sha(Path(growth.__file__).resolve())},
                "recursive_cognitive_compiler": {"path": str(Path(recursive_module.__file__).resolve()),
                                                  "sha256": file_sha(Path(recursive_module.__file__).resolve())},
                "proving_evaluator": {"path": str(Path(proving.__file__).resolve()),
                                      "sha256": file_sha(Path(proving.__file__).resolve())},
                "cache_helper": {"path": str(Path(__file__).with_name("h7_cached_baseline_eval.py").resolve()),
                                 "sha256": file_sha(Path(__file__).with_name("h7_cached_baseline_eval.py"))},
            },
            "checkpoint_sha256": file_sha(self.checkpoint),
            "wall_elapsed_seconds": round(time.perf_counter() - self.started, 6),
            "baseline_select_call_count": self.select_calls,
            "event_count": len(self.events),
            "events": self.events,
            "interpretation_boundary": (
                "Diagnostic replay through the pre-probe-1 boundary only. "
                "Memoization is in the proving-ground baseline selector; no ACSIE runtime file changes. "
                "A parity pass does not qualify Gen5/Gen6 or establish AGI/ASI."
            ),
            **details,
        }
        self.report_path.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        self.emit(
            "FINAL_REPORT",
            status=status,
            wall_elapsed_seconds=report["wall_elapsed_seconds"],
            baseline_select_call_count=self.select_calls,
            report_sha256=file_sha(self.report_path),
        )


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--checkpoint", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--events", required=True)
    args = parser.parse_args()
    checkpoint = Path(args.checkpoint)
    report_path = Path(args.output)
    envelope = load_checkpoint(
        checkpoint, expected_seed=SEED,
        expected_metadata={"proving_sha": PROVING_SHA, "acsie_ref": RUNTIME_SHA},
    )
    if int(envelope["generation_next"]) != GENERATION:
        raise ValueError(f"expected generation_next={GENERATION}")
    restore_checkpoint(
        envelope,
        expected_metadata={"proving_sha": PROVING_SHA, "acsie_ref": RUNTIME_SHA},
    )

    replay = Replay(report_path, Path(args.events), checkpoint)
    replay.stream = Path(args.events).open("w", encoding="utf-8", buffering=1)
    replay.emit(
        "RUN_START",
        checkpoint_sha256=file_sha(checkpoint),
        generation_next=envelope["generation_next"],
        runtime_sha=RUNTIME_SHA,
        proving_sha=PROVING_SHA,
    )
    replay.install()
    status = "INCONCLUSIVE"
    try:
        proving.run_seed(
            SEED, GENERATIONS, resume_from=str(checkpoint),
            checkpoint_metadata={"proving_sha": PROVING_SHA, "acsie_ref": RUNTIME_SHA},
        )
        status = "TARGET_BOUNDARY_NOT_REACHED"
    except StopAtTargetBoundary:
        status = "BOUNDARY_REACHED_BEFORE_PROBE1_SEARCH"
    except BaseException as exc:
        status = "CRASHED"
        replay.emit("RUN_EXCEPTION", exception_type=type(exc).__name__, exception=str(exc)[:1000])
        if isinstance(exc, KeyboardInterrupt):
            raise
    finally:
        replay.restore()
        replay.stream.close()
        replay.finish(status)


if __name__ == "__main__":
    main()
