#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import json
from pathlib import Path

A_SHA = "7e905d73c15ece4b8b5d7cb73218bac34c2e1314"
B_SHA = "9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be"
PROVING_SHA = "33259fc5c69d87b99c604f0050508c433a279daf"
SEED = 2026100305
ARTIFACT_ID = 11536026911
ARTIFACT_DIGEST = "sha256:49e476c47322957dfd21ecaa05db8104b620deae2de254d3c1476876febac1ef"
WORKFLOW_RUN = 37738748858
FORENSIC_JOB = 113184264996
OUTPUT = Path("evidence/phase6-forensic-report.json")


def load(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def snapshots_for(data: dict, generation: int, search_ids: set[int]) -> list[dict]:
    out = []
    for row in data.get("searches", []):
        if (
            row.get("generation") == generation
            and row.get("stage") == "synthesize_process"
            and row.get("search_id") in search_ids
            and isinstance(row.get("snapshot"), dict)
        ):
            out.append(row["snapshot"])
    return out


def sha256_file(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main() -> int:
    evidence = Path("evidence")
    a = load(evidence / "A-forensics.json")
    b = load(evidence / "B-forensics.json")

    assert a["runtime_sha"] == A_SHA
    assert b["runtime_sha"] == B_SHA
    assert a["proving_sha"] == PROVING_SHA
    assert b["proving_sha"] == PROVING_SHA
    assert a["seed"] == SEED and b["seed"] == SEED

    a27 = snapshots_for(a, 5, {27})[-1]
    a28 = snapshots_for(a, 5, {28})[-1]
    a29 = snapshots_for(a, 5, {29})[-1]
    b26 = snapshots_for(b, 5, {26})[-1]
    b27 = snapshots_for(b, 5, {27})[-1]

    b_depth4 = [b26, b27]
    assert all(x["depth"] == 4 for x in b_depth4)
    assert [x["frontier_size_total"] for x in b_depth4] == [951, 940]
    assert [x["unique_executable_lineage_signatures"] for x in b_depth4] == [165, 154]
    assert [x["representatives_removed_if_executable_lineage_ignored"] for x in b_depth4] == [842, 837]
    assert [x["maximum_executable_lineage_size"] for x in b_depth4] == [10, 9]

    b_checkpoint_total = sum(x["seconds"] for x in b.get("checkpoint_timings", []))
    b_checkpoint_fraction = b_checkpoint_total / b["wall_seconds"]

    report = {
        "protocol": "ACSIE.PR110.phase6.archive.v1",
        "phase": "PHASE6",
        "archive_status": "PASSED AS DIAGNOSTIC",
        "SCALABILITY_CAUSE": "LINEAGE_PRESERVATION_EXPLOSION",
        "PR110": "INCONCLUSIVE",
        "provenance": {
            "A_runtime_sha": A_SHA,
            "B_runtime_sha": B_SHA,
            "seed": SEED,
            "evaluator": PROVING_SHA,
            "artifact": ARTIFACT_ID,
            "artifact_digest": ARTIFACT_DIGEST,
            "workflow_run": WORKFLOW_RUN,
            "forensic_job": FORENSIC_JOB,
        },
        "raw_artifact_validity": {
            "raw_telemetry_present": True,
            "raw_telemetry_used_without_trajectory_rerun": True,
            "final_aggregation_harness_originally_failed": True,
            "trajectory_steps_A": "SUCCESS",
            "trajectory_steps_B": "SUCCESS",
            "artifact_upload": "SUCCESS",
            "raw_telemetry_remains_valid": True,
        },
        "supported_conclusions": {
            "behavioral_equivalence_growth": "modest",
            "lineage_distinct_representatives": "explosive",
            "gen5_depth4_B_frontier": "~951",
            "gen5_depth4_B_frontier_observed_range": [940, 951],
            "gen5_depth4_B_lineage_signatures": "~154–165",
            "gen5_depth4_B_representatives_removable_when_executable_lineage_is_ignored": "~837–842",
            "max_lineage_size": "~9–10",
            "frontier_explosion": "primary observed scalability bottleneck",
            "serialization_checkpointing": "not primary cause",
            "behavioral_state_explosion": "not primary cause",
            "8_alternative_truncation": "not immediate cause",
            "lineage_computation_timing": "UNDETERMINED",
        },
        "observed_evidence": {
            "A_wall_seconds": a["wall_seconds"],
            "B_wall_seconds": b["wall_seconds"],
            "B_checkpoint_total_seconds": b_checkpoint_total,
            "B_checkpoint_fraction_of_wall": b_checkpoint_fraction,
            "B_gen5_depth4": {
                "frontier_total": [x["frontier_size_total"] for x in b_depth4],
                "frontier_max_class": [x["frontier_size_max_class"] for x in b_depth4],
                "behavioral_equivalence_classes": [x["behavioral_equivalence_class_count"] for x in b_depth4],
                "unique_semantic_states": [x["unique_semantic_states"] for x in b_depth4],
                "unique_executable_lineage_signatures": [x["unique_executable_lineage_signatures"] for x in b_depth4],
                "representatives_removed_if_executable_lineage_ignored": [x["representatives_removed_if_executable_lineage_ignored"] for x in b_depth4],
                "maximum_executable_lineage_size": [x["maximum_executable_lineage_size"] for x in b_depth4],
                "lineage_overlap_jaccard": [x["average_candidate_lineage_overlap_jaccard"] for x in b_depth4],
            },
            "A_gen5_depth4_reference": {
                "frontier_total": [a27["frontier_size_total"], a28["frontier_size_total"], a29["frontier_size_total"]],
                "frontier_max_class": [a27["frontier_size_max_class"], a28["frontier_size_max_class"], a29["frontier_size_max_class"]],
                "behavioral_equivalence_classes": [a27["behavioral_equivalence_class_count"], a28["behavioral_equivalence_class_count"], a29["behavioral_equivalence_class_count"]],
            },
        },
        "telemetry_limits": {
            "lineage_computation_timing": "UNDETERMINED: the prior observer did not capture valid nonzero inner lineage timing.",
            "no_inner_lineage_timing_inference": True,
            "no_cache_conclusion": "cache hypothesis remains unjustified",
        },
        "controls_untouched": {
            "five_by_twelve_gate": False,
            "runtime_algorithm_changes": False,
            "cache_changes": False,
            "frontier_policy_changes": False,
            "ranking_changes": False,
            "threshold_changes": False,
            "H3_alternative_width_experiment": False,
        },
        "input_file_sha256": {
            name: sha256_file(evidence / name)
            for name in ("A-forensics.json", "B-forensics.json", "A-checkpoint.json", "B-checkpoint.json")
            if (evidence / name).exists()
        },
    }

    OUTPUT.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
