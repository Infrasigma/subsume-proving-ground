#!/usr/bin/env python3
"""Preflight-gated pruned expression-key lifetime test on the actual ACSIE Gen6 proving path.

Control and candidate share the exact pinned source and the same 128-entry
lineage-frontier cache. Only candidate avoids materializing a parent AST for a same-bucket duplicate that can be decided from metadata already computed in register_binary.
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
SOURCE_SHA256 = "2ff981875397b7d0edb2fef1c33c655b0a15b8c404b13c50d13a804f77535aea"
SEED = 2026100301
GENERATIONS = 7
CHILD_TIMEOUT_SECONDS = 120
RSS_GUARD_BYTES = 2 * 1024 * 1024 * 1024
TEST_FILES = (
    "tests/test_open_ended_growth.py",
    "tests/test_core014_recursive_cognitive_compiler.py",
    "tests/test_core015_semantic_operator_discovery.py",
    "tests/test_core016_open_semantic_substrate.py",
    "tests/test_adaptive_capability_repair.py",
    "tests/test_asi_claim_gate.py",
)


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def run_checked(argv: list[str], *, cwd: Path, env: dict[str, str], output: Path) -> str:
    with output.open("w", encoding="utf-8") as handle:
        proc = subprocess.run(argv, cwd=cwd, env=env, stdout=handle, stderr=subprocess.STDOUT, text=True)
    text = output.read_text(errors="replace")
    if proc.returncode != 0:
        raise RuntimeError(f"preflight command failed rc={proc.returncode}: {' '.join(argv)}; output={output}")
    return text


def env_for(*python_paths: Path) -> dict[str, str]:
    env = os.environ.copy()
    env["PYTHONHASHSEED"] = "0"
    env["PYTHONPATH"] = os.pathsep.join(str(p) for p in python_paths)
    env.pop("ACSIE_DISABLE_LF_CACHE", None)
    return env


def make_sampler(diag_dir: Path) -> None:
    diag_dir.mkdir(parents=True, exist_ok=True)
    script = r'''import faulthandler,json,os,signal,sys,time
faulthandler.enable(file=sys.stderr,all_threads=True)
_PROCESS_START=time.monotonic()
def _snapshot(sig,frame):
    faulthandler.dump_traceback(file=sys.stderr,all_threads=True)
    f=frame
    while f is not None and f.f_code.co_name!="find_exact_expression":
        f=f.f_back
    row={"elapsed_process_s":round(time.monotonic()-_PROCESS_START,3),"signal":int(sig)}
    if f is not None:
        loc=f.f_locals
        now=time.perf_counter()
        row["search_call_id"]=loc.get("_H7_KEY_SEARCH_CALL_ID")
        row["search_frame_id"]=loc.get("_H7_KEY_SEARCH_FRAME_ID",id(f))
        started=loc.get("_H7_KEY_SEARCH_CALL_STARTED")
        row["elapsed_inside_search_call_s"]=round(now-started,3) if isinstance(started,(int,float)) else None
        st=loc.get("stats",{})
        row["search"]={k:st.get(k) for k in ("rows","max_depth","primitive_count","generated_expressions","unique_states","duplicate_states","dominance_pruned_states","dominance_frontier_max","status")}
        row["key_lifetime"]=dict(loc.get("_H7_KEY_LIFETIME_STATS",{}))
        row["frontier_cache"]=dict(loc.get("_h7_lf_cache_stats",{}))
        row["frontier_cache_entries"]=len(loc.get("_h7_lf_cache",{}))
        key_map=loc.get("expression_key_by_state_token",{})
        levels=loc.get("levels",{})
        active_tokens=set()
        for depth_bucket in levels.values():
            for active_state in depth_bucket.values():
                if isinstance(active_state,tuple) and len(active_state)>5:
                    active_tokens.add(active_state[5])
        row["expression_key_map_entries"]=len(key_map)
        row["active_bucket_state_tokens"]=len(active_tokens)
        row["stale_expression_key_entries"]=sum(token not in active_tokens for token in key_map)
        row["next_state_token"]=loc.get("next_state_token")
        row["online_metadata_count"]=len(loc.get("_H7_online_metadata",{}))
        ancestor=f.f_back
        while ancestor is not None:
            if ancestor.f_code.co_name=="run_seed":
                run_loc=ancestor.f_locals
                row["generation_index"]=run_loc.get("generation")
                row["retained_capability_count"]=len(run_loc.get("retained",()))
                row["completed_generation_records"]=len(run_loc.get("generations_out",()))
                break
            ancestor=ancestor.f_back
    path=os.environ.get("ACSIE_PRUNED_KEY_RELEASE_SNAPSHOT_LOG")
    if path:
        with open(path,"a",encoding="utf-8",buffering=1) as handle:
            handle.write(json.dumps(row,sort_keys=True,separators=(",",":"))+"\n")
signal.signal(signal.SIGUSR1,_snapshot)
'''
    (diag_dir / "sitecustomize.py").write_text(script, encoding="utf-8")


def read_rss_bytes(pid: int) -> int:
    try:
        for line in Path(f"/proc/{pid}/status").read_text(errors="replace").splitlines():
            if line.startswith("VmRSS:"):
                return int(line.split()[1]) * 1024
    except (OSError, ValueError, IndexError):
        pass
    return 0


def run_actual_arm(
    *, arm: str, source_root: Path, lab_dir: Path, diag_dir: Path,
    evidence: Path, seed: int, generations: int,
    timeout_seconds: int, rss_guard_bytes: int,
) -> dict:
    stdout_path=evidence/f"{arm}.stdout.jsonl"
    stderr_path=evidence/f"{arm}.stderr"
    snapshot_path=evidence/f"{arm}-snapshots.jsonl"
    for p in (stdout_path,stderr_path,snapshot_path):
        p.unlink(missing_ok=True)
    env=env_for(diag_dir,lab_dir,source_root)
    env["ACSIE_PRUNED_KEY_RELEASE_SNAPSHOT_LOG"]=str(snapshot_path)
    code=(
        "import json,sys,acsie_open_ended_self_extending_proving as g;"
        "print(json.dumps(g.run_seed(int(sys.argv[1]),int(sys.argv[2])),"
        "sort_keys=True,separators=(',',':')),flush=True)"
    )
    started=time.monotonic()
    start_epoch=time.time()
    max_rss=0
    timed_out=False
    rss_hit=False
    next_snapshot=started+10
    with stdout_path.open("w",encoding="utf-8") as stdout, stderr_path.open("w",encoding="utf-8") as stderr:
        proc=subprocess.Popen([sys.executable,"-c",code,str(seed),str(generations)],cwd=lab_dir,env=env,stdout=stdout,stderr=stderr,text=True)
        while proc.poll() is None:
            now=time.monotonic()
            max_rss=max(max_rss,read_rss_bytes(proc.pid))
            if max_rss>=rss_guard_bytes:
                rss_hit=True
                proc.terminate()
                try: proc.wait(timeout=3)
                except subprocess.TimeoutExpired: proc.kill()
                break
            if now>=next_snapshot:
                try: os.kill(proc.pid,signal.SIGUSR1)
                except ProcessLookupError: pass
                next_snapshot=now+10
            if now-started>=timeout_seconds:
                timed_out=True
                proc.terminate()
                try: proc.wait(timeout=3)
                except subprocess.TimeoutExpired: proc.kill()
                break
            time.sleep(0.5)
        try: rc=proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill();rc=proc.wait(timeout=5)
    elapsed=time.monotonic()-started
    out_text=stdout_path.read_text(errors="replace")
    lines=[x for x in out_text.splitlines() if x.strip()]
    record=None
    parse="NO_COMPLETE_RECORD"
    if not timed_out and not rss_hit and rc==0 and len(lines)==1:
        try: record=json.loads(lines[0]);parse="PASS"
        except Exception as exc: parse=f"INVALID_JSON:{type(exc).__name__}:{exc}"
    if rss_hit: status="RESOURCE_GUARD_RSS_LIMIT"
    elif timed_out: status="INCONCLUSIVE_CHILD_WALL_TIMEOUT"
    elif rc==0 and record is not None: status="CHILD_RETURNED_COMPLETE_RECORD"
    elif rc!=0: status="CHILD_CRASHED_OR_EXITED_NONZERO"
    else: status="CHILD_EMPTY_OR_INVALID_OUTPUT"
    integrity_clean=(
        record is not None
        and record.get("external_model") is False
        and record.get("target_identity_available_to_runtime") is False
        and record.get("task_family_route") is False
    )
    scientific="INCONCLUSIVE" if timed_out or rss_hit or record is None else (
        "FINITE_SEVEN_GENERATION_GATE_PASSED_DIAGNOSTIC_ONLY"
        if integrity_clean and record.get("finite_open_ended_growth_gate") is True
        else "FINITE_OPEN_ENDED_GATE_FAILURE_REPORTED" if integrity_clean
        else "INVALID_RUNTIME_INTEGRITY_FLAGS"
    )
    result={
        "arm":arm,"seed":seed,"generations_requested":generations,
        "generation_indices_requested":list(range(generations)),
        "child_wall_limit_seconds":timeout_seconds,"rss_guard_bytes":rss_guard_bytes,
        "pythonhashseed":"0","start_epoch":start_epoch,"elapsed_seconds":round(elapsed,3),
        "max_observed_rss_bytes":max_rss,"child_returncode":rc,
        "timeout_hit":timed_out,"rss_guard_hit":rss_hit,"status":status,
        "scientific_disposition":scientific,"runtime_integrity_clean":integrity_clean,
        "finite_open_ended_growth_gate":record.get("finite_open_ended_growth_gate") if record else None,
        "record_parse_status":parse,"stdout_bytes":stdout_path.stat().st_size,
        "stderr_bytes":stderr_path.stat().st_size,"complete_record":record,
    }
    (evidence/f"{arm}-status.json").write_text(json.dumps(result,indent=2,sort_keys=True)+"\n",encoding="utf-8")
    return result


def signature_probe_text() -> str:
    return r'''import json
from cognitive_core.behavioral_search import find_exact_expression
from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import ArchiveRecord,CognitivePrimitive,Trace
def build(op):
 c=OpenEndedRecursiveCognitiveCompiler(max_depth=4,seed=771221)
 px=CognitivePrimitive("prim:x",{"op":"get","key":"x"},("x","y"),"delta",0,(),("opaque",),0.0,0.0,0.0,1,{})
 py=CognitivePrimitive("prim:y",{"op":"get","key":"y"},("x","y"),"delta",0,(),("opaque",),0.0,0.0,0.0,1,{})
 c.primitives={px.primitive_id:px,py.primitive_id:py}
 c.archive={px.primitive_id:ArchiveRecord(px),py.primitive_id:ArchiveRecord(py)}
 rows=[]
 for i in range(8):
  x,y=float(i),float(i+1)
  target=max(x,y) if op=="max" else min(x,y) if op=="min" else x+y
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
  "key_lifetime":dict(getattr(find_exact_expression,"_H7_LAST_KEY_LIFETIME_STATS",{})),
  "frontier_cache":dict(getattr(find_exact_expression,"_H7_LAST_LF_CACHE_STATS",{})),
 }
print(json.dumps({"signatures":signatures,"counters":counters},sort_keys=True,separators=(",",":")))
'''


def main() -> int:
    ap=argparse.ArgumentParser()
    ap.add_argument("--repo-root",type=Path,required=True)
    ap.add_argument("--acsie-root",type=Path,required=True)
    ap.add_argument("--evidence",type=Path,required=True)
    ap.add_argument("--seed",type=int,default=SEED)
    ap.add_argument("--generations",type=int,default=GENERATIONS)
    ap.add_argument("--timeout-seconds",type=int,default=CHILD_TIMEOUT_SECONDS)
    ap.add_argument("--rss-guard-bytes",type=int,default=RSS_GUARD_BYTES)
    ap.add_argument("--executor-sha",default=os.environ.get("GITHUB_SHA","unknown"))
    ap.add_argument("--workflow-run-id",default=os.environ.get("GITHUB_RUN_ID","unknown"))
    args=ap.parse_args()
    repo_root=args.repo_root.resolve()
    acsie_root=args.acsie_root.resolve()
    lab_dir=repo_root/"lab"
    evidence=args.evidence.resolve()
    evidence.mkdir(parents=True,exist_ok=True)
    diag_dir=repo_root/"diagnostics"/"h7-pruned-key-release"
    make_sampler(diag_dir)

    src=acsie_root/"cognitive_core"/"behavioral_search.py"
    if sha256_file(src)!=SOURCE_SHA256:
        raise SystemExit("exact ACSIE source SHA check failed")
    head=subprocess.run(["git","-C",str(acsie_root),"rev-parse","HEAD"],capture_output=True,text=True,check=True).stdout.strip()
    tree=subprocess.run(["git","-C",str(acsie_root),"rev-parse","HEAD^{tree}"],capture_output=True,text=True,check=True).stdout.strip()
    if (head,tree)!=(SOURCE_COMMIT,SOURCE_TREE):
        raise SystemExit(f"exact ACSIE commit/tree mismatch: {(head,tree)}")

    pristine=repo_root/"bucket-duplicate-pristine-acsie"
    control=repo_root/"bucket-duplicate-control-acsie"
    for worktree in (pristine,control):
        subprocess.run(["git","-C",str(acsie_root),"worktree","remove","--force",str(worktree)],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        if worktree.exists():
            import shutil
            shutil.rmtree(worktree)
        subprocess.run(["git","-C",str(acsie_root),"worktree","add","--detach",str(worktree),SOURCE_COMMIT],check=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True)

    patcher=lab_dir/"patch_h7_pruned_key_release.py"
    for mode,root in (("control",control),("candidate",acsie_root)):
        out=evidence/f"patch-{mode}.txt"
        run_checked([sys.executable,str(patcher),str(root/"cognitive_core"/"behavioral_search.py"),"--mode",mode],cwd=repo_root,env=os.environ.copy(),output=out)
        compile((root/"cognitive_core"/"behavioral_search.py").read_text(encoding="utf-8"),str(root/"cognitive_core"/"behavioral_search.py"),"exec")

    provenance={
        "schema":"ACSIE.h7_pruned_key_release.provenance.v1",
        "executor_commit":args.executor_sha,"workflow_run_id":str(args.workflow_run_id),
        "source_commit":SOURCE_COMMIT,"source_tree":SOURCE_TREE,"original_source_sha256":SOURCE_SHA256,
        "control_diagnostic_sha256":sha256_file(control/"cognitive_core"/"behavioral_search.py"),
        "candidate_diagnostic_sha256":sha256_file(acsie_root/"cognitive_core"/"behavioral_search.py"),
        "factor":"metadata-only register_binary handling for dominated/rejected state tokens, preserving register() side effects and retained representative",
        "fixed_factor":"lineage-frontier FIFO cache 128 in both arms",
        "other_caches":"structural-key interning OFF; macro-ID memoization OFF; structural-expansion cache OFF; grouping OFF",
        "seed":args.seed,"generation_indices":list(range(args.generations)),
        "child_timeout_seconds":args.timeout_seconds,"rss_guard_bytes":args.rss_guard_bytes,
        "runtime_integrity":{"external_model":False,"target_access_during_discovery":False,"target_identity_available_to_runtime":False,"task_family_route":False},
    }
    (evidence/"provenance.json").write_text(json.dumps(provenance,indent=2,sort_keys=True)+"\n",encoding="utf-8")

    tests_by_mode={}
    for mode,root in (("control",control),("candidate",acsie_root)):
        env=env_for(root)
        text=run_checked([sys.executable,"-m","py_compile",str(root/"cognitive_core"/"behavioral_search.py")],cwd=repo_root,env=env,output=evidence/f"pycompile-{mode}.txt")
        output=evidence/f"six-file-regression-{mode}.txt"
        test_text=run_checked([sys.executable,"-m","pytest","-q",*[str(root/t) for t in TEST_FILES]],cwd=repo_root,env=env,output=output)
        m=re.search(r"(\d+) passed(?:,|\s)",test_text)
        if not m:
            raise RuntimeError(f"pytest pass count unavailable in {output}")
        tests_by_mode[mode]={"passed":int(m.group(1)),"output_file":output.name,"tail":test_text[-400:]}

    probe=evidence/"signature_probe.py"
    probe.write_text(signature_probe_text(),encoding="utf-8")
    sigs={}
    for mode,root in (("pristine",pristine),("control",control),("candidate",acsie_root)):
        output=evidence/f"signatures-{mode}.json"
        run_checked([sys.executable,str(probe)],cwd=repo_root,env=env_for(root),output=output)
        sigs[mode]=json.loads(output.read_text(encoding="utf-8"))
    pristine_sigs=sigs["pristine"]["signatures"]
    parity={
        "schema":"ACSIE.h7_pruned_key_release.parity.v1",
        "control_matches_pristine":sigs["control"]["signatures"]==pristine_sigs,
        "candidate_matches_pristine":sigs["candidate"]["signatures"]==pristine_sigs,
        "control_matches_candidate":sigs["control"]["signatures"]==sigs["candidate"]["signatures"],
        "six_file_regression":tests_by_mode,
        "signatures_and_counters":sigs,
    }
    (evidence/"preflight-parity.json").write_text(json.dumps(parity,indent=2,sort_keys=True)+"\n",encoding="utf-8")
    if not parity["control_matches_pristine"] or not parity["candidate_matches_pristine"] or not parity["control_matches_candidate"]:
        raise RuntimeError("pruned-key diagnostic changed an exact science fixture signature")
    if any(x["passed"]!=61 for x in tests_by_mode.values()):
        raise RuntimeError("six-file suite failed the 61/61 gate")
    candidate_removed_rejected = sum(
        int(sigs["candidate"]["counters"][op]["key_lifetime"].get("keys_removed_rejected",0))
        for op in ("max","min","sum")
    )
    candidate_removed_dominated = sum(
        int(sigs["candidate"]["counters"][op]["key_lifetime"].get("keys_removed_dominated",0))
        for op in ("max","min","sum")
    )
    if candidate_removed_rejected < 1:
        raise RuntimeError("dominance-rejected candidate key-release path was not exercised by parity fixtures")
    dominated_path_fixture_coverage = candidate_removed_dominated > 0
    # The existing-state-prune path may not arise in the compact max/min/sum
    # fixture. Do not falsify coverage by weakening the record: continue only
    # as a diagnostic and report the gap explicitly; Gen6 must show the path
    # before this sub-mechanism can be considered tested end to end.

    result={}
    for arm,root in (("control_frontier128_pruned_key_release_off",control),("candidate_frontier128_pruned_key_release_on",acsie_root)):
        result[arm]=run_actual_arm(arm=arm,source_root=root,lab_dir=lab_dir,diag_dir=diag_dir,evidence=evidence,seed=args.seed,generations=args.generations,timeout_seconds=args.timeout_seconds,rss_guard_bytes=args.rss_guard_bytes)

    snapshot_summary={}
    for arm in result:
        stderr=(evidence/f"{arm}.stderr").read_text(errors="replace")
        snaps=[]
        sp=evidence/f"{arm}-snapshots.jsonl"
        if sp.exists():
            for line in sp.read_text(errors="replace").splitlines():
                try: snaps.append(json.loads(line))
                except json.JSONDecodeError: pass
        blocks=re.split(r"(?=Current thread 0x)",stderr)
        freq=collections.Counter()
        stack_count=0
        for block in blocks:
            frames=re.findall(r'File "([^"]+)", line (\d+) in ([^\n]+)',block)
            if frames:
                stack_count+=1
                freq.update(f[2] for f in frames)
        grouped={}
        by_gen={}
        for snap in snaps:
            grouped[(snap.get("generation_index"),snap.get("search_call_id"),snap.get("search_frame_id"))]=snap
            by_gen[str(snap.get("generation_index"))]=snap
        snapshot_summary[arm]={
            "live_snapshot_count":len(snaps),"stack_snapshot_count":stack_count,
            "distinct_search_segments_observed":len(grouped),
            "generation_indices_observed":sorted({x.get("generation_index") for x in snaps if x.get("generation_index") is not None}),
            "latest_snapshot":snaps[-1] if snaps else None,
            "latest_snapshot_by_generation":by_gen,
            "stack_function_frequency":dict(freq),
        }

    final={
        "schema":"ACSIE.h7_pruned_key_release.diagnostic.v1",
        "provenance":provenance,
        "preflight":{"status":"PASSED" if dominated_path_fixture_coverage else "PASSED_WITH_EXISTING_STATE_PRUNE_PATH_UNCOVERED_IN_COMPACT_FIXTURES","existing_state_prune_path_exercised_by_fixtures":dominated_path_fixture_coverage,"exact_signature_parity":{k:parity[k] for k in ("control_matches_pristine","candidate_matches_pristine","control_matches_candidate")},"six_file_test_counts":{k:v["passed"] for k,v in tests_by_mode.items()},"candidate_keys_removed_rejected":candidate_removed_rejected,"candidate_keys_removed_dominated":candidate_removed_dominated},
        "actual_gen6":result,
        "stack_and_live_telemetry":snapshot_summary,
        "science_interpretation":"A complete child record is required to decide the finite seven-generation gate. Target-candidate snapshots and timeouts are diagnostic/inconclusive; this is not a frozen 5x12 qualification."
    }
    (evidence/"h7-pruned-key-release-summary.json").write_text(json.dumps(final,indent=2,sort_keys=True)+"\n",encoding="utf-8")
    print(json.dumps({
        "preflight":final["preflight"],
        "actual_gen6":{arm:{k:v.get(k) for k in ("status","scientific_disposition","elapsed_seconds","max_observed_rss_bytes","record_parse_status","stdout_bytes")} for arm,v in result.items()},
        "snapshots":{arm:{"generations":v["generation_indices_observed"],"count":v["live_snapshot_count"],"latest":v["latest_snapshot"]} for arm,v in snapshot_summary.items()},
        "summary":"h7-pruned-key-release-summary.json"
    },indent=2,sort_keys=True))
    return 0


if __name__=="__main__":
    raise SystemExit(main())
