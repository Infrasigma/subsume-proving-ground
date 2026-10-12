#!/usr/bin/env python3
"""Summarize a frozen-predecessor Gen3 scaling matrix without promoting partial rows."""
from __future__ import annotations

import argparse
import hashlib
import json
import statistics
from pathlib import Path

EXPECTED_SEEDS = (771221, 771222)
EXPECTED_STREAMS = (3, 6, 12)
EXPECTED_PREDECESSOR = "b31109864ae547320d877e6c1575b805d950edf9"
EXPECTED_ACSIE_COMMIT = "d83cb60286f195eec02b4d7aa45a60539316d115"
EXPECTED_WRAPPER_SCHEMA = "ACSIE.gen3_frozen_scalability_diagnostic.v1"

def load_json(path: Path):
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except Exception:
        return None

def signature_sha(value) -> str:
    raw = json.dumps(value, sort_keys=True, separators=(",", ":"), default=str).encode()
    return hashlib.sha256(raw).hexdigest()

def inspect_cell(root: Path, streams: int, seed: int) -> dict:
    stem = f"gen3-streams-{streams}-seed-{seed}"
    raw_path = root / f"{stem}.json"
    status_path = root / f"{stem}.status.json"
    stderr_path = root / f"{stem}.stderr"
    status = load_json(status_path) or {}
    record = load_json(raw_path) if raw_path.exists() and raw_path.stat().st_size else None
    row = {
        "streams": streams,
        "seed": seed,
        "child_returncode": status.get("child_returncode"),
        "wrapper_status": status.get("wrapper_status"),
        "stdout_bytes": raw_path.stat().st_size if raw_path.exists() else 0,
        "stderr_bytes": stderr_path.stat().st_size if stderr_path.exists() else 0,
        "record_schema": record.get("schema") if isinstance(record, dict) else None,
        "record_present": isinstance(record, dict),
        "record_status": "INCOMPLETE_OR_INVALID",
        "checks": {},
    }
    if not isinstance(record, dict):
        row["record_status"] = "INCONCLUSIVE_NO_COMPLETE_JSON_RECORD"
        row["error"] = status.get("error") or "missing or invalid child JSON"
        return row

    if record.get("schema") != EXPECTED_WRAPPER_SCHEMA:
        row["record_status"] = "CHILD_DIAGNOSTIC_FAILURE_OR_SCHEMA_MISMATCH"
        row["error"] = record.get("error") or record.get("schema")
        row["record"] = record
        return row

    scaled = record.get("scaled_gen3_inner_stream_builds") or []
    integrity = record.get("integrity") or {}
    original = record.get("original_unmodified_gen3_result") or {}
    gen3_integrity = original.get("integrity") or {}
    stream_check = (
        len(scaled) == 1
        and scaled[0].get("scaled_gen3_inner_only") is True
        and scaled[0].get("actual_count") == streams
        and scaled[0].get("requested_count") == 6
        and scaled[0].get("rule") == "parity_relation"
        and scaled[0].get("stream_seed") == seed + 8000
    )
    checks = {
        "seed_matches": record.get("seed") == seed,
        "stream_count_matches": record.get("gen3_inner_stream_count") == streams,
        "historical_predecessor_pinned": record.get("frozen_predecessor_commit") == EXPECTED_PREDECESSOR,
        "current_acsie_source_pinned": record.get("executed_gen3_source_commit") == EXPECTED_ACSIE_COMMIT,
        "single_declared_gen3_inner_group_scaled": stream_check,
        "all_outer_stream_counts_fixed": record.get("all_outer_stream_counts_fixed") is True,
        "wrapper_integrity_clean": all(integrity.get(k) is True for k in (
            "holdout_contamination", "task_routing", "external_model", "network_dependency",
            "outer_streams_unchanged", "exactly_one_target_stream_group_scaled",
        )),
        "science_integrity_clean": all(gen3_integrity.get(k) is False for k in (
            "holdout_contamination", "task_routing", "external_model", "network_dependency",
        )),
        "scientific_status_is_terminal": record.get("gen3_scientific_status") in {"PASSED", "FAILED", "BLOCKED"},
        "signature_digest_matches": record.get("scientific_signature_sha256") == signature_sha(original),
    }
    row["checks"] = checks
    row["all_integrity_and_provenance_checks_pass"] = all(checks.values())
    row["gen3_scientific_status"] = record.get("gen3_scientific_status")
    row["gen3_reason"] = record.get("gen3_reason")
    row["run_elapsed_ms"] = record.get("run_elapsed_ms_including_lightweight_telemetry")
    row["peak_rss_mb"] = record.get("process_peak_rss_mb")
    row["candidate_count"] = record.get("candidate_count")
    row["eligible_count"] = record.get("eligible_count")
    row["holdout_gain"] = record.get("gen3_holdout_gain")
    row["transfer_gain"] = record.get("gen3_transfer_gain")
    row["retention"] = record.get("retention")
    row["scaled_group"] = scaled[0] if len(scaled) == 1 else scaled
    row["scientific_signature_sha256"] = record.get("scientific_signature_sha256")
    if not all(checks.values()):
        row["record_status"] = "INVALID_PIN_OR_INTEGRITY_MISMATCH"
        row["record"] = record
    else:
        row["record_status"] = "COMPLETE_AUDITABLE_SCIENTIFIC_RECORD"
        row["record"] = record
    return row

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--evidence", type=Path, required=True)
    args = ap.parse_args()
    root = args.evidence.resolve()
    verification = load_json(root / "historical-gen2-predecessor-verification.json")
    if not verification or verification.get("scientific_status") != "PASSED_PREDECESSOR_COMPATIBILITY_ONLY":
        raise SystemExit("historical predecessor verification absent or did not pass")

    cells = [inspect_cell(root, s, seed) for s in EXPECTED_STREAMS for seed in EXPECTED_SEEDS]
    complete = [r for r in cells if r["record_status"] == "COMPLETE_AUDITABLE_SCIENTIFIC_RECORD"]
    missing = [r for r in cells if r["record_status"] != "COMPLETE_AUDITABLE_SCIENTIFIC_RECORD"]
    science = [r.get("gen3_scientific_status") for r in complete]
    if missing:
        classification = "INCONCLUSIVE_INCOMPLETE_OR_INVALID_SCALE_MATRIX"
    elif any(s == "FAILED" for s in science):
        classification = "GEN3_SCIENTIFIC_FAILURE_PRESENT"
    elif any(s == "BLOCKED" for s in science):
        classification = "GEN3_BLOCKED_CELL_PRESENT"
    elif len(complete) == len(EXPECTED_STREAMS) * len(EXPECTED_SEEDS) and all(s == "PASSED" for s in science):
        classification = "GEN3_SCIENTIFIC_GATE_PASSED_ALL_HISTORICAL_PREDECESSOR_SCALE_CELLS"
    else:
        classification = "GEN3_MATRIX_COMPLETE_WITH_UNEXPECTED_NONPASS_STATUS"

    by_scale = []
    for stream_count in EXPECTED_STREAMS:
        group = [r for r in complete if r["streams"] == stream_count]
        times = [r["run_elapsed_ms"] for r in group if isinstance(r.get("run_elapsed_ms"), (int, float))]
        rss = [r["peak_rss_mb"] for r in group if isinstance(r.get("peak_rss_mb"), (int, float))]
        candidates = [r["candidate_count"] for r in group if isinstance(r.get("candidate_count"), (int, float))]
        holds = [r["holdout_gain"] for r in group if isinstance(r.get("holdout_gain"), (int, float))]
        transfers = [r["transfer_gain"] for r in group if isinstance(r.get("transfer_gain"), (int, float))]
        by_scale.append({
            "gen3_inner_streams": stream_count,
            "complete_record_count": len(group),
            "seed_statuses": {str(r["seed"]): r.get("gen3_scientific_status") for r in group},
            "all_cells_passed": len(group) == len(EXPECTED_SEEDS) and all(r.get("gen3_scientific_status") == "PASSED" for r in group),
            "median_run_elapsed_ms": round(statistics.median(times), 3) if times else None,
            "median_peak_rss_mb": round(statistics.median(rss), 3) if rss else None,
            "median_candidate_count": statistics.median(candidates) if candidates else None,
            "median_holdout_gain": round(statistics.median(holds), 6) if holds else None,
            "median_transfer_gain": round(statistics.median(transfers), 6) if transfers else None,
        })

    summary = {
        "schema": "ACSIE.gen3_historical_predecessor_scaling_summary.v1",
        "arm_type": "HISTORICAL_VERIFIED_PREDECESSOR_CURRENT_GEN3_COMPATIBILITY_AND_SCALING",
        "classification": classification,
        "scientific_scope": "Bounded current Gen3 continuation from a verified historical Gen2 state; not current Gen2 reproducibility, not frozen 5x12 qualification, not AGI/ASI.",
        "historical_predecessor": {
            "artifact_id": 10950102751,
            "artifact_sha256": verification.get("archive_sha256"),
            "state_sha256": verification.get("state_sha256"),
            "scientific_parent_commit": verification.get("historical_export", {}).get("scientific_parent_commit"),
            "exporter_commit": verification.get("historical_export", {}).get("executed_export_commit"),
            "current_schema_compatibility": verification.get("current_state_compatibility"),
        },
        "current_gen2_exporter": {
            "commit": verification.get("current_gen2_exporter_commit"),
            "disposition": "FAILED_SEPARATE_RESULT_NOT_SUBSTITUTED",
        },
        "acsie_source": verification.get("current_acsie_source"),
        "seeds": list(EXPECTED_SEEDS),
        "gen3_inner_stream_counts": list(EXPECTED_STREAMS),
        "fixed_candidate_program_cap": 128,
        "fixed_dimensions": {
            "Gen1/Gen2 discovery streams": 6,
            "Gen1/Gen2 historical regression streams": 6,
            "Gen3 outer streams": 6,
            "gain_gate": 0.05,
        },
        "expected_cells": len(EXPECTED_SEEDS) * len(EXPECTED_STREAMS),
        "completed_auditable_cells": len(complete),
        "incomplete_or_invalid_cells": len(missing),
        "by_scale": by_scale,
        "cells": cells,
        "no_historical_predecessor_substitution_for_current_exporter": True,
    }
    out = root / "historical-gen3-scaling-summary.json"
    out.write_text(json.dumps(summary, indent=2, sort_keys=True, allow_nan=False) + "\n", encoding="utf-8")
    print(json.dumps({k: v for k, v in summary.items() if k not in {"cells"}}, indent=2, sort_keys=True, allow_nan=False))
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
