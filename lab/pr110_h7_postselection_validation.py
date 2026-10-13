#!/usr/bin/env python3
"""Post-selection validation of H7 representative selectors.

This script is intentionally downstream of frontier capture. Candidate selection
uses only captured discovery-frontier metadata. Holdout/transfer/OOD inputs and
targets are consumed here, after selection has been completed, solely to measure
how many distinct observed behaviours or target-matching signatures are lost.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import sys
from pathlib import Path
from typing import Any

from cognitive_core.recursive_cognitive_compiler import eval_expr
from lab.pr110_h7_offline_counterfactual import KS, POLICIES, greedy_select


def canonical(value: Any) -> str:
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=True, allow_nan=False)


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def float_signature(value: float) -> str:
    value = float(value)
    if math.isnan(value):
        return "NONFINITE:nan"
    if math.isinf(value):
        return "NONFINITE:+inf" if value > 0 else "NONFINITE:-inf"
    return value.hex()


def evaluate_rep(rep: dict[str, Any], rows: list[dict[str, Any]], macros: dict[str, Any]) -> dict[str, Any]:
    predictions = []
    errors = []
    eval_failures = []
    for index, row in enumerate(rows):
        try:
            prediction = float(eval_expr(rep["expression"], row["inputs"], macros))
            predictions.append(float_signature(prediction))
            if not math.isfinite(prediction):
                errors.append(None)
            else:
                errors.append(abs(prediction - float(row["target"])))
        except Exception as exc:
            predictions.append("EVAL_ERROR:" + type(exc).__name__ + ":" + str(exc)[:160])
            errors.append(None)
            eval_failures.append({"row_index": index, "type": type(exc).__name__, "message": str(exc)[:160]})

    signature = canonical(predictions)
    valid_errors = [x for x in errors if x is not None]
    mae = None if len(valid_errors) != len(rows) or not rows else sum(valid_errors) / len(rows)
    return {
        "signature": signature,
        "row_count": len(rows),
        "predictions_signature": predictions,
        "eval_failures": eval_failures,
        "mean_absolute_error": mae,
        "target_perfect": bool(rows) and mae is not None and mae <= 1e-9,
    }


def summarize_capture(
    capture: dict[str, Any],
    bundle: dict[str, Any],
    *,
    k: int | None,
    policy: str,
    evaluated: dict[str, dict[str, dict[str, Any]]],
) -> dict[str, Any]:
    splits_out = {}
    for split_name, rows in bundle["splits"].items():
        total_classes = 0
        total_representatives = 0
        selected_representatives = 0
        classes_with_signature_loss = 0
        unique_signatures_original = 0
        unique_signatures_selected = 0
        unique_signatures_lost = 0
        classes_with_target_perfect_signature_loss = 0
        target_perfect_signatures_original = 0
        target_perfect_signatures_selected = 0
        target_perfect_signatures_lost = 0
        target_perfect_representatives_original = 0
        target_perfect_representatives_selected = 0
        original_eval_errors = 0
        selected_eval_errors = 0
        max_class_size = 0
        class_loss_examples = []

        for cls in capture.get("classes", []):
            reps = list(cls.get("representatives", []))
            if not reps:
                continue
            chosen = greedy_select(reps, k, policy)
            total_classes += 1
            total_representatives += len(reps)
            selected_representatives += len(chosen)
            max_class_size = max(max_class_size, len(reps))

            signatures = {
                evaluated[split_name][str(rep["state_token"])]["signature"]
                for rep in reps
            }
            selected_signatures = {
                evaluated[split_name][str(rep["state_token"])]["signature"]
                for rep in chosen
            }
            lost = signatures - selected_signatures
            unique_signatures_original += len(signatures)
            unique_signatures_selected += len(selected_signatures)
            unique_signatures_lost += len(lost)
            if lost:
                classes_with_signature_loss += 1
                if len(class_loss_examples) < 20:
                    class_loss_examples.append({
                        "behavior_key_sha256": sha256_bytes(canonical(cls.get("behavior_key", {})).encode()),
                        "original_representatives": len(reps),
                        "selected_representatives": len(chosen),
                        "unique_validation_signatures_original": len(signatures),
                        "unique_validation_signatures_selected": len(selected_signatures),
                        "unique_signatures_lost": len(lost),
                    })

            original_perfect = {
                evaluated[split_name][str(rep["state_token"])]["signature"]
                for rep in reps
                if evaluated[split_name][str(rep["state_token"])]["target_perfect"]
            }
            selected_perfect = {
                evaluated[split_name][str(rep["state_token"])]["signature"]
                for rep in chosen
                if evaluated[split_name][str(rep["state_token"])]["target_perfect"]
            }
            perfect_lost = original_perfect - selected_perfect
            target_perfect_signatures_original += len(original_perfect)
            target_perfect_signatures_selected += len(selected_perfect)
            target_perfect_signatures_lost += len(perfect_lost)
            if perfect_lost:
                classes_with_target_perfect_signature_loss += 1

            target_perfect_representatives_original += sum(
                evaluated[split_name][str(rep["state_token"])]["target_perfect"] for rep in reps
            )
            target_perfect_representatives_selected += sum(
                evaluated[split_name][str(rep["state_token"])]["target_perfect"] for rep in chosen
            )
            original_eval_errors += sum(
                bool(evaluated[split_name][str(rep["state_token"])]["eval_failures"]) for rep in reps
            )
            selected_eval_errors += sum(
                bool(evaluated[split_name][str(rep["state_token"])]["eval_failures"]) for rep in chosen
            )

        splits_out[split_name] = {
            "validation_rows": len(rows),
            "behavioral_classes": total_classes,
            "original_representatives": total_representatives,
            "selected_representatives": selected_representatives,
            "fraction_representatives_removed": (
                0.0 if not total_representatives else 1.0 - selected_representatives / total_representatives
            ),
            "unique_validation_signatures_original": unique_signatures_original,
            "unique_validation_signatures_selected": unique_signatures_selected,
            "unique_validation_signatures_lost": unique_signatures_lost,
            "classes_with_validation_signature_loss": classes_with_signature_loss,
            "target_perfect_representatives_original": target_perfect_representatives_original,
            "target_perfect_representatives_selected": target_perfect_representatives_selected,
            "unique_target_perfect_signatures_original": target_perfect_signatures_original,
            "unique_target_perfect_signatures_selected": target_perfect_signatures_selected,
            "unique_target_perfect_signatures_lost": target_perfect_signatures_lost,
            "classes_with_target_perfect_signature_loss": classes_with_target_perfect_signature_loss,
            "representatives_with_evaluation_errors_original": original_eval_errors,
            "representatives_with_evaluation_errors_selected": selected_eval_errors,
            "max_representatives_in_one_class": max_class_size,
            "first_20_class_loss_examples": class_loss_examples,
        }

    return {"K": k, "policy": policy, "by_split": splits_out}


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True, help="H7 frontier-capture JSON")
    parser.add_argument("--output", required=True, help="report destination")
    args = parser.parse_args()

    input_path = Path(args.input)
    data = json.loads(input_path.read_text(encoding="utf-8"))
    bundle = data.get("postselection_validation_bundle")
    if not isinstance(bundle, dict):
        raise ValueError("capture lacks postselection_validation_bundle; do not infer validation data")
    if bundle.get("schema") != "ACSIE.PR110.H7.postselection-validation-inputs.v1":
        raise ValueError("unexpected postselection validation bundle schema")
    boundary = bundle.get("selection_boundary", {})
    if boundary.get("holdout_transfer_ood_used_for_frontier_selection") is not False:
        raise ValueError("validation boundary flag does not forbid validation-data use during selection")
    if boundary.get("postselection_validation_bundle_used_by_frontier_selector") is not False:
        raise ValueError("validation bundle was not explicitly excluded from selector inputs")
    if boundary.get("target_values_are_postselection_only") is not True:
        raise ValueError("target values are not marked postselection-only")

    captures = data.get("captures", [])
    if not captures:
        raise ValueError("no frontier captures found")
    macros = bundle.get("primitive_expression_archive", {})
    if not isinstance(macros, dict):
        raise ValueError("primitive expression archive must be an object")
    splits = bundle.get("splits", {})
    required_splits = {"holdout", "transfer", "ood"}
    if set(splits) != required_splits:
        raise ValueError(f"expected exactly validation splits {sorted(required_splits)}, got {sorted(splits)}")
    for split_name, rows in splits.items():
        if not rows:
            raise ValueError(f"validation split {split_name!r} is empty")

    report = {
        "schema": "ACSIE.PR110.H7.postselection-validation.v1",
        "status": "DESCRIPTIVE_POSTSELECTION_VALIDATION",
        "input_capture_sha256": sha256_bytes(input_path.read_bytes()),
        "validation_bundle_sha256": sha256_bytes(canonical(bundle).encode("utf-8")),
        "validation_rows_by_split": {name: len(rows) for name, rows in splits.items()},
        "source": {
            "runtime_sha": data["runtime_sha"],
            "proving_sha": data["proving_sha"],
            "seed": data["seed"],
            "generation": data["generation"],
            "capture_status": data.get("status"),
            "capture_role": data.get("requested_capture_role"),
            "selected_search_call_index": data.get("selected_search_call_index"),
            "primitive_expression_count": len(macros),
            "python_version": sys.version,
            "pythonhashseed": os.environ.get("PYTHONHASHSEED", "unset"),
            "eval_expr_module_file": str(Path(sys.modules["cognitive_core.recursive_cognitive_compiler"].__file__).resolve()),
            "eval_expr_module_sha256": sha256_bytes(Path(sys.modules["cognitive_core.recursive_cognitive_compiler"].__file__).read_bytes()),
            "validator_script_sha256": sha256_bytes(Path(__file__).read_bytes()),
        },
        "information_boundary": {
            "selector_reads_validation_rows_or_targets": False,
            "validation_data_used_only_after_selector_returns": True,
            "discovery_targets_used_by_validation_selector": False,
            "target_values_used_to_choose_representatives": False,
            "uses_holdout_transfer_and_ood_only_as_postselection_measurements": True,
            "input_frontier_was_generated_by_target_conditioned_search": True,
            "input_frontier_is_already_dominance_pruned": True,
            "losses_before_capture_are_not_measured": True,
        },
        "interpretation_boundary": (
            "This is an empirical test of whether selectors preserve distinct behaviours on these specific "
            "post-selection splits. It does not prove future compositional substitutability, behavioural "
            "congruence over all possible inputs, or safe runtime deletion. No runtime compression is applied."
        ),
        "results": [],
    }

    for capture in captures:
        # Cache evaluation vectors once, independently of policy/K, so selector
        # choices cannot influence evaluation or hide a class's omitted behaviour.
        evaluated: dict[str, dict[str, dict[str, Any]]] = {name: {} for name in splits}
        classes = capture.get("classes", [])
        for cls in classes:
            for rep in cls.get("representatives", []):
                token = str(rep["state_token"])
                for split_name, rows in splits.items():
                    evaluated[split_name][token] = evaluate_rep(rep, rows, macros)

        for policy in POLICIES:
            for k in KS:
                result = summarize_capture(capture, bundle, k=k, policy=policy, evaluated=evaluated)
                report["results"].append({
                    "capture_depth": int(capture["depth"]),
                    "behavioral_classes": len(classes),
                    **result,
                })

    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    serialized = json.dumps(report, indent=2, sort_keys=True, allow_nan=False) + "\n"
    output_path.write_text(serialized, encoding="utf-8")
    summary = []
    for row in report["results"]:
        if row["K"] in (4, 8):
            summary.append({
                "depth": row["capture_depth"],
                "policy": row["policy"],
                "K": row["K"],
                "holdout_signature_classes_lost": row["by_split"]["holdout"]["classes_with_validation_signature_loss"],
                "transfer_signature_classes_lost": row["by_split"]["transfer"]["classes_with_validation_signature_loss"],
                "ood_signature_classes_lost": row["by_split"]["ood"]["classes_with_validation_signature_loss"],
                "holdout_target_perfect_signatures_lost": row["by_split"]["holdout"]["unique_target_perfect_signatures_lost"],
                "transfer_target_perfect_signatures_lost": row["by_split"]["transfer"]["unique_target_perfect_signatures_lost"],
                "ood_target_perfect_signatures_lost": row["by_split"]["ood"]["unique_target_perfect_signatures_lost"],
            })
    print(json.dumps({
        "status": report["status"],
        "source_sha256": report["source"]["eval_expr_module_sha256"],
        "validation_splits": report["validation_rows_by_split"],
        "result_rows": len(report["results"]),
        "K4_K8_summary": summary,
        "output_sha256": sha256_bytes(output_path.read_bytes()),
    }, sort_keys=True))


if __name__ == "__main__":
    main()
