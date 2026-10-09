#!/usr/bin/env python3
"""Observe an unchanged H7 checkpoint-continuation command; never steer ACSIE."""
from __future__ import annotations
import argparse, datetime as dt, hashlib, json, os, signal, subprocess, sys, time
from pathlib import Path

T0=time.monotonic(); LOG=None; CHILD=None; GOT_SIGNAL=None; LAST_CPU=None; LAST_MONO=None
SOURCE_CHECKPOINT="325f0be706ccfdba1aa2c850752610f1bc860162eba4dd208da7ffc8c8f4034d"
RUNTIME="9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be"
EVALUATOR="33259fc5c69d87b99c604f0050508c433a279daf"
SEED=2026100305

def utc(): return dt.datetime.now(dt.timezone.utc).isoformat(timespec="milliseconds").replace("+00:00","Z")
def emit(name, **extra):
    row={"event":name,"utc":utc(),"elapsed_wall_seconds":round(time.monotonic()-T0,3),
         "observer_pid":os.getpid(),"runner_name":os.getenv("RUNNER_NAME","UNKNOWN"),
         "runner_os":os.getenv("RUNNER_OS","UNKNOWN"),"github_run_id":os.getenv("GITHUB_RUN_ID","UNKNOWN"),
         "github_job":os.getenv("GITHUB_JOB","UNKNOWN"),"pythonhashseed":os.getenv("PYTHONHASHSEED","UNKNOWN"),
         "representative_policy":os.getenv("ACSIE_H7_REPRESENTATIVE_K","UNKNOWN")}
    row.update(extra); line=json.dumps(row,sort_keys=True,separators=(",",":"))+"\n"
    with LOG.open("a",encoding="utf-8") as f:
        f.write(line); f.flush(); os.fsync(f.fileno())
    print(line,end="",flush=True)

def proc_table():
    out={}; hz=float(os.sysconf("SC_CLK_TCK"))
    for p in Path("/proc").iterdir():
        if not p.name.isdigit(): continue
        pid=int(p.name)
        try:
            raw=(p/"stat").read_text(); i=raw.rfind(")"); comm=raw[raw.find("(")+1:i]
            q=raw[i+2:].split(); state=q[0]; ppid=int(q[1]); cpu=(int(q[11])+int(q[12]))/hz
            status=(p/"status").read_text(errors="replace"); rss=hwm=None
            for line in status.splitlines():
                if line.startswith("VmRSS:"): rss=int(line.split()[1])
                elif line.startswith("VmHWM:"): hwm=int(line.split()[1])
            try: cmd=(p/"cmdline").read_bytes().replace(b"\0",b" ").decode(errors="replace").strip()
            except OSError: cmd=comm
            out[pid]={"pid":pid,"ppid":ppid,"comm":comm,"state":state,"cpu_seconds":round(cpu,3),
                      "rss_kb":rss,"hwm_kb":hwm,"cmdline":cmd[:400]}
        except (OSError,ValueError,IndexError): pass
    return out

def sample(root):
    tab=proc_table(); ids={root}; changed=True
    while changed:
        changed=False
        for pid,row in tab.items():
            if row["ppid"] in ids and pid not in ids: ids.add(pid); changed=True
    tree=[tab[i] for i in sorted(ids) if i in tab]
    agents=[r for r in tab.values() if any(x in (r["comm"]+" "+r["cmdline"]).lower()
             for x in ("runner.listener","runner.worker","runsvc.sh"))]
    return tree,{"process_tree_count":len(tree),"process_tree":tree,
        "process_tree_cpu_seconds":round(sum(x["cpu_seconds"] for x in tree),3),
        "process_tree_rss_sum_kb":sum(x["rss_kb"] or 0 for x in tree),
        "process_tree_max_hwm_kb":max([x["hwm_kb"] or 0 for x in tree] or [0]),
        "runner_agent_observation":"OBSERVED" if agents else "NOT_OBSERVED",
        "runner_agent_processes":agents[:8]}

def sha(path):
    h=hashlib.sha256()
    try:
        with Path(path).open("rb") as f:
            for chunk in iter(lambda:f.read(1<<20),b""): h.update(chunk)
        return h.hexdigest()
    except OSError: return None

