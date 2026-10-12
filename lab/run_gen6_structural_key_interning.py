#!/usr/bin/env python3
"""Run a predeclared Gen6 exact structural-key interning diagnostic.

Preflight fails closed. Actual Gen6 timeouts are recorded as inconclusive data,
not converted to a scientific pass or made to suppress artifact upload.
"""
from __future__ import annotations

import argparse
import collections
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import time

SOURCE_COMMIT = "d83cb60286f195eec02b4d7aa45a60539316d115"
SOURCE_TREE = "12d2fa8fc1f1e34d3d7c442eef85aa6a71380420"
SOURCE_SHA = "2ff981875397b7d0edb2fef1c33c655b0a15b8c404b13c50d13a804f77535aea"
TEST_FILES = (
    "tests/test_open_ended_growth.py",
    "tests/test_core014_recursive_cognitive_compiler.py",
    "tests/test_core015_semantic_operator_discovery.py",
    "tests/test_core016_open_semantic_substrate.py",
    "tests/test_adaptive_capability_repair.py",
    "tests/test_asi_claim_gate.py",
)
SEED = 2026100301
GENERATIONS = 7
CHILD_TIMEOUT_SECONDS = 120
RSS_GUARD_BYTES = 2 * 1024 * 1024 * 1024


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def run_checked(argv: list[str], *, cwd: Path, env: dict[str, str], out_path: Path) -> None:
    with out_path.open("w", encoding="utf-8") as out:
        proc = subprocess.run(argv, cwd=cwd, env=env, stdout=out, stderr=subprocess.STDOUT, text=True)
    if proc.returncode != 0:
        raise RuntimeError(f"preflight command failed rc={proc.returncode}: {' '.join(argv)}; see {out_path}")


def base_env(pythonpath: list[Path]) -> dict[str, str]:
    env = os.environ.copy()
    env["PYTHONHASHSEED"] = "0"
    env["PYTHONPATH"] = os.pathsep.join(str(p) for p in pythonpath)
    env.pop("ACSIE_DISABLE_LF_CACHE", None)
    return env


def install_sampler(diag_dir: Path) -> None:
    diag_dir.mkdir(parents=True, exist_ok=True)
    (diag_dir / "sitecustomize.py").write_text(r'''import faulthandler,json,os,signal,sys,time
faulthandler.enable(file=sys.stderr,all_threads=True)
_STARTED=time.monotonic()
def _snapshot(sig,frame):
    faulthandler.dump_traceback(file=sys.stderr,all_threads=True)
    f=frame
    while f is not None and f.f_code.co_name!="find_exact_expression":
        f=f.f_back
    row={"elapsed_s":round(time.monotonic()-_STARTED,3),"signal":int(sig)}
    if f is not None:
        loc=f.f_locals
        st=loc.get("stats",{})
        row["search"]={k:st.get(k) for k in ("rows","max_depth","primitive_count","generated_expressions","unique_states","duplicate_states","dominance_pruned_states","dominance_frontier_max","status")}
        row["structural_ids"]=dict(loc.get("_H7_STRUCTURAL_ID_STATS",{}))
        row["frontier_cache"]=dict(loc.get("_h7_lf_cache_stats",{}))
        row["frontier_cache_entries"]=len(loc.get("_h7_lf_cache",{}))
        row["structural_lineage_key_count"]=len(loc.get("structural_lineage_by_key",{}))
        row["seen_expression_key_count"]=len(loc.get("seen_expression_keys",()))
        row["online_metadata_count"]=len(loc.get("_H7_online_metadata",{}))
    path=os.environ.get("ACSIE_STRUCTURAL_ID_SNAPSHOT_LOG")
    if path:
        with open(path,"a",encoding="utf-8",buffering=1) as h:
            h.write(json.dumps(row,sort_keys=True,separators=(",",":"))+"\n")
signal.signal(signal.SIGUSR1,_snapshot)
''', encoding="utf-8")


def read_rss_bytes(pid: int) -> int:
    status = Path(f"/proc/{pid}/status")
    try:
        for line in status.read_text(errors="replace").splitlines():
            if line.startswith("VmRSS:"):
                return int(line.split()[1]) * 1024
    except (OSError, ValueError, IndexError):
        pass
    return 0


