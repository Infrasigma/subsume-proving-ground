#!/usr/bin/env python3
"""Verify historical frozen Gen2 evidence and current Gen3 schema compatibility.

This is a separate historical-predecessor arm. It never rewrites or replaces
the failed current-branch Gen2 exporter result.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import zipfile

EXPECTED_ARCHIVE_SHA256 = "8a1afedbac85317e031ec043aace643c1f4c87822dd302a19b888e23c58596c9"
EXPECTED_STATE_SHA256 = "0b599e436c4578115cc0312dd6545f5d56c14777fcb55562592c5d8f4e8f80a9"
EXPECTED_STATE_PARENT = "b31109864ae547320d877e6c1575b805d950edf9"
EXPECTED_EXPORTER_COMMIT = "94a2a770a5799fd386217998bc21b585632ef7ed"
EXPECTED_HISTORICAL_GEN3_CORE_COMMIT = "eced0c7ed816b0baa15769fd841abde62fcccbd5"
EXPECTED_ACSIE_COMMIT = "d83cb60286f195eec02b4d7aa45a60539316d115"
EXPECTED_ACSIE_TREE = "12d2fa8fc1f1e34d3d7c442eef85aa6a71380420"
REQUIRED_FILES = (
    "verified-gen2-export.json",
    "verified-gen2-state.json",
    "state.sha256",
    "gen3-final.json",
    "execution-provenance.txt",
    "recovery-provenance.txt",
)

def sha256_bytes(raw: bytes) -> str:
    return hashlib.sha256(raw).hexdigest()

def normalized(value):
    return json.loads(json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False))

def write_json(path: Path, value) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, sort_keys=True, allow_nan=False) + "\n", encoding="utf-8")

def verify_text_provenance(text: str, expected: dict[str, str]) -> dict[str, bool]:
    observed = {}
    for line in text.splitlines():
        if "=" in line:
            k, v = line.split("=", 1)
            observed[k.strip()] = v.strip()
    return {k: observed.get(k) == v for k, v in expected.items()}

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--archive", type=Path, required=True)
    ap.add_argument("--materialized-dir", type=Path, required=True)
    ap.add_argument("--acsie-root", type=Path, required=True)
    ap.add_argument("--evidence", type=Path, required=True)
    ap.add_argument("--current-gen2-exporter-commit", default="0e9c2ad9a146775c162a278b0330c596d29055b9")
    args = ap.parse_args()

    args.materialized_dir.mkdir(parents=True, exist_ok=True)
    args.evidence.mkdir(parents=True, exist_ok=True)
    checks: dict[str, object] = {
        "schema": "ACSIE.historical_gen2_predecessor_compatibility.v1",
        "arm_type": "HISTORICAL_VERIFIED_PREDECESSOR_CURRENT_GEN3_COMPATIBILITY_AND_SCALING",
        "historical_artifact_id": 10950102751,
        "historical_workflow_run_id": 36374197614,
        "historical_artifact_expected_sha256": EXPECTED_ARCHIVE_SHA256,
        "current_gen2_exporter_commit": args.current_gen2_exporter_commit,
        "current_gen2_exporter_disposition": "FAILED_SEPARATE_RESULT_NOT_SUBSTITUTED",
        "checks": {},
    }
    errors = []
    try:
        archive_sha = sha256_bytes(args.archive.read_bytes())
        checks["archive_sha256"] = archive_sha
        checks["checks"]["archive_sha256_matches"] = archive_sha == EXPECTED_ARCHIVE_SHA256
        if archive_sha != EXPECTED_ARCHIVE_SHA256:
            raise ValueError(f"historical artifact ZIP digest mismatch: {archive_sha}")

        with zipfile.ZipFile(args.archive, "r") as zf:
            names = set(zf.namelist())
            unsafe = [
                n for n in names
                if n.startswith("/") or ".." in Path(n).parts or "\\" in n
            ]
            checks["archive_member_count"] = len(names)
            checks["archive_required_files_present"] = {
                name: name in names for name in REQUIRED_FILES
            }
            if unsafe:
                raise ValueError(f"unsafe ZIP member path(s): {unsafe}")
            missing = [name for name in REQUIRED_FILES if name not in names]
            if missing:
                raise ValueError(f"historical archive is missing required members: {missing}")
            payload = {name: zf.read(name) for name in REQUIRED_FILES}

        state_raw = payload["verified-gen2-state.json"]
        state_sha = sha256_bytes(state_raw)
        state_sha_record = payload["state.sha256"].decode("utf-8", errors="strict").strip().split()
        checks["state_sha256"] = state_sha
        checks["state_sha_record"] = state_sha_record
        checks["checks"]["state_sha256_matches_expected"] = state_sha == EXPECTED_STATE_SHA256
        checks["checks"]["state_sha256_manifest_matches"] = bool(state_sha_record) and state_sha_record[0] == state_sha
        if state_sha != EXPECTED_STATE_SHA256:
            raise ValueError(f"raw frozen-state digest mismatch: {state_sha}")
        if not state_sha_record or state_sha_record[0] != state_sha:
            raise ValueError("state.sha256 does not match verified-gen2-state.json")

        export_obj = json.loads(payload["verified-gen2-export.json"])
        state_obj = json.loads(state_raw)
        historic_gen3 = json.loads(payload["gen3-final.json"])
        checks["checks"]["embedded_frozen_state_equals_raw_state"] = export_obj.get("frozen_state") == state_obj
        if export_obj.get("frozen_state") != state_obj:
            raise ValueError("embedded frozen_state differs from raw verified-gen2-state.json")

        checks["historical_export"] = {
            "schema": export_obj.get("schema"),
            "scientific_status": export_obj.get("scientific_status"),
            "scientific_parent_commit": export_obj.get("scientific_parent_commit"),
            "executed_export_commit": export_obj.get("executed_export_commit"),
            "verified_gen2_protocol": export_obj.get("verified_gen2_protocol"),
            "state_invariants": export_obj.get("state_invariants"),
            "stored_fingerprint_count": len(export_obj.get("stored_hypothesis_fingerprints", [])),
        }
        checks["historical_gen3_result"] = {
            "schema": historic_gen3.get("schema"),
            "scientific_status": historic_gen3.get("scientific_status"),
            "gen3": historic_gen3.get("gen3"),
            "retention": historic_gen3.get("retention"),
            "integrity": historic_gen3.get("integrity"),
        }
        checks["state_identity"] = {
            "schema": state_obj.get("schema"),
            "seed": state_obj.get("seed"),
            "generation": state_obj.get("generation"),
            "hypothesis_count": len(state_obj.get("hypotheses", {})),
            "has_protected_hypotheses_field": "protected_hypotheses" in state_obj,
        }

        expected_provenance = verify_text_provenance(
            payload["execution-provenance.txt"].decode("utf-8", errors="strict"),
            {
                "source_candidate_run": "36373562036",
                "merge_workflow": "36374197614",
                "merge_workflow_sha": "b902eb59dc526142682b79f186c2580379ed50c8",
                "acsie_source": "23b6de62a062154b3e6e2badb5d499854bc6e6fc",
                "scientific_core_source": EXPECTED_HISTORICAL_GEN3_CORE_COMMIT,
            },
        )
        recovery_provenance = verify_text_provenance(
            payload["recovery-provenance.txt"].decode("utf-8", errors="strict"),
            {
                "SOURCE_RUN": "36373562036",
                "ACSIE_SOURCE": "23b6de62a062154b3e6e2badb5d499854bc6e6fc",
                "VERIFIED_PARENT": EXPECTED_STATE_PARENT,
            },
        )
        checks["historical_execution_provenance_matches"] = expected_provenance
        checks["historical_recovery_provenance_matches"] = recovery_provenance

        gen2_protocol = export_obj.get("verified_gen2_protocol") or {}
        state_invariants = export_obj.get("state_invariants") or {}
        historic_integrity = historic_gen3.get("integrity") or {}
        historic_g3 = historic_gen3.get("gen3") or {}
        historic_retention = historic_gen3.get("retention") or {}
        historic_checks = {
            "export_schema": export_obj.get("schema") == "ACSIE.verified-gen2-state-export.v1",
            "export_status_passed": export_obj.get("scientific_status") == "PASSED",
            "parent_commit_pinned": export_obj.get("scientific_parent_commit") == EXPECTED_STATE_PARENT,
            "exporter_commit_pinned": export_obj.get("executed_export_commit") == EXPECTED_EXPORTER_COMMIT,
            "gen2_protocol_passed": gen2_protocol.get("status") == "PASSED",
            "historical_gen2_holdout_gain_gte_gate": isinstance(gen2_protocol.get("holdout_gain"), (int, float)) and gen2_protocol["holdout_gain"] >= 0.05,
            "historical_gen2_transfer_gain_gte_gate": isinstance(gen2_protocol.get("transfer_gain"), (int, float)) and gen2_protocol["transfer_gain"] >= 0.05,
            "gen2_stored_retention_is_one": gen2_protocol.get("stored_retention_ratio") == 1.0,
            "state_roundtrip_learning_kernel_equal": state_invariants.get("state_roundtrip_learning_kernel_equal") is True,
            "state_roundtrip_hypothesis_count_equal": state_invariants.get("state_roundtrip_hypothesis_count_equal") is True,
            "historical_gen3_schema": historic_gen3.get("schema") == "ACSIE.gen3-frozen-verified-gen2-sharded.v1",
            "historical_gen3_passed": historic_gen3.get("scientific_status") == "PASSED",
            "historical_gen3_holdout_gain": isinstance(historic_g3.get("holdout_gain"), (int, float)) and historic_g3["holdout_gain"] >= 0.05,
            "historical_gen3_transfer_gain": isinstance(historic_g3.get("transfer_gain"), (int, float)) and historic_g3["transfer_gain"] >= 0.05,
            "historical_gen3_replay": historic_g3.get("deterministic_replay") is True,
            "historical_gen1_retention": historic_retention.get("regression_preserved") is True and historic_retention.get("gen1_before") == historic_retention.get("gen1_after") == 0.75,
            "historical_gen2_retention": historic_retention.get("gen2_before") == historic_retention.get("gen2_after") == 0.375,
            "historical_stored_retention": historic_retention.get("stored_preserved") is True and historic_retention.get("stored_before") == historic_retention.get("stored_after") == 0.75,
            "historical_integrity_clean": all(historic_integrity.get(k) is False for k in ("holdout_contamination", "task_routing", "external_model", "network_dependency")),
            "execution_provenance_matches": all(expected_provenance.values()),
            "recovery_provenance_matches": all(recovery_provenance.values()),
        }
        checks["checks"].update(historic_checks)
        failed = [name for name, ok in checks["checks"].items() if not ok]
        if failed:
            raise ValueError(f"historical evidence validation failed checks: {failed}")

        acsie_root = args.acsie_root.resolve()
        head = subprocess.run(["git", "-C", str(acsie_root), "rev-parse", "HEAD"], check=True, capture_output=True, text=True).stdout.strip()
        tree = subprocess.run(["git", "-C", str(acsie_root), "rev-parse", "HEAD^{tree}"], check=True, capture_output=True, text=True).stdout.strip()
        if (head, tree) != (EXPECTED_ACSIE_COMMIT, EXPECTED_ACSIE_TREE):
            raise ValueError(f"current ACSIE pin mismatch: commit={head}, tree={tree}")
        source_hashes = {}
        for relative in (
            "cognitive_core/native_independent_core.py",
            "research/native_self_learning_kernel_gen3_compositional_retention_v1.py",
            "research/gen3_from_frozen_verified_gen2_v1.py",
        ):
            source_hashes[relative] = sha256_bytes((acsie_root / relative).read_bytes())
        checks["current_acsie_source"] = {"commit": head, "tree": tree, "file_sha256": source_hashes}

        sys.path.insert(0, str(acsie_root))
        from cognitive_core.native_independent_core import NativeCognitiveCore
        current_schema = getattr(NativeCognitiveCore, "SCHEMA", None)
        checks["current_state_compatibility"] = {
            "current_core_schema": current_schema,
            "old_state_schema": state_obj.get("schema"),
            "state_schema_matches_current": current_schema == state_obj.get("schema"),
            "roundtrip_existing_fields_match": False,
            "mismatched_fields": [],
            "added_current_fields": [],
            "loaded_hypothesis_count": None,
            "loaded_generation": None,
            "loaded_seed": None,
        }
        if current_schema != state_obj.get("schema"):
            raise ValueError(f"state schema mismatch: state={state_obj.get('schema')!r}, current={current_schema!r}")

        core = NativeCognitiveCore.from_state(state_obj)
        roundtrip = normalized(core.export_state())
        normalized_state = normalized(state_obj)
        mismatches = [
            key for key, value in normalized_state.items()
            if key not in roundtrip or roundtrip[key] != value
        ]
        added = sorted(set(roundtrip) - set(normalized_state))
        checks["current_state_compatibility"].update({
            "roundtrip_existing_fields_match": not mismatches,
            "mismatched_fields": mismatches,
            "added_current_fields": added,
            "loaded_hypothesis_count": len(core.hypotheses),
            "loaded_generation": core.generation,
            "loaded_seed": core.seed,
            "loaded_learning_kernel_matches": roundtrip.get("learning_kernel") == normalized_state.get("learning_kernel"),
            "loaded_state_counts_match": (
                len(core.hypotheses) == len(state_obj.get("hypotheses", {}))
                and len(core.experience) == len(state_obj.get("experience", []))
                and core.generation == state_obj.get("generation")
                and core.seed == state_obj.get("seed")
            ),
        })
        if mismatches:
            raise ValueError(f"current NativeCognitiveCore roundtrip differs on historical fields: {mismatches}")
        if not checks["current_state_compatibility"]["loaded_state_counts_match"]:
            raise ValueError("current NativeCognitiveCore did not preserve historical state counts")
        if not checks["current_state_compatibility"]["loaded_learning_kernel_matches"]:
            raise ValueError("current NativeCognitiveCore changed the historical learning kernel in roundtrip")

        # Materialize the exact verified export envelope consumed by the Gen3 runner.
        for name, raw in payload.items():
            (args.materialized_dir / name).write_bytes(raw)
        checks["checks"]["exact_export_envelope_materialized"] = True
        checks["checks"]["current_gen2_failure_not_substituted"] = True
        checks["disposition"] = "HISTORICAL_PREDECESSOR_VERIFIED_CURRENT_SCHEMA_COMPATIBLE"
        checks["scientific_status"] = "PASSED_PREDECESSOR_COMPATIBILITY_ONLY"
    except BaseException as exc:
        checks["disposition"] = "BLOCKED_PREDECESSOR_COMPATIBILITY_OR_EVIDENCE_VALIDATION"
        checks["scientific_status"] = "BLOCKED"
        checks["error"] = f"{type(exc).__name__}: {exc}"
        errors.append(checks["error"])

    report_path = args.evidence / "historical-gen2-predecessor-verification.json"
    write_json(report_path, checks)
    print(json.dumps(checks, indent=2, sort_keys=True, allow_nan=False))
    return 0 if not errors else 2

if __name__ == "__main__":
    raise SystemExit(main())
