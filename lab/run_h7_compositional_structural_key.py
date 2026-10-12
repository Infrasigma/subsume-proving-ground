#!/usr/bin/env python3
"""Parity-gated H7 compositional structural-key experiment on actual Gen6."""
from __future__ import annotations

import argparse
import collections
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time

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

def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()

def run_checked(argv: list[str], *, cwd: Path, env: dict[str, str], output: Path) -> str:
    with output.open("w", encoding="utf-8") as handle:
        proc = subprocess.run(argv, cwd=cwd, env=env, stdout=handle, stderr=subprocess.STDOUT, text=True)
    out = output.read_text(errors="replace")
    if proc.returncode != 0:
        raise RuntimeError(f"preflight command failed rc={proc.returncode}: {' '.join(argv)}; output={output}")
    return out

def env_for(*python_paths: Path) -> dict[str, str]:
    env = os.environ.copy()
    env["PYTHONHASHSEED"] = "0"
    env["PYTHONPATH"] = os.pathsep.join(str(p) for p in python_paths)
    env.pop("ACSIE_DISABLE_LF_CACHE", None)
    return env

def make_sampler(diag_dir: Path) -> None:
    """Install only a periodic stack/live-state sampler; no per-node counters."""
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
        row["search_call_id"]=loc.get("_H7_BUCKET_SEARCH_CALL_ID")
        row["search_frame_id"]=loc.get("_H7_BUCKET_SEARCH_FRAME_ID",id(f))
        st=loc.get("stats",{})
        row["search"]={k:st.get(k) for k in ("rows","max_depth","primitive_count","generated_expressions","unique_states","duplicate_states","dominance_pruned_states","dominance_frontier_max","status")}
        row["frontier_cache"]=dict(loc.get("_h7_lf_cache_stats",{}))
        row["frontier_cache_entries"]=len(loc.get("_h7_lf_cache",{}))
        row["online_metadata_count"]=len(loc.get("_H7_online_metadata",{}))
        row["structural_lineage_key_count"]=len(loc.get("structural_lineage_by_key",{}))
        row["expression_key_map_entries"]=len(loc.get("expression_key_by_state_token",{}))
        ancestor=f.f_back
        while ancestor is not None:
            if ancestor.f_code.co_name=="run_seed":
                run_loc=ancestor.f_locals
                row["generation_index"]=run_loc.get("generation")
                row["retained_capability_count"]=len(run_loc.get("retained",()))
                row["completed_generation_records"]=len(run_loc.get("generations_out",()))
                break
            ancestor=ancestor.f_back
    path=os.environ.get("ACSIE_COMPOSITIONAL_KEY_SNAPSHOT_LOG")
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

def run_actual_arm(*, arm: str, source_root: Path, lab_dir: Path, diag_dir: Path,
                   evidence: Path, seed: int, generations: int,
                   timeout_seconds: int, rss_guard_bytes: int) -> dict:
    stdout_path=evidence/f"{arm}.stdout.jsonl"
    stderr_path=evidence/f"{arm}.stderr"
    snapshot_path=evidence/f"{arm}-snapshots.jsonl"
    for p in (stdout_path,stderr_path,snapshot_path):
        p.unlink(missing_ok=True)
    env=env_for(diag_dir,lab_dir,source_root)
    env["ACSIE_COMPOSITIONAL_KEY_SNAPSHOT_LOG"]=str(snapshot_path)
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
signatures={}
for op in ("max","min","sum"):
 c,rows,depth,obj=build(op)
 r=find_exact_expression(c,rows,max_depth=depth,require_macro=True,selection_objective=obj,max_alternatives=8)
 signatures[op]=sig(r)
print(json.dumps({"signatures":signatures},sort_keys=True,separators=(",",":")))
'''

def structural_key_probe_text() -> str:
    return r'''import json
from cognitive_core.behavioral_search import find_exact_expression
from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import ArchiveRecord,CognitivePrimitive,Trace

def prim(pid,expr):
 return CognitivePrimitive(pid,expr,("x","y","z"),"delta",0,(),("opaque",),0.0,0.0,0.0,1,{})
