#!/usr/bin/env python3
"""Phase-4 evidence-only semantic neutrality comparator for PR110."""

from __future__ import annotations

import argparse
import copy
import hashlib
import json
from collections import Counter
from pathlib import Path
from typing import Any

BOUNDARIES = (0, 1, 3, 5, 6)
UNORDERED_LINEAGE_KEYS = {
    "selected_structural_lineage_ids",
    "selected_executable_lineage_ids",
}
RUNTIME_SHA_KEYS = ("old_runtime", "repaired_runtime")
EXPECTED_OLD_RUNTIME = "af0c7ef503c51a5bc7872717b263a01f495c855f"
EXPECTED_REPAIRED_RUNTIME = "9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be"


def _canon(value: Any, key: str | None = None) -> Any:
    if isinstance(value, dict):
        out: dict[str, Any] = {}
        for k in sorted(value):
            if k == "state_digest":
                continue
            if k == "acsie_ref" and key == "runtime_identity":
                continue
            out[k] = _canon(value[k], k)
        return out
    if isinstance(value, list):
        vals = [_canon(v, None) for v in value]
        if key in UNORDERED_LINEAGE_KEYS:
            return sorted(
                vals,
                key=lambda x: json.dumps(x, sort_keys=True, separators=(",", ":")),
            )
        return vals
    return value


def semantic_projection(checkpoint: dict[str, Any]) -> dict[str, Any]:
    """Remove only explicitly permitted representation/runtime differences."""
    return _canon(checkpoint)


def _json_counter(items: list[Any]) -> Counter[str]:
    return Counter(
        json.dumps(x, sort_keys=True, separators=(",", ":"))
        for x in items
    )


def process_identity_multiset(checkpoint: dict[str, Any]) -> Counter[str]:
    processes = checkpoint["learner_state"]["processes"]
    return _json_counter([_canon(v) for v in processes.values()])


def process_content_multiset(checkpoint: dict[str, Any]) -> Counter[str]:
    processes = checkpoint["learner_state"]["processes"]
    values = []
    for process in processes.values():
        projected = _canon(process)
        if isinstance(projected, dict):
            projected = {
                k: v
                for k, v in projected.items()
                if k != "process_id"
            }
        values.append(projected)
    return _json_counter(values)


def frontier_ids(checkpoint: dict[str, Any]) -> list[Any]:
    return list(checkpoint["learner_state"]["process_frontier"])


def frontier_content_multiset(checkpoint: dict[str, Any]) -> Counter[str]:
    processes = checkpoint["learner_state"]["processes"]
    values = []
    for process_id in frontier_ids(checkpoint):
        if process_id in processes:
            values.append(_canon(processes[process_id]))
    return _json_counter(values)


def _target_sequence(checkpoint: dict[str, Any]) -> list[Any]:
    return [x.get("target_digest") for x in checkpoint.get("generations_out", [])]


def _generation_metric_projection(checkpoint: dict[str, Any]) -> list[dict[str, Any]]:
    keys = (
        "generation",
        "target_digest",
        "accepted",
        "acceptance_reason",
        "closure_reuse_rate",
        "probe_reuse_rate",
        "recursive_reuse_ok",
        "lineage_ok",
        "resource_cost",
    )
    return [
        {k: row.get(k) for k in keys}
        for row in checkpoint.get("generations_out", [])
    ]


def first_difference(a: Any, b: Any, path: str = "") -> dict[str, Any] | None:
    if type(a) is not type(b):
        return {"path": path, "kind": "type", "old": type(a).__name__, "new": type(b).__name__}
    if isinstance(a, dict):
        for key in sorted(set(a) | set(b)):
            if key not in a:
                return {"path": f"{path}/{key}", "kind": "missing_old"}
            if key not in b:
                return {"path": f"{path}/{key}", "kind": "missing_new"}
            diff = first_difference(a[key], b[key], f"{path}/{key}")
            if diff:
                return diff
        return None
    if isinstance(a, list):
        if len(a) != len(b):
            return {"path": path, "kind": "length", "old": len(a), "new": len(b)}
        for i, (x, y) in enumerate(zip(a, b)):
            diff = first_difference(x, y, f"{path}/{i}")
            if diff:
                return diff
        return None
    if a != b:
        return {"path": path, "kind": "value", "old": a, "new": b}
    return None


