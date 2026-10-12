#!/usr/bin/env python3
"""Single-cell Gen3 phase localization from the authentic historical Gen2 state.

This is observer-only: current pinned Gen3 selection/gain/retention code and all
acceptance gates remain unchanged. It emits one 180-second diagnostic record
with periodic stack/live-candidate snapshots and low-frequency per-candidate
evaluation events, so timeouts remain useful evidence rather than empty runs.
"""
from __future__ import annotations

import argparse
import collections
import faulthandler
import hashlib
import importlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time

SOURCE_COMMIT = "d83cb60286f195eec02b4d7aa45a60539316d115"
SOURCE_TREE = "12d2fa8fc1f1e34d3d7c442eef85aa6a71380420"
HISTORICAL_ARTIFACT_ID = 10950102751
HISTORICAL_ARTIFACT_SHA256 = "8a1afedbac85317e031ec043aace643c1f4c87822dd302a19b888e23c58596c9"
HISTORICAL_STATE_SHA256 = "0b599e436c4578115cc0312dd6545f5d56c14777fcb55562592c5d8f4e8f80a9"
HISTORICAL_PARENT = "b31109864ae547320d877e6c1575b805d950edf9"
HISTORICAL_EXPORTER = "94a2a770a5799fd386217998bc21b585632ef7ed"
FAILED_CURRENT_EXPORTER = "0e9c2ad9a146775c162a278b0330c596d29055b9"

def dump_json(path: Path, obj) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(obj, indent=2, sort_keys=True, allow_nan=True) + "\n", encoding="utf-8")

def sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda:f.read(1024*1024), b""):
            h.update(chunk)
    return h.hexdigest()

def normalized_json(value) -> str:
    return json.dumps(value, sort_keys=True, separators=(",", ":"), default=str, allow_nan=True)

def append_event(path: Path, row: dict) -> None:
    row = {"t_monotonic": round(time.monotonic(), 3), **row}
    with path.open("a", encoding="utf-8", buffering=1) as handle:
        handle.write(normalized_json(row) + "\n")

