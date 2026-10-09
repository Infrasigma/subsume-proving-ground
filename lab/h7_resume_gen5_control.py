#!/usr/bin/env python3
"""Continue the exact PR110 H7 UNBOUNDED control from its verified Gen5 checkpoint.

This is proving-ground continuation infrastructure only. It does not change the
ACSIE runtime, evaluator, search policy, target sequence, seed, or scientific gate.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import resource
import shutil
import time
from pathlib import Path
from typing import Any

from lab.acsie_open_ended_self_extending_proving import run_seed
from lab.exact_checkpoint import load_checkpoint


RUNTIME_SHA = "9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be"
EVALUATOR_SHA = "33259fc5c69d87b99c604f0050508c433a279daf"
SEED = 2026100305
SOURCE_RUN_ID = 37828029305
SOURCE_ARTIFACT_ID = 11583669251
SOURCE_HEAD_SHA = "cd710c18584d0956dcdd05d1612ab4a8fe9dd6a1"
SOURCE_CHECKPOINT_SHA256 = (
    "325f0be706ccfdba1aa2c850752610f1bc860162eba4dd208da7ffc8c8f4034d"
)
SOURCE_STATE_DIGEST = (
    "74af36700d1a857a97cf1f14f890e745975faa579f8a333380e6020f9fd9215b"
)
METADATA = {
    "acsie_ref": RUNTIME_SHA,
    "proving_sha": EVALUATOR_SHA,
    "h7_representative_k": "UNBOUNDED",
}


def file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--checkpoint", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    policy = os.environ.get("ACSIE_H7_REPRESENTATIVE_K", "").strip().upper()
    if policy != "UNBOUNDED":
        raise SystemExit(f"expected exact UNBOUNDED control, got {policy!r}")

    source = Path(args.checkpoint)
    if not source.is_file():
        raise SystemExit(f"missing source checkpoint: {source}")
    actual_file_sha = file_sha256(source)
    if actual_file_sha != SOURCE_CHECKPOINT_SHA256:
        raise SystemExit(
            "source checkpoint raw-byte digest mismatch: "
            f"expected={SOURCE_CHECKPOINT_SHA256} actual={actual_file_sha}"
        )

    source_envelope = load_checkpoint(
        source,
        expected_seed=SEED,
        expected_metadata=METADATA,
    )
    if int(source_envelope["generation_next"]) != 6:
        raise SystemExit(
            "source checkpoint is not the expected Gen5 boundary: "
            f"generation_next={source_envelope['generation_next']}"
        )
    if source_envelope["state_digest"] != SOURCE_STATE_DIGEST:
        raise SystemExit(
            "source checkpoint internal state digest mismatch: "
            f"{source_envelope['state_digest']}"
        )

    gen5_copy = source.parent / "checkpoint-gen5.json"
    if gen5_copy.exists():
        if file_sha256(gen5_copy) != SOURCE_CHECKPOINT_SHA256:
            raise SystemExit("checkpoint-gen5.json does not match the pinned Gen5 bytes")

    outdir = Path(args.output)
    outdir.mkdir(parents=True, exist_ok=True)
    input_copy = outdir / "checkpoint-gen5-input.json"
    shutil.copy2(source, input_copy)
    if file_sha256(input_copy) != SOURCE_CHECKPOINT_SHA256:
        raise SystemExit("preserved Gen5 input checkpoint failed raw-byte verification")

    # Preserve earlier checkpoints from the same uploaded artifact when present.
    for label in ("gen0", "gen1", "gen3", "gen5"):
        candidate = source.parent / f"checkpoint-{label}.json"
        if candidate.is_file():
            shutil.copy2(candidate, outdir / f"source-checkpoint-{label}.json")

    current_checkpoint = outdir / "checkpoint-current.json"
    shutil.copy2(input_copy, current_checkpoint)

    started = time.perf_counter()
    summary = run_seed(
        SEED,
        7,
        checkpoint_path=str(current_checkpoint),
        resume_from=str(current_checkpoint),
        stop_after=7,
        checkpoint_metadata=METADATA,
    )
    elapsed = time.perf_counter() - started

    final_envelope = load_checkpoint(
        current_checkpoint,
        expected_seed=SEED,
        expected_metadata=METADATA,
    )
    generation_next = int(final_envelope["generation_next"])
    if generation_next != 7:
        raise RuntimeError(
            f"Gen6 not completed: generation_next={generation_next}, expected 7"
        )

    details = list(summary.get("generations_detail") or [])
    generations = [int(row["generation"]) for row in details]
    if generations != list(range(7)):
        raise RuntimeError(
            f"completed trajectory history is incomplete or reordered: {generations}"
        )
    if int(summary.get("completed_generations", 0)) != 7:
        raise RuntimeError(
            "run_seed summary does not confirm all seven generations: "
            f"{summary.get('completed_generations')}"
        )

    shutil.copy2(current_checkpoint, outdir / "checkpoint-gen6.json")
    generation_metrics: list[dict[str, Any]] = []
    for row in details:
        search = dict(row.get("search_stats") or {})
        generation_metrics.append({
            "generation": int(row["generation"]),
            "generated_expressions": int(search.get("generated_expressions", 0)),
            "unique_semantic_states": int(search.get("unique_states", 0)),
            "behavioral_equivalence_classes": int(
                search.get("behavioral_equivalence_classes", 0)
            ),
            "executable_lineage_representatives": int(
                search.get("h7_current_representatives", 0)
            ),
            "maximum_frontier": int(
                search.get(
                    "h7_effective_frontier_max",
                    search.get("dominance_frontier_max", 0),
                )
            ),
            "dominance_pruned_states": int(search.get("dominance_pruned_states", 0)),
            "representatives_removed": int(search.get("h7_representatives_removed", 0)),
            "lineage_ids_represented": int(search.get("h7_lineage_ids_represented", 0)),
            "lineage_ids_lost": int(search.get("h7_lineage_ids_lost", 0)),
            "maximum_lineage_size": int(search.get("max_lineage_coverage", 0)),
            "closure_reuse": float(row.get("closure_reuse_rate", 0.0)),
            "probe_reuse": float(row.get("probe_reuse_rate", 0.0)),
            "recursive_reuse": bool(row.get("recursive_reuse_ok", False)),
            "lineage_ok": bool(row.get("lineage_ok", False)),
            "retained_capabilities": int(row.get("retained_count", 0)),
            "generation_next": int(row["generation"]) + 1,
            "target_depth": int(row.get("target_depth", 0)),
            "status": search.get("status"),
            "h7_representative_k": search.get("h7_representative_k"),
        })

    result = {
        "schema": "ACSIE.PR110.H7.unbounded-control-resume.v1",
        "classification": "COMPLETED_GEN6" if generation_next == 7 else "INCOMPLETE",
        "trajectory_mode": "exact_checkpoint_continuation",
        "scientific_mechanism_change": "NONE",
        "source": {
            "repository": "Infrasigma/subsume-proving-ground",
            "workflow_run_id": SOURCE_RUN_ID,
            "artifact_id": SOURCE_ARTIFACT_ID,
            "artifact_name": "pr110-h7-runtime-CONTROL",
            "source_head_sha": SOURCE_HEAD_SHA,
            "source_checkpoint_raw_sha256": SOURCE_CHECKPOINT_SHA256,
            "source_checkpoint_state_digest": SOURCE_STATE_DIGEST,
            "source_generation_next": 6,
        },
        "runtime_sha": RUNTIME_SHA,
        "evaluator_sha": EVALUATOR_SHA,
        "seed": SEED,
        "pythonhashseed": os.environ.get("PYTHONHASHSEED"),
        "h7_representative_k": policy,
        "generations_requested": 7,
        "generation_completed": 6,
        "generation_next": generation_next,
        "trajectory_exit_code": 0,
        "resume_elapsed_seconds": elapsed,
        "process_max_rss_kb": int(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss),
        "final_checkpoint_raw_sha256": file_sha256(current_checkpoint),
        "final_checkpoint_state_digest": final_envelope["state_digest"],
        "generations": generation_metrics,
        "scientific_summary": {
            "closure_reuse_mean": summary.get("closure_reuse_gain"),
            "probe_gain": summary.get("probe_gain"),
            "recursive_generations": summary.get("recursive_generations"),
            "lineage_closed": summary.get("lineage_closed"),
            "accepted_generations": summary.get("accepted_generations"),
            "completed_generations": summary.get("completed_generations"),
            "every_generation_accepted": summary.get("every_generation_accepted"),
        },
        "future_information_used": False,
    }
    (outdir / "result.json").write_text(
        json.dumps(result, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(json.dumps({
        "event": "H7_UNBOUNDED_CHECKPOINT_CONTINUATION_COMPLETE",
        "generation_next": generation_next,
        "gen6_recursive_reuse": generation_metrics[-1]["recursive_reuse"],
        "gen6_lineage_ok": generation_metrics[-1]["lineage_ok"],
        "resume_elapsed_seconds": elapsed,
        "checkpoint_sha256": result["final_checkpoint_raw_sha256"],
        "state_digest": final_envelope["state_digest"],
    }, sort_keys=True), flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
