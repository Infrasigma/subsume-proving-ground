#!/usr/bin/env python3
from __future__ import annotations

import json
import math
import statistics
from collections import defaultdict
from pathlib import Path


def load(path: Path) -> dict:
    if not path.exists():
        return {
            "label": path.stem.split("-")[0],
            "searches": [],
            "checkpoint_timings": [],
        }
    return json.loads(path.read_text(encoding="utf-8"))


def inv_searches(data: dict, generation: int, stage: str) -> list[dict]:
    return [
        s for s in data.get("searches", [])
        if s.get("generation") == generation and s.get("stage") == stage and "snapshot" in s
    ]


def latest_primary(data: dict, generation: int) -> dict | None:
    xs = inv_searches(data, generation, "invent_primitive")
    return xs[-1] if xs else None


def depth_rows(record: dict | None) -> dict[int, dict]:
    if not record:
        return {}
    return {int(x["depth"]): x["snapshot"] for x in record.get("snapshot", [])}


def ratio(a: float | int | None, b: float | int | None) -> float | None:
    if a in (None, 0) or b is None:
        return None
    return float(b) / float(a)


def first_divergence(a: dict, b: dict) -> dict | None:
    da, db = depth_rows(a), depth_rows(b)
    for depth in sorted(set(da) | set(db)):
        if depth not in da or depth not in db:
            return {
                "depth": depth,
                "reason": "missing_boundary",
                "A_present": depth in da,
                "B_present": depth in db,
            }
        metrics = [
            "unique_semantic_states",
            "frontier_size_max_class",
            "frontier_size_total",
            "unique_executable_lineage_signatures",
            "behavioral_equivalence_class_count",
            "representatives_removed_if_executable_lineage_ignored",
        ]
        for metric in metrics:
            av, bv = da[depth].get(metric), db[depth].get(metric)
            if isinstance(av, (int, float)) and isinstance(bv, (int, float)):
                if av == 0 and bv == 0:
                    continue
                scale = max(1.0, abs(float(av)), abs(float(bv)))
                if abs(float(bv) - float(av)) / scale >= 0.50:
                    return {
                        "depth": depth,
                        "reason": ">=50_percent_change",
                        "metric": metric,
                        "A": av,
                        "B": bv,
                        "B_over_A": ratio(av, bv),
                    }
    return None


def compare_depths(a: dict | None, b: dict | None) -> list[dict]:
    da, db = depth_rows(a), depth_rows(b)
    rows = []
    for depth in sorted(set(da) | set(db)):
        aa, bb = da.get(depth, {}), db.get(depth, {})
        rows.append({
            "depth": depth,
            "A_states": aa.get("unique_semantic_states"),
            "B_states": bb.get("unique_semantic_states"),
            "A_frontier": aa.get("frontier_size_max_class"),
            "B_frontier": bb.get("frontier_size_max_class"),
            "A_frontier_total": aa.get("frontier_size_total"),
            "B_frontier_total": bb.get("frontier_size_total"),
            "B_over_A_states": ratio(aa.get("unique_semantic_states"), bb.get("unique_semantic_states")),
            "B_over_A_frontier": ratio(aa.get("frontier_size_max_class"), bb.get("frontier_size_max_class")),
            "B_equivalence_classes": bb.get("behavioral_equivalence_class_count"),
            "B_exec_lineage_signatures": bb.get("unique_executable_lineage_signatures"),
            "B_redundant_if_lineage_ignored": bb.get("representatives_removed_if_executable_lineage_ignored"),
        })
    return rows


def search_summary(record: dict | None) -> dict:
    if not record:
        return {}
    stats = record.get("stats", {})
    snapshots = record.get("snapshot", [])
    wall = record.get("search_wall_seconds")
    if wall is None and snapshots:
        wall = snapshots[-1].get("max_rss_kb")
    max_frontier = max((x["snapshot"].get("frontier_size_max_class", 0) for x in snapshots), default=0)
    final_snap = snapshots[-1]["snapshot"] if snapshots else {}
    return {
        "search_id": record.get("search_id"),
        "generation": record.get("generation"),
        "stage": record.get("stage"),
        "completed_depths": record.get("completed_depths", []),
        "max_frontier": max_frontier,
        "final_snapshot": final_snap,
        "stats": {
            "generated_expressions": stats.get("generated_expressions"),
            "unique_states": stats.get("unique_states"),
            "duplicate_states": stats.get("duplicate_states"),
            "dominance_pruned_states": stats.get("dominance_pruned_states"),
            "dominance_frontier_max": stats.get("dominance_frontier_max"),
            "target_candidate_count": stats.get("target_candidate_count"),
        },
        "search_wall_seconds": wall,
    }


