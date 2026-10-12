#!/usr/bin/env python3
"""Adversarial parity review of the H7 metadata-only same-bucket fast path.

This is proving-ground-only. It pins ACSIE source, applies the same fixed
frontier-cache diagnostic to all arms, then compares pristine/control/candidate
on associative and non-associative operators, target and non-target traces,
direct-vocabulary selection, and a transitive macro-alias fixture. Actual Gen6
is delegated to the existing runner only after every parity gate succeeds.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import textwrap

SOURCE_COMMIT = "d83cb60286f195eec02b4d7aa45a60539316d115"
SOURCE_TREE = "12d2fa8fc1f1e34d3d7c442eef85aa6a71380420"
SOURCE_SHA256 = "2ff981875397b7d0edb2fef1c33c655b0a15b8c404b13c50d13a804f77535aea"
TEST_FILES = (
    "tests/test_open_ended_growth.py",
    "tests/test_core014_recursive_cognitive_compiler.py",
    "tests/test_core015_semantic_operator_discovery.py",
    "tests/test_core016_open_semantic_substrate.py",
    "tests/test_adaptive_capability_repair.py",
    "tests/test_asi_claim_gate.py",
)

def sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()

def run(argv, *, cwd: Path, env: dict[str, str], log: Path) -> str:
    proc = subprocess.run(argv, cwd=cwd, env=env, stdout=subprocess.PIPE,
                          stderr=subprocess.STDOUT, text=True)
    log.write_text(proc.stdout, encoding="utf-8")
    if proc.returncode:
        raise RuntimeError(f"command failed rc={proc.returncode}: {argv}; see {log}")
    return proc.stdout

def worktree(acsi_root: Path, path: Path) -> None:
    subprocess.run(["git", "-C", str(acsi_root), "worktree", "add", "--detach",
                    str(path), SOURCE_COMMIT], check=True, stdout=subprocess.PIPE,
                   stderr=subprocess.STDOUT, text=True)

def probe_text() -> str:
    return r'''import json, math
from cognitive_core.behavioral_search import find_exact_expression
from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive, Trace

ROWS = ((-3,-2,-1),(-2,1,3),(-1,2,-3),(0,0,0),(1,-2,2),(2,3,-1),(3,-1,1),(4,2,2))
def primitive(pid, expr):
    return CognitivePrimitive(pid, expr, ("x","y","z"), "delta", 0, (), ("opaque",),
                              0.0, 0.0, 0.0, 1, {})
def evaluate_case(case, op, alias, objective):
    compiler = OpenEndedRecursiveCognitiveCompiler(max_depth=4, seed=771221)
    base = [
        primitive("prim:x", {"op":"get","key":"x"}),
        primitive("prim:y", {"op":"get","key":"y"}),
        primitive("prim:z", {"op":"get","key":"z"}),
    ]
    if alias:
        base.append(primitive("prim:x_alias", {"op":"macro","id":"prim:x"}))
    compiler.primitives = {p.primitive_id:p for p in base}
    compiler.archive = {p.primitive_id:ArchiveRecord(p) for p in base}
    rows = []
    for i, (x,y,z) in enumerate(ROWS):
        if case == "unrepresentable_non_target":
            target = float(x+y+z+17)
        elif op == "add":
            target = float(x+y+z)
        elif op == "sub":
            target = float(x-y-z)
        elif op == "mul":
            target = float(x*y*z)
        elif op == "max":
            target = float(max(x,y,z))
        elif op == "min":
            target = float(min(x,y,z))
        else:
            raise ValueError(case)
        rows.append(Trace({"x":float(x),"y":float(y),"z":float(z)},
                          target, "opaque", {}, f"{case}-{i}"))
    result = find_exact_expression(compiler, tuple(rows), max_depth=3,
                                   require_macro=True, selection_objective=objective,
                                   max_alternatives=16)
    sig = None if result is None else {
        "expression":result.expression,
        "used_primitives":result.used_primitives,
        "alternatives":result.alternatives,
        "stats":dict(result.stats),
    }
    extra = dict(getattr(find_exact_expression, "_H7_LAST_BUCKET_DUPLICATE_STATS", {})) if result is not None else None
    return {"signature":sig, "bucket_telemetry":extra}

cases = [
    ("add_associativity","add",False,"minimize_expanded_cost"),
    ("sub_equivalent_parenthesization","sub",False,"minimize_expanded_cost"),
    ("mul_associativity","mul",False,"minimize_expanded_cost"),
    ("max_associativity","max",False,"minimize_expanded_cost"),
    ("min_associativity","min",False,"minimize_expanded_cost"),
    ("unrepresentable_non_target","offset",False,"minimize_expanded_cost"),
    ("lineage_alias_add","add",True,"maximize_direct_vocabulary"),
]
out = {name:evaluate_case(name,op,alias,obj) for name,op,alias,obj in cases}
print(json.dumps({"cases":out},sort_keys=True,separators=(",",":"),allow_nan=True))
'''

def env_for(root: Path) -> dict[str, str]:
    env = os.environ.copy()
    env["PYTHONHASHSEED"] = "0"
    env["PYTHONPATH"] = str(root)
    env.pop("ACSIE_DISABLE_LF_CACHE", None)
    return env

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo-root", type=Path, required=True)
    ap.add_argument("--acsie-root", type=Path, required=True)
    ap.add_argument("--evidence", type=Path, required=True)
    ap.add_argument("--seed", type=int, default=2026100301)
    ap.add_argument("--timeout-seconds", type=int, default=180)
    args = ap.parse_args()
    repo = args.repo_root.resolve()
    acsie = args.acsie_root.resolve()
    ev = args.evidence.resolve()
    ev.mkdir(parents=True, exist_ok=True)
    src = acsie / "cognitive_core" / "behavioral_search.py"
    head = subprocess.run(["git","-C",str(acsie),"rev-parse","HEAD"],check=True,
                          capture_output=True,text=True).stdout.strip()
    tree = subprocess.run(["git","-C",str(acsie),"rev-parse","HEAD^{tree}"],check=True,
                          capture_output=True,text=True).stdout.strip()
    if (head,tree,sha256(src)) != (SOURCE_COMMIT,SOURCE_TREE,SOURCE_SHA256):
        raise SystemExit(f"pinned ACSIE identity mismatch: {head} {tree} {sha256(src)}")
    sys.path.insert(0, str(repo / "lab"))
    from patch_h7_metadata_only_bucket_duplicate import apply_candidate, apply_frontier_cache
    probe = ev / "adversarial_signature_probe.py"
    probe.write_text(probe_text(), encoding="utf-8")
    temp = Path(tempfile.mkdtemp(prefix="h7-bucket-review-"))
    roots = {k:temp/k for k in ("pristine","control","candidate")}
    try:
        for root in roots.values():
            worktree(acsie, root)
        original = (roots["control"] / "cognitive_core" / "behavioral_search.py").read_text()
        (roots["control"] / "cognitive_core" / "behavioral_search.py").write_text(
            apply_frontier_cache(original), encoding="utf-8")
        original = (roots["candidate"] / "cognitive_core" / "behavioral_search.py").read_text()
        candidate_source = apply_candidate(apply_frontier_cache(original))
        (roots["candidate"] / "cognitive_core" / "behavioral_search.py").write_text(
            candidate_source, encoding="utf-8")
        patched_hashes = {k:sha256(v/"cognitive_core"/"behavioral_search.py") for k,v in roots.items()}
        if any(subprocess.run([sys.executable,"-m","py_compile",
                               str(root/"cognitive_core"/"behavioral_search.py")],
                              capture_output=True).returncode for root in roots.values()):
            raise RuntimeError("py_compile failed in at least one arm")
        matrices = {}
        for name, root in roots.items():
            outpath = ev / f"adversarial-signatures-{name}.json"
            raw = run([sys.executable,str(probe)],cwd=repo,env=env_for(root),log=outpath)
            matrices[name] = json.loads(raw)
        baseline = matrices["pristine"]["cases"]
        def signatures(matrix):
            return {case: row["signature"] for case,row in matrix["cases"].items()}
        baseline_signatures = signatures(matrices["pristine"])
        control_signatures = signatures(matrices["control"])
        candidate_signatures = signatures(matrices["candidate"])
        parity = {
            "control_matches_pristine": control_signatures == baseline_signatures,
            "candidate_matches_pristine": candidate_signatures == baseline_signatures,
            "control_matches_candidate": control_signatures == candidate_signatures,
            "cases": {},
        }
        candidate_cases = matrices["candidate"]["cases"]
        for case in baseline:
            sigs = [matrices[name]["cases"][case]["signature"] for name in roots]
            parity["cases"][case] = {
                "all_three_signatures_identical": sigs[0] == sigs[1] == sigs[2],
                "candidate_bucket_telemetry": candidate_cases[case]["bucket_telemetry"],
                "baseline_exact_duplicate_skips": baseline[case]["signature"]["stats"].get("exact_expression_duplicates_skipped",0)
                    if baseline[case]["signature"] else None,
                "target_candidate_count": baseline[case]["signature"]["stats"].get("target_candidate_count",0)
                    if baseline[case]["signature"] else None,
            }
        required_bucket_cases = ("add_associativity","sub_equivalent_parenthesization",
                                 "mul_associativity","max_associativity","min_associativity")
        parity["required_candidate_bucket_hits"] = {
            case: int(candidate_cases[case]["bucket_telemetry"].get("metadata_only_bucket_hits",0))
            for case in required_bucket_cases
        }
        parity["all_required_operator_collisions_exercised"] = all(
            parity["required_candidate_bucket_hits"][case] > 0 for case in required_bucket_cases
        )
        parity["non_target_negative_control_has_no_solution"] = all(
            matrices[name]["cases"]["unrepresentable_non_target"]["signature"] is None
            for name in roots
        )
        candidate_telemetry_rows = [
            candidate_cases[name]["bucket_telemetry"] for name in required_bucket_cases
        ]
        parity["candidate_bucket_telemetry_consistent"] = all(
            row is not None
            and row.get("metadata_only_bucket_hits",0)
                == row.get("target_bucket_hits",0) + row.get("non_target_bucket_hits",0)
            and row.get("lineage_keys_updated",0) == row.get("metadata_only_bucket_hits",0)
            and row.get("lineage_ids_added",0) >= 0
            and row.get("lineage_growth_events",0) >= 0
            for row in candidate_telemetry_rows
        )
        parity["target_and_non_target_fastpaths_exercised"] = (
            sum(row.get("target_bucket_hits",0) for row in candidate_telemetry_rows) > 0
            and sum(row.get("non_target_bucket_hits",0) for row in candidate_telemetry_rows) > 0
        )
        parity["pinned_source"] = {"commit":head,"tree":tree,"source_sha256":sha256(src)}
        parity["diagnostic_source_sha256"] = patched_hashes
        parity["factor_contract"] = {
            "frontier_cache_capacity":128,
            "control":"frontier-cache-only; original register() same-bucket duplicate path",
            "candidate":"same frontier cache plus metadata-only same-bucket duplicate path",
            "production_source_modified":False,
        }
        (ev/"adversarial-parity-summary.json").write_text(
            json.dumps(parity,indent=2,sort_keys=True,allow_nan=True)+"\n",encoding="utf-8")
        print(json.dumps(parity,indent=2,sort_keys=True,allow_nan=True))
        if not all(parity[k] for k in ("control_matches_pristine","candidate_matches_pristine",
                                      "control_matches_candidate","all_required_operator_collisions_exercised",
                                      "non_target_negative_control_has_no_solution",
                                      "candidate_bucket_telemetry_consistent",
                                      "target_and_non_target_fastpaths_exercised")):
            raise RuntimeError("adversarial signature/coverage gate failed; do not run Gen6")
        # Regression is required on both diagnostic arms before any actual seed.
        for name in ("control","candidate"):
            root = roots[name]
            env = env_for(root)
            log = ev / f"six-file-regression-{name}.txt"
            out = run([sys.executable,"-m","pytest","-q",
                       *[str(root/t) for t in TEST_FILES]],cwd=repo,env=env,log=log)
            import re
            match = re.search(r"(\d+) passed(?:,|\s)",out)
            if not match or int(match.group(1)) != 61:
                raise RuntimeError(f"{name} six-file suite did not pass 61/61")
        report = {
            "schema":"ACSIE.h7_metadata_only_bucket_duplicate.adversarial_review.v1",
            "preflight":"PASSED",
            "source":{"commit":head,"tree":tree,"sha256":sha256(src)},
            "diagnostic_source_sha256":patched_hashes,
            "signature_matrix_cases":list(baseline),
            "exact_parity":{"control":True,"candidate":True,"against_pristine":True},
            "regression_tests":{"control":61,"candidate":61},
            "candidate_bucket_hits":parity["required_candidate_bucket_hits"],
            "non_target_negative_control_has_no_solution":parity["non_target_negative_control_has_no_solution"],
            "candidate_bucket_telemetry_consistent":parity["candidate_bucket_telemetry_consistent"],
            "target_and_non_target_fastpaths_exercised":parity["target_and_non_target_fastpaths_exercised"],
            "hypothesis":"metadata-only same-bucket duplicate path preserves selected executable expression, alternatives, vocabulary, lineage and result statistics on the registered fixture matrix",
            "interpretation_guard":"Fixture parity is not Gen6 acceptance. Actual Gen6 must return complete records; live target candidates are not completion.",
        }
        (ev/"adversarial-review-preflight.json").write_text(
            json.dumps(report,indent=2,sort_keys=True)+"\n",encoding="utf-8")
    finally:
        for root in roots.values():
            subprocess.run(["git","-C",str(acsie),"worktree","remove","--force",str(root)],
                           stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        shutil.rmtree(temp,ignore_errors=True)
    # Only after extended parity and 61/61 regression pass, run the existing paired
    # Gen6 driver with a new, predeclared 180-second budget and unchanged RSS limit.
    gen6_dir = ev / "paired-gen6"
    gen6_dir.mkdir(exist_ok=True)
    cmd = [sys.executable,str(repo/"lab"/"run_h7_metadata_only_bucket_duplicate.py"),
           "--repo-root",str(repo),"--acsie-root",str(acsie),
           "--evidence",str(gen6_dir),"--seed",str(args.seed),
           "--generations","7","--timeout-seconds",str(args.timeout_seconds),
           "--rss-guard-bytes",str(2*1024*1024*1024)]
    gen6_log = ev / "paired-gen6-runner.log"
    run(cmd,cwd=repo,env=os.environ.copy(),log=gen6_log)
    (ev/"research-contract.json").write_text(json.dumps({
        "schema":"ACSIE.h7_metadata_only_bucket_duplicate.followup.v1",
        "predeclared_budget_seconds_per_arm":args.timeout_seconds,
        "rss_guard_bytes":2*1024*1024*1024,
        "seed":args.seed,"generation_indices":list(range(7)),
        "candidate_and_control_source":"same pinned ACSIE commit/tree; only metadata-only same-bucket treatment differs",
        "source_mutation":"diagnostic worktrees only; no ACSIE commit pushed",
        "science_gate":"complete returned record required; timeout/empty stdout is INCONCLUSIVE",
    },indent=2,sort_keys=True)+"\n",encoding="utf-8")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