def worker(acsie_root: Path, frozen_path: Path, evidence: Path, seed: int) -> int:
    sys.path.insert(0, str(acsie_root))
    event_path = evidence / "candidate-phase-events.jsonl"
    snapshot_path = evidence / "candidate-phase-snapshots.jsonl"
    for p in (event_path, snapshot_path):
        p.unlink(missing_ok=True)

    gen3 = importlib.import_module("research.gen3_from_frozen_verified_gen2_v1")
    core_type = importlib.import_module("cognitive_core.native_independent_core").NativeCognitiveCore
    original_variants = core_type._compositional_kernel_variants
    original_eval = core_type._evaluate_kernel
    original_regression = gen3._candidate_regression_scores
    state = {
        "candidate_index_by_id": {},
        "candidate_fingerprints": {},
        "variant_count": None,
        "eval_calls": 0,
        "regression_calls": 0,
        "eval_in_flight": None,
        "regression_in_flight": None,
        "last_finished_candidate_index": None,
    }

    def variant_wrapper(cls, base, streams, *, max_programs=128):
        started = time.monotonic()
        values = original_variants(base, streams, max_programs=max_programs)
        state["variant_count"] = len(values)
        state["candidate_index_by_id"] = {id(v): i for i, v in enumerate(values)}
        state["candidate_fingerprints"] = {
            id(v): hashlib.sha256(normalized_json(v.__dict__).encode()).hexdigest()
            for v in values
        }
        append_event(event_path, {
            "event":"variants_generated",
            "variant_count":len(values),
            "input_stream_count":len(streams),
            "input_row_count":sum(len(s) for s in streams),
            "max_programs":max_programs,
            "elapsed_ms":round((time.monotonic()-started)*1000,3),
        })
        return values

    def evaluate_wrapper(kernel, streams, *, seed=0, include_retention=True, protected_hypotheses=None):
        started = time.monotonic()
        idx = state["candidate_index_by_id"].get(id(kernel))
        state["eval_calls"] += 1
        call_id = state["eval_calls"]
        state["eval_in_flight"] = {
            "call_id":call_id,"candidate_index":idx,"stream_count":len(streams),
            "row_count":sum(len(s) for s in streams),"include_retention":bool(include_retention),
        }
        append_event(event_path, {
            "event":"kernel_eval_start","call_id":call_id,"candidate_index":idx,
            "kernel_fingerprint":state["candidate_fingerprints"].get(id(kernel)),
            "stream_count":len(streams),"row_count":sum(len(s) for s in streams),
            "seed":int(seed),"include_retention":bool(include_retention),
        })
        error = None
        result = None
        try:
            result = original_eval(
                kernel, streams, seed=seed, include_retention=include_retention,
                protected_hypotheses=protected_hypotheses,
            )
            return result
        except BaseException as exc:
            error=f"{type(exc).__name__}: {exc}"
            raise
        finally:
            row={
                "event":"kernel_eval_end","call_id":call_id,"candidate_index":idx,
                "kernel_fingerprint":state["candidate_fingerprints"].get(id(kernel)),
                "elapsed_ms":round((time.monotonic()-started)*1000,3),
                "scores":list(result) if result is not None else None,
                "error":error,
            }
            append_event(event_path,row)
            if idx is not None and result is not None and not include_retention:
                state["last_finished_candidate_index"]=idx
            state["eval_in_flight"]=None

    def regression_wrapper(pre_state, candidate, regression_sets):
        started=time.monotonic()
        idx=state["candidate_index_by_id"].get(id(candidate))
        state["regression_calls"]+=1
        call_id=state["regression_calls"]
        state["regression_in_flight"]={"call_id":call_id,"candidate_index":idx}
        append_event(event_path,{
            "event":"historical_regression_start","call_id":call_id,
            "candidate_index":idx,"set_count":len(regression_sets),
            "stream_count":sum(len(g) for g in regression_sets),
            "row_count":sum(len(s) for g in regression_sets for s in g),
            "candidate_fingerprint":state["candidate_fingerprints"].get(id(candidate)),
        })
        error=None
        result=None
        try:
            result=original_regression(pre_state,candidate,regression_sets)
            return result
        except BaseException as exc:
            error=f"{type(exc).__name__}: {exc}"
            raise
        finally:
            append_event(event_path,{
                "event":"historical_regression_end","call_id":call_id,
                "candidate_index":idx,
                "elapsed_ms":round((time.monotonic()-started)*1000,3),
                "scores":list(result) if result is not None else None,
                "error":error,
            })
            state["regression_in_flight"]=None

    def on_snapshot(signum, frame):
        try:
            faulthandler.dump_traceback(file=sys.stderr, all_threads=True)
        except Exception:
            pass
        select_frame=None
        f=frame
        while f is not None:
            if f.f_code.co_name == "_select_candidate":
                select_frame=f
                break
            f=f.f_back
        loc=select_frame.f_locals if select_frame is not None else {}
        candidate=loc.get("candidate")
        idx=loc.get("idx")
        diag=loc.get("diagnostics",[])
        eligible=loc.get("candidates",[])
        row={
            "event":"periodic_snapshot","elapsed_process_s":round(time.monotonic()-child_started,3),
            "signal":int(signum),"select_candidate_frame_found":select_frame is not None,
            "current_candidate_index":idx,"variant_count":len(loc.get("variants",[])) if loc.get("variants") is not None else state["variant_count"],
            "diagnostic_rows_appended":len(diag) if hasattr(diag,"__len__") else None,
            "eligible_candidates":len(eligible) if hasattr(eligible,"__len__") else None,
            "candidate_type":type(candidate).__name__ if candidate is not None else None,
            "candidate_fingerprint":state["candidate_fingerprints"].get(id(candidate)) if candidate is not None else None,
            "evaluate_calls_completed_or_started":state["eval_calls"],
            "regression_calls_completed_or_started":state["regression_calls"],
            "eval_in_flight":state["eval_in_flight"],
            "regression_in_flight":state["regression_in_flight"],
            "last_finished_candidate_index":state["last_finished_candidate_index"],
        }
        append_event(snapshot_path,row)

    child_started=time.monotonic()
    signal.signal(signal.SIGUSR1,on_snapshot)
    faulthandler.enable(file=sys.stderr, all_threads=True)
    gen3._candidate_regression_scores=regression_wrapper
    core_type._compositional_kernel_variants=classmethod(variant_wrapper)
    core_type._evaluate_kernel=staticmethod(evaluate_wrapper)
    try:
        result=gen3.run(str(frozen_path),seed=seed)
        envelope={
            "schema":"ACSIE.gen3_phase_localization.result.v1",
            "mode":"single_cell_frozen_historical_predecessor_phase_localization",
            "seed":seed,"gen3_inner_stream_count":6,
            "elapsed_s":round(time.monotonic()-child_started,3),
            "candidate_variant_count":state["variant_count"],
            "evaluate_calls":state["eval_calls"],"historical_regression_calls":state["regression_calls"],
            "original_gen3_result":result,
            "source":{"commit":SOURCE_COMMIT,"tree":SOURCE_TREE},
            "historical_predecessor":{
                "artifact_id":HISTORICAL_ARTIFACT_ID,"archive_sha256":HISTORICAL_ARTIFACT_SHA256,
                "state_sha256":HISTORICAL_STATE_SHA256,"parent_commit":HISTORICAL_PARENT,
                "exporter_commit":HISTORICAL_EXPORTER,
            },
            "current_gen2_exporter":{
                "commit":FAILED_CURRENT_EXPORTER,"scientific_status":"FAILED",
                "disposition":"SEPARATE_PROTOCOL_FAILURE_NOT_SUBSTITUTED",
            },
            "integrity":result.get("integrity"),
        }
        print(normalized_json(envelope),flush=True)
        return 0
    finally:
        gen3._candidate_regression_scores=original_regression
        core_type._compositional_kernel_variants=classmethod(original_variants.__func__)
        core_type._evaluate_kernel=staticmethod(original_eval)