def main() -> int:
    a = load(Path("evidence/A-forensics.json"))
    b = load(Path("evidence/B-forensics.json"))

    report = {
        "protocol": "ACSIE.PR110.phase6.scalability-forensics.v1",
        "scientific_status": "DIAGNOSTIC_ONLY",
        "A_runtime_sha": "7e905d73c15ece4b8b5d7cb73218bac34c2e1314",
        "B_runtime_sha": "9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be",
        "proving_sha": "33259fc5c69d87b99c604f0050508c433a279daf",
        "seed": 2026100305,
        "generations_requested": 6,
        "selected_generations": [5, 6],
        "five_by_twelve_gate_changed": False,
        "algorithmic_fix_applied": False,
        "cache_applied": False,
        "frontier_policy_changed": False,
        "ranking_changed": False,
        "thresholds_changed": False,
        "A_checkpoint_timings": a.get("checkpoint_timings", []),
        "B_checkpoint_timings": b.get("checkpoint_timings", []),
        "generations": {},
    }

    for generation in [5, 6]:
        ar = latest_primary(a, generation)
        br = latest_primary(b, generation)
        report["generations"][str(generation)] = {
            "A": search_summary(ar),
            "B": search_summary(br),
            "depth_table_invent_primitive": compare_depths(ar, br),
            "first_divergence": first_divergence(ar, br),
        }

    g5 = report["generations"]["5"]
    b5 = g5["B"].get("final_snapshot", {})
    a5 = g5["A"].get("final_snapshot", {})
    lineage_removed = b5.get("representatives_removed_if_executable_lineage_ignored", 0)
    b_frontier = b5.get("frontier_size_max_class", 0)
    lineage_removed_fraction = (
        lineage_removed / b_frontier if b_frontier else 0.0
    )

    b5_stats = g5["B"].get("stats", {})
    b5_wall = g5["B"].get("search_wall_seconds") or 0.0
    b5_lineage = b5.get("lineage_time_seconds", 0.0)
    b5_dominance = b5.get("dominance_time_seconds", 0.0)
    b5_frontier_time = b5.get("frontier_maintenance_time_seconds", 0.0)
    b5_checkpoint = sum(x.get("seconds", 0.0) for x in b.get("checkpoint_timings", []))

    root_causes = []
    if lineage_removed_fraction >= 0.70:
        root_causes.append("LINEAGE_PRESERVATION_EXPLOSION")
    if b_frontier and a5.get("frontier_size_max_class", 0) is not None and b_frontier >= 50 * max(1, a5.get("frontier_size_max_class", 1)):
        root_causes.append("WEAK_DOMINANCE")
    if b5_lineage > 0.5 * max(b5_wall, 1e-9):
        root_causes.append("LINEAGE_COMPUTATION_COST")
    if b5_checkpoint > 0.2 * max(b5_wall, 1e-9):
        root_causes.append("SERIALIZATION")
    if b5.get("unique_semantic_states", 0) and a5.get("unique_semantic_states", 0):
        if b5.get("unique_semantic_states", 0) >= 5 * max(1, a5.get("unique_semantic_states", 0)) and lineage_removed_fraction < 0.3:
            root_causes.append("BEHAVIORAL_STATE_EXPLOSION")
    if not root_causes:
        root_causes.append("UNKNOWN")

    report["Gen5_root_cause_metrics"] = {
        "B_frontier": b_frontier,
        "B_representatives_removed_if_exec_lineage_ignored": lineage_removed,
        "B_lineage_only_representative_fraction": lineage_removed_fraction,
        "B_lineage_time_fraction_of_search_wall": b5_lineage / max(b5_wall, 1e-9),
        "B_dominance_time_fraction_of_search_wall": b5_dominance / max(b5_wall, 1e-9),
        "B_frontier_maintenance_time_fraction_of_search_wall": b5_frontier_time / max(b5_wall, 1e-9),
        "B_checkpoint_time_seconds_observed": b5_checkpoint,
        "B_generated_expressions": b5_stats.get("generated_expressions"),
        "B_unique_states": b5_stats.get("unique_states"),
        "B_dominance_pruned": b5_stats.get("dominance_pruned_states"),
    }
    report["SCALABILITY_CAUSE"] = root_causes[0] if len(root_causes) == 1 else "MIXED"
    report["supported_root_cause_hypotheses"] = root_causes
    report["alternatives_not_supported_by_this_diagnostic"] = [
        "A pure serialization cause is unsupported unless checkpoint time fraction is large.",
        "A pure lineage-computation cause is unsupported unless lineage time dominates search wall time or repeated lineage signatures are high.",
        "A pure behavioral-state explosion cause is unsupported when lineage-only representatives explain most of the frontier.",
        "An 8-way frontier-policy hypothesis is deliberately not manipulated in this experiment.",
    ]

    Path("evidence/phase6-forensic-report.json").write_text(
        json.dumps(report, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(json.dumps(report, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
