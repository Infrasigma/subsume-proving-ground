from __future__ import annotations

"""Refuse self-certified ASI claims unless an independent attestation is present.

The internal ACSIE evaluator may produce readiness evidence, but this file only emits
an ASI qualification status when an externally supplied attestation explicitly confirms
the required independent gates for the exact source commit.
"""

import argparse
import json
from pathlib import Path


REQUIRED_TRUE = (
    "independent_evaluator",
    "final_hidden_suite",
    "network_independence_demonstrated",
    "external_replication_pass",
    "all_pre_registered_thresholds_passed",
)


def qualify(attestation: dict, expected_commit: str) -> dict:
    errors = []
    if attestation.get("schema") != "ACSIE.asi-qualification.attestation.v1":
        errors.append("invalid attestation schema")
    if not expected_commit:
        errors.append("missing expected source commit")
    elif attestation.get("source_commit") != expected_commit:
        errors.append("source commit mismatch")
    for key in REQUIRED_TRUE:
        if attestation.get(key) is not True:
            errors.append(f"{key} is not true")
    if not str(attestation.get("independent_evaluator_id", "")).strip():
        errors.append("missing independent evaluator id")
    if not str(attestation.get("evaluator_manifest_hash", "")).strip():
        errors.append("missing evaluator manifest hash")

    qualified = not errors
    return {
        "source_commit": expected_commit,
        "qualification_status": (
            "ASI-qualified-under-ACSIE-protocol-v1" if qualified else "NOT_QUALIFIED"
        ),
        "qualified": qualified,
        "errors": errors,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--attestation", required=True)
    parser.add_argument("--source-commit", required=True)
    args = parser.parse_args()

    attestation = json.loads(Path(args.attestation).read_text(encoding="utf-8"))
    result = qualify(attestation, args.source_commit)
    print(json.dumps(result, indent=2, sort_keys=True))
    return 0 if result["qualified"] else 2


if __name__ == "__main__":
    raise SystemExit(main())
