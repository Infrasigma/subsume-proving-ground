#!/usr/bin/env python3
"""Post-selection audit of expected-parent function classes in executable lineage.

This diagnostic does not replace the exact parent-ID gate or alter runtime
selection/admission. It reports whether executable macro ancestry contains a
symbolically normalized function representative for each distinct expected
parent function class.
"""
from __future__ import annotations
import argparse
import json
from pathlib import Path
from typing import Any
import sympy as sp
from lab.pr110_gen6_symbolic_equivalence_audit import AuditError, as_sympy, sha256

SCHEMA = "ACSIE.PR110.Gen6ParentFunctionClassCoverage.v1"
EVENT = "CLOSURE_LINEAGE_MISMATCH"
EXPECTED_RAW_SHA256 = "74caa5a04008a118d897ca41d03e50089a9bfc5fb67ce94579ed02977531f3a2"
EXPECTED_CHECKPOINT_SHA256 = "22828e4d70d7728c87e887dd8e507a4fe43240a154117edcce9f899d218838ee"


def signature(expr: Any, macros: dict[str, dict[str, Any]]) -> tuple[str, str]:
    normalized = sp.simplify(as_sympy(expr, macros))
    serialized = sp.srepr(normalized)
    return sha256(serialized.encode()), str(normalized)


