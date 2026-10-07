#!/usr/bin/env python3
"""Two-twin deterministic B replay through generation 6."""

from __future__ import annotations

import hashlib
import json
import random
import sys
from pathlib import Path

from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from lab.acsie_open_ended_self_extending_proving import run_seed
from lab.exact_checkpoint import save_checkpoint

SEED = 2026100305
GENERATIONS = 6
PROVING_SHA = "33259fc5c69d87b99c604f0050508c433a279daf"
ACSIE_REF = "9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be"


def write_initial_checkpoint(path: Path) -> None:
    learner = OpenEndedRecursiveCognitiveCompiler(max_depth=2, population=24, seed=SEED)
    save_checkpoint(
        path,
        seed=SEED,
        generation_next=0,
        initial_depth=2,
        local_rng=random.Random(SEED),
        learner=learner,
        retained=[],
        generations_out=[],
        metadata={"proving_sha": PROVING_SHA, "acsie_ref": ACSIE_REF},
    )


def checkpoint_record(gen: int, path: Path) -> dict:
    blob = path.read_bytes()
    envelope = json.loads(blob.decode("utf-8"))
    generations_out = list(envelope.get("generations_out", []))
    target_digest = generations_out[-1].get("target_digest") if generations_out else None
    return {
        "generation": gen,
        "seed": SEED,
        "generation_next": envelope.get("generation_next"),
        "state_digest": envelope.get("state_digest"),
        "checkpoint_sha256": hashlib.sha256(blob).hexdigest(),
        "checkpoint_bytes": len(blob),
        "target_digest": target_digest,
        "runtime_sha": ACSIE_REF,
        "proving_sha": PROVING_SHA,
    }


def main(out_dir: str) -> None:
    root = Path(out_dir)
    root.mkdir(parents=True, exist_ok=True)

    cp0 = root / "checkpoint-gen0.json"
    write_initial_checkpoint(cp0)
    records = [checkpoint_record(0, cp0)]

    previous = cp0
    for gen in (1, 3, 5, 6):
        current = root / f"checkpoint-gen{gen}.json"
        if gen != 1:
            current.write_bytes(previous.read_bytes())
        result = run_seed(
            SEED,
            GENERATIONS,
            checkpoint_path=str(current),
            resume_from=None if gen == 1 else str(current),
            stop_after=gen,
            checkpoint_metadata={
                "proving_sha": PROVING_SHA,
                "acsie_ref": ACSIE_REF,
            },
        )
        rec = checkpoint_record(gen, current)
        rec["trajectory_exit"] = {
            "status": result.get("status"),
            "generation_count": result.get("generation_count"),
            "resumed_from_checkpoint": result.get("resumed_from_checkpoint"),
            "last_checkpoint_digest": result.get("last_checkpoint_digest"),
            "final_target_depth": result.get("final_target_depth"),
        }
        records.append(rec)
        previous = current

    manifest = {
        "schema": "ACSIE.PR110.B.determinism-twin.v1",
        "seed": SEED,
        "generations": GENERATIONS,
        "runtime_sha": ACSIE_REF,
        "proving_sha": PROVING_SHA,
        "checkpoints": records,
    }
    (root / "twin-manifest.json").write_text(
        json.dumps(manifest, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(json.dumps(manifest, indent=2, sort_keys=True))


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("usage: pr110_b_determinism_twin.py OUT_DIR")
    main(sys.argv[1])