def rss_bytes(pid:int)->int:
    try:
        for line in Path(f"/proc/{pid}/status").read_text(errors="replace").splitlines():
            if line.startswith("VmRSS:"):
                return int(line.split()[1])*1024
    except (OSError,ValueError,IndexError):
        pass
    return 0

def parent(args) -> int:
    repo=args.repo_root.resolve()
    acsie=args.acsie_root.resolve()
    ev=args.evidence.resolve()
    ev.mkdir(parents=True,exist_ok=True)
    source=acsie/"cognitive_core"/"native_independent_core.py"
    gen3src=acsie/"research"/"gen3_from_frozen_verified_gen2_v1.py"
    commit=subprocess.run(["git","-C",str(acsie),"rev-parse","HEAD"],check=True,capture_output=True,text=True).stdout.strip()
    tree=subprocess.run(["git","-C",str(acsie),"rev-parse","HEAD^{tree}"],check=True,capture_output=True,text=True).stdout.strip()
    identity={
        "acsie_commit":commit,"acsie_tree":tree,
        "native_core_sha256":sha256(source),"gen3_runner_sha256":sha256(gen3src),
        "expected_commit":SOURCE_COMMIT,"expected_tree":SOURCE_TREE,
    }
    if (commit,tree)!=(SOURCE_COMMIT,SOURCE_TREE):
        raise SystemExit(f"source identity mismatch: {identity}")
    frozen=ev/"materialized"/"verified-gen2-export.json"
    if not frozen.is_file():
        raise SystemExit(f"verified historical predecessor export missing: {frozen}")
    frozen_obj=json.loads(frozen.read_text(encoding="utf-8"))
    state_sha=hashlib.sha256((ev/"materialized"/"verified-gen2-state.json").read_bytes()).hexdigest()
    if frozen_obj.get("scientific_status")!="PASSED" or state_sha!=HISTORICAL_STATE_SHA256:
        raise SystemExit("historical predecessor envelope/state failed revalidation")
    if frozen_obj.get("scientific_parent_commit")!=HISTORICAL_PARENT or frozen_obj.get("executed_export_commit")!=HISTORICAL_EXPORTER:
        raise SystemExit("historical predecessor source provenance mismatch")
    with frozen.open("rb") as f:
        input_digest=hashlib.sha256(f.read()).hexdigest()
    code_path=Path(__file__).resolve()
    stdout_path=ev/"gen3-child.stdout.json"
    stderr_path=ev/"gen3-child.stderr"
    events_path=ev/"candidate-phase-events.jsonl"
    snapshots_path=ev/"candidate-phase-snapshots.jsonl"
    for p in (stdout_path,stderr_path,events_path,snapshots_path):
        p.unlink(missing_ok=True)
    contract={
      "schema":"ACSIE.gen3_phase_localization.contract.v1",
      "mode":"single_cell_frozen_historical_predecessor_phase_localization",
      "seed":args.seed,"gen3_inner_stream_count":6,"candidate_cap":128,
      "timeout_seconds":args.timeout_seconds,"rss_guard_bytes":args.rss_guard_bytes,
      "signal_snapshot_interval_seconds":10,
      "observer":"low-frequency candidate-level event log + 10s stack/live candidate-index snapshots; no per-row/per-node counters",
      "source_identity":identity,
      "historical_predecessor":{
       "artifact_id":HISTORICAL_ARTIFACT_ID,"archive_sha256":HISTORICAL_ARTIFACT_SHA256,
       "state_sha256":state_sha,"export_envelope_sha256":input_digest,
       "parent_commit":HISTORICAL_PARENT,"exporter_commit":HISTORICAL_EXPORTER,
      },
      "current_gen2_exporter":{
        "commit":FAILED_CURRENT_EXPORTER,"status":"FAILED",
        "gen2_holdout_gain":0.0,"gen2_transfer_gain":0.0,"frozen_state":None,
        "disposition":"NOT_SUBSTITUTED",
      },
      "science_gate":"original Gen3 scientific gates unchanged; timeout/empty stdout => INCONCLUSIVE",
    }
    dump_json(ev/"contract.json",contract)
    started=time.monotonic();max_rss=0;timed_out=False;rss_hit=False;next_signal=started+10
    env=os.environ.copy();env["PYTHONHASHSEED"]="0";env["PYTHONPATH"]=str(acsie)
    env["ACSIE_GEN3_PHASE_EVENTS"]=str(events_path)
    # The child gets the actual ACSIE checkout as CWD so provenance source_commit() reads the pinned SHA.
    child=[sys.executable,str(code_path),"--worker",
           "--acsie-root",str(acsie),"--frozen-state",str(frozen),
           "--evidence",str(ev),"--seed",str(args.seed)]
    with stdout_path.open("w",encoding="utf-8") as stdout, stderr_path.open("w",encoding="utf-8") as stderr:
        proc=subprocess.Popen(child,cwd=acsie,env=env,stdout=stdout,stderr=stderr,text=True)
        while proc.poll() is None:
            now=time.monotonic()
            max_rss=max(max_rss,rss_bytes(proc.pid))
            if max_rss>=args.rss_guard_bytes:
                rss_hit=True
                proc.terminate()
                try: proc.wait(timeout=3)
                except subprocess.TimeoutExpired: proc.kill()
                break
            if now>=next_signal:
                try: os.kill(proc.pid,signal.SIGUSR1)
                except ProcessLookupError: pass
                next_signal=now+10
            if now-started>=args.timeout_seconds:
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
    out=stdout_path.read_text(errors="replace").strip()
    result=None
    parse_status="NO_COMPLETE_RECORD"
    if not timed_out and not rss_hit and rc==0 and out:
        try:
            result=json.loads(out)
            parse_status="COMPLETE_JSON_RECORD"
        except Exception as exc:
            parse_status=f"INVALID_JSON:{type(exc).__name__}:{exc}"
    if rss_hit: status="RESOURCE_GUARD_RSS_LIMIT"
    elif timed_out: status="INCONCLUSIVE_CHILD_WALL_TIMEOUT"
    elif result is not None: status="CHILD_RETURNED_COMPLETE_RECORD"
    elif rc!=0: status="CHILD_CRASHED_OR_EXITED_NONZERO"
    else: status="CHILD_EMPTY_OR_INVALID_OUTPUT"
    events=[]
    for line in events_path.read_text(errors="replace").splitlines() if events_path.exists() else []:
        try: events.append(json.loads(line))
        except json.JSONDecodeError: pass
    snapshots=[]
    for line in snapshots_path.read_text(errors="replace").splitlines() if snapshots_path.exists() else []:
        try: snapshots.append(json.loads(line))
        except json.JSONDecodeError: pass
    event_counts=collections.Counter(x.get("event","unknown") for x in events)
    eval_ends=[x for x in events if x.get("event")=="kernel_eval_end"]
    reg_ends=[x for x in events if x.get("event")=="historical_regression_end"]
    candidate_eval_times=[x for x in eval_ends if isinstance(x.get("candidate_index"),int)]
    result_status=((result or {}).get("original_gen3_result") or {}).get("scientific_status")
    summary={
      "schema":"ACSIE.gen3_phase_localization.summary.v1",
      "source_identity":identity,
      "historical_predecessor":contract["historical_predecessor"],
      "current_gen2_exporter":contract["current_gen2_exporter"],
      "run":{"seed":args.seed,"gen3_inner_stream_count":6,"candidate_cap":128,
        "timeout_seconds":args.timeout_seconds,"rss_guard_bytes":args.rss_guard_bytes,
        "elapsed_seconds":round(elapsed,3),"max_observed_rss_bytes":max_rss,
        "child_returncode":rc,"timeout_hit":timed_out,"rss_guard_hit":rss_hit,
        "status":status,"record_parse_status":parse_status,
        "stdout_bytes":stdout_path.stat().st_size,"stderr_bytes":stderr_path.stat().st_size},
      "observed_progress":{"event_counts":dict(event_counts),
        "completed_kernel_evaluations":len(eval_ends),
        "completed_historical_regression_calls":len(reg_ends),
        "completed_candidate_kernel_evaluations":len(candidate_eval_times),
        "distinct_candidate_indices_evaluated":len({x.get("candidate_index") for x in candidate_eval_times}),
        "max_candidate_index_finished":max((int(x["candidate_index"]) for x in candidate_eval_times),default=None),
        "candidate_eval_elapsed_ms_median":None,
        "historical_regression_elapsed_ms_median":None,
        "latest_snapshot":snapshots[-1] if snapshots else None,
        "snapshot_count":len(snapshots),
        "last_events":events[-12:]},
      "complete_gen3_scientific_status":result_status,
      "complete_child_result":result,
      "interpretation_guard":"Single-cell phase localization only, not Gen3 qualification. All original scientific gates are unchanged. Current failed Gen2 export is retained as separate protocol and is not substituted.",
    }
    if candidate_eval_times:
        v=sorted(float(x["elapsed_ms"]) for x in candidate_eval_times if isinstance(x.get("elapsed_ms"),(int,float)))
        summary["observed_progress"]["candidate_eval_elapsed_ms_median"]=v[len(v)//2] if v else None
    if reg_ends:
        v=sorted(float(x["elapsed_ms"]) for x in reg_ends if isinstance(x.get("elapsed_ms"),(int,float)))
        summary["observed_progress"]["historical_regression_elapsed_ms_median"]=v[len(v)//2] if v else None
    dump_json(ev/"phase-localization-summary.json",summary)
    print(json.dumps({k:v for k,v in summary.items() if k!="complete_child_result"},indent=2,sort_keys=True,allow_nan=True))
    # Timeout is expected evidence for the diagnostic; only source/input integrity failures are job failures.
    return 0 if status in {"INCONCLUSIVE_CHILD_WALL_TIMEOUT","RESOURCE_GUARD_RSS_LIMIT","CHILD_RETURNED_COMPLETE_RECORD"} else 2

def main() -> int:
    ap=argparse.ArgumentParser()
    ap.add_argument("--repo-root",type=Path,default=Path(__file__).resolve().parents[1])
    ap.add_argument("--acsie-root",type=Path,required=False)
    ap.add_argument("--evidence",type=Path,required=True)
    ap.add_argument("--frozen-state",type=Path,required=False)
    ap.add_argument("--seed",type=int,default=771221)
    ap.add_argument("--timeout-seconds",type=int,default=180)
    ap.add_argument("--rss-guard-bytes",type=int,default=2*1024*1024*1024)
    ap.add_argument("--worker",action="store_true")
    args=ap.parse_args()
    if args.worker:
        if not args.acsie_root or not args.frozen_state:
            raise SystemExit("--worker requires --acsie-root and --frozen-state")
        return worker(args.acsie_root.resolve(),args.frozen_state.resolve(),args.evidence.resolve(),args.seed)
    if not args.acsie_root:
        raise SystemExit("--acsie-root required in parent mode")
    if args.timeout_seconds<30 or args.timeout_seconds>300:
        raise SystemExit("phase diagnostic budget must be predeclared within 30..300 seconds")
    return parent(args)

if __name__=="__main__":
    raise SystemExit(main())
