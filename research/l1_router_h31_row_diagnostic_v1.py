from __future__ import annotations
import copy, json, random
from cognitive_core.native_independent_core import NativeCognitiveCore
from research.l1_independent_procedure_bank_h30_v1 import (
    Procedure, ProcedureBank, _make_gen3_kernel, _make_gen4_kernel,
    _make_gen1_branch, _make_gen12_streams, _make_gen3_streams, _make_gen4_streams,
)

SEED=2026092801

def make_bank(seed):
    g1_core,g1_inner,g1_outer,_,_= _make_gen1_branch(seed)
    g2i=_make_gen12_streams(seed+3000,6,rule="sign")
    g2o=_make_gen12_streams(seed+4000,6,rule="sign")
    g3i=_make_gen3_streams(seed+8000,6,rule="parity_relation")
    g3o=_make_gen3_streams(seed+9200,6,rule="parity_relation")
    g4i=_make_gen4_streams(seed+12000,6)
    g4o=_make_gen4_streams(seed+13200,6)
    # Use the exact kernels used by H30.
    p0=Procedure("p0",0,copy.deepcopy(g1_core.learning_kernel))
    from cognitive_core.native_independent_core import LearningKernel
    p1=Procedure("p1",1,LearningKernel(0.5000000000000001,2,"adaptive_union",6,"ensemble",0.2,64))
    p2=Procedure("p2",2,_make_gen3_kernel())
    p3=Procedure("p3",3,_make_gen4_kernel())
    return ProcedureBank((p0,p1,p2,p3)), ((g1i,g2i,g3i,g4i) if False else (g1_inner,g2i,g3i,g4i)), (g1_outer,g2o,g3o,g4o)

def rank_for(bank, items):
    ranked=[]
    for x in items:
        rec=bank.stats[x["procedure"]].get(x["evidence_key"], bank.global_stats[x["procedure"]])
        support=float(rec[1]); score=(float(rec[0])+1.0)/(support+2.0)
        ranked.append({**x,"support":support,"score":score})
    return sorted([x for x in ranked if x["prediction"] is not None], key=lambda x:(x["score"],x["support"],-x["index"]), reverse=True)

def main():
    bank,inners,outers=make_bank(SEED)
    inner_models={p.name:[bank._train_episode(p,stream,i) for i,stream in enumerate(inners[p.index])] for p in bank.procedures}
    bank.calibrate(inner_models)
    # Phase rows: dump all procedure predictions/support/score and selected outcome.
    phase_dump={}
    for gi,label in enumerate(("gen1","gen2","gen3","gen4")):
        models={p.name:bank._train_episode(p,outers[p.index][gi],100+gi) for p in bank.procedures}
        rows=[]
        for row_i in range(len(models["p0"].holdout)):
            items=[]
            for p in bank.procedures:
                item=bank.candidates(models[p.name])[row_i]
                items.append(item)
            ranked=rank_for(bank,items)
            picked=bank.choose(items,feedback=False)
            rows.append({"row":row_i,"target":items[0]["target"],"ranked":ranked,"picked":picked})
        phase_dump[label]=rows
    # Exact conflict dump.
    trained={p.name:inner_models[p.name][0].core for p in bank.procedures}
    rng=random.Random(SEED); deltas=(1,2,3,5)
    conflicts=[]
    for i in range(24):
        z=rng.randint(-12,12); obs={"x":-2,"y":1,"z":z}; target={"x":-2,"y":1,"z":z+deltas[i%4]}
        items=[]
        for p in bank.procedures:
            pred=trained[p.name].predict(obs,"step").get("prediction")
            from research.l1_independent_procedure_bank_h30_v1 import _evidence_key
            key=_evidence_key(trained[p.name],obs,"step",pred)
            items.append({"procedure":p.name,"index":p.index,"prediction":pred,"evidence_key":key,"target":target})
        conflicts.append({"i":i,"obs":obs,"target":target,"ranked":rank_for(bank,items),"picked":bank.choose(items,feedback=False)})
    print(json.dumps({"schema":"H31.router-row-diagnostic.v1","seed":SEED,"phase_dump":phase_dump,"conflicts":conflicts},indent=2,sort_keys=True))

if __name__=="__main__":
    main()
