#!/usr/bin/env python3
"""Read-only aggregation of PR110 Gen6 lineage-identifiability JSONL audits.

This tool deliberately does not change candidate ranking, admission, or gates.
It reports empirical ambiguity from the evaluator-only fresh-input audit and
keeps candidate-instance counts distinct from deduplicated process identifiers.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from collections import Counter
from pathlib import Path
from typing import Any

SCHEMA = "ACSIE.PR110.Gen6LineageIdentifiabilityReport.v1"
EVENT = "CLOSURE_LINEAGE_MISMATCH"
AUDIT_SCHEMA = "ACSIE.lineage-identifiability-audit.v1"
DEFAULT_TOLERANCE = 1e-9


def _parse_jsonl(text: str) -> tuple[list[dict[str, Any]], int]:
    records: list[dict[str, Any]] = []
    malformed = 0
    for line in text.splitlines():
        if not line.strip():
            continue
        try:
            value = json.loads(line)
        except (json.JSONDecodeError, TypeError):
            malformed += 1
            continue
        if isinstance(value, dict):
            records.append(value)
        else:
            malformed += 1
    return records, malformed


def _candidate_exact(candidate: dict[str, Any], tolerance: float) -> bool:
    if not candidate.get("closure_competent_on_declared_validation", False):
        return False
    if not candidate.get("exact_target_match_on_fresh_inputs", False):
        return False
    error = candidate.get("max_absolute_error")
    try:
        return error is not None and float(error) <= tolerance
    except (TypeError, ValueError):
        return False


def summarize(text: str, *, source_sha256: str, tolerance: float = DEFAULT_TOLERANCE) -> dict[str, Any]:
    """Aggregate the auditable, read-only lineage-identifiability events."""
    if tolerance < 0:
        raise ValueError("tolerance must be non-negative")
    records, malformed_lines = _parse_jsonl(text)
    events = [row for row in records if row.get("event") == EVENT]
    rows: list[dict[str, Any]] = []
    status_counts: Counter[str] = Counter()
    all_audited_ambiguous = bool(events)
    all_read_only = True
    all_used_same_sample_count = True
    sample_counts: set[int] = set()
    candidate_instances = 0
    wrong_lineage_instances_recorded = 0
    expected_lineage_instances_recorded = 0
    wrong_lineage_instances_recomputed = 0
    wrong_lineage_process_ids: set[str] = set()
    expected_lineage_process_ids: set[str] = set()

    for event in events:
        audit = event.get("lineage_identifiability_audit")
        if not isinstance(audit, dict) or audit.get("schema") != AUDIT_SCHEMA:
            all_audited_ambiguous = False
            rows.append({
                "seed": event.get("seed"),
                "generation": event.get("generation"),
                "expected_parent_ids": event.get("expected_parent_ids", []),
                "audit_status": "AUDIT_MISSING_OR_SCHEMA_MISMATCH",
            })
            continue

        status = str(audit.get("status", "MISSING_STATUS"))
        status_counts[status] += 1
        all_audited_ambiguous = all_audited_ambiguous and status == "EMPIRICALLY_AMBIGUOUS"
        all_read_only = all_read_only and audit.get("used_for_selection_or_admission") is False and audit.get("runtime_selection_changed") is False
        sample_n = audit.get("sample_count")
        if isinstance(sample_n, int):
            sample_counts.add(sample_n)
        else:
            all_used_same_sample_count = False

        candidates = audit.get("candidate_results")
        if not isinstance(candidates, list):
            candidates = []
            all_used_same_sample_count = False
        candidate_instances += len(candidates)
        wrong_recomputed = 0
        expected_recomputed = 0
        for candidate in candidates:
            if not isinstance(candidate, dict) or not _candidate_exact(candidate, tolerance):
                continue
            has_expected = candidate.get("contains_expected_executable_parents")
            process_id = candidate.get("process_id")
            if has_expected is True:
                expected_recomputed += 1
                if isinstance(process_id, str):
                    expected_lineage_process_ids.add(process_id)
            elif has_expected is False:
                wrong_recomputed += 1
                if isinstance(process_id, str):
                    wrong_lineage_process_ids.add(process_id)

        wrong_recorded = audit.get("competent_wrong_lineage_exact_candidate_count")
        expected_recorded = audit.get("exact_expected_lineage_candidate_count")
        if isinstance(wrong_recorded, int):
            wrong_lineage_instances_recorded += wrong_recorded
        if isinstance(expected_recorded, int):
            expected_lineage_instances_recorded += expected_recorded
        wrong_lineage_instances_recomputed += wrong_recomputed

        rows.append({
            "seed": event.get("seed"),
            "generation": event.get("generation"),
            "expected_parent_ids": event.get("expected_parent_ids", []),
            "selected_process_id": event.get("selected_process_id"),
            "selected_candidate_reuse_ok": event.get("selected_candidate_reuse_ok"),
            "audit_status": status,
            "sample_count": sample_n,
            "candidate_count": len(candidates),
            "wrong_lineage_exact_candidate_instances_recorded": wrong_recorded,
            "wrong_lineage_exact_candidate_instances_recomputed": wrong_recomputed,
            "expected_lineage_exact_candidate_instances_recorded": expected_recorded,
            "expected_lineage_exact_candidate_instances_recomputed": expected_recomputed,
            "input_design_sha256": audit.get("input_design_sha256"),
            "target_output_sha256": audit.get("target_output_sha256"),
            "used_for_selection_or_admission": audit.get("used_for_selection_or_admission"),
            "runtime_selection_changed": audit.get("runtime_selection_changed"),
        })

    if len(sample_counts) > 1:
        all_used_same_sample_count = False
    if malformed_lines:
        parse_status = "RAW_LOG_HAS_MALFORMED_LINES"
    elif not events:
        parse_status = "NO_LINEAGE_MISMATCH_EVENTS_FOUND"
    elif len(rows) != len(events) or any(row.get("audit_status") == "AUDIT_MISSING_OR_SCHEMA_MISMATCH" for row in rows):
        parse_status = "INCOMPLETE_IDENTIFIABILITY_AUDIT"
    else:
        parse_status = "COMPLETE_IDENTIFIABILITY_AUDIT"

    if parse_status != "COMPLETE_IDENTIFIABILITY_AUDIT":
        finding = "INCONCLUSIVE_INCOMPLETE_EVIDENCE"
    elif all_audited_ambiguous:
        finding = "EMPIRICAL_NON_IDENTIFIABILITY_OBSERVED"
    else:
        finding = "NO_UNIVERSAL_AMBIGUITY_FINDING; DOES_NOT_PROVE_IDENTIFIABILITY"

    return {
        "schema": SCHEMA,
        "source_raw_log_sha256": source_sha256,
        "tolerance": tolerance,
        "finding": finding,
        "parse_status": parse_status,
        "event_count": len(events),
        "audit_status_counts": dict(sorted(status_counts.items())),
        "sample_counts": sorted(sample_counts),
        "same_sample_count_for_every_audit": all_used_same_sample_count and len(sample_counts) == 1,
        "candidate_instances_across_audit_rows": candidate_instances,
        "competent_wrong_lineage_exact_candidate_instances_recorded_sum": wrong_lineage_instances_recorded,
        "competent_wrong_lineage_exact_candidate_instances_recomputed_sum": wrong_lineage_instances_recomputed,
        "exact_expected_lineage_candidate_instances_recorded_sum": expected_lineage_instances_recorded,
        "distinct_wrong_lineage_exact_process_ids_seen": len(wrong_lineage_process_ids),
        "distinct_expected_lineage_exact_process_ids_seen": len(expected_lineage_process_ids),
        "candidate_count_warning": "Sums across pair audits are candidate instances, not unique programs; candidates may recur across rows.",
        "selection_invariance_verified": all_read_only,
        "selection_invariance_warning": None if all_read_only else "At least one audit did not explicitly attest that it was read-only.",
        "formal_equivalence_claim": False,
        "interpretation": (
            "A validation-competent candidate with wrong executable-parent lineage matched the hidden target "
            "on every sampled fresh input in each ambiguous row. This is finite-sample empirical ambiguity, "
            "not a proof of mathematical equivalence. Do not use hidden expected parent IDs in runtime selection."
        ),
        "malformed_or_non_object_jsonl_lines": malformed_lines,
        "rows": rows,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input", type=Path, help="Raw Gen6 JSONL log")
    parser.add_argument("--output", type=Path, help="Write JSON report here; otherwise print to stdout")
    parser.add_argument("--tolerance", type=float, default=DEFAULT_TOLERANCE)
    args = parser.parse_args()
    raw = args.input.read_bytes()
    report = summarize(raw.decode("utf-8", errors="replace"), source_sha256=hashlib.sha256(raw).hexdigest(), tolerance=args.tolerance)
    rendered = json.dumps(report, indent=2, sort_keys=True) + "\n"
    if args.output:
        args.output.write_text(rendered, encoding="utf-8")
    else:
        print(rendered, end="")
    return 0 if report["parse_status"] == "COMPLETE_IDENTIFIABILITY_AUDIT" else 2


if __name__ == "__main__":
    raise SystemExit(main())