def compare_checkpoints(old: dict[str, Any], new: dict[str, Any]) -> dict[str, Any]:
    old_sem = semantic_projection(old)
    new_sem = semantic_projection(new)

    old_runtime = old.get("learner_state", {}).get("runtime_identity", {})
    new_runtime = new.get("learner_state", {}).get("runtime_identity", {})
    runtime_identity_equal_except_sha = {
        k: old_runtime.get(k) == new_runtime.get(k)
        for k in set(old_runtime) | set(new_runtime)
        if k != "acsie_ref"
    }

    old_lineage = {
        "structural": old.get("learner_state", {})
        .get("meta_policy", {})
        .get("selected_structural_lineage_ids"),
        "executable": old.get("learner_state", {})
        .get("meta_policy", {})
        .get("selected_executable_lineage_ids"),
    }
    del old_lineage  # kept out of the authoritative policy comparison

    checks = {
        "generation_next_equal": old.get("generation_next") == new.get("generation_next"),
        "seed_equal": old.get("seed") == new.get("seed"),
        "schema_equal": old.get("schema") == new.get("schema"),
        "runtime_identity_equal_except_sha": all(runtime_identity_equal_except_sha.values()) if runtime_identity_equal_except_sha else True,
        "old_runtime_sha_expected": old_runtime.get("acsie_ref") == EXPECTED_OLD_RUNTIME,
        "repaired_runtime_sha_expected": new_runtime.get("acsie_ref") == EXPECTED_REPAIRED_RUNTIME,
        "meta_policy_equal": old["learner_state"]["meta_policy"] == new["learner_state"]["meta_policy"],
        "local_rng_state_equal": old.get("local_rng_state") == new.get("local_rng_state"),
        "candidate_identity_multiset_equal": process_identity_multiset(old) == process_identity_multiset(new),
        "candidate_content_multiset_equal": process_content_multiset(old) == process_content_multiset(new),
        "frontier_ids_equal": frontier_ids(old) == frontier_ids(new),
        "frontier_content_multiset_equal": frontier_content_multiset(old) == frontier_content_multiset(new),
        "retained_capabilities_equal": _canon(old["retained"]) == _canon(new["retained"]),
        "primitives_equal": _canon(old["learner_state"]["primitives"]) == _canon(new["learner_state"]["primitives"]),
        "generation_results_equal": _canon(old["generations_out"]) == _canon(new["generations_out"]),
        "events_equal": _canon(old["learner_state"]["events"]) == _canon(new["learner_state"]["events"]),
        "target_sequence_equal": _target_sequence(old) == _target_sequence(new),
        "acceptance_sequence_equal": [x.get("accepted") for x in old.get("generations_out", [])]
            == [x.get("accepted") for x in new.get("generations_out", [])],
        "closure_reuse_equal": [x.get("closure_reuse_rate") for x in old.get("generations_out", [])]
            == [x.get("closure_reuse_rate") for x in new.get("generations_out", [])],
        "probe_reuse_equal": [x.get("probe_reuse_rate") for x in old.get("generations_out", [])]
            == [x.get("probe_reuse_rate") for x in new.get("generations_out", [])],
        "recursive_reuse_equal": [x.get("recursive_reuse_ok") for x in old.get("generations_out", [])]
            == [x.get("recursive_reuse_ok") for x in new.get("generations_out", [])],
        "lineage_validity_equal": [x.get("lineage_ok") for x in old.get("generations_out", [])]
            == [x.get("lineage_ok") for x in new.get("generations_out", [])],
        "resource_cost_equal": [x.get("resource_cost") for x in old.get("generations_out", [])]
            == [x.get("resource_cost") for x in new.get("generations_out", [])],
        "generation_metric_projection_equal": _generation_metric_projection(old)
            == _generation_metric_projection(new),
        "semantic_state_equal_after_allowed_canonicalization": old_sem == new_sem,
    }

    allowed_runtime_difference = (
        old_runtime.get("acsie_ref") != new_runtime.get("acsie_ref")
    )

    scientifically_relevant_checks = [
        value for key, value in checks.items()
        if key not in {
            "runtime_identity_equal_except_sha",
            "old_runtime_sha_expected",
            "repaired_runtime_sha_expected",
        }
    ]
    scientifically_neutral = all(scientifically_relevant_checks)

    return {
        "scientifically_neutral_at_boundary": scientifically_neutral,
        "allowed_runtime_sha_difference_observed": allowed_runtime_difference,
        "checks": checks,
        "old_state_digest": old.get("state_digest"),
        "repaired_state_digest": new.get("state_digest"),
        "state_digest_difference_allowed": old.get("state_digest") != new.get("state_digest"),
        "first_semantic_difference": None if old_sem == new_sem else first_difference(old_sem, new_sem),
    }


def compare_artifact(evidence_dir: Path) -> dict[str, Any]:
    boundaries = []
    for generation in BOUNDARIES:
        old_path = evidence_dir / f"old-gen{generation}.checkpoint.json"
        new_path = evidence_dir / f"repaired-gen{generation}.checkpoint.json"
        if not old_path.exists() or not new_path.exists():
            raise FileNotFoundError(f"missing boundary evidence for gen{generation}")
        old = json.loads(old_path.read_text(encoding="utf-8"))
        new = json.loads(new_path.read_text(encoding="utf-8"))
        row = compare_checkpoints(old, new)
        row.update({
            "generation": generation,
            "old_checkpoint_sha256": hashlib.sha256(old_path.read_bytes()).hexdigest(),
            "repaired_checkpoint_sha256": hashlib.sha256(new_path.read_bytes()).hexdigest(),
        })
        boundaries.append(row)

    passed = all(row["scientifically_neutral_at_boundary"] for row in boundaries)
    return {
        "protocol": "ACSIE.PR110.phase4.scientific-neutrality.existing-artifact.v1",
        "boundaries": boundaries,
        "semantic_neutrality": "PASSED" if passed else "FAILED",
        "comparator": "PASSED" if passed else "FAILED",
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--evidence-dir", type=Path, required=True)
    parser.add_argument("--report", type=Path, required=True)
    args = parser.parse_args()

    report = compare_artifact(args.evidence_dir)
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report, indent=2, sort_keys=True))
    return 0 if report["semantic_neutrality"] == "PASSED" else 20


if __name__ == "__main__":
    raise SystemExit(main())
