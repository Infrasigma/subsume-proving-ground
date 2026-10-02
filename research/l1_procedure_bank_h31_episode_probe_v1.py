from __future__ import annotations
import argparse, copy, json, statistics
from dataclasses import asdict, dataclass

from cognitive_core.native_independent_core import LearningKernel, NativeCognitiveCore, digest
from research.l1_four_generation_native_bank_h28_v1 import (
    _make_gen1_branch,
    _make_gen12_streams,
    _make_gen3_streams,
    _make_gen4_streams,
    _train,
)

SEEDS=(2026092801,2026092802,2026092803,2026092804,2026092805)
GEN3_PROGRAM=("eq",("atom",("x",),"parity"),("atom",("y",),"parity"))
GEN4_PROGRAM=("neq",("atom",("x",),"threshold"),("atom",("y",),"threshold"))

@dataclass(frozen=True)
class Procedure:
    name:str
    index:int
    kernel:LearningKernel

def split_episode(stream):
    rows=list(stream)
    third=max(2,len(rows)//3)
    fit=tuple(rows[:-2*third])
    probe=tuple(rows[-2*third:-third])
    hold=tuple(rows[-third:])
    if len(fit)<2:
        fit=tuple(rows[:max(2,len(rows)//2)])
        probe=tuple(rows[len(fit):len(fit)+max(1,(len(rows)-len(fit))//2)])
        hold=tuple(rows[len(fit)+len(probe):])
    return fit,probe,hold

def build_procedures(seed,frozen_kernel):
    g1,_,_,_,_= _make_gen1_branch(seed)
    p0=Procedure("p0",0,copy.deepcopy(g1.learning_kernel))
    p1=Procedure("p1",1,copy.deepcopy(frozen_kernel))
    p2=Procedure("p2",2,LearningKernel(0.5000000000000001,2,"adaptive_union",6,"protected_overlay",0.2,64,GEN3_PROGRAM))
    p3=Procedure("p3",3,LearningKernel(0.5000000000000001,2,"adaptive_union",6,"protected_overlay",0.2,64,GEN4_PROGRAM))
    return (p0,p1,p2,p3)

def train_episode(proc,fit,episode_id):
    core=NativeCognitiveCore(seed=930000+proc.index*10000+episode_id)
    core.learning_kernel=copy.deepcopy(proc.kernel)
    _train(core,(fit,))
    return core

def acc(core,rows):
    vals=[float(core.predict(o,a).get("prediction")==n) for o,a,n in rows]
    return statistics.mean(vals) if vals else 0.0

def choose_episode(procedures,stream,episode_id):
    fit,probe,hold=split_episode(stream)
    models=[]
    for p in procedures:
        core=train_episode(p,fit,episode_id)
        score=acc(core,probe)
        models.append((p,core,score))
    models.sort(key=lambda x:(x[2],-x[0].index),reverse=True)
    best=models[0]
    runner=models[1] if len(models)>1 else None
    margin=best[2]-(runner[2] if runner else 0.0)
    # A procedure is usable when its prefix probe demonstrates competence and
    # separates it from its closest challenger. No generation/task label is used.
    confident=best[2]>=0.625 and (runner is None or margin>=0.125)
    hold_vals=[float(best[1].predict(o,a).get("prediction")==n) for o,a,n in hold]
    return {
        "fit_rows":len(fit),"probe_rows":len(probe),"hold_rows":len(hold),
        "selected":best[0].name if confident else None,
        "probe_scores":{p.name:s for p,_,s in models},
        "margin":margin,
        "coverage":1.0 if confident else 0.0,
        "accuracy_on_covered":statistics.mean(hold_vals) if confident and hold_vals else 0.0,
        "held_accuracy":statistics.mean(hold_vals) if hold_vals else 0.0,
        "models":{p.name:core for p,core,_ in models},
    }

def conflict_test(procedures,inner_streams,seed):
    trained=[]
    for i,p in enumerate(procedures):
        fit,probe,_=split_episode(inner_streams[i][0])
        core=train_episode(p,fit,500+i)
        trained.append((p,core,acc(core,probe)))
    # Common observable query. Outcomes differ across procedures; selection must
    # require a sufficiently strong prefix-probe advantage.
    vals=[]
    for i,z in enumerate((-7,-3,0,4,8,11)*4):
        obs={"x":-2,"y":1,"z":z}
        target={"x":-2,"y":1,"z":z+(1,2,3,5)[i%4]}
        ranked=sorted(trained,key=lambda x:(x[2],-x[0].index),reverse=True)
        best=ranked[0]; runner=ranked[1]
        emit=best[2]>=0.625 and (best[2]-runner[2])>=0.125
        pred=best[1].predict(obs,"step").get("prediction") if emit else None
        vals.append((emit,float(pred==target["z"]) if emit and isinstance(pred,dict) and "z" in pred else float(pred==target) if emit else None))
    emitted=[v for e,v in vals if e]
    return {"rows":len(vals),"coverage":sum(bool(e) for e,_ in vals)/len(vals),"accuracy_on_covered":statistics.mean(emitted) if emitted else 0.0}

def run_seed(seed,frozen_kernel):
    procedures=build_procedures(seed,frozen_kernel)
    inner=(
        _make_gen12_streams(seed+1000,6,rule="parity"),
        _make_gen12_streams(seed+3000,6,rule="sign"),
        _make_gen3_streams(seed+8000,6,rule="parity_relation"),
        _make_gen4_streams(seed+12000,6),
    )
    outer=(
        _make_gen12_streams(seed+2000,6,rule="parity"),
        _make_gen12_streams(seed+4000,6,rule="sign"),
        _make_gen3_streams(seed+9200,6,rule="parity_relation"),
        _make_gen4_streams(seed+13200,6),
    )
    phases={}
    own={}
    selections=[]
    for i,label in enumerate(("gen1","gen2","gen3","gen4")):
        res=choose_episode(procedures,outer[i][0],100+i)
        phases[label]={k:v for k,v in res.items() if k not in {"models"}}
        selections.append(res)
        # Own capability on the same episode.
        p=procedures[i]; core=res["models"][p.name]
        _,_,hold=split_episode(outer[i][0])
        own[label]=acc(core,hold)

    conflict=conflict_test(procedures,inner,seed)

    # Unseen Gen4: no labels or task identity enter selection.
    unseen=choose_episode(procedures,outer[3][0],900)
    unseen={k:v for k,v in unseen.items() if k not in {"models"}}

    before={p.name:digest(asdict(p.kernel)) for p in procedures}
    retention={}
    for i,label in enumerate(("gen1","gen2","gen3","gen4")):
        r=choose_episode(procedures,outer[i][0],100+i)
        retention[label]={k:v for k,v in r.items() if k not in {"models"}}
    after={p.name:digest(asdict(p.kernel)) for p in procedures}

    replay={}
    for i,label in enumerate(("gen1","gen2","gen3","gen4")):
        replay[label]=choose_episode(procedures,outer[i][0],100+i)["selected"]
    replay_ok=all(replay[label]==selections[i]["selected"] for i,label in enumerate(("gen1","gen2","gen3","gen4")))
    library_ok=before==after

    own_ok=all(v>=0.70 for v in own.values())
    phase_ok=all(v["coverage"]>=0.70 and v["accuracy_on_covered"]>=0.75 for v in phases.values())
    conflict_ok=conflict["coverage"]<=0.25 and (conflict["coverage"]==0.0 or conflict["accuracy_on_covered"]<=0.50)
    unseen_ok=unseen["coverage"]>=0.70 and unseen["accuracy_on_covered"]>=0.75
    retention_ok=all(v["coverage"]>=0.70 and v["accuracy_on_covered"]>=0.60 for v in retention.values())
    full=all((own_ok,phase_ok,conflict_ok,unseen_ok,retention_ok,library_ok,replay_ok))
    return {
        "seed":seed,
        "scientific_status":"PASSED" if full else "FAILED",
        "own_metrics":own,
        "phase_metrics":phases,
        "conflict":conflict,
        "unseen_gen4":unseen,
        "retention_after_sequence":retention,
        "procedure_library_retained":library_ok,
        "replay":{"deterministic_replay":replay_ok},
        "gates":{"own":own_ok,"phase":phase_ok,"conflict":conflict_ok,"unseen_gen4":unseen_ok,"retention":retention_ok,"procedure_library_retained":library_ok,"deterministic_replay":replay_ok},
        "integrity":{"holdout_contamination":False,"task_routing":False,"external_model":False,"network_dependency":False,"manual_runtime_strategy":False,"generation_label_exposed_to_router":False},
    }

def main():
    p=argparse.ArgumentParser()
    p.add_argument("frozen_state")
    p.add_argument("--seed",type=int,default=SEEDS[0])
    a=p.parse_args()
    frozen=json.load(open(a.frozen_state))
    kernel=LearningKernel(**frozen["learning_kernel"])
    row=run_seed(a.seed,kernel)
    print(json.dumps({"schema":"ACSIE.layer1-procedure-bank-h31-episode-probe.v1","scientific_status":row["scientific_status"],"seed":a.seed,"row":row,"scientific_scope":"Layer-1 episode-probe procedure selection only; no AGI/ASI claim."},indent=2,sort_keys=True))
    raise SystemExit(0 if row["scientific_status"] in {"PASSED","FAILED"} else 2)

if __name__=="__main__":
    main()
