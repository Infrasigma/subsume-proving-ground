#!/usr/bin/env python3
"""Offline H7 representative-compression counterfactuals.

This file does not alter runtime search or choose a candidate for deployment. It
compares bounded selectors over a captured, already-pruned Gen5 frontier.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
from typing import Any

KS: tuple[int | None, ...] = (1, 2, 4, 8, None)
POLICIES = ("cost_only", "lineage_only", "direct_first", "dual_coverage")


def canonical(value: Any) -> str:
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=True)


def choose_key(rep: dict[str, Any]) -> tuple[Any, ...]:
    return (
        int(rep["expanded_execution_depth"]),
        int(rep["expanded_execution_nodes"]),
        str(rep["expression_key"]),
        int(rep["state_token"]),
    )


def direct_ids(rep: dict[str, Any]) -> set[str]:
    return {str(x) for x in rep.get("used_primitives", ())}


def lineage_ids(rep: dict[str, Any]) -> set[str]:
    return {str(x) for x in rep.get("executable_lineage_ids", ())}


def greedy_select(
    reps: list[dict[str, Any]], k: int | None, policy: str
) -> list[dict[str, Any]]:
    if policy not in POLICIES:
        raise ValueError(f"unknown policy: {policy}")
    if k is None or k >= len(reps):
        return list(reps)
    if k <= 0:
        raise ValueError("K must be positive or None")
    if policy == "cost_only":
        return sorted(reps, key=choose_key)[:k]

    remaining = list(reps)
    covered_direct: set[str] = set()
    covered_lineage: set[str] = set()
    chosen: list[dict[str, Any]] = []
    while remaining and len(chosen) < k:
        ranked = []
        for rep in remaining:
            new_direct = direct_ids(rep) - covered_direct
            new_lineage = lineage_ids(rep) - covered_lineage
            base = choose_key(rep)
            if policy == "lineage_only":
                score = (-len(new_lineage), *base)
            elif policy == "direct_first":
                score = (-len(new_direct), -len(new_lineage), *base)
            else:  # dual_coverage: separate, equally weighted direct and transitive dimensions
                score = (
                    -(len(new_direct) + len(new_lineage)),
                    -len(new_direct),
                    -len(new_lineage),
                    *base,
                )
            ranked.append((score, rep))
        ranked.sort(key=lambda item: item[0])
        pick = ranked[0][1]
        chosen.append(pick)
        covered_direct.update(direct_ids(pick))
        covered_lineage.update(lineage_ids(pick))
        remaining.remove(pick)
    return chosen


def coverage(original: set[str], kept: set[str]) -> float:
    return 1.0 if not original else len(original & kept) / len(original)


def summarize_capture(
    data: dict[str, Any], k: int | None, policy: str
) -> dict[str, Any]:
    overall = {
        "original_representatives": 0,
        "compressed_representatives": 0,
        "behavioral_classes": 0,
        "classes_with_direct_vocabulary_loss": 0,
        "classes_with_transitive_lineage_loss": 0,
        "direct_ids_lost_by_class": [],
        "lineage_ids_lost_by_class": [],
        "all_direct_ids": set(),
        "kept_direct_ids": set(),
        "all_lineage_ids": set(),
        "kept_lineage_ids": set(),
        "class_direct_coverage": [],
        "class_lineage_coverage": [],
        "by_depth": {},
    }

    for capture in data.get("captures", []):
        depth = int(capture["depth"])
        depth_rows = {
            "behavioral_classes": 0,
            "original_representatives": 0,
            "compressed_representatives": 0,
            "classes_with_direct_vocabulary_loss": 0,
            "classes_with_transitive_lineage_loss": 0,
            "all_direct_ids": set(),
            "kept_direct_ids": set(),
            "all_lineage_ids": set(),
            "kept_lineage_ids": set(),
            "class_direct_coverage": [],
            "class_lineage_coverage": [],
            "direct_ids_lost_by_class": [],
            "lineage_ids_lost_by_class": [],
        }
        for cls in capture.get("classes", []):
            reps = list(cls.get("representatives", []))
            if not reps:
                continue
            selected = greedy_select(reps, k, policy)
            orig_direct = set().union(*(direct_ids(rep) for rep in reps))
            kept_direct = set().union(*(direct_ids(rep) for rep in selected))
            orig_lineage = set().union(*(lineage_ids(rep) for rep in reps))
            kept_lineage = set().union(*(lineage_ids(rep) for rep in selected))
            direct_lost = sorted(orig_direct - kept_direct)
            lineage_lost = sorted(orig_lineage - kept_lineage)
            class_hash = hashlib.sha256(
                canonical(cls.get("behavior_key", {})).encode("utf-8")
            ).hexdigest()

            for target in (overall, depth_rows):
                target["behavioral_classes"] = target.get("behavioral_classes", 0) + 1
                target["original_representatives"] += len(reps)
                target["compressed_representatives"] += len(selected)
                target["all_direct_ids"].update(orig_direct)
                target["kept_direct_ids"].update(kept_direct)
                target["all_lineage_ids"].update(orig_lineage)
                target["kept_lineage_ids"].update(kept_lineage)
                target["class_direct_coverage"].append(coverage(orig_direct, kept_direct))
                target["class_lineage_coverage"].append(coverage(orig_lineage, kept_lineage))
                if direct_lost:
                    target["classes_with_direct_vocabulary_loss"] += 1
                    target["direct_ids_lost_by_class"].append(
                        {"behavior_key_sha256": class_hash, "ids": direct_lost}
                    )
                if lineage_lost:
                    target["classes_with_transitive_lineage_loss"] += 1
                    target["lineage_ids_lost_by_class"].append(
                        {"behavior_key_sha256": class_hash, "ids": lineage_lost}
                    )

        def finalize(row: dict[str, Any]) -> dict[str, Any]:
            original = int(row["original_representatives"])
            compressed = int(row["compressed_representatives"])
            all_direct = row.pop("all_direct_ids")
            kept_direct = row.pop("kept_direct_ids")
            all_lineage = row.pop("all_lineage_ids")
            kept_lineage = row.pop("kept_lineage_ids")
            direct_class = row.pop("class_direct_coverage")
            lineage_class = row.pop("class_lineage_coverage")
            row["fraction_removed"] = 0.0 if not original else 1.0 - compressed / original
            row["direct_vocabulary_coverage_retained"] = coverage(all_direct, kept_direct)
            row["transitive_lineage_coverage_retained"] = coverage(all_lineage, kept_lineage)
            row["mean_class_direct_coverage_retained"] = (
                1.0 if not direct_class else sum(direct_class) / len(direct_class)
            )
            row["mean_class_lineage_coverage_retained"] = (
                1.0 if not lineage_class else sum(lineage_class) / len(lineage_class)
            )
            row["direct_ids_lost_count_global"] = len(all_direct - kept_direct)
            row["lineage_ids_lost_count_global"] = len(all_lineage - kept_lineage)
            row["direct_coverage_lossless_by_class"] = row["classes_with_direct_vocabulary_loss"] == 0
            row["lineage_coverage_lossless_by_class"] = row["classes_with_transitive_lineage_loss"] == 0
            row["representative_subsumption_proved"] = False
            return row

        overall["by_depth"][str(depth)] = finalize(depth_rows)

    return finalize(overall)


def run_all(data: dict[str, Any]) -> dict[str, Any]:
    report: dict[str, Any] = {
        "schema": "ACSIE.PR110.H7.offline-counterfactual.v2",
        "runtime_sha": data["runtime_sha"],
        "proving_sha": data["proving_sha"],
        "seed": data["seed"],
        "generation": data["generation"],
        "K_values": ["UNBOUNDED" if k is None else k for k in KS],
        "policies": {},
        "information_boundary": {
            "future_validation_or_probe_information_used_by_selector": False,
            "target_values_used_by_selector": False,
            "frontier_was_generated_by_target_conditioned_search": True,
            "input_is_existing_non_dominated_search_frontier": True,
            "measures_losses_from_existing_dominance_before_capture": False,
            "current_state_counterfactual_only": True,
        },
        "interpretation_boundary": (
            "Coverage is descriptive evidence only; even perfect identity coverage "
            "does not prove behavioral congruence, future compositional substitutability, "
            "or safe runtime deletion."
        ),
    }
    for policy in POLICIES:
        report["policies"][policy] = {}
        for k in KS:
            label = "UNBOUNDED" if k is None else f"K={k}"
            result = summarize_capture(data, k, policy)
            result["K"] = k
            result["policy"] = policy
            result["selection_rule"] = {
                "cost_only": "lowest expanded depth/nodes, then canonical expression/token",
                "lineage_only": "marginal transitive executable lineage, then expanded cost/canonical tie-break",
                "direct_first": "marginal direct executable primitive vocabulary, then transitive lineage, then expanded cost/canonical tie-break",
                "dual_coverage": "sum of marginal direct and transitive identity coverage, then each dimension and expanded cost/canonical tie-break",
            }[policy]
            report["policies"][policy][label] = result
    return report


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--input", required=True)
    ap.add_argument("--output", required=True)
    args = ap.parse_args()
    data = json.loads(Path(args.input).read_text(encoding="utf-8"))
    report = run_all(data)
    Path(args.output).write_text(
        json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    print(json.dumps(report, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
