#!/usr/bin/env python3
"""Post-selection symbolic audit for Gen6 lineage-mismatch events.

The audit does not change selection, admission, or qualification. It reconstructs
the hidden closure target from a pinned checkpoint and uses SymPy to compare it
with candidates already selected by the learner. Zero means a symbolic-simplifier
result, not a machine-checked theorem.
"""
from __future__ import annotations
import argparse
import hashlib
import json
from pathlib import Path
from typing import Any, Mapping
import sympy as sp

SCHEMA = "ACSIE.PR110.Gen6SymbolicEquivalenceAudit.v1"
EVENT = "CLOSURE_LINEAGE_MISMATCH"
EXPECTED_RAW_SHA256 = "74caa5a04008a118d897ca41d03e50089a9bfc5fb67ce94579ed02977531f3a2"
EXPECTED_CHECKPOINT_SHA256 = "22828e4d70d7728c87e887dd8e507a4fe43240a154117edcce9f899d218838ee"
SYMBOLS = {key: sp.Symbol(key, real=True) for key in ("x", "y", "z")}


class AuditError(ValueError):
    pass


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def as_sympy(expr: Any, macros: Mapping[str, Mapping[str, Any]], active: tuple[str, ...] = ()) -> Any:
    if not isinstance(expr, dict):
        raise AuditError("expression node is not a mapping")
    op = expr.get("op")
    if op == "macro":
        pid = str(expr.get("id"))
        if pid in active:
            raise AuditError("cyclic macro: " + " -> ".join(active + (pid,)))
        definition = macros.get(pid)
        if not isinstance(definition, Mapping):
            raise AuditError("unresolved macro: " + pid)
        return as_sympy(definition, macros, active + (pid,))
    if op == "get":
        key = str(expr.get("key"))
        if key not in SYMBOLS:
            raise AuditError("unsupported input key: " + key)
        return SYMBOLS[key]
    if op == "const":
        value = expr.get("value")
        if isinstance(value, bool) or not isinstance(value, (int, float)):
            raise AuditError("unsupported constant")
        return sp.Rational(str(value))
    if op in {"add", "sub", "mul", "max", "min"}:
        a = as_sympy(expr.get("left"), macros, active)
        b = as_sympy(expr.get("right"), macros, active)
        if op == "add": return a + b
        if op == "sub": return a - b
        if op == "mul": return a * b
        if op == "max": return sp.Max(a, b)
        return sp.Min(a, b)
    if op in {"abs", "neg", "threshold"}:
        a = as_sympy(expr.get("arg"), macros, active)
        if op == "abs": return sp.Abs(a)
        if op == "neg": return -a
        lo = expr.get("lo")
        if isinstance(lo, bool) or not isinstance(lo, (int, float)):
            raise AuditError("unsupported threshold")
        return sp.Piecewise((sp.Integer(1), a >= sp.Rational(str(lo))), (sp.Integer(0), True))
    raise AuditError("unsupported operator: " + repr(op))


