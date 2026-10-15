#!/usr/bin/env python3
"""Replay target-blind candidate-ranking heuristics on saved PR110 Gen6 traces.

This is a post-hoc diagnostic, not a qualification run. Policy scores use only
candidate structure/dependency-count features and discovery order. Hidden target
parent IDs, validation/transfer/OOD results, and candidate correctness labels are
read only after selection, for retrospective evaluation.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
from typing import Any, Callable

SCHEMA = "ACSIE.PR110.Gen6BlindSelectionReplay.v1"
EVENT = "CLOSURE_LINEAGE_MISMATCH"


def _ast_features(expression: Any) -> dict[str, int]:
    def visit(node: Any) -> tuple[int, int, int, int]:
        if not isinstance(node, dict):
            return (1, 0, 0, 1)
        children = [visit(value) for key, value in node.items() if isinstance(value, dict)]
        nodes = 1 + sum(row[0] for row in children)
        macros = (1 if node.get("op") == "macro" else 0) + sum(row[1] for row in children)
        depth = 1 + max((row[2] for row in children), default=0)
        literals = (1 if node.get("op") == "const" else 0) + sum(row[3] for row in children)
        return nodes, macros, depth, literals

    nodes, macros, depth, literals = visit(expression)
    return {"ast_nodes": nodes, "macro_nodes": macros, "ast_depth": depth, "literal_nodes": literals}


def _features(candidate: dict[str, Any]) -> dict[str, int]:
    lineage = candidate.get("candidate_lineage") or []
    used = candidate.get("used_primitives") or []
    result = _ast_features(candidate.get("expression"))
    result["direct_primitive_count"] = len(set(x for x in used if isinstance(x, str)))
    result["ancestor_lineage_count"] = len(set(x for x in lineage if isinstance(x, str)))
    return result


# Every score is minimized; the final index tie-break preserves discovery order.
_POLICIES: dict[str, Callable[[dict[str, int], int], tuple[int, int]]] = {
    "discovery_first": lambda f, i: (0, i),
    "max_direct_primitive_count": lambda f, i: (-f["direct_primitive_count"], i),
    "max_ancestor_lineage_count": lambda f, i: (-f["ancestor_lineage_count"], i),
    "min_ancestor_lineage_count": lambda f, i: (f["ancestor_lineage_count"], i),
    "min_ast_nodes": lambda f, i: (f["ast_nodes"], i),
    "max_ast_nodes": lambda f, i: (-f["ast_nodes"], i),
    "max_macro_nodes": lambda f, i: (-f["macro_nodes"], i),
    "min_macro_nodes": lambda f, i: (f["macro_nodes"], i),
    "shallowest_ast": lambda f, i: (f["ast_depth"], i),
    "deepest_ast": lambda f, i: (-f["ast_depth"], i),
}


def _events(raw_text: str) -> tuple[list[dict[str, Any]], int]:
    rows: list[dict[str, Any]] = []
    malformed = 0
    for line in raw_text.splitlines():
        if not line.strip():
            continue
        try:
            row = json.loads(line)
        except json.JSONDecodeError:
            malformed += 1
            continue
        if not isinstance(row, dict):
            malformed += 1
            continue
        if row.get("event") == EVENT:
            rows.append(row)
    return rows, malformed


def summarize(raw_text: str, *, source_sha256: str) -> dict[str, Any]:
    events, malformed = _events(raw_text)
    policy_rows: dict[str, list[dict[str, Any]]] = {name: [] for name in _POLICIES}
    for event in events:
        candidates = event.get("frontier_candidate_audit")
        if not isinstance(candidates, list) or not candidates:
            for name in _POLICIES:
                policy_rows[name].append({"status": "INCONCLUSIVE_NO_CANDIDATES"})
            continue
        scored = []
        for index, candidate in enumerate(candidates):
            if not isinstance(candidate, dict):
                candidate = {}
            scored.append((candidate, _features(candidate), index))
        for name, policy in _POLICIES.items():
            candidate, features, index = min(scored, key=lambda row: policy(row[1], row[2]))
            # Evaluator-only labels are consulted after selection; none affects scores.
            policy_rows[name].append({
                "seed": event.get("seed"),
                "generation": event.get("generation"),
                "selected_process_id": candidate.get("process_id"),
                "selected_index": index,
                "selected_contains_expected_parents": candidate.get("contains_expected_parents"),
                "selected_closure_competent_diagnostic": candidate.get("closure_competent"),
                "selected_feature_vector": features,
            })

    policies_out = {}
    for name, rows in policy_rows.items():
        evaluated = [row for row in rows if row.get("status") is None]
        matched = sum(row.get("selected_contains_expected_parents") is True for row in evaluated)
        competent = sum(row.get("selected_closure_competent_diagnostic") is True for row in evaluated)
        policies_out[name] = {
            "description": "Target-blind structural score; tie-break is original frontier/discovery order.",
            "selected_candidate_count": len(evaluated),
            "expected_lineage_match_count_posthoc": matched,
            "expected_lineage_match_rate_posthoc": matched / len(evaluated) if evaluated else None,
            "closure_competent_count_posthoc": competent,
            "posthoc_labels_used_in_selection": False,
            "rows": rows,
        }

    return {
        "schema": SCHEMA,
        "source_raw_log_sha256": source_sha256,
        "event_count": len(events),
        "malformed_or_non_object_jsonl_lines": malformed,
        "selection_contract": {
            "target_parent_ids_used_for_scoring": False,
            "validation_or_holdout_used_for_scoring": False,
            "transfer_or_ood_used_for_scoring": False,
            "candidate_correctness_labels_used_for_scoring": False,
            "tie_break": "original frontier candidate order",
            "purpose": "Retrospective screening only; does not alter or replace the scientific gate.",
        },
        "policies": policies_out,
        "interpretation": (
            "A low rate shows that these generic structural signals do not reliably identify the benchmark's expected parent IDs. "
            "A higher retrospective rate is hypothesis-generating only: these same 13 events were inspected, so it is not independent validation."
        ),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    raw = args.input.read_bytes()
    report = summarize(raw.decode("utf-8", errors="replace"), source_sha256=hashlib.sha256(raw).hexdigest())
    rendered = json.dumps(report, indent=2, sort_keys=True) + "\n"
    if args.output:
        args.output.write_text(rendered, encoding="utf-8")
    else:
        print(rendered, end="")
    return 0 if report["event_count"] > 0 and report["malformed_or_non_object_jsonl_lines"] == 0 else 2


if __name__ == "__main__":
    raise SystemExit(main())