def summarize(raw: bytes, checkpoint_bytes: bytes, *, require_pinned_hashes: bool = True) -> dict[str, Any]:
    raw_hash, checkpoint_hash = sha256(raw), sha256(checkpoint_bytes)
    if require_pinned_hashes and raw_hash != EXPECTED_RAW_SHA256:
        raise AuditError("raw log SHA mismatch")
    if require_pinned_hashes and checkpoint_hash != EXPECTED_CHECKPOINT_SHA256:
        raise AuditError("Gen6 checkpoint SHA mismatch")
    checkpoint = json.loads(checkpoint_bytes)
    if checkpoint.get("seed") != 2026100305 or checkpoint.get("generation_next") != 7:
        raise AuditError("wrong Gen6 checkpoint")
    learner = checkpoint.get("learner_state", {})
    primitives, retained = learner.get("primitives"), checkpoint.get("retained")
    if not isinstance(primitives, dict) or not isinstance(retained, list):
        raise AuditError("checkpoint primitives/retained missing")
    runtime_macros = {str(pid): row["expression"] for pid, row in primitives.items()
                      if isinstance(row, dict) and isinstance(row.get("expression"), dict)}
    hidden_macros = {str(row["hidden_id"]): row["expression"] for row in retained
                     if isinstance(row, dict) and isinstance(row.get("hidden_id"), str)
                     and isinstance(row.get("expression"), dict)}
    primitive_to_hidden = {str(row["primitive_id"]): str(row["hidden_id"]) for row in retained
                           if isinstance(row, dict) and isinstance(row.get("primitive_id"), str)
                           and isinstance(row.get("hidden_id"), str)}
    runtime_signatures = {}
    for pid in runtime_macros:
        try:
            runtime_signatures[pid] = signature({"op": "macro", "id": pid}, runtime_macros)[0]
        except Exception:
            continue

    rows, malformed = [], 0
    for line_no, line in enumerate(raw.decode("utf-8", errors="replace").splitlines(), 1):
        if not line.strip():
            continue
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            malformed += 1
            continue
        if not isinstance(event, dict) or event.get("event") != EVENT:
            continue
        expected, frontier = event.get("expected_parent_ids"), event.get("frontier_candidate_audit")
        if not isinstance(expected, list) or len(expected) != 2 or not isinstance(frontier, list):
            rows.append({"source_line": line_no, "status": "INCOMPLETE_EVENT"})
            continue
        try:
            selected = [c for c in frontier if isinstance(c, dict) and c.get("is_preselected_candidate") is True]
            if len(selected) != 1:
                raise AuditError("expected one preselected candidate")
            candidate = selected[0]
            lineage = candidate.get("candidate_lineage")
            if not isinstance(lineage, list):
                raise AuditError("candidate executable lineage missing")
            parent_classes = []
            for pid in expected:
                hid = primitive_to_hidden[str(pid)]
                sig, text = signature({"op": "macro", "id": hid}, hidden_macros)
                parent_classes.append({"parent_id": str(pid), "hidden_id": hid,
                                       "function_signature": sig, "normalized_function": text})
            unique_classes = {}
            for parent in parent_classes:
                unique_classes.setdefault(parent["function_signature"], parent["normalized_function"])
            lineage_ids = sorted(set(map(str, lineage)))
            matches = {sig: sorted(pid for pid in lineage_ids if runtime_signatures.get(pid) == sig)
                       for sig in unique_classes}
            covered = all(bool(matches[sig]) for sig in unique_classes)
            strict_ids_pass = set(map(str, expected)).issubset(set(lineage_ids))
            unknown_ids = sorted(pid for pid in lineage_ids if pid not in runtime_signatures)
            rows.append({
                "source_line": line_no, "generation": event.get("generation"),
                "selected_process_id": candidate.get("process_id"),
                "selected_candidate_competent": candidate.get("closure_competent"),
                "expected_parent_ids": list(map(str, expected)),
                "selected_executable_lineage_ids": lineage_ids,
                "selected_contains_exact_expected_parent_ids": strict_ids_pass,
                "expected_parent_count": len(expected),
                "unique_expected_function_class_count": len(unique_classes),
                "expected_parent_function_classes": parent_classes,
                "matching_executable_lineage_ids_by_function_class": matches,
                "all_unique_expected_parent_function_classes_covered": covered,
                "unresolved_or_unsupported_lineage_primitive_ids": unknown_ids,
                "status": "FUNCTION_CLASSES_COVERED_EXACT_IDS_MISMATCH" if covered and not strict_ids_pass
                          else "FUNCTION_CLASSES_AND_EXACT_IDS_COVERED" if covered
                          else "FUNCTION_CLASS_NOT_COVERED",
                "used_for_runtime_selection": False, "used_for_admission_or_gate": False,
            })
        except Exception as exc:
            rows.append({"source_line": line_no, "generation": event.get("generation"),
                         "status": "AUDIT_ERROR", "error_type": type(exc).__name__,
                         "error": str(exc)[:300], "used_for_runtime_selection": False,
                         "used_for_admission_or_gate": False})
    covered_rows = [row for row in rows if row.get("all_unique_expected_parent_function_classes_covered") is True]
    strict_rows = [row for row in rows if row.get("selected_contains_exact_expected_parent_ids") is True]
    return {
        "schema": SCHEMA, "source_raw_log_sha256": raw_hash,
        "source_gen6_checkpoint_sha256": checkpoint_hash, "sympy_version": sp.__version__,
        "event_count": len(rows), "function_class_coverage_count": len(covered_rows),
        "exact_expected_parent_id_lineage_count": len(strict_rows),
        "function_classes_covered_but_exact_ids_mismatch_count": sum(
            row.get("status") == "FUNCTION_CLASSES_COVERED_EXACT_IDS_MISMATCH" for row in rows),
        "audit_error_count": sum(row.get("status") == "AUDIT_ERROR" for row in rows),
        "malformed_jsonl_lines": malformed,
        "all_unique_parent_function_classes_covered": bool(rows) and len(covered_rows) == len(rows),
        "runtime_selection_changed": False, "qualification_gate_changed": False,
        "strict_exact_parent_lineage_gate_reinterpreted": False,
        "interpretation": "Evaluator-only metric: ancestry contains at least one macro functionally matching each distinct expected parent function. It does not satisfy or replace exact parent-ID lineage qualification.",
        "rows": rows,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("raw_log", type=Path)
    parser.add_argument("gen6_checkpoint", type=Path)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--allow-non-pinned-inputs", action="store_true")
    args = parser.parse_args()
    result = summarize(args.raw_log.read_bytes(), args.gen6_checkpoint.read_bytes(),
                       require_pinned_hashes=not args.allow_non_pinned_inputs)
    rendered = json.dumps(result, sort_keys=True, indent=2) + "\n"
    if args.output:
        args.output.write_text(rendered, encoding="utf-8")
    else:
        print(rendered, end="")
    return 0 if result["event_count"] > 0 and result["all_unique_parent_function_classes_covered"]
        and result["audit_error_count"] == 0 else 2


if __name__ == "__main__":
    raise SystemExit(main())