def summarize(raw: bytes, checkpoint_bytes: bytes, *, require_pinned_hashes: bool = True) -> dict[str, Any]:
    raw_hash, checkpoint_hash = sha256(raw), sha256(checkpoint_bytes)
    if require_pinned_hashes and raw_hash != EXPECTED_RAW_SHA256:
        raise AuditError("raw log SHA mismatch")
    if require_pinned_hashes and checkpoint_hash != EXPECTED_CHECKPOINT_SHA256:
        raise AuditError("checkpoint SHA mismatch")
    checkpoint = json.loads(checkpoint_bytes)
    if checkpoint.get("generation_next") != 7 or checkpoint.get("seed") != 2026100305:
        raise AuditError("wrong seed/generation checkpoint")
    learner = checkpoint.get("learner_state", {})
    primitives, retained = learner.get("primitives"), checkpoint.get("retained")
    if not isinstance(primitives, dict) or not isinstance(retained, list):
        raise AuditError("checkpoint primitive/retained maps missing")
    runtime_macros = {str(k): v["expression"] for k, v in primitives.items()
                      if isinstance(v, dict) and isinstance(v.get("expression"), dict)}
    hidden_macros = {str(v["hidden_id"]): v["expression"] for v in retained
                     if isinstance(v, dict) and isinstance(v.get("hidden_id"), str)
                     and isinstance(v.get("expression"), dict)}
    primitive_to_hidden = {str(v["primitive_id"]): str(v["hidden_id"]) for v in retained
                           if isinstance(v, dict) and isinstance(v.get("primitive_id"), str)
                           and isinstance(v.get("hidden_id"), str)}
    records, malformed = [], 0
    for n, line in enumerate(raw.decode("utf-8", errors="replace").splitlines(), 1):
        if not line.strip(): continue
        try: event = json.loads(line)
        except json.JSONDecodeError:
            malformed += 1
            continue
        if not isinstance(event, dict) or event.get("event") != EVENT: continue
        expected, frontier = event.get("expected_parent_ids"), event.get("frontier_candidate_audit")
        if not isinstance(expected, list) or len(expected) != 2 or not isinstance(frontier, list):
            records.append({"source_line": n, "status": "INCOMPLETE_EVENT"})
            continue
        try:
            hs = [primitive_to_hidden[str(pid)] for pid in expected]
            target_ast = {"op": "add", "left": {"op": "macro", "id": hs[0]},
                          "right": {"op": "macro", "id": hs[1]}}
            target = sp.simplify(as_sympy(target_ast, hidden_macros))
            selected = [c for c in frontier if isinstance(c, dict) and c.get("is_preselected_candidate") is True]
            if len(selected) != 1: raise AuditError("expected exactly one preselected candidate")
            candidate = selected[0]
            if not isinstance(candidate.get("expression"), dict): raise AuditError("selected expression missing")
            candidate_expr = sp.simplify(as_sympy(candidate["expression"], runtime_macros))
            diff = sp.simplify(candidate_expr - target)
            zero = diff == sp.Integer(0)
            target_srepr, candidate_srepr, diff_srepr = sp.srepr(target), sp.srepr(candidate_expr), sp.srepr(diff)
            records.append({
                "source_line": n, "generation": event.get("generation"),
                "expected_parent_ids": expected, "selected_process_id": candidate.get("process_id"),
                "selected_candidate_competent": candidate.get("closure_competent"),
                "selected_contains_expected_parents": candidate.get("contains_expected_parents"),
                "status": "SYMBOLIC_SIMPLIFIER_ZERO" if zero else "NOT_PROVEN_BY_SIMPLIFIER",
                "sympy_simplified_difference_is_exact_zero": bool(zero),
                "target_srepr_sha256": sha256(target_srepr.encode()),
                "candidate_srepr_sha256": sha256(candidate_srepr.encode()),
                "simplified_difference_srepr_sha256": sha256(diff_srepr.encode()),
                "simplified_difference_srepr": diff_srepr if not zero else "Integer(0)",
                "used_for_runtime_selection": False, "used_for_admission_or_gate": False,
            })
        except Exception as exc:
            records.append({"source_line": n, "generation": event.get("generation"),
                            "expected_parent_ids": expected, "status": "AUDIT_ERROR",
                            "error_type": type(exc).__name__, "error": str(exc)[:300],
                            "used_for_runtime_selection": False, "used_for_admission_or_gate": False})
    zeros = [r for r in records if r.get("status") == "SYMBOLIC_SIMPLIFIER_ZERO"]
    alias_groups = {}
    for item in retained:
        hid = str(item.get("hidden_id"))
        try:
            expr = sp.simplify(as_sympy({"op": "macro", "id": hid}, hidden_macros))
            sig = sha256(sp.srepr(expr).encode())
            group = alias_groups.setdefault(sig, {"normalized_symbolic_srepr_sha256": sig, "hidden_ids": [],
                "primitive_ids": [], "generations": [], "normalized_expression": str(expr)})
            group["hidden_ids"].append(hid)
            group["primitive_ids"].append(str(item.get("primitive_id")))
            group["generations"].append(item.get("generation"))
        except Exception as exc:
            alias_groups.setdefault("AUDIT_ERROR:" + type(exc).__name__, {
                "normalized_symbolic_srepr_sha256": None, "hidden_ids": [hid],
                "primitive_ids": [str(item.get("primitive_id"))], "generations": [item.get("generation")],
                "audit_error": type(exc).__name__})
    aliases = sorted(alias_groups.values(), key=lambda r: (r.get("normalized_symbolic_srepr_sha256") or "", r["hidden_ids"]))
    target_hashes = {r.get("target_srepr_sha256") for r in zeros}
    all_zero = bool(records) and len(zeros) == len(records)
    return {
        "schema": SCHEMA, "source_raw_log_sha256": raw_hash,
        "source_gen6_checkpoint_sha256": checkpoint_hash, "sympy_version": sp.__version__,
        "event_count": len(records), "symbolic_simplifier_zero_count": len(zeros),
        "not_proven_count": sum(r.get("status") == "NOT_PROVEN_BY_SIMPLIFIER" for r in records),
        "audit_error_count": sum(r.get("status") == "AUDIT_ERROR" for r in records),
        "malformed_jsonl_lines": malformed, "all_records_zero": all_zero,
        "distinct_target_closure_symbolic_forms": len(target_hashes),
        "selected_candidate_symbolic_identity_matches_target": bool(records) and all(
            r.get("candidate_srepr_sha256") == r.get("target_srepr_sha256") for r in zeros)
            and len(zeros) == len(records),
        "retained_capability_count": len(retained),
        "distinct_retained_capability_symbolic_forms": len(alias_groups),
        "retained_capability_symbolic_alias_groups": aliases,
        "all_selected_candidates_marked_wrong_lineage": bool(records) and all(
            r.get("selected_contains_expected_parents") is False for r in zeros) and len(zeros) == len(records),
        "uses_hidden_parent_ids_for_reconstruction_only": True,
        "runtime_selection_changed": False, "qualification_gate_changed": False,
        "formal_machine_checked_proof": False,
        "interpretation": "SymPy simplified each selected candidate-minus-target expression to zero. This is symbolic equivalence evidence under the pure evaluator, not a machine-checked proof.",
        "records": records,
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
    output = json.dumps(result, sort_keys=True, indent=2) + "\n"
    if args.output: args.output.write_text(output, encoding="utf-8")
    else: print(output, end="")
    return 0 if result["event_count"] and result["all_records_zero"] and result["audit_error_count"] == 0 else 2


if __name__ == "__main__":
    raise SystemExit(main())
