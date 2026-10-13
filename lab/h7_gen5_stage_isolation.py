#!/usr/bin/env python3
"""Locate the long interval before the second Gen5 probe without line tracing.

Runs the exact repaired-B checkpoint and instruments only function boundaries.
The experiment stops immediately before the learner's secondary search for
Generation 5, probe_idx=1. It does not change function return values, candidate
ordering, thresholds, splits, or runtime files.
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
from cognitive_core.recursive_cognitive_compiler import RecursiveCognitiveCompiler
import cognitive_core.behavioral_search as behavioral_search
import cognitive_core.open_ended_growth as growth
import cognitive_core.recursive_cognitive_compiler as recursive_module
from lab.exact_checkpoint import load_checkpoint, restore_checkpoint
import lab.acsie_open_ended_self_extending_proving as proving

SEED = 2026100305
GENERATIONS = 6
GENERATION = 5
STOP_PROBE_INDEX = 1
RUNTIME_SHA = "9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be"
PROVING_SHA = "33259fc5c69d87b99c604f0050508c433a279daf"


class StopAtTargetBoundary(BaseException):
    pass


def sha256_file(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def compact(value):
    try:
        if value is None or isinstance(value, (str, int, float, bool)):
            return value
        if isinstance(value, (tuple, list, set, frozenset)):
            return {"type": type(value).__name__, "length": len(value)}
        if isinstance(value, dict):
            return {"type": "dict", "length": len(value), "keys": sorted(str(k) for k in value)[:20]}
        attrs = {}
        for key in (
            "process_id", "primitive_id", "accepted", "train_error", "holdout_error",
            "transfer_error", "ood_error", "complexity", "generation",
        ):
            if hasattr(value, key):
                item = getattr(value, key)
                if isinstance(item, (str, int, float, bool)) or item is None:
                    attrs[key] = item
        return {"type": type(value).__name__, "attributes": attrs}
    except Exception as exc:
        return {"summary_error": type(exc).__name__}


class StageObserver:
    def __init__(self, path: Path):
        self.path = path
        self.started = time.perf_counter()
        self.events: list[dict] = []
        self.patches = []
        self.stream = None
        self.boundary_reached = False

    def emit(self, event: str, **kwargs):
        row = {
            "elapsed_seconds": round(time.perf_counter() - self.started, 6),
            "event": event,
            **kwargs,
        }
        self.events.append(row)
        if self.stream is not None:
            self.stream.write(json.dumps(row, sort_keys=True, default=str) + "\n")
            self.stream.flush()
        print(json.dumps(row, sort_keys=True, default=str), flush=True)

    @staticmethod
    def run_seed_context():
        frame = sys._getframe(2)
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

    def install_function(self, owner, attr: str, label: str, *, stop_before_probe1=False):
        original = getattr(owner, attr)
        observer = self

        def wrapped(*args, **kwargs):
            ctx = observer.run_seed_context()
            instance = args[0] if args else None
            inst_generation = getattr(instance, "generation", None)
            if stop_before_probe1 and (
                type(instance).__name__ == "OpenEndedRecursiveCognitiveCompiler"
                and inst_generation == GENERATION
                and ctx.get("generation") == GENERATION
                and ctx.get("probe_idx") == STOP_PROBE_INDEX
            ):
                observer.boundary_reached = True
                observer.emit(
                    "TARGET_BOUNDARY_BEFORE_CALL",
                    stage=label,
                    runtime_class=type(instance).__name__,
                    context=ctx,
                    generation=inst_generation,
                    note="stopped before the probe-1 learner search; original function not executed",
                )
                raise StopAtTargetBoundary()

            start = time.perf_counter()
            observer.emit(
                "STAGE_START",
                stage=label,
                runtime_class=type(instance).__name__ if instance is not None else None,
                generation=inst_generation,
                context=ctx,
            )
            try:
                value = original(*args, **kwargs)
            except BaseException as exc:
                observer.emit(
                    "STAGE_EXCEPTION",
                    stage=label,
                    runtime_class=type(instance).__name__ if instance is not None else None,
                    generation=inst_generation,
                    context=ctx,
                    duration_seconds=round(time.perf_counter() - start, 6),
                    exception_type=type(exc).__name__,
                    exception=str(exc)[:300],
                )
                raise
            observer.emit(
                "STAGE_END",
                stage=label,
                runtime_class=type(instance).__name__ if instance is not None else None,
                generation=inst_generation,
                context=ctx,
                duration_seconds=round(time.perf_counter() - start, 6),
                result=compact(value),
            )
            return value

        setattr(owner, attr, wrapped)
        self.patches.append((owner, attr, original))

    def install(self):
        # Instrument only function boundaries. No sys.settrace/sys.setprofile,
        # no source-file I/O during search, no changes to returned values.
        self.install_function(
            OpenEndedRecursiveCognitiveCompiler, "synthesize_process",
            "learner.synthesize_process", stop_before_probe1=True,
        )
        self.install_function(
            OpenEndedRecursiveCognitiveCompiler, "invent_primitive",
            "learner.invent_primitive",
        )
        self.install_function(
            OpenEndedRecursiveCognitiveCompiler, "retain",
            "learner.retain",
        )
        self.install_function(
            RecursiveCognitiveCompiler, "synthesize_process_frontier",
            "base.synthesize_process_frontier",
        )
        self.install_function(
            RecursiveCognitiveCompiler, "evaluate",
            "compiler.evaluate",
        )
        self.install_function(
            RecursiveCognitiveCompiler, "_exprs",
            "base._exprs",
        )
        for name in (
            "synthesize_discovery_only",
            "semantic_parent_match_count",
            "retained_representation_error",
            "executable_lineage_ids",
        ):
            if hasattr(proving, name):
                self.install_function(proving, name, f"proving.{name}")

    def restore(self):
        for owner, attr, original in reversed(self.patches):
            setattr(owner, attr, original)
        self.patches.clear()

    def finish(self, status: str, **details):
        self.path.parent.mkdir(parents=True, exist_ok=True)
        report = {
            "schema": "ACSIE.H7.Gen5.stage-isolation.v1",
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
                "sys_path": sys.path,
            },
            "modules": {
                "behavioral_search": {
                    "path": str(Path(behavioral_search.__file__).resolve()),
                    "sha256": sha256_file(Path(behavioral_search.__file__).resolve()),
                },
                "open_ended_growth": {
                    "path": str(Path(growth.__file__).resolve()),
                    "sha256": sha256_file(Path(growth.__file__).resolve()),
                },
                "recursive_cognitive_compiler": {
                    "path": str(Path(recursive_module.__file__).resolve()),
                    "sha256": sha256_file(Path(recursive_module.__file__).resolve()),
                },
                "proving_evaluator": {
                    "path": str(Path(proving.__file__).resolve()),
                    "sha256": sha256_file(Path(proving.__file__).resolve()),
                },
            },
            "wall_elapsed_seconds": round(time.perf_counter() - self.started, 6),
            "event_count": len(self.events),
            "events": self.events,
            **details,
        }
        self.path.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        print(json.dumps({
            "status": status,
            "wall_elapsed_seconds": report["wall_elapsed_seconds"],
            "event_count": len(self.events),
            "report_sha256": sha256_file(self.path),
            "report": str(self.path),
        }, sort_keys=True), flush=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--checkpoint", required=True)
    parser.add_argument("--output", required=True)
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

    observer = StageObserver(report_path)
    observer.stream = report_path.with_suffix(".jsonl").open("w", encoding="utf-8", buffering=1)
    observer.emit(
        "RUN_START",
        checkpoint_path=str(checkpoint.resolve()),
        checkpoint_sha256=sha256_file(checkpoint),
        generation_next=envelope["generation_next"],
        runtime_sha=RUNTIME_SHA,
        proving_sha=PROVING_SHA,
    )
    observer.install()
    status = "INCONCLUSIVE"
    detail = {}
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
        detail = {"exception_type": type(exc).__name__, "exception": str(exc)[:1000]}
        if isinstance(exc, KeyboardInterrupt):
            raise
    finally:
        observer.restore()
        observer.stream.close()
        observer.finish(status, **detail)


if __name__ == "__main__":
    main()