def validate(out):
    rp=out/"result.json"; cp=out/"checkpoint-gen6.json"; cur=out/"checkpoint-current.json"
    missing=[p.name for p in (rp,cp) if not p.is_file()]
    if missing: return {"status":"INCOMPLETE","missing_files":missing,"checkpoint_current_sha256":sha(cur)}
    try: r=json.loads(rp.read_text()); c=json.loads(cp.read_text())
    except Exception as e: return {"status":"INVALID","error":f"{type(e).__name__}: {e}"}
    failures=[]; gens=r.get("generations")
    checks=[(r.get("generation_completed")==6,"generation_completed!=6"),
            (r.get("generation_next")==7,"result.generation_next!=7"),
            (r.get("runtime_sha")==RUNTIME,"runtime_sha_mismatch"),
            (r.get("evaluator_sha")==EVALUATOR,"evaluator_sha_mismatch"),
            (r.get("seed")==SEED,"seed_mismatch"),(r.get("pythonhashseed")=="0","pythonhashseed_mismatch"),
            (r.get("h7_representative_k")=="UNBOUNDED","policy_mismatch"),
            (r.get("trajectory_exit_code")==0,"trajectory_exit_code_not_zero"),
            (r.get("scientific_summary",{}).get("completed_generations")==7,"completed_generations!=7"),
            (r.get("source",{}).get("workflow_run_id")==37828029305,"source_run_mismatch"),
            (r.get("source",{}).get("artifact_id")==11583669251,"source_artifact_mismatch"),
            (r.get("source",{}).get("source_checkpoint_raw_sha256")==SOURCE_CHECKPOINT,"source_checkpoint_mismatch"),
            (r.get("source",{}).get("source_generation_next")==6,"source_boundary_mismatch"),
            (c.get("generation_next")==7,"checkpoint.generation_next!=7"),
            (c.get("seed")==SEED,"checkpoint.seed_mismatch"),
            (isinstance(gens,list) and [g.get("generation") for g in gens if isinstance(g,dict)]==list(range(7)),"generation_history!=0..6")]
    failures.extend(label for ok,label in checks if not ok)
    digest=sha(cp)
    if r.get("final_checkpoint_raw_sha256")!=digest: failures.append("final_checkpoint_raw_sha256_mismatch")
    if c.get("state_digest")!=r.get("final_checkpoint_state_digest"): failures.append("final_state_digest_mismatch")
    if r.get("classification")!="COMPLETED_GEN6": failures.append("result_classification_not_completed_gen6")
    return {"status":"PASSED" if not failures else "INVALID","failures":failures,
        "generation_completed":r.get("generation_completed"),"generation_next":r.get("generation_next"),
        "completed_generations":r.get("scientific_summary",{}).get("completed_generations"),
        "generation_history":[g.get("generation") for g in gens if isinstance(g,dict)] if isinstance(gens,list) else None,
        "checkpoint_generation_next":c.get("generation_next),"checkpoint_sha256":digest,
        "checkpoint_state_digest":c.get("state_digest"),
        "gen6_metrics":gens[-1] if isinstance(gens,list) and gens and isinstance(gens[-1],dict) else None}

def handle_signal(sig, _frame):
    global GOT_SIGNAL; GOT_SIGNAL=sig
    emit("signal_observed",signal_number=sig,signal_name=signal.Signals(sig).name)
    if CHILD is not None and CHILD.poll() is None:
        try: os.killpg(CHILD.pid,sig); emit("signal_forwarded",child_pid=CHILD.pid,signal_name=signal.Signals(sig).name)
        except OSError as e: emit("signal_forward_error",child_pid=CHILD.pid,error=str(e))

def main():
    global LOG,CHILD,LAST_CPU,LAST_MONO
    ap=argparse.ArgumentParser(); ap.add_argument("--log",required=True); ap.add_argument("--evidence-dir",required=True)
    ap.add_argument("--interval-seconds",type=float,default=45); ap.add_argument("command",nargs=argparse.REMAINDER)
    a=ap.parse_args(); cmd=list(a.command)
    if cmd and cmd[0]=="--": cmd=cmd[1:]
    if not cmd or a.interval_seconds<1: ap.error("provide command after -- and interval >= 1 second")
    LOG=Path(a.log); LOG.parent.mkdir(parents=True,exist_ok=True); out=Path(a.evidence_dir); out.mkdir(parents=True,exist_ok=True)
    for sig in (signal.SIGTERM,signal.SIGINT,signal.SIGHUP): signal.signal(sig,handle_signal)
    emit("observer_start",interval_seconds=a.interval_seconds,child_command=cmd,evidence_dir=str(out.resolve()))
    started=time.monotonic()
    try: CHILD=subprocess.Popen(cmd,start_new_session=True)
    except Exception as e: emit("child_launch_failed",error=f"{type(e).__name__}: {e}"); return 127
    emit("child_started",child_pid=CHILD.pid,child_command=cmd); peak_rss=peak_hwm=0; last=time.monotonic()
    while CHILD.poll() is None:
        time.sleep(min(1.0,a.interval_seconds))
        now=time.monotonic()
        if now-last<a.interval_seconds and CHILD.poll() is None: continue
        _,m=sample(CHILD.pid); peak_rss=max(peak_rss,m["process_tree_rss_sum_kb"]); peak_hwm=max(peak_hwm,m["process_tree_max_hwm_kb"])
        cpu=m["process_tree_cpu_seconds"]; util=None
        if LAST_CPU is not None and LAST_MONO is not None and now>LAST_MONO: util=round(max(0,cpu-LAST_CPU)/(now-LAST_MONO)*100,2)
        LAST_CPU,LAST_MONO=cpu,now
        emit("heartbeat",child_pid=CHILD.pid,child_running=CHILD.poll() is None,signal_observed=GOT_SIGNAL,
             observed_peak_tree_rss_sum_kb=peak_rss,observed_peak_process_hwm_kb=peak_hwm,
             cpu_utilization_percent_since_previous=util,**m); last=now
    code=CHILD.wait(); emit("child_exited",child_pid=CHILD.pid,child_exit_status=code,
        child_elapsed_wall_seconds=round(time.monotonic()-started,3),signal_observed=GOT_SIGNAL,
        observed_peak_tree_rss_sum_kb=peak_rss,observed_peak_process_hwm_kb=peak_hwm)
    val=validate(out); emit("terminal_evidence_validation",**val)
    emit("observer_end",child_exit_status=code,signal_observed=GOT_SIGNAL,terminal_validation_status=val.get("status"),
         observed_peak_tree_rss_sum_kb=peak_rss,observed_peak_process_hwm_kb=peak_hwm)
    return 128+GOT_SIGNAL if GOT_SIGNAL is not None and code==0 else code

if __name__=="__main__": raise SystemExit(main())
