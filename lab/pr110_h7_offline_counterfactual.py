#!/usr/bin/env python3
from __future__ import annotations

import argparse
import itertools
import json
from collections import defaultdict
from pathlib import Path

KS = [1, 2, 4, 8, None]


def choose_key(rep):
    return (
        rep["expanded_execution_depth"],
        rep["expanded_execution_nodes"],
        rep["expression_key"],
        rep["state_token"],
    )


def greedy_select(reps, k):
    if k is None or k >= len(reps):
        return list(reps)
    remaining = list(reps)
    covered = set()
    chosen = []
    while remaining and len(chosen) < k:
        ranked = []
        for rep in remaining:
            lineage = set(rep["executable_lineage_ids"])
            marginal = len(lineage - covered)
            ranked.append((
                -marginal,
                rep["expanded_execution_depth"],
                rep["expanded_execution_nodes"],
                rep["expression_key"],
                rep["state_token"],
                rep,
            ))
        ranked.sort(key=lambda x: x[:-1])
        pick = ranked[0][-1]
        chosen.append(pick)
        covered.update(pick["executable_lineage_ids"])
        remaining.remove(pick)
    return chosen


def summarize_capture(data):
    report = {
        "schema": "ACSIE.PR110.H7.offline-counterfactual.v1",
        "runtime_sha": data["runtime_sha"],
        "proving_sha": data["proving_sha"],
        "seed": data["seed"],
        "generation": data["generation"],
        "objective": "maximize current-state executable-lineage coverage within each behavioral-equivalence class",
        "selection_rule": "greedy marginal lineage coverage, then smaller expanded execution cost, then canonical expression/state-token",
        "future_information_used": False,
        "target_information_used": False,
        "configs": {},
    }

    for k in KS:
        label = "UNBOUNDED" if k is None else f"K={k}"
        totals = defaultdict(int)
        lineage_union = set()
        lineage_kept = set()
        per_depth = {}

        for capture in data["captures"]:
            depth = str(capture["depth"])
            d = {
                "behavioral_class_count": capture["class_count"],
                "original_representatives": 0,
                "compressed_representatives": 0,
                "fraction_removed": 0.0,
                "lineage_coverage_retained": 1.0,
                "lineage_ids_lost": [],
                "classes_with_loss": 0,
            }
            for cls in capture["classes"]:
                reps = cls["representatives"]
                if not reps:
                    continue
                selected = greedy_select(reps, k)
                all_ids = set().union(*(set(r["executable_lineage_ids"]) for r in reps))
                kept_ids = set().union(*(set(r["executable_lineage_ids"]) for r in selected))
                lost = sorted(all_ids - kept_ids)
                d["original_representatives"] += len(reps)
                d["compressed_representatives"] += len(selected)
                totals["original_representatives"] += len(reps)
                totals["compressed_representatives"] += len(selected)
                lineage_union.update(all_ids)
                lineage_kept.update(kept_ids)
                if lost:
                    d["classes_with_loss"] += 1
                    d["lineage_ids_lost"].extend(lost)
            if d["original_representatives"]:
                d["fraction_removed"] = 1.0 - d["compressed_representatives"] / d["original_representatives"]
            d["lineage_ids_lost"] = sorted(set(d["lineage_ids_lost"]))
            if lineage_union:
                d["lineage_coverage_retained"] = len(lineage_kept) / len(lineage_union)
            per_depth[depth] = d

        original = totals["original_representatives"]
        compressed = totals["compressed_representatives"]
        total_lost = sorted(lineage_union - lineage_kept)
        report["configs"][label] = {
            "K": k,
            "original_representatives": original,
            "compressed_representatives": compressed,
            "fraction_removed": (1.0 - compressed / original) if original else 0.0,
            "lineage_coverage_retained": (len(lineage_kept) / len(lineage_union)) if lineage_union else 1.0,
            "lineage_ids_lost": total_lost,
            "lineage_ids_lost_count": len(total_lost),
            "behavioral_class_count": sum(v["behavioral_class_count"] for v in per_depth.values()),
            "by_depth": per_depth,
        }
    return report


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--input", required=True)
    ap.add_argument("--output", required=True)
    args = ap.parse_args()
    data = json.loads(Path(args.input).read_text(encoding="utf-8"))
    report = summarize_capture(data)
    Path(args.output).write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
