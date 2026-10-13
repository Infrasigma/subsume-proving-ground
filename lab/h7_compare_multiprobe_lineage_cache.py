#!/usr/bin/env python3
"""Exact depth-2/3 frontier and timing comparison for a single four-probe replay."""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
from typing import Any

PROBES = (0, 1, 2, 3)
DEPTH_FIELDS = (
    "depth",
    "class_count",
    "representative_count",
    "classes",
    "frontier_live_representatives_by_ast_depth",
    "frontier_bucket_sizes_by_ast_depth",
    "search_stats_at_capture",
)


def canonical(value: Any) -> str:
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=True, allow_nan=False)


def sha256_file(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load_probe(root: Path, probe: int) -> tuple[Path, dict[str, Any]]:
    path = root / f"probe-{probe}-frontier.json"
    if not path.exists():
        raise FileNotFoundError(f"missing probe {probe} capture: {path}")
    payload = json.loads(path.read_text(encoding="utf-8"))
    return path, payload


def project_invocation(data: dict[str, Any]) -> dict[str, Any]:
    inv = dict(data["captures"][0]["invocation"])
    inv.pop("hook_elapsed_seconds_from_run_seed_start", None)
    return inv


def project_depth(row: dict[str, Any]) -> dict[str, Any]:
    return {key: row.get(key) for key in DEPTH_FIELDS}


def depth_row(data, depth):
    return next(row for row in data["captures"] if int(row["depth"]) == depth)


def elapsed_to_depth(data, depth):
    return float(depth_row(data, depth)["capture_hook_entered_elapsed_seconds"])


def selected_search_entry(data):
    return float(data["captures"][0]["invocation"]["hook_elapsed_seconds_from_run_seed_start"])


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--baseline-dir", required=True)
    parser.add_argument("--candidate-dir", required=True)
    parser.add_argument("--baseline-module", required=True)
    parser.add_argument("--candidate-module", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    baseline_dir = Path(args.baseline_dir)
    candidate_dir = Path(args.candidate_dir)
    baseline_module = Path(args.baseline_module)
    candidate_module = Path(args.candidate_module)

    checks = []
    probe_results = []

    def check(name, ok, **details):
        checks.append({"name": name, "status": "PASS" if ok else "FAIL", **details})
        if not ok:
            raise AssertionError(f"{name} failed: {details}")

    import subprocess
    baseline_blob = subprocess.check_output(["git", "hash-object", str(baseline_module)], text=True).strip()
    candidate_blob = subprocess.check_output(["git", "hash-object", str(candidate_module)], text=True).strip()
    baseline_sha = sha256_file(baseline_module)
    candidate_sha = sha256_file(candidate_module)
    check("baseline_module_blob", baseline_blob == "c126aa723b5c5f6013ac7ee53a9401875c2395ee", actual=baseline_blob)
    check("candidate_module_blob", candidate_blob == "de7c20f453afc9ca0cdb90dfc9625abe960fd957", actual=candidate_blob)

    for probe in PROBES:
        b_path, b = load_probe(baseline_dir, probe)
        c_path, c = load_probe(candidate_dir, probe)
        ident_keys = ("runtime_sha", "proving_sha", "seed", "generation", "requested_capture_role",
                      "requested_probe_index", "stop_at_depth", "selected_search_call_index", "status")
        identity_equal = all(b.get(key) == c.get(key) for key in ident_keys)
        check(f"probe_{probe}_identity", identity_equal,
              baseline={key:b.get(key) for key in ident_keys},
              candidate={key:c.get(key) for key in ident_keys})
        check(f"probe_{probe}_requested_index", b.get("requested_probe_index") == probe == c.get("requested_probe_index"),
              baseline=b.get("requested_probe_index"), candidate=c.get("requested_probe_index"))
        check(f"probe_{probe}_capture_status",
              b.get("status") == c.get("status") == "CAPTURE_COMPLETE_DEPTH3",
              baseline=b.get("status"), candidate=c.get("status"))
        check(f"probe_{probe}_baseline_module_provenance",
              b.get("observer", {}).get("original_behavioral_search_sha256") == baseline_sha,
              reported=b.get("observer", {}).get("original_behavioral_search_sha256"), expected=baseline_sha)
        check(f"probe_{probe}_candidate_module_provenance",
              c.get("observer", {}).get("original_behavioral_search_sha256") == candidate_sha,
              reported=c.get("observer", {}).get("original_behavioral_search_sha256"), expected=candidate_sha)
        check(f"probe_{probe}_same_call_context",
              canonical(project_invocation(b)) == canonical(project_invocation(c)),
              baseline=project_invocation(b), candidate=project_invocation(c))
        bd = [int(row["depth"]) for row in b.get("captures", [])]
        cd = [int(row["depth"]) for row in c.get("captures", [])]
        check(f"probe_{probe}_captured_depths", bd == cd == [2, 3], baseline=bd, candidate=cd)

        depth_comparisons = []
        for depth in (2, 3):
            br = depth_row(b, depth)
            cr = depth_row(c, depth)
            same = canonical(project_depth(br)) == canonical(project_depth(cr))
            detail = {
                "depth": depth,
                "exact_frontier_and_full_original_stats_parity": same,
                "baseline_class_count": br["class_count"],
                "candidate_class_count": cr["class_count"],
                "baseline_representative_count": br["representative_count"],
                "candidate_representative_count": cr["representative_count"],
                "baseline_stats_sha256": hashlib.sha256(canonical(br["search_stats_at_capture"]).encode()).hexdigest(),
                "candidate_stats_sha256": hashlib.sha256(canonical(cr["search_stats_at_capture"]).encode()).hexdigest(),
            }
            check(f"probe_{probe}_depth_{depth}_exact_parity", same, **detail)
            depth_comparisons.append(detail)

        b_entry = selected_search_entry(b)
        c_entry = selected_search_entry(c)
        b_d2, c_d2 = elapsed_to_depth(b, 2), elapsed_to_depth(c, 2)
        b_d3, c_d3 = elapsed_to_depth(b, 3), elapsed_to_depth(c, 3)
        timings = {
            "baseline_search_entry_seconds": b_entry,
            "candidate_search_entry_seconds": c_entry,
            "pre_search_prelude_speedup": b_entry / max(c_entry, 1e-12),
            "baseline_elapsed_to_depth2_seconds": b_d2,
            "candidate_elapsed_to_depth2_seconds": c_d2,
            "elapsed_to_depth2_speedup": b_d2 / max(c_d2, 1e-12),
            "baseline_elapsed_to_depth3_seconds": b_d3,
            "candidate_elapsed_to_depth3_seconds": c_d3,
            "elapsed_to_depth3_speedup": b_d3 / max(c_d3, 1e-12),
            "baseline_search_call_to_depth2_seconds": b_d2 - b_entry,
            "candidate_search_call_to_depth2_seconds": c_d2 - c_entry,
            "search_call_depth2_speedup": (b_d2-b_entry) / max(c_d2-c_entry, 1e-12),
            "baseline_search_depth2_to_depth3_seconds": b_d3 - b_d2,
            "candidate_search_depth2_to_depth3_seconds": c_d3 - c_d2,
            "search_extension_depth3_speedup": (b_d3-b_d2) / max(c_d3-c_d2, 1e-12),
        }
        probe_results.append({
            "probe_index": probe,
            "baseline_capture_sha256": sha256_file(b_path),
            "candidate_capture_sha256": sha256_file(c_path),
            "depth_comparisons": depth_comparisons,
            "timings": timings,
        })

    all_pass = all(x["status"] == "PASS" for x in checks)
    report = {
        "schema": "ACSIE.H7.lineage-cache.multi-probe-exact-parity.v1",
        "status": "PASSED_PARITY" if all_pass else "FAILED_PARITY",
        "hypothesis": "H7-LC1: invocation-local lineage closure memoization",
        "baseline_module": {"git_blob_sha1": baseline_blob, "sha256": baseline_sha},
        "candidate_module": {"git_blob_sha1": candidate_blob, "sha256": candidate_sha},
        "checks": checks,
        "probe_results": probe_results,
        "interpretation_boundary": (
            "Exact depth-2/3 frontier and search-statistics parity over four Gen5 probe calls is bounded evidence for an "
            "observationally transparent memoization optimization. It does not complete Gen5/Gen6 trajectory qualification "
            "or the frozen 5-seed x 12-generation gate. No runtime compression/deletion is tested."
        ),
    }
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps({
        "status": report["status"],
        "checks": len(checks),
        "probes": len(probe_results),
        "all_frontiers_exact": all(x["status"] == "PASS" for x in checks if "_depth_" in x["name"]),
        "probe_prelude_speedups": {str(x["probe_index"]): round(x["timings"]["pre_search_prelude_speedup"], 3) for x in probe_results},
        "report_sha256": sha256_file(output),
    }, sort_keys=True))


if __name__ == "__main__":
    main()
