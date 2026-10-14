#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import importlib
import json
import os
import subprocess
import sys
import time
from pathlib import Path

from lab.exact_checkpoint import load_checkpoint
from lab.acsie_open_ended_self_extending_proving import run_seed

SEED = 2026100305
GENERATIONS = 12
STOP_AFTER = 7
PROVING_SHA = "33259fc5c69d87b99c604f0050508c433a279daf"
RUNTIME_SHA = "af0c7ef503c51a5bc7872717b263a01f495c855f"
INPUT_CHECKPOINT_SHA256 = "f21654220651001f3ac63f67a983a516ff8ba236732bb58e47f611c41dd87ced"
INPUT_STATE_DIGEST = "e284699d06649788b58071783651f087c57e0511ee10f93948b878d68f49e28a"


def emit(event: str, **payload: object) -> None:
    print(json.dumps({"event": event, **payload}, sort_keys=True), flush=True)


def main() -> int:
    checkpoint = Path(os.environ["CHECKPOINT_PATH"]).resolve()
    expected_root = Path(os.environ["ARM_ROOT"]).resolve()
    label = os.environ["ARM_LABEL"]
    mode = os.environ["ACSIE_DOMINANCE_BITSETS"]
    expected_blob = os.environ["EXPECTED_SEARCH_BLOB"]
    if not checkpoint.is_file():
        raise FileNotFoundError(checkpoint)

    raw_sha = hashlib.sha256(checkpoint.read_bytes()).hexdigest()
    if raw_sha != INPUT_CHECKPOINT_SHA256:
        raise RuntimeError(f"wrong checkpoint bytes: {raw_sha}")
    envelope = load_checkpoint(
        checkpoint,
        expected_seed=SEED,
        expected_metadata={"proving_sha": PROVING_SHA, "acsie_ref": RUNTIME_SHA},
    )
    if envelope.get("state_digest") != INPUT_STATE_DIGEST or envelope.get("generation_next") != 6:
        raise RuntimeError("wrong input state or continuation boundary")

    module = importlib.import_module("cognitive_core.behavioral_search")
    module_path = Path(module.__file__).resolve()
    expected_path = (expected_root / "cognitive_core" / "behavioral_search.py").resolve()
    if module_path != expected_path:
        raise RuntimeError(f"wrong loaded search module: {module_path} != {expected_path}")
    actual_blob = subprocess.check_output(
        ["git", "hash-object", str(module_path)], text=True
    ).strip()
    if actual_blob != expected_blob:
        raise RuntimeError(f"wrong loaded search blob: {actual_blob} != {expected_blob}")

    emit(
        "GEN6_DOMINANCE_PROFILE_START",
        arm=label,
        dominance_mode="bitset-cache" if mode == "1" else "set-reference",
        input_checkpoint_sha256=raw_sha,
        input_state_digest=envelope["state_digest"],
        generation_next=envelope["generation_next"],
        loaded_module_file=str(module_path),
        loaded_module_git_blob_sha1=actual_blob,
        loaded_module_file_sha256=hashlib.sha256(module_path.read_bytes()).hexdigest(),
        evaluator_git_blob_sha1=subprocess.check_output(
            ["git", "hash-object", "lab/acsie_open_ended_self_extending_proving.py"],
            text=True,
        ).strip(),
        python_version=sys.version,
        pythonhashseed=os.environ.get("PYTHONHASHSEED"),
        hash_randomization_flag=sys.flags.hash_randomization,
        max_dominance_comparisons=int(os.environ.get("ACSIE_SEARCH_MAX_DOMINANCE_COMPARISONS", "0")),
        trace_enabled=os.environ.get("ACSIE_SEARCH_TRACE") == "1",
        promotion_status="DO_NOT_PROMOTE",
    )

    started = time.perf_counter()
    try:
        result = run_seed(
            SEED,
            GENERATIONS,
            checkpoint_path=str(checkpoint),
            resume_from=str(checkpoint),
            stop_after=STOP_AFTER,
            checkpoint_metadata={"proving_sha": PROVING_SHA, "acsie_ref": RUNTIME_SHA},
        )
    except SystemExit as exc:
        elapsed = time.perf_counter() - started
        if exc.code != 76:
            emit("UNEXPECTED_SYSTEM_EXIT", arm=label, exit_code=exc.code, elapsed_seconds=elapsed)
            raise
        emit(
            "FIXED_WORK_BUDGET_REACHED",
            arm=label,
            exit_code=76,
            elapsed_seconds=round(elapsed, 6),
            checkpoint_sha256_after=hashlib.sha256(checkpoint.read_bytes()).hexdigest(),
            checkpoint_unchanged=hashlib.sha256(checkpoint.read_bytes()).hexdigest() == INPUT_CHECKPOINT_SHA256,
            promotion_status="DO_NOT_PROMOTE",
        )
        return 0

    emit(
        "PROFILE_RETURNED_BEFORE_CUTOFF",
        arm=label,
        elapsed_seconds=round(time.perf_counter() - started, 6),
        result_status=result.get("status"),
        completed_generations=result.get("completed_generations"),
        checkpoint_sha256_after=hashlib.sha256(checkpoint.read_bytes()).hexdigest(),
        promotion_status="DO_NOT_PROMOTE",
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