def run_gen6_arm(
    *,
    arm: str,
    pythonpath: list[Path],
    lab_dir: Path,
    evidence: Path,
    seed: int,
    generations: int,
    timeout_seconds: int,
    rss_guard_bytes: int,
) -> dict:
    stdout_path = evidence / f"{arm}.stdout.jsonl"
    stderr_path = evidence / f"{arm}.stderr"
    snapshots_path = evidence / f"{arm}-live-snapshots.jsonl"
    for p in (stdout_path, stderr_path, snapshots_path):
        p.unlink(missing_ok=True)

    env = base_env(pythonpath)
    env["ACSIE_STRUCTURAL_ID_SNAPSHOT_LOG"] = str(snapshots_path)
    code = (
        "import json,sys,acsie_open_ended_self_extending_proving as g;"
        "print(json.dumps(g.run_seed(int(sys.argv[1]),int(sys.argv[2])),"
        "sort_keys=True,separators=(',',':')),flush=True)"
    )
    started = time.monotonic()
    start_epoch = time.time()
    max_rss = 0
    next_snapshot = started + 10
    timed_out = False
    rss_hit = False
    with stdout_path.open("w", encoding="utf-8") as stdout, stderr_path.open("w", encoding="utf-8") as stderr:
        proc = subprocess.Popen(
            [sys.executable, "-c", code, str(seed), str(generations)],
            cwd=lab_dir,
            env=env,
            stdout=stdout,
            stderr=stderr,
            text=True,
        )
        while proc.poll() is None:
            now = time.monotonic()
            rss = read_rss_bytes(proc.pid)
            max_rss = max(max_rss, rss)
            if rss >= rss_guard_bytes:
                rss_hit = True
                proc.terminate()
                try:
                    proc.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    proc.kill()
                break
            if now >= next_snapshot:
                try:
                    os.kill(proc.pid, signal.SIGUSR1)
                except ProcessLookupError:
                    pass
                next_snapshot = now + 10
            if now - started >= timeout_seconds:
                timed_out = True
                proc.terminate()
                try:
                    proc.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    proc.kill()
                break
            time.sleep(0.5)
        try:
            rc = proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            rc = proc.wait(timeout=5)

    elapsed = time.monotonic() - started
    stdout_text = stdout_path.read_text(errors="replace")
    lines = [line for line in stdout_text.splitlines() if line.strip()]
    record = None
    parse_status = "NO_COMPLETE_RECORD"
    if not timed_out and not rss_hit and rc == 0 and len(lines) == 1:
        try:
            record = json.loads(lines[0])
            parse_status = "PASS"
        except Exception as exc:
            parse_status = f"INVALID_JSON:{type(exc).__name__}:{exc}"
    elif rc == 0 and lines:
        parse_status = "EXPECTED_EXACTLY_ONE_JSON_RECORD"

    if rss_hit:
        status = "RESOURCE_GUARD_RSS_LIMIT"
    elif timed_out:
        status = "INCONCLUSIVE_CHILD_WALL_TIMEOUT"
    elif rc == 0 and record is not None:
        status = "CHILD_RETURNED_COMPLETE_RECORD"
    elif rc != 0:
        status = "CHILD_CRASHED_OR_EXITED_NONZERO"
    else:
        status = "CHILD_RETURNED_EMPTY_OR_INVALID_OUTPUT"

    result = {
        "arm": arm,
        "seed": seed,
        "generations_requested": generations,
        "generation_indices_requested": list(range(generations)),
        "child_wall_limit_seconds": timeout_seconds,
        "rss_guard_bytes": rss_guard_bytes,
        "pythonhashseed": "0",
        "start_epoch": start_epoch,
        "elapsed_seconds": round(elapsed, 3),
        "max_observed_rss_bytes": max_rss,
        "child_returncode": rc,
        "timeout_hit": timed_out,
        "rss_guard_hit": rss_hit,
        "status": status,
        "record_parse_status": parse_status,
        "stdout_bytes": stdout_path.stat().st_size,
        "stderr_bytes": stderr_path.stat().st_size,
        "complete_record": record,
    }
    (evidence / f"{arm}-status.json").write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return result


