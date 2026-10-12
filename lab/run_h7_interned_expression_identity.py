#!/usr/bin/env python3
"""Run exact interned-syntax identity beside the same compositional-key control.

The shared runner performs source pinning, parity, six-file regression, RSS/wall
limits, live stack snapshots, and raw artifact collection. This adapter
normalizes metadata so archived evidence labels the *actual* between-arm factor.
"""
from __future__ import annotations

import contextlib
import io
import json
import sys
from pathlib import Path

LAB = Path(__file__).resolve().parent
sys.path.insert(0, str(LAB))

import patch_h7_compositional_structural_key as base_patch
from patch_h7_interned_expression_identity import apply_source as interned_apply_source

# The interning patch captures the original helper before this reassignment.
# Both arms therefore use the exact same compositional structural key; only
# candidate identity storage differs.
base_patch.apply_source = interned_apply_source

import run_h7_compositional_structural_key as runner

ARM_RENAMES = {
    "control_frontier128_original_structural_key":
        "control_compositional_key_json_identity",
    "candidate_frontier128_compositional_structural_key":
        "candidate_compositional_key_interned_identity",
}

def normalise_evidence(evidence: Path) -> dict:
    summary_path = evidence / "summary.json"
    summary = json.loads(summary_path.read_text(encoding="utf-8"))

    summary["schema"] = "ACSIE.h7_interned_expression_identity.diagnostic.v1"
    summary["factor"] = (
        "replace each generated candidate's full canonical JSON syntax identity "
        "with an exact interned structural ID before duplicate/dominance checks"
    )
    summary["fixed_factor"] = (
        "the identical compositional structural-key implementation and "
        "lineage-frontier FIFO cache capacity 128 in both arms"
    )
    summary["identity_contract"] = {
        "control": "canonical compact JSON expression identity",
        "candidate": "interned exact raw-syntax identity",
        "equivalence": "must match ACSIE canonical(expr) + json.dumps semantics exactly",
        "target_access_during_discovery": False,
        "external_model": False,
        "task_family_route": False,
        "production_source_modified": False,
    }

    actual = summary.get("actual_gen6", {})
    for old, new in ARM_RENAMES.items():
        if old in actual:
            record = actual.pop(old)
            record["arm"] = new
            record["raw_telemetry_prefix"] = old
            actual[new] = record
            status_path = evidence / f"{old}-status.json"
            if status_path.exists():
                status = json.loads(status_path.read_text(encoding="utf-8"))
                status["arm"] = new
                status["raw_telemetry_prefix"] = old
                status_path.write_text(
                    json.dumps(status, indent=2, sort_keys=True) + "\n",
                    encoding="utf-8",
                )

        stack = summary.get("stack_and_live_telemetry", {})
        if old in stack:
            stack[new] = stack.pop(old)

    summary["interpretation_guard"] = (
        "A wrapper success is not a scientific pass. A candidate must preserve exact "
        "syntax identity, signature parity, regressions and emit a complete seven-generation "
        "record; either child timing out remains INCONCLUSIVE."
    )
    summary_path.write_text(
        json.dumps(summary, indent=2, sort_keys=True, allow_nan=True) + "\n",
        encoding="utf-8",
    )
    return summary

def main() -> int:
    captured = io.StringIO()
    with contextlib.redirect_stdout(captured):
        rc = runner.main()
    if rc != 0:
        print(captured.getvalue(), end="")
        return rc

    evidence = None
    for index, item in enumerate(sys.argv[:-1]):
        if item == "--evidence":
            evidence = Path(sys.argv[index + 1]).resolve()
            break
    if evidence is None:
        print(captured.getvalue(), end="")
        raise SystemExit("missing --evidence for normalized report")
    summary = normalise_evidence(evidence)
    print(json.dumps({
        "experiment": "H7 interned expression identity",
        "preflight": summary.get("preflight"),
        "actual_gen6": {
            arm: {
                key: row.get(key) for key in (
                    "status", "scientific_disposition", "elapsed_seconds",
                    "max_observed_rss_bytes", "record_parse_status", "stdout_bytes"
                )
            }
            for arm, row in summary.get("actual_gen6", {}).items()
        },
        "generation_snapshots": {
            arm: {
                "generations": row.get("generation_indices_observed", []),
                "latest": row.get("latest_snapshot"),
            }
            for arm, row in summary.get("stack_and_live_telemetry", {}).items()
        },
        "summary": "summary.json",
        "production_source_modified": False,
    }, indent=2, sort_keys=True, allow_nan=True))
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
