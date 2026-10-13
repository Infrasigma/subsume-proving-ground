#!/usr/bin/env python3
"""Candidate RC1 driver for D-versus-RC0 decision-trace parity on the saved PR110 Gen7 state.

Bounded primary-process synthesis profiler for the saved PR110 Gen7 state.

Only the separate laboratory snapshot F is imported. The driver stops after a
fixed number of deterministic search operations, preserving the generation-7
checkpoint and leaving the frozen scientific gate untouched.
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
from lab.exact_checkpoint import load_checkpoint

SEED = 2026100305
GENERATIONS = 12
PROVING_SHA = "33259fc5c69d87b99c604f0050508c433a279daf"
RUNTIME_LABEL = "af0c7ef503c51a5bc7872717b263a01f495c855f"
CHECKPOINT = Path("evidence/working-checkpoint.json")


def emit(event: str, **values: object) -> None:
    print(json.dumps({"event": event, **values}, sort_keys=True), flush=True)


def main() -> int:
    if os.environ.get("ACSIE_SEARCH_PROFILE") != "1":
        raise RuntimeError("ACSIE_SEARCH_PROFILE=1 is required.")
    if os.environ.get("ACSIE_SEARCH_PROFILE_PHASE") != "primary_process_synthesis":
        raise RuntimeError("profile phase must be primary_process_synthesis.")
    if os.environ.get("ACSIE_ACTIVE_SYNTHESIS_PHASE"):
        raise RuntimeError("active synthesis phase must be unset before run_seed.")

    expected_checkpoint_sha = "666cc42ee22a92d51f8d55ca77b82b55b831a58f77ee23935be3c42d22a05fc4"
    expected_state = "cfe3391d48284e41416ef3241050b43f4be6df9fc6bfc04dc35f2c0726d3a37b"
    if not CHECKPOINT.is_file():
        raise FileNotFoundError(CHECKPOINT)
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

    module = importlib.import_module("cognitive_core.behavioral_search")
    runtime_path = Path(module.__file__).resolve()
    expected_runtime_path = (
        Path("research/pr110_acsie_snapshot/F/cognitive_core/behavioral_search.py").resolve()
    )
    if runtime_path != expected_runtime_path:
        raise RuntimeError(f"wrong imported runtime: {runtime_path} != {expected_runtime_path}")

    emit(
        "PRIMARY_SEARCH_PROFILE_START",
        seed=SEED,
        checkpoint_raw_sha256=checkpoint_sha,
        checkpoint_state_digest=envelope["state_digest"],
        generation_next=envelope["generation_next"],
        runtime_module_file=str(runtime_path),
        runtime_file_sha256=hashlib.sha256(runtime_path.read_bytes()).hexdigest(),
        evaluator_file_sha256=hashlib.sha256(
            Path("lab/acsie_open_ended_self_extending_proving.py").read_bytes()
        ).hexdigest(),
        pythonhashseed=os.environ.get("PYTHONHASHSEED"),
        hash_randomization_flag=sys.flags.hash_randomization,
        max_dominance_comparisons=int(os.environ.get("ACSIE_SEARCH_MAX_DOMINANCE_COMPARISONS", "0")),
        max_register_calls=int(os.environ.get("ACSIE_SEARCH_MAX_REGISTER_CALLS", "0")),
        progress_interval=int(os.environ.get("ACSIE_SEARCH_PROGRESS_INTERVAL", "0")),
        frozen_5x12_budget="UNTOUCHED",
        promotion_status="DO_NOT_PROMOTE",
    )

    start = time.perf_counter()
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
        elapsed = time.perf_counter() - start
        if exc.code == 76:
            emit(
                "FIXED_WORK_BUDGET_REACHED",
                elapsed_seconds=round(elapsed, 6),
                process_exit_code=76,
                checkpoint_raw_sha256=hashlib.sha256(CHECKPOINT.read_bytes()).hexdigest(),
                frozen_5x12_budget="UNTOUCHED",
                promotion_status="DO_NOT_PROMOTE",
            )
            return 0
        emit(
            "UNEXPECTED_SYSTEM_EXIT",
            exit_code=exc.code,
            elapsed_seconds=round(elapsed, 6),
        )
        raise

    emit(
        "PROFILE_RETURNED_WITHOUT_WORK_CUTOFF",
        elapsed_seconds=round(time.perf_counter() - start, 6),
        completed_generations=result.get("completed_generations"),
        result_status=result.get("status"),
        frozen_5x12_budget="UNTOUCHED",
        promotion_status="DO_NOT_PROMOTE",
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
