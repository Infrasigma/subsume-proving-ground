#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import resource
import shutil
import time
from pathlib import Path

from lab.acsie_open_ended_self_extending_proving import run_seed
from lab.exact_checkpoint import load_checkpoint


RUNTIME_SHA = "9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be"
PROVING_SHA = "33259fc5c69d87b99c604f0050508c433a279daf"
CHECKPOINTS = ((1, "gen0"), (2, "gen1"), (4, "gen3"), (6, "gen5"), (7, "gen6"))


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--seed", type=int, default=2026100305)
    parser.add_argument("--generations", type=int, default=7)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    if args.generations != 7:
        raise SystemExit("H7 runtime ablation is frozen to generations 0..6 (generations=7)")

    k_label = str(os.environ.get("ACSIE_H7_REPRESENTATIVE_K", "UNBOUNDED")).strip().upper()
    if k_label not in {"UNBOUNDED", "2", "4"}:
        raise SystemExit("ACSIE_H7_REPRESENTATIVE_K must be UNBOUNDED, 2, or 4")

    outdir = Path(args.output)
    outdir.mkdir(parents=True, exist_ok=True)
    tmp_checkpoint = outdir / "checkpoint-current.json"
    metadata = {
        "acsie_ref": RUNTIME_SHA,
        "proving_sha": PROVING_SHA,
        "h7_representative_k": k_label,
    }

    segment_records = []
    current_generation_next = 0
    final_summary = None
    process_start = time.perf_counter()

    for stop_after, label in CHECKPOINTS:
        if stop_after > args.generations:
            continue
        start = time.perf_counter()
        if current_generation_next == 0:
            summary = run_seed(
                args.seed,
                args.generations,
                checkpoint_path=str(tmp_checkpoint),
                stop_after=stop_after,
                checkpoint_metadata=metadata,
            )
        else:
            summary = run_seed(
                args.seed,
                args.generations,
                checkpoint_path=str(tmp_checkpoint),
                resume_from=str(tmp_checkpoint),
                stop_after=stop_after,
                checkpoint_metadata=metadata,
            )
        elapsed = time.perf_counter() - start
        envelope = load_checkpoint(
            tmp_checkpoint,
            expected_seed=args.seed,
            expected_metadata=metadata,
        )
        current_generation_next = int(envelope["generation_next"])
        checkpoint_copy = outdir / f"checkpoint-{label}.json"
        shutil.copy2(tmp_checkpoint, checkpoint_copy)
        segment_records.append({
            "stop_after": stop_after,
            "checkpoint_label": label,
            "generation_next": current_generation_next,
            "checkpoint_digest": envelope["state_digest"],
            "elapsed_seconds": elapsed,
            "max_rss_kb_process": int(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss),
        })
        final_summary = summary

    if final_summary is None or current_generation_next != args.generations:
        raise RuntimeError(
            f"incomplete H7 trajectory: generation_next={current_generation_next}, requested={args.generations}"
        )

    details = final_summary["generations_detail"]
    generation_metrics = []
    for row in details:
        search = dict(row.get("search_stats") or {})
        generation_metrics.append({
            "generation": int(row["generation"]),
            "generated_expressions": int(search.get("generated_expressions", 0)),
            "unique_semantic_states": int(search.get("unique_states", 0)),
            "behavioral_equivalence_classes": int(search.get("behavioral_equivalence_classes", 0)),
            "executable_lineage_representatives": int(
                search.get("h7_current_representatives", 0)
            ),
            "maximum_frontier": int(
                search.get("h7_effective_frontier_max", search.get("dominance_frontier_max", 0))
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
        "schema": "ACSIE.PR110.H7.runtime-ablation.result.v1",
        "runtime_sha": RUNTIME_SHA,
        "proving_sha": PROVING_SHA,
        "seed": args.seed,
        "h7_representative_k": k_label,
        "generations_requested": args.generations,
        "generation_next": current_generation_next,
        "trajectory_exit_code": 0,
        "overall_runtime_seconds": time.perf_counter() - process_start,
        "process_max_rss_kb": int(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss),
        "segment_records": segment_records,
        "generations": generation_metrics,
        "scientific_summary": {
            "closure_reuse_mean": final_summary.get("closure_reuse_gain"),
            "probe_gain": final_summary.get("probe_gain"),
            "recursive_generations": final_summary.get("recursive_generations"),
            "lineage_closed": final_summary.get("lineage_closed"),
            "accepted_generations": final_summary.get("accepted_generations"),
            "completed_generations": final_summary.get("completed_generations"),
        },
        "future_information_used": False,
        "runtime_policy_changed_only": "executable-lineage representative retention",
    }
    (outdir / "result.json").write_text(
        json.dumps(result, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(json.dumps({
        "event": "H7_RUNTIME_ABLATION_COMPLETE",
        "k": k_label,
        "generation_next": current_generation_next,
        "runtime_seconds": result["overall_runtime_seconds"],
        "max_rss_kb": result["process_max_rss_kb"],
    }, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