def make_signature_probe(evidence: Path) -> Path:
    p = evidence / "signature_probe.py"
    p.write_text(r'''import json,sys
from cognitive_core.behavioral_search import find_exact_expression
from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import ArchiveRecord,CognitivePrimitive,Trace
def build(op):
 c=OpenEndedRecursiveCognitiveCompiler(max_depth=4,seed=771221)
 px=CognitivePrimitive("prim:x",{"op":"get","key":"x"},("x","y"),"delta",0,(),("opaque",),0.0,0.0,0.0,1,{})
 py=CognitivePrimitive("prim:y",{"op":"get","key":"y"},("x","y"),"delta",0,(),("opaque",),0.0,0.0,0.0,1,{})
 c.primitives={px.primitive_id:px,py.primitive_id:py};c.archive={px.primitive_id:ArchiveRecord(px),py.primitive_id:ArchiveRecord(py)}
 rows=[]
 for i in range(8):
  x,y=float(i),float(i+1);target=max(x,y) if op=="max" else min(x,y) if op=="min" else x+y
  rows.append(Trace({"x":x,"y":y},target,"opaque",{},f"{op}-{i}"))
 return c,tuple(rows),2 if op=="sum" else 4,"maximize_direct_vocabulary" if op=="sum" else "minimize_expanded_cost"
def sig(r):
 return None if r is None else {"expression":r.expression,"used_primitives":r.used_primitives,"alternatives":r.alternatives,"stats":dict(r.stats)}
signatures={};counters={}
for op in ("max","min","sum"):
 c,rows,depth,obj=build(op)
 r=find_exact_expression(c,rows,max_depth=depth,require_macro=True,selection_objective=obj,max_alternatives=8)
 signatures[op]=sig(r)
 counters[op]={
  "found":r is not None,
  "frontier_cache":dict(getattr(find_exact_expression,"_H7_LAST_LF_CACHE_STATS",{})),
  "structural_ids":dict(getattr(find_exact_expression,"_H7_LAST_STRUCTURAL_ID_STATS",{})),
 }
print(json.dumps({"mode":sys.argv[1],"signatures":signatures,"counters":counters},sort_keys=True,separators=(",",":")))
''', encoding="utf-8")
    return p


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo-root", type=Path, required=True)
    ap.add_argument("--acsie-root", type=Path, required=True)
    ap.add_argument("--evidence", type=Path, required=True)
    ap.add_argument("--seed", type=int, default=SEED)
    ap.add_argument("--generations", type=int, default=GENERATIONS)
    ap.add_argument("--timeout-seconds", type=int, default=CHILD_TIMEOUT_SECONDS)
    ap.add_argument("--rss-guard-bytes", type=int, default=RSS_GUARD_BYTES)
    ap.add_argument("--executor-sha", default=os.environ.get("GITHUB_SHA", "unknown"))
    ap.add_argument("--workflow-run-id", default=os.environ.get("GITHUB_RUN_ID", "unknown"))
    args = ap.parse_args()

    repo_root = args.repo_root.resolve()
    acsie_root = args.acsie_root.resolve()
    evidence = args.evidence.resolve()
    evidence.mkdir(parents=True, exist_ok=True)
    diag_dir = repo_root / "diagnostics" / "gen6-structural-key-interning"
    install_sampler(diag_dir)

    source_file = acsie_root / "cognitive_core" / "behavioral_search.py"
    if sha256_file(source_file) != SOURCE_SHA:
        raise SystemExit("pinned ACSIE source SHA mismatch before experiment")
    head = subprocess.run(["git", "-C", str(acsie_root), "rev-parse", "HEAD"], capture_output=True, text=True, check=True).stdout.strip()
    tree = subprocess.run(["git", "-C", str(acsie_root), "rev-parse", "HEAD^{tree}"], capture_output=True, text=True, check=True).stdout.strip()
    if (head, tree) != (SOURCE_COMMIT, SOURCE_TREE):
        raise SystemExit(f"pinned ACSIE identity mismatch: {(head, tree)}")

    pristine = repo_root / "pristine-acsie"
    control = repo_root / "control-acsie"
    for worktree in (pristine, control):
        subprocess.run(["git", "-C", str(acsie_root), "worktree", "remove", "--force", str(worktree)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        worktree.mkdir(parents=True, exist_ok=True)
        # Git requires an absent target for worktree add.
        worktree.rmdir()
        subprocess.run(["git", "-C", str(acsie_root), "worktree", "add", "--detach", str(worktree), SOURCE_COMMIT], check=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)

    helper = repo_root / "lab" / "patch_gen6_structural_key_interning.py"
    patch_reports = {}
    for mode, source_root in (("control", control), ("candidate", acsie_root)):
        patched_source = source_root / "cognitive_core" / "behavioral_search.py"
        report_path = evidence / f"patch-{mode}.txt"
        run_checked([sys.executable, str(helper), str(patched_source), "--mode", mode], cwd=repo_root, env=os.environ.copy(), out_path=report_path)
        patch_reports[mode] = report_path.read_text(encoding="utf-8")
        compile(patched_source.read_text(encoding="utf-8"), str(patched_source), "exec")
    identity = {
        "schema": "ACSIE.gen6_structural_key_interning.provenance.v1",
        "executor_commit": args.executor_sha,
        "workflow_run_id": str(args.workflow_run_id),
        "source_commit": SOURCE_COMMIT,
        "source_tree": SOURCE_TREE,
        "pristine_source_sha256": SOURCE_SHA,
        "pristine_source_file": str(pristine / "cognitive_core" / "behavioral_search.py"),
        "control_diagnostic_sha256": sha256_file(control / "cognitive_core" / "behavioral_search.py"),
        "candidate_diagnostic_sha256": sha256_file(acsie_root / "cognitive_core" / "behavioral_search.py"),
        "control_report": patch_reports["control"],
        "candidate_report": patch_reports["candidate"],
        "factor": "control=frontier-cache-128 with original expanded JSON structural keys; candidate=same cache with exact collision-safe hash-consed structural IDs",
        "other_caches": "structural-expansion cache OFF; arbitrary/direct AST-key cache OFF; candidate grouping OFF",
        "seed": args.seed,
        "generations_requested": args.generations,
        "generation_indices": list(range(args.generations)),
        "child_timeout_seconds": args.timeout_seconds,
        "rss_guard_bytes": args.rss_guard_bytes,
        "pythonhashseed": "0 for all preflight and actual children",
        "runtime_integrity": {
            "external_model": False,
            "task_family_route": False,
            "target_identity_available_to_runtime": False,
            "target_access_during_discovery": False,
        },
    }
    (evidence / "provenance.json").write_text(json.dumps(identity, indent=2, sort_keys=True) + "\n", encoding="utf-8")

    # Cheap syntax and parity preflight, strict before any Gen6 work.
    for mode, source_root in (("control", control), ("candidate", acsie_root)):
        env = base_env([source_root])
        run_checked(
            [sys.executable, "-m", "py_compile", str(source_root / "cognitive_core" / "behavioral_search.py")],
            cwd=repo_root, env=env, out_path=evidence / f"pycompile-{mode}.txt"
        )

    tests_by_mode = {}
    for mode, source_root in (("control", control), ("candidate", acsie_root)):
        env = base_env([source_root])
        cmd = [sys.executable, "-m", "pytest", "-q", *[str(source_root / t) for t in TEST_FILES]]
        out = evidence / f"six-file-regression-{mode}.txt"
        run_checked(cmd, cwd=repo_root, env=env, out_path=out)
        text = out.read_text(errors="replace")
        m = re.search(r"(\d+) passed(?:,|\s)", text)
        if not m:
            raise RuntimeError(f"could not verify pytest pass count in {out}")
        tests_by_mode[mode] = {"passed": int(m.group(1)), "output_file": out.name, "output_tail": text[-500:]}

    probe = make_signature_probe(evidence)
    signatures = {}
    for mode, source_root in (("pristine", pristine), ("control", control), ("candidate", acsie_root)):
        env = base_env([source_root])
        out = evidence / f"signatures-{mode}.json"
        run_checked([sys.executable, str(probe), mode], cwd=repo_root, env=env, out_path=out)
        signatures[mode] = json.loads(out.read_text())
    baseline_sig = signatures["pristine"]["signatures"]
    parity = {
        "schema": "ACSIE.gen6_structural_key_interning.parity.v1",
        "source_commit": SOURCE_COMMIT,
        "control_matches_pristine": signatures["control"]["signatures"] == baseline_sig,
        "candidate_matches_pristine": signatures["candidate"]["signatures"] == baseline_sig,
        "control_matches_candidate": signatures["control"]["signatures"] == signatures["candidate"]["signatures"],
        "six_file_regression": tests_by_mode,
        "signatures_and_counters": signatures,
    }
    (evidence / "preflight-parity.json").write_text(json.dumps(parity, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    if not parity["control_matches_pristine"] or not parity["candidate_matches_pristine"] or not parity["control_matches_candidate"]:
        raise RuntimeError("exact returned scientific signatures do not match pristine")
    if any(v["passed"] != 61 for v in tests_by_mode.values()):
        raise RuntimeError("six-file regression gate did not pass 61/61 for both arms")

    # Only begin expensive children after source, syntax, regression, and exact-signature gates pass.
    actual_results = {}
    arms = (
        ("control_frontier128_json_structural_keys", control),
        ("candidate_frontier128_interned_structural_ids", acsie_root),
    )
    lab_dir = repo_root / "lab"
    for arm, source_root in arms:
        actual_results[arm] = run_gen6_arm(
            arm=arm,
            pythonpath=[diag_dir, lab_dir, source_root],
            lab_dir=lab_dir,
            evidence=evidence,
            seed=args.seed,
            generations=args.generations,
            timeout_seconds=args.timeout_seconds,
            rss_guard_bytes=args.rss_guard_bytes,
        )

    stack_summary = {}
    for arm, _ in arms:
        stderr = (evidence / f"{arm}.stderr").read_text(errors="replace")
        snapshots = []
        snapshot_path = evidence / f"{arm}-live-snapshots.jsonl"
        if snapshot_path.exists():
            for line in snapshot_path.read_text(errors="replace").splitlines():
                try:
                    snapshots.append(json.loads(line))
                except json.JSONDecodeError:
                    pass
        blocks = re.split(r"(?=Current thread 0x)", stderr)
        freq = collections.Counter()
        stack_count = 0
        for block in blocks:
            frames = re.findall(r'File "([^"]+)", line (\d+) in ([^\n]+)', block)
            if frames:
                stack_count += 1
                freq.update(frame[2] for frame in frames)
        stack_summary[arm] = {
            "stack_snapshot_count": stack_count,
            "stack_function_frequency": dict(freq),
            "live_snapshot_count": len(snapshots),
            "latest_snapshot": snapshots[-1] if snapshots else None,
        }

    final = {
        "schema": "ACSIE.gen6_structural_key_interning.diagnostic.v1",
        "provenance": identity,
        "preflight": {
            "status": "PASSED",
            "exact_signature_parity": {
                "control_matches_pristine": parity["control_matches_pristine"],
                "candidate_matches_pristine": parity["candidate_matches_pristine"],
                "control_matches_candidate": parity["control_matches_candidate"],
            },
            "six_file_test_counts": {k: v["passed"] for k, v in tests_by_mode.items()},
        },
        "actual_gen6": actual_results,
        "stack_and_live_telemetry": stack_summary,
        "science_interpretation": (
            "A complete Gen6 result exists only if child status is CHILD_RETURNED_COMPLETE_RECORD, "
            "record generations include indices 0..6, and all finite/open-ended scientific gates pass. "
            "A target-found stack snapshot is diagnostic only."
        ),
    }
    (evidence / "gen6-structural-key-interning-summary.json").write_text(json.dumps(final, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps({
        "schema": final["schema"],
        "preflight": final["preflight"],
        "actual_gen6": {arm: {k: v[k] for k in ("status", "elapsed_seconds", "max_observed_rss_bytes", "child_returncode", "timeout_hit", "rss_guard_hit", "record_parse_status")} for arm, v in actual_results.items()},
        "stack_and_live_telemetry": stack_summary,
        "summary_path": str(evidence / "gen6-structural-key-interning-summary.json"),
    }, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
