#!/usr/bin/env python3
"""Compare original and memoized baseline-selector results from one replay."""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def canonical(value) -> str:
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=True)


def context_key(context: dict) -> tuple:
    return tuple(context.get(k) for k in ("generation", "pair_idx", "probe_idx", "target_depth"))


def original_calls(report: dict) -> list[dict]:
    rows = []
    for event in report.get("events", []):
        if event.get("event") != "STAGE_END" or event.get("stage") != "proving.synthesize_discovery_only":
            continue
        result = event.get("result")
        attrs = result.get("attributes", {}) if isinstance(result, dict) else {}
        rows.append({
            "context": event.get("context", {}),
            "duration_seconds": float(event.get("duration_seconds", 0.0)),
            "process_id": attrs.get("process_id") if isinstance(attrs, dict) else None,
            "complexity": attrs.get("complexity") if isinstance(attrs, dict) else None,
        })
    return rows


def cached_calls(report: dict) -> list[dict]:
    return [
        {
            "context": event.get("context", {}),
            "duration_seconds": float(event.get("duration_seconds", 0.0)),
            "process_id": event.get("result_process_id"),
            "expression_sha256": event.get("result_expression_sha256"),
            "policy": event.get("result_policy"),
            "metrics": event.get("metrics", {}),
        }
        for event in report.get("events", [])
        if event.get("event") == "BASELINE_SELECT_END"
    ]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--baseline-stage", required=True)
    parser.add_argument("--cached-replay", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    baseline_path = Path(args.baseline_stage)
    cached_path = Path(args.cached_replay)
    baseline = json.loads(baseline_path.read_text(encoding="utf-8"))
    cached = json.loads(cached_path.read_text(encoding="utf-8"))

    assert baseline["schema"] == "ACSIE.H7.Gen5.stage-isolation.v1"
    assert baseline["status"] == "BOUNDARY_REACHED_BEFORE_PROBE1_SEARCH"
    assert baseline["runtime_sha"] == cached["runtime_sha"] == "9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be"
    assert baseline["proving_sha"] == cached["proving_sha"] == "33259fc5c69d87b99c604f0050508c433a279daf"
    assert baseline["seed"] == cached["seed"] == 2026100305
    assert baseline["generation"] == cached["generation"] == 5
    assert cached["status"] == "BOUNDARY_REACHED_BEFORE_PROBE1_SEARCH"

    original = original_calls(baseline)
    candidate = cached_calls(cached)
    if not original:
        raise AssertionError("original stage report contains no completed synthesize_discovery_only calls")
    if len(original) != len(candidate):
        raise AssertionError(f"baseline selector invocation count changed: {len(original)} vs {len(candidate)}")

    call_rows = []
    for index, (left, right) in enumerate(zip(original, candidate), 1):
        same_context = context_key(left["context"]) == context_key(right["context"])
        same_process = left["process_id"] == right["process_id"]
        speedup = left["duration_seconds"] / max(right["duration_seconds"], 1e-12)
        row = {
            "index": index,
            "context": left["context"],
            "same_context": same_context,
            "original_process_id": left["process_id"],
            "cached_process_id": right["process_id"],
            "same_selected_process_id": same_process,
            "original_duration_seconds": left["duration_seconds"],
            "cached_duration_seconds": right["duration_seconds"],
            "per_call_speedup_ratio": speedup,
            "cached_expression_sha256": right["expression_sha256"],
            "cached_policy": right["policy"],
            "cache_hits": right["metrics"].get("cache_hits"),
            "cache_misses": right["metrics"].get("cache_misses"),
            "row_evaluation_requests": right["metrics"].get("row_evaluation_requests"),
            "qualified_candidate_count": right["metrics"].get("qualified_candidate_count"),
        }
        if not same_context or not same_process:
            raise AssertionError(f"baseline selector parity failed on call {index}: {row}")
        call_rows.append(row)

    original_seconds = sum(row["duration_seconds"] for row in original)
    cached_seconds = sum(row["duration_seconds"] for row in candidate)
    report = {
        "schema": "ACSIE.H7.cached-baseline-selector-parity.v1",
        "status": "PASSED_SELECTOR_PARITY",
        "baseline_stage_report_sha256": sha256(baseline_path),
        "cached_replay_report_sha256": sha256(cached_path),
        "baseline_call_count": len(original),
        "cached_call_count": len(candidate),
        "all_contexts_and_selected_process_ids_match": True,
        "original_synthesize_discovery_only_total_seconds": original_seconds,
        "cached_synthesize_discovery_only_total_seconds": cached_seconds,
        "selector_stage_speedup_ratio": original_seconds / max(cached_seconds, 1e-12),
        "original_whole_replay_wall_seconds": baseline["wall_elapsed_seconds"],
        "cached_whole_replay_wall_seconds": cached["wall_elapsed_seconds"],
        "whole_replay_speedup_ratio": baseline["wall_elapsed_seconds"] / max(cached["wall_elapsed_seconds"], 1e-12),
        "calls": call_rows,
        "interpretation_boundary": (
            "Selection parity is empirically checked for all baseline-selector calls up to the same probe-1 boundary. "
            "Timing is a bounded single-seed diagnostic, not full Gen5/Gen6 qualification. "
            "No ACSIE runtime, threshold, split, target, or candidate ordering was changed."
        ),
    }
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps({
        "status": report["status"],
        "calls": len(call_rows),
        "baseline_selector_seconds": round(original_seconds, 6),
        "cached_selector_seconds": round(cached_seconds, 6),
        "selector_speedup": round(report["selector_stage_speedup_ratio"], 3),
        "whole_replay_speedup": round(report["whole_replay_speedup_ratio"], 3),
        "report_sha256": sha256(output),
    }, sort_keys=True))


if __name__ == "__main__":
    main()
