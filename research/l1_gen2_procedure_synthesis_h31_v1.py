from __future__ import annotations
import argparse, copy, json, statistics
from cognitive_core.native_independent_core import NativeCognitiveCore, digest
from research.native_self_learning_kernel_v2 import _make_streams

def score_expr(expr,streams):
    from cognitive_core.native_independent_core import NativeCognitiveCore as C
    vals=[]
    for stream in streams:
        for obs,action,nxt in stream:
            target_path=("z",)
            pred_z=C._relation_eval(expr,obs)
            vals.append(float(pred_z==nxt[target_path[0]]))
    return statistics.mean(vals) if vals else 0.0

def run(seed):
    gen1_inner=_make_streams(seed+1000,6,rule="parity")
    legacy=_make_streams(seed+5000,4,rule="parity")
    gen2_inner=_make_streams(seed+3000,6,rule="sign")
    gen2_outer=_make_streams(seed+4000,6,rule="sign")
    gen1_outer=_make_streams(seed+2000,6,rule="parity")

    core=NativeCognitiveCore(seed=seed)
    d1=core.self_improve_learning(gen1_inner)
    for s in legacy: core.observe_batch(s)

    transition_rows=lambda streams: [
        (obs, nxt) for stream in streams for obs, action, nxt in stream if action=="step"
    ]
    discovery=transition_rows(gen2_inner[:4])
    holdout=transition_rows([gen2_inner[4]])
    transfer=transition_rows([gen2_inner[5]])
    candidate=core.solve_examples_anytime(discovery,holdout,transfer,budget=256)

    if not candidate.verified:
        return {
          "schema":"ACSIE.l1-gen2-procedure-synthesis-h31.v1",
          "scientific_status":"FAILED","seed":seed,
          "solver":candidate.solver,"verified":False,"candidate":candidate.result,
          "failure_boundary":"native procedure portfolio produced no verified candidate on Gen2 inner split"
        }

    expr=None
    path=None
    if candidate.solver=="relation_induction" and isinstance(candidate.result,dict):
        expr=candidate.result.get("expression")
        path=tuple(candidate.result.get("path",()))
    elif candidate.solver=="program_induction":
        # This mechanism screen deliberately records program induction but does not
        # claim relation-expression executable semantics unless relation induction won.
        return {
          "schema":"ACSIE.l1-gen2-procedure-synthesis-h31.v1",
          "scientific_status":"FAILED","seed":seed,
          "solver":"program_induction","verified":True,
          "failure_boundary":"verified program candidate lacks a direct state-transition evaluator in this bank protocol"
        }

    outer=score_expr(expr,gen2_outer)
    parity_outer=score_expr(expr,gen1_outer)
    replay_expr=json.loads(json.dumps(expr))
    replay_outer=score_expr(replay_expr,gen2_outer)
    passed=outer>=0.75 and replay_outer==outer
    return {
      "schema":"ACSIE.l1-gen2-procedure-synthesis-h31.v1",
      "scientific_status":"PASSED" if passed else "FAILED",
      "seed":seed,
      "solver":candidate.solver,
      "candidate_score":candidate.score,
      "candidate_result":candidate.result,
      "verified":candidate.verified,
      "selected_expression":expr,
      "selected_path":path,
      "gen2_outer_accuracy":outer,
      "gen1_outer_expression_accuracy":parity_outer,
      "replay_outer_accuracy":replay_outer,
      "deterministic_replay":replay_outer==outer,
      "integrity":{"holdout_contamination":False,"task_routing":False,"external_model":False,"network_dependency":False,"manual_runtime_strategy":False},
      "scope":"One-seed procedure-synthesis screen; not a four-generation qualification."
    }

if __name__=="__main__":
    p=argparse.ArgumentParser(); p.add_argument("--seed",type=int,default=771221); a=p.parse_args()
    r=run(a.seed); print(json.dumps(r,indent=2,sort_keys=True)); raise SystemExit(0 if r["scientific_status"] in {"PASSED","FAILED"} else 2)
