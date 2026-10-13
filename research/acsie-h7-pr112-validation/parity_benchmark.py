#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import json
import os
import statistics
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent
WORKER = ROOT / "benchmark_worker.py"
VARIANTS = ("baseline", "candidate")
CASES = [(n, task) for n in (16, 24, 32) for task in ("reachable_add", "unreachable")]


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=True)


def sha256(value):
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


def run_variant(variant, n, task):
    variant_root = ROOT / variant
    env = os.environ.copy()
    env["PYTHONPATH"] = str(variant_root)
    env["PYTHONHASHSEED"] = "0"
    command = [
        sys.executable, str(WORKER),
        "--n", str(n), "--task", task, "--trials", "3",
    ]
    completed = subprocess.run(
        command, cwd=ROOT.parents[2], env=env,
        text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        timeout=120, check=False,
    )
    if completed.returncode:
        raise RuntimeError(
            f"{variant} {n} {task} failed rc={completed.returncode}\n"
            f"stdout:\n{completed.stdout}\nstderr:\n{completed.stderr}"
        )
    lines = [line for line in completed.stdout.splitlines() if line.strip()]
    if not lines:
        raise RuntimeError(f"{variant} {n} {task} produced no result")
    return json.loads(lines[-1]), completed.stderr


def main():
    report = {
        "schema": "ACSIE.H7.PR112.independent-parity-benchmark.v1",
        "source": "pinned ACSIE PR111/PR112 snapshots",
        "environment": {
            "python": sys.version,
            "pythonhashseed": os.environ.get("PYTHONHASHSEED", "0"),
            "cases": [{"primitives": n, "task": task, "rows": 10, "depth": 2} for n, task in CASES],
            "trials_per_variant_cell": 3,
        },
        "cells": [],
        "all_full_result_and_stats_parity": True,
        "interpretation": "Independent bounded synthetic parity and microbenchmark only; not end-to-end Gen5/Gen6 or scientific qualification.",
    }
    failures = []
    for n, task in CASES:
        baseline, _ = run_variant("baseline", n, task)
        candidate, _ = run_variant("candidate", n, task)
        left = baseline["full_result_and_stats"]
        right = candidate["full_result_and_stats"]
        parity = left == right
        speedup = baseline["median_seconds"] / candidate["median_seconds"]
        cell = {
            "n": n,
            "task": task,
            "baseline_median_seconds": baseline["median_seconds"],
            "candidate_median_seconds": candidate["median_seconds"],
            "candidate_speedup_ratio": speedup,
            "baseline_trial_seconds": baseline["trials_seconds"],
            "candidate_trial_seconds": candidate["trials_seconds"],
            "full_result_and_stats_parity": parity,
            "baseline_result_sha256": sha256(canonical(left)),
            "candidate_result_sha256": sha256(canonical(right)),
            "baseline_selected": left["result"],
            "candidate_selected": right["result"],
            "baseline_stats": left["stats"],
            "candidate_stats": right["stats"],
        }
        report["cells"].append(cell)
        if not parity:
            failures.append({"n": n, "task": task, "baseline": left, "candidate": right})
    report["all_full_result_and_stats_parity"] = not failures
    report["median_cell_speedup_ratio"] = statistics.median(
        row["candidate_speedup_ratio"] for row in report["cells"]
    )
    out = ROOT / "evidence" / "independent-parity-benchmark.json"
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps({
        "status": "PASSED_PARITY" if not failures else "FAILED_PARITY",
        "cells": len(report["cells"]),
        "parity_cells": sum(row["full_result_and_stats_parity"] for row in report["cells"]),
        "median_cell_speedup_ratio": report["median_cell_speedup_ratio"],
        "result_sha256": hashlib.sha256(out.read_bytes()).hexdigest(),
    }, sort_keys=True))
    if failures:
        print(json.dumps(failures, sort_keys=True), file=sys.stderr)
        raise SystemExit(2)


if __name__ == "__main__":
    main()
