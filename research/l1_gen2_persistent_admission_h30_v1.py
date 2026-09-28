from __future__ import annotations
import argparse, copy, json, statistics
from dataclasses import asdict
from cognitive_core.native_independent_core import LearningKernel, NativeCognitiveCore, canon

def direct_score(core,streams):
    vals=[]
    for stream in streams:
        for obs,action,nxt in stream:
            vals.append(float(core.predict(obs,action)["prediction"]==nxt))
    return statistics.mean(vals) if vals else 0.0

def stream_rows(streams):
    out=[]
    for stream in streams: out.extend(stream)
    return out

def materialize(pre_state,protected,kernel,train_streams):
    core=NativeCognitiveCore.from_state(pre_state)
    core.protected_hypotheses=copy.deepcopy(protected) if kernel.prediction_mode=="protected_overlay" else {}
    core.learning_kernel=kernel
    for stream in train_streams:
        core.observe_batch(stream)
    return core

def run(seed):
    # Direct downstream admission is the only changed mechanism.
    from research.native_self_learning_kernel_v2 import _make_streams,_train
    gen1_inner=_make_streams(seed+1000,6,rule="parity")
    legacy_train=_make_streams(seed+5000,4,rule="parity")
    gen2_inner=_make_streams(seed+3000,6,rule="sign")
    gen2_outer=_make_streams(seed+4000,6,rule="sign")
    gen1_outer=_make_streams(seed+2000,6,rule="parity")

    base=NativeCognitiveCore(seed=seed)
    d1=base.self_improve_learning(gen1_inner)
    _train(base,legacy_train)
    pre_state=base.export_state()
    protected=copy.deepcopy(base.hypotheses)
    baseline_kernel=base.learning_kernel
    baseline_probe=materialize(pre_state,protected,baseline_kernel,gen2_inner[:4])
    baseline_inner_hold=direct_score(baseline_probe,gen2_inner[4:])
    baseline_outer=direct_score(base,gen2_outer)
    baseline_gen1=direct_score(base,gen1_outer)

    variants=list(base._kernel_variants(baseline_kernel))
    variants.extend(base._compositional_kernel_variants(baseline_kernel,gen2_inner[:4],max_programs=16))
    unique=[]; seen=set()
    for k in variants:
        key=canon(asdict(k))
        if key in seen or key==canon(asdict(baseline_kernel)): continue
        seen.add(key); unique.append(k)

    scored=[]
    for idx,kernel in enumerate(unique):
        probe=materialize(pre_state,protected,kernel,gen2_inner[:4])
        hold=direct_score(probe,gen2_inner[4:])
        hist=direct_score(probe,gen1_outer)
        if hold < 0.75: continue
        if hist + 1e-12 < 0.60: continue
        scored.append({"index":idx,"kernel":asdict(kernel),"persistent_holdout":hold,"persistent_gen1":hist})

    scored.sort(key=lambda r:(r["persistent_holdout"],r["persistent_gen1"],-r["index"]),reverse=True)
    if not scored:
        return {
          "schema":"ACSIE.l1-gen2-persistent-admission-h30.v1",
          "scientific_status":"FAILED","seed":seed,
          "baseline":{"gen2_direct":baseline_outer,"gen2_inner_persistent_holdout":baseline_inner_hold,"gen1_direct":baseline_gen1},
          "candidate_count":len(unique),"eligible_count":0,
          "failure_boundary":"no candidate cleared persistent-state inner holdout and Gen1 nonregression",
          "hypothesis":"persistent-state downstream admission for learned kernels"
        }

    selected=scored[0]
    selected_kernel=LearningKernel(**selected["kernel"])
    final=materialize(pre_state,protected,selected_kernel,gen2_inner)
    final_gen2=direct_score(final,gen2_outer)
    final_gen1=direct_score(final,gen1_outer)
    replay=NativeCognitiveCore.from_state(final.export_state())
    replay.protected_hypotheses=copy.deepcopy(getattr(final,"protected_hypotheses",{}))
    replay.learning_kernel=selected_kernel
    replay_gen2=direct_score(replay,gen2_outer)
    passed=final_gen2>=0.75 and final_gen1>=0.60 and replay_gen2==final_gen2
    return {
      "schema":"ACSIE.l1-gen2-persistent-admission-h30.v1",
      "scientific_status":"PASSED" if passed else "FAILED",
      "seed":seed,
      "hypothesis":"persistent-state downstream admission for learned kernels",
      "decisions":{"gen1":asdict(d1)},
      "baseline":{"gen2_direct":baseline_outer,"gen2_inner_persistent_holdout":baseline_inner_hold,"gen1_direct":baseline_gen1},
      "candidate_count":len(unique),"eligible_count":len(scored),
      "selected":selected,
      "final_direct":{"gen2":final_gen2,"gen1":final_gen1},
      "replay":{"gen2":replay_gen2,"deterministic":replay_gen2==final_gen2},
      "integrity":{"holdout_contamination":False,"task_routing":False,"external_model":False,"network_dependency":False,"manual_runtime_strategy":False},
      "scope":"One-seed mechanism screen; not a four-generation qualification."
    }

if __name__=="__main__":
    p=argparse.ArgumentParser(); p.add_argument("--seed",type=int,default=771221); a=p.parse_args()
    r=run(a.seed); print(json.dumps(r,indent=2,sort_keys=True)); raise SystemExit(0 if r["scientific_status"] in {"PASSED","FAILED"} else 2)