def run():
 c=OpenEndedRecursiveCognitiveCompiler(max_depth=4,seed=771221)
 base=[
  prim("prim:x",{"op":"get","key":"x"}),
  prim("prim:y",{"op":"get","key":"y"}),
  prim("prim:z",{"op":"get","key":"z"}),
  prim("prim:x_alias",{"op":"macro","id":"prim:x"}),
 ]
 c.primitives={p.primitive_id:p for p in base}
 c.archive={p.primitive_id:ArchiveRecord(p) for p in base}
 rows=tuple(Trace({"x":float(i),"y":float(i+1),"z":float(i%3)},float(i),"opaque",{},f"keyprobe-{i}") for i in range(4))
 # Enter a trivial depth-zero search once to create invocation-local closure hooks.
 find_exact_expression(c,rows,max_depth=0,require_macro=False,max_alternatives=2)
 keyfn=getattr(find_exact_expression,"_H7_STRUCTURAL_KEY_FN")
 expandfn=getattr(find_exact_expression,"_H7_STRUCTURAL_EXPAND_FN")
 pmap=getattr(find_exact_expression,"_H7_STRUCTURAL_PMAP")
 # Add an adversarial macro cycle to the test map only. It is not a runtime
 # primitive, target, task route, or search candidate.
 pmap["diag:cycle-a"]={"op":"macro","id":"diag:cycle-b"}
 pmap["diag:cycle-b"]={"op":"macro","id":"diag:cycle-a"}
 gx={"op":"get","key":"x"}; gy={"op":"get","key":"y"}; gz={"op":"get","key":"z"}
 cases={
  "get_x":gx,
  "const_float":{"op":"const","value":1.25},
  "const_nan":{"op":"const","value":float("nan"),"tag":{"z":1,"a":"é"}},
  "leaf_extra_sorted_keys":{"op":"get","key":"unicode-ß","extra":{"z":1,"a":"é"}},
  "add_xy":{"op":"add","left":gx,"right":gy},
  "add_yx":{"op":"add","left":gy,"right":gx},
  "nested_commutative":{"op":"add","left":{"op":"mul","left":gx,"right":{"op":"const","value":2.0}},"right":{"op":"macro","id":"prim:x_alias"}},
  "sub_xy":{"op":"sub","left":gx,"right":gy},
  "sub_yx":{"op":"sub","left":gy,"right":gx},
  "mul_nested":{"op":"mul","left":{"op":"max","left":gx,"right":gy},"right":{"op":"min","left":gz,"right":{"op":"const","value":-0.0}}},
  "abs":{"op":"abs","arg":{"op":"neg","arg":gx}},
  "threshold":{"op":"threshold","arg":gy,"lo":1.5},
  "macro_alias":{"op":"macro","id":"prim:x_alias"},
  "missing_macro":{"op":"macro","id":"diag:missing"},
  "macro_cycle":{"op":"macro","id":"diag:cycle-a"},
 }
 out={}
 for name,expr in cases.items():
  expected=json.dumps(expandfn(expr),sort_keys=True,separators=(",",":"))
  actual=keyfn(expr)
  out[name]={"actual":actual,"expected":expected,"matches_reference":actual==expected}
 out["invariants"]={
  "commutative_add_reordering_equal":keyfn(cases["add_xy"])==keyfn(cases["add_yx"]),
  "noncommutative_sub_reordering_distinct":keyfn(cases["sub_xy"])!=keyfn(cases["sub_yx"]),
  "macro_alias_expands_to_base":keyfn(cases["macro_alias"])==keyfn(gx),
  "macro_cycle_terminates_and_matches_reference":out["macro_cycle"]["matches_reference"],
 }
 print(json.dumps(out,sort_keys=True,separators=(",",":"),allow_nan=True))
