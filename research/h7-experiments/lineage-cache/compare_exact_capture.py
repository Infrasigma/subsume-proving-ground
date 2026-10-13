#!/usr/bin/env python3
"""Exact behavioural-search parity checker for H7 lineage memoization."""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
from typing import Any

BASELINE_FRONTIER_SHA256 = "676fe88653c88332cb0265085104fa3d9d185e4a6e8e997e91a6e686bc2cb061"
BASELINE_MODULE_SHA256 = "242d5399c72e474b2402dae761afb215f9a4ae51cbba1b1d6d566ba46fae45ee"
CANDIDATE_MODULE_BLOB_SHA1 = "de7c20f453afc9ca0cdb90dfc9625abe960fd957"
DEPTH_FIELDS = (
    "depth",
    "class_count",
    "representative_count",
    "classes",
    "frontier_live_representatives_by_ast_depth",
    "frontier_bucket_sizes_by_ast_depth",
    "search_stats_at_capture",
)


def canonical(obj: Any) -> str:
    return json.dumps(obj, sort_keys=True, separators=(",", ":"), ensure_ascii=True, allow_nan=False)


def sha256_file(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load_capture(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def elapsed_before_depth(capture: dict[str, Any], depth: int) -> float:
    item = next(row for row in capture["captures"] if int(row["depth"]) == depth)
    return float(item["capture_hook_entered_elapsed_seconds"])


def search_duration_to_depth(capture: dict[str, Any], depth: int) -> float:
    item = next(row for row in capture["captures"] if int(row["depth"]) == depth)
    entry = float(item["invocation"]["hook_elapsed_seconds_from_run_seed_start"])
    reached = float(item["capture_hook_entered_elapsed_seconds"])
    return reached - entry


def invocation_projection(capture: dict[str, Any]) -> dict[str, Any]:
    inv = dict(capture["captures"][0]["invocation"])
    inv.pop("hook_elapsed_seconds_from_run_seed_start", None)
    return inv


def depth_projection(item: dict[str, Any]) -> dict[str, Any]:
    return {field: item.get(field) for field in DEPTH_FIELDS}


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--baseline", required=True)
    ap.add_argument("--candidate", required=True)
    ap.add_argument("--candidate-module", required=True)
    ap.add_argument("--output", required=True)
    args = ap.parse_args()

    base_path = Path(args.baseline)
    candidate_path = Path(args.candidate)
    module_path = Path(args.candidate_module)
    base = load_capture(base_path)
    cand = load_capture(candidate_path)
    module_blob_sha = __import__("subprocess").check_output(
        ["git", "hash-object", str(module_path)], text=True
    ).strip()
    module_sha = sha256_file(module_path)
    baseline_sha = sha256_file(base_path)

    checks: list[dict[str, Any]] = []

    def check(name: str, ok: bool, **details):
        checks.append({"name": name, "status": "PASS" if ok else "FAIL", **details})
        if not ok:
            raise AssertionError(f"{name} failed: {details}")

    check("baseline_capture_sha256", baseline_sha == BASELINE_FRONTIER_SHA256,
          expected=BASELINE_FRONTIER_SHA256, actual=baseline_sha)
    check("baseline_module_sha256", base["observer"]["original_behavioral_search_sha256"] == BASELINE_MODULE_SHA256,
          expected=BASELINE_MODULE_SHA256, actual=base["observer"]["original_behavioral_search_sha256"])
    check("candidate_module_blob_sha1", module_blob_sha == CANDIDATE_MODULE_BLOB_SHA1,
          expected=CANDIDATE_MODULE_BLOB_SHA1, actual=module_blob_sha)
    check("candidate_module_sha_matches_loaded_provenance",
          cand["modules"]["behavioral_search"]["sha256"] == module_sha,
          loaded=cand["modules"]["behavioral_search"]["sha256"], disk=module_sha)
    check("candidate_loaded_from_experiment_snapshot",
          "/research/h7-experiments/lineage-cache/B/cognitive_core/behavioral_search.py" in cand["modules"]["behavioral_search"]["path"],
          path=cand["modules"]["behavioral_search"]["path"])

    for key in ("runtime_sha", "proving_sha", "seed", "generation", "requested_capture_role",
                "requested_probe_index", "stop_at_depth", "selected_search_call_index"):
        check(f"identity_{key}", base.get(key) == cand.get(key),
              baseline=base.get(key), candidate=cand.get(key))
    check("capture_status",
          base["status"] == "CAPTURE_COMPLETE_DEPTH3" and cand["status"] == "CAPTURE_COMPLETE_DEPTH3",
          baseline=base["status"], candidate=cand["status"])
    check("same_caller_and_probe", invocation_projection(base) == invocation_projection(cand),
          baseline=invocation_projection(base), candidate=invocation_projection(cand))

    base_depths = [int(row["depth"]) for row in base["captures"]]
    cand_depths = [int(row["depth"]) for row in cand["captures"]]
    check("same_captured_depths", base_depths == cand_depths == [2, 3],
          baseline=base_depths, candidate=cand_depths)

    depth_diffs = []
    for b_row, c_row in zip(base["captures"], cand["captures"]):
        b_proj = depth_projection(b_row)
        c_proj = depth_projection(c_row)
        same = canonical(b_proj) == canonical(c_proj)
        depth_diffs.append({
            "depth": int(b_row["depth"]),
            "status": "PASS" if same else "FAIL",
            "baseline_class_count": b_row["class_count"],
            "candidate_class_count": c_row["class_count"],
            "baseline_representative_count": b_row["representative_count"],
            "candidate_representative_count": c_row["representative_count"],
            "baseline_search_stats_sha256": hashlib.sha256(canonical(b_row["search_stats_at_capture"]).encode()).hexdigest(),
            "candidate_search_stats_sha256": hashlib.sha256(canonical(c_row["search_stats_at_capture"]).encode()).hexdigest(),
            "exact_full_frontier_and_stats_parity": same,
        })
        if not same:
            raise AssertionError(f"exact frontier/stats parity failed at depth {b_row['depth']}")
    checks.append({"name":"all_captured_depths_exact_frontier_and_stats_parity",
                   "status":"PASS" if all(x["status"]=="PASS" for x in depth_diffs) else "FAIL",
                   "depths":depth_diffs})

    elapsed = {
        "baseline_elapsed_to_depth2_seconds": elapsed_before_depth(base, 2),
        "candidate_elapsed_to_depth2_seconds": elapsed_before_depth(cand, 2),
        "total_elapsed_to_depth2_speedup_ratio": elapsed_before_depth(base, 2) / max(elapsed_before_depth(cand, 2), 1e-12),
        "baseline_search_call_to_depth2_seconds": search_duration_to_depth(base, 2),
        "candidate_search_call_to_depth2_seconds": search_duration_to_depth(cand, 2),
        "search_call_depth2_speedup_ratio": search_duration_to_depth(base, 2) / max(search_duration_to_depth(cand, 2), 1e-12),
        "baseline_search_call_depth2_to_depth3_seconds": elapsed_before_depth(base, 3) - elapsed_before_depth(base, 2),
        "candidate_search_call_depth2_to_depth3_seconds": elapsed_before_depth(cand, 3) - elapsed_before_depth(cand, 2),
        "note": "Elapsed-to-depth includes all prior run_seed work; isolated search-call duration subtracts that invocation's recorded entry time.",
    }
    report = {
        "schema": "ACSIE.H7.lineage-cache.exact-parity.v1",
        "status": "PASSED_PARITY" if all(x["status"] == "PASS" for x in checks) else "FAILED_PARITY",
        "hypothesis": "H7-LC1",
        "baseline_capture_sha256": baseline_sha,
        "candidate_capture_sha256": sha256_file(candidate_path),
        "baseline_module_sha256": BASELINE_MODULE_SHA256,
        "candidate_module_blob_sha1": module_blob_sha,
        "candidate_module_sha256": module_sha,
        "checks": checks,
        "depth_comparisons": depth_diffs,
        "timings": elapsed,
        "interpretation_boundary": (
            "Exact frontier and search-statistics parity on one pinned Gen5 probe is local diagnostic evidence only. "
            "It does not qualify a complete Gen5/Gen6 trajectory, prove compression safe, or demonstrate AGI/ASI."
        ),
    }
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps({"status":report["status"],"checks":len(checks),
                      "depth2_speedup":elapsed["search_call_depth2_speedup_ratio"],
                      "candidate_sha256":report["candidate_module_sha256"],
                      "report_sha256":sha256_file(output),"output":str(output)},sort_keys=True))
    

if __name__ == "__main__":
    main()
