#!/usr/bin/env python3
"""Bounded Gen7 replay that stops after the corrected lineage-mismatch audit.

This diagnostic uses the saved Gen6-produced checkpoint and the pinned B lab
snapshot. It does not alter ACSIE runtime code or the frozen 5x12 gate.
"""
from __future__ import annotations

import hashlib
import importlib
import json
import os
import sys
import time
from pathlib import Path

from lab.acsie_open_ended_self_extending_proving import run_seed


SEED = 2026100305
GENERATIONS = 12
PROVING_SHA = "33259fc5c69d87b99c604f0050508c433a279daf"
RUNTIME_SHA = "af0c7ef503c51a5bc7872717b263a01f495c855f"
CHECKPOINT = Path("evidence/working-checkpoint.json")


def emit(event: str, **payload: object) -> None:
    print(json.dumps({"event": event, **payload}, sort_keys=True), flush=True)


def main() -> int:
    if os.environ.get("ACSIE_DIAGNOSTIC_STOP_AFTER_LINEAGE_MISMATCH") != "1":
        raise RuntimeError("Refusing to run without the diagnostic stop guard enabled.")
    if not CHECKPOINT.is_file():
        raise FileNotFoundError(CHECKPOINT)

    runtime_module = importlib.import_module("cognitive_core.behavioral_search")
    runtime_path = Path(runtime_module.__file__).resolve()
    emit(
        "DIAGNOSTIC_PROCESS_START",
        seed=SEED,
        generations=GENERATIONS,
        checkpoint=str(CHECKPOINT),
        checkpoint_raw_sha256=hashlib.sha256(CHECKPOINT.read_bytes()).hexdigest(),
        imported_runtime_module=str(runtime_path),
        imported_runtime_file_sha256=hashlib.sha256(runtime_path.read_bytes()).hexdigest(),
        python_executable=sys.executable,
        python_version=sys.version,
        pythonhashseed_environment=os.environ.get("PYTHONHASHSEED"),
        hash_randomization_flag=sys.flags.hash_randomization,
        sys_path=sys.path,
        proving_sha=PROVING_SHA,
        checkpoint_runtime_identity=RUNTIME_SHA,
        diagnostic_stop_guard=True,
    )

    started = time.perf_counter()
    try:
        result = run_seed(
            SEED,
            GENERATIONS,
            checkpoint_path=str(CHECKPOINT),
            resume_from=str(CHECKPOINT),
            stop_after=12,
            checkpoint_metadata={
                "proving_sha": PROVING_SHA,
                "acsie_ref": RUNTIME_SHA,
            },
        )
    except SystemExit as exc:
        elapsed = time.perf_counter() - started
        if exc.code != 75:
            emit(
                "DIAGNOSTIC_UNEXPECTED_SYSTEM_EXIT",
                exit_code=exc.code,
                elapsed_seconds=round(elapsed, 6),
            )
            raise
        emit(
            "OBSERVER_DIAGNOSTIC_STOP_CAUGHT",
            exit_code=75,
            elapsed_seconds=round(elapsed, 6),
            reason="The corrected evaluator emitted the first lineage mismatch and requested a stop before primary-process synthesis.",
        )
        return 0

    emit(
        "DIAGNOSTIC_RETURNED_WITHOUT_MISMATCH_STOP",
        elapsed_seconds=round(time.perf_counter() - started, 6),
        completed_generations=result.get("completed_generations"),
        status="REVIEW_REQUIRED",
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
