#!/usr/bin/env python3
"""Run one immutable Gen7 closure synthesis and stop after the frontier is materialized."""
from __future__ import annotations

import hashlib
import importlib
import json
import os
import sys
import time
from pathlib import Path

from lab.pr110_closure_policy_quotient_evaluator import run_seed
from lab.exact_checkpoint import load_checkpoint

SEED = 2026100305
GENERATIONS = 12
PROVING_SHA = "33259fc5c69d87b99c604f0050508c433a279daf"
RUNTIME_LABEL = "af0c7ef503c51a5bc7872717b263a01f495c855f"
CHECKPOINT = Path("evidence/working-checkpoint.json")


def emit(event: str, **values: object) -> None:
    print(json.dumps({"event": event, **values}, sort_keys=True), flush=True)


def main() -> int:
    if os.environ.get("ACSIE_DIAGNOSTIC_STOP_AFTER_FIRST_CLOSURE_FRONTIER") != "1":
        raise RuntimeError("Refusing to run without ACSIE_DIAGNOSTIC_STOP_AFTER_FIRST_CLOSURE_FRONTIER=1.")
    if not CHECKPOINT.is_file():
        raise FileNotFoundError(CHECKPOINT)

    expected_checkpoint_sha = "666cc42ee22a92d51f8d55ca77b82b55b831a58f77ee23935be3c42d22a05fc4"
    expected_state = "cfe3391d48284e41416ef3241050b43f4be6df9fc6bfc04dc35f2c0726d3a37b"
    checkpoint_sha = hashlib.sha256(CHECKPOINT.read_bytes()).hexdigest()
    if checkpoint_sha != expected_checkpoint_sha:
        raise RuntimeError(f"wrong checkpoint bytes: {checkpoint_sha}")
    envelope = load_checkpoint(
        CHECKPOINT,
        expected_seed=SEED,
        expected_metadata={"proving_sha": PROVING_SHA, "acsie_ref": RUNTIME_LABEL},
    )
    if envelope.get("state_digest") != expected_state or envelope.get("generation_next") != 7:
        raise RuntimeError("wrong checkpoint state or continuation boundary")

    compiler_module = importlib.import_module("cognitive_core.recursive_cognitive_compiler")
    compiler_path = Path(compiler_module.__file__).resolve()
    if compiler_path.name != "recursive_cognitive_compiler.py" or "cognitive_core" not in compiler_path.parts:
        raise RuntimeError(f"unexpected closure compiler import: {compiler_path}")
    evaluator_path = Path("lab/pr110_closure_policy_quotient_evaluator.py")
    emit(
        "CLOSURE_POLICY_QUOTIENT_DRIVER_START",
        seed=SEED,
        checkpoint_raw_sha256=checkpoint_sha,
        checkpoint_state_digest=envelope["state_digest"],
        generation_next=envelope["generation_next"],
        python_version=sys.version,
        python_executable=sys.executable,
        pythonhashseed=os.environ.get("PYTHONHASHSEED"),
        hash_randomization_flag=sys.flags.hash_randomization,
        closure_compiler_module=str(compiler_path),
        closure_compiler_sha256=hashlib.sha256(compiler_path.read_bytes()).hexdigest(),
        evaluator_git_blob_sha1=os.environ.get("ACSIE_EVALUATOR_GIT_BLOB"),
        evaluator_file_sha256=hashlib.sha256(evaluator_path.read_bytes()).hexdigest(),
        active_synthesis_phase=os.environ.get("ACSIE_ACTIVE_SYNTHESIS_PHASE"),
        profiling_enabled=os.environ.get("ACSIE_CLOSURE_FRONTIER_PROFILE") == "1",
        frozen_5x12_budget="UNTOUCHED",
        promotion_status="DO_NOT_PROMOTE",
    )

    started = time.perf_counter()
    try:
        result = run_seed(
            SEED,
            GENERATIONS,
            checkpoint_path=str(CHECKPOINT),
            resume_from=str(CHECKPOINT),
            stop_after=12,
            checkpoint_metadata={"proving_sha": PROVING_SHA, "acsie_ref": RUNTIME_LABEL},
        )
    except SystemExit as exc:
        elapsed = time.perf_counter() - started
        if exc.code == 75:
            emit(
                "CLOSURE_POLICY_QUOTIENT_STOP_CAPTURED",
                elapsed_seconds=round(elapsed, 6),
                process_exit_code=75,
                checkpoint_raw_sha256=hashlib.sha256(CHECKPOINT.read_bytes()).hexdigest(),
                frozen_5x12_budget="UNTOUCHED",
                promotion_status="DO_NOT_PROMOTE",
            )
            return 0
        emit("UNEXPECTED_SYSTEM_EXIT", exit_code=exc.code, elapsed_seconds=round(elapsed, 6))
        raise
    emit(
        "CLOSURE_POLICY_QUOTIENT_STOP_MISSING",
        elapsed_seconds=round(time.perf_counter() - started, 6),
        completed_generations=result.get("completed_generations"),
        result_status=result.get("status"),
        promotion_status="DO_NOT_PROMOTE",
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