if __name__=="__main__": run()
'''

def main() -> int:
    ap=argparse.ArgumentParser()
    ap.add_argument("--repo-root",type=Path,required=True)
    ap.add_argument("--acsie-root",type=Path,required=True)
    ap.add_argument("--evidence",type=Path,required=True)
    ap.add_argument("--seed",type=int,default=2026100301)
    ap.add_argument("--generations",type=int,default=7)
    ap.add_argument("--timeout-seconds",type=int,default=180)
    ap.add_argument("--rss-guard-bytes",type=int,default=2*1024*1024*1024)
    args=ap.parse_args()
    repo=args.repo_root.resolve()
    acsie=args.acsie_root.resolve()
    evidence=args.evidence.resolve()
    evidence.mkdir(parents=True,exist_ok=True)
    source=acsie/"cognitive_core"/"behavioral_search.py"
    head=subprocess.run(["git","-C",str(acsie),"rev-parse","HEAD"],check=True,capture_output=True,text=True).stdout.strip()
    tree=subprocess.run(["git","-C",str(acsie),"rev-parse","HEAD^{tree}"],check=True,capture_output=True,text=True).stdout.strip()
    if (head,tree,sha256_file(source))!=(SOURCE_COMMIT,SOURCE_TREE,SOURCE_SHA256):
        raise SystemExit(f"pinned source identity mismatch: {head} {tree} {sha256_file(source)}")
    sys.path.insert(0,str(repo/"lab"))
    from patch_h7_compositional_structural_key import apply_source
    temp=Path(tempfile.mkdtemp(prefix="h7-compositional-key-"))
    roots={name:temp/name for name in ("pristine","control","candidate")}
    summary={
      "schema":"ACSIE.h7_compositional_structural_key.diagnostic.v1",
      "source":{"commit":head,"tree":tree,"behavioral_search_sha256":sha256_file(source)},
      "factor":"replace expanded-subtree JSON dumps for structural-key construction with exact compositional canonical JSON key construction",
      "fixed_factor":"same lineage-frontier FIFO cache capacity 128 in control and candidate",
      "seed":args.seed,"generation_indices":list(range(args.generations)),
      "child_timeout_seconds":args.timeout_seconds,"rss_guard_bytes":args.rss_guard_bytes,
      "external_model":False,"target_access_during_discovery":False,"task_family_route":False,
      "preflight":{},"actual_gen6":{},"stack_and_live_telemetry":{},
    }
    try:
        for name,root in roots.items():
            subprocess.run(["git","-C",str(acsie),"worktree","add","--detach",str(root),SOURCE_COMMIT],
                           check=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True)
        for mode in ("control","candidate"):
            root=roots[mode]
            src=root/"cognitive_core"/"behavioral_search.py"
            before=sha256_file(src)
            patched=apply_source(src.read_text(encoding="utf-8"),mode)
            src.write_text(patched,encoding="utf-8")
            compile(patched,str(src),"exec")
            (evidence/f"patch-{mode}.txt").write_text(
                f"MODE={mode}\nSOURCE_COMMIT={SOURCE_COMMIT}\nSOURCE_TREE={SOURCE_TREE}\n"
                f"ORIGINAL_SOURCE_SHA256={before}\nDIAGNOSTIC_SOURCE_SHA256={sha256_file(src)}\n",
                encoding="utf-8")
            run_checked([sys.executable,"-m","py_compile",str(src)],cwd=repo,env=env_for(root),output=evidence/f"pycompile-{mode}.txt")
        # Save diagnostic probes so the raw contract is reproducible.
        sig_probe=evidence/"signature-probe.py"
        sig_probe.write_text(signature_probe_text(),encoding="utf-8")
        key_probe=evidence/"structural-key-probe.py"
        key_probe.write_text(structural_key_probe_text(),encoding="utf-8")
        signature_matrix={}
        key_matrix={}
        for mode,root in roots.items():
            sig_log=evidence/f"signatures-{mode}.json"
            sig_out=run_checked([sys.executable,str(sig_probe)],cwd=repo,env=env_for(root),output=sig_log)
            signature_matrix[mode]=json.loads(sig_out)
            if mode!="pristine":
                key_log=evidence/f"structural-keys-{mode}.json"
                key_out=run_checked([sys.executable,str(key_probe)],cwd=repo,env=env_for(root),output=key_log)
                key_matrix[mode]=json.loads(key_out)
        sigs={k:v["signatures"] for k,v in signature_matrix.items()}
        signature_parity={
            "control_matches_pristine":sigs["control"]==sigs["pristine"],
            "candidate_matches_pristine":sigs["candidate"]==sigs["pristine"],
            "control_matches_candidate":sigs["control"]==sigs["candidate"],
        }
        key_checks={}
        for mode,obj in key_matrix.items():
            cases={k:v for k,v in obj.items() if isinstance(v,dict) and "matches_reference" in v}
            invariants=obj.get("invariants",{})
            key_checks[mode]={
                "all_cases_match_expanded_reference":all(v["matches_reference"] for v in cases.values()),
                "case_count":len(cases),
                "mismatch_cases":[k for k,v in cases.items() if not v["matches_reference"]],
                "commutative_add_reordering_equal":invariants.get("commutative_add_reordering_equal") is True,
                "noncommutative_sub_reordering_distinct":invariants.get("noncommutative_sub_reordering_distinct") is True,
                "macro_alias_expands_to_base":invariants.get("macro_alias_expands_to_base") is True,
                "macro_cycle_terminates_and_matches_reference":invariants.get("macro_cycle_terminates_and_matches_reference") is True,
            }
        key_parity=key_matrix.get("control")==key_matrix.get("candidate")
        preflight={
            "status":"PASSED",
            "signature_parity":signature_parity,
            "structural_key_reference_checks":key_checks,
            "control_candidate_key_probe_identical":key_parity,
            "fixed_frontier_cache_capacity":128,
            "production_source_modified":False,
        }
        (evidence/"preflight.json").write_text(json.dumps(preflight,indent=2,sort_keys=True,allow_nan=True)+"\n",encoding="utf-8")
        summary["preflight"]=preflight
        if not all(signature_parity.values()) or not key_parity or not all(
            item["all_cases_match_expanded_reference"] and item["commutative_add_reordering_equal"]
            and item["noncommutative_sub_reordering_distinct"] and item["macro_alias_expands_to_base"]
            and item["macro_cycle_terminates_and_matches_reference"]
            for item in key_checks.values()
        ):
            preflight["status"]="FAILED"
            (evidence/"preflight.json").write_text(json.dumps(preflight,indent=2,sort_keys=True,allow_nan=True)+"\n",encoding="utf-8")
            summary["preflight"]=preflight
            raise RuntimeError("structural-key/signature adversarial preflight failed; actual Gen6 not run")
        # The six-file regression suite must pass in both source-pinned diagnostic arms.
        test_report={}
        for mode in ("control","candidate"):
            root=roots[mode]
            test_log=evidence/f"six-file-regression-{mode}.txt"
            out=run_checked([sys.executable,"-m","pytest","-q",
                             *[str(root/t) for t in TEST_FILES]],
                            cwd=repo,env=env_for(root),output=test_log)
            match=re.search(r"(\d+) passed(?:,|\s)",out)
            if not match or int(match.group(1))!=61:
                raise RuntimeError(f"{mode} regression suite did not pass exactly 61/61")
            test_report[mode]={"passed":int(match.group(1)),"log":test_log.name}
        preflight["six_file_regression"]=test_report
        (evidence/"preflight.json").write_text(json.dumps(preflight,indent=2,sort_keys=True,allow_nan=True)+"\n",encoding="utf-8")
        summary["preflight"]=preflight

        diag_dir=evidence/"sampler"
        make_sampler(diag_dir)
        roots["control"].resolve()
        for mode in ("control","candidate"):
            arm=f"{mode}_frontier128_{'original_structural_key' if mode=='control' else 'compositional_structural_key'}"
            result=run_actual_arm(arm=arm,source_root=roots[mode],lab_dir=repo/"lab",
                                  diag_dir=diag_dir,evidence=evidence,seed=args.seed,
                                  generations=args.generations,timeout_seconds=args.timeout_seconds,
                                  rss_guard_bytes=args.rss_guard_bytes)
            summary["actual_gen6"][arm]=result
        stack_summary={}
        for arm in summary["actual_gen6"]:
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
            stack_summary[arm]={
                "live_snapshot_count":len(snaps),"stack_snapshot_count":stack_count,
                "generation_indices_observed":sorted({x.get("generation_index") for x in snaps if x.get("generation_index") is not None}),
                "latest_snapshot":snaps[-1] if snaps else None,
                "stack_function_frequency":dict(freq),
            }
        summary["stack_and_live_telemetry"]=stack_summary
        summary["interpretation_guard"]="A live target candidate or a timeout is not a completed generation trajectory; do not treat wrapper success as scientific acceptance."
        (evidence/"summary.json").write_text(json.dumps(summary,indent=2,sort_keys=True,allow_nan=True)+"\n",encoding="utf-8")
        print(json.dumps({
            "preflight":summary["preflight"],
            "actual_gen6":{arm:{k:v.get(k) for k in ("status","scientific_disposition","elapsed_seconds","max_observed_rss_bytes","record_parse_status","stdout_bytes")} for arm,v in summary["actual_gen6"].items()},
            "snapshots":{arm:{"generations":v["generation_indices_observed"],"count":v["live_snapshot_count"],"latest":v["latest_snapshot"]} for arm,v in stack_summary.items()},
            "summary":"summary.json"
        },indent=2,sort_keys=True,allow_nan=True))
        return 0
    finally:
        for root in roots.values():
            subprocess.run(["git","-C",str(acsie),"worktree","remove","--force",str(root)],
                           stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        shutil.rmtree(temp,ignore_errors=True)

if __name__=="__main__":
    raise SystemExit(main())
