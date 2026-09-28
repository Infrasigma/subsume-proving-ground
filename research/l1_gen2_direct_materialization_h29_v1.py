from __future__ import annotations
import argparse, copy, json, statistics
from dataclasses import asdict
from cognitive_core.native_independent_core import LearningKernel, NativeCognitiveCore, digest
from research.native_self_learning_kernel_v2 import _make_streams, _train

def direct_score(core, streams):
    vals=[]
    for stream in streams:
        for obs, action, nxt in stream:
            info=core.predict(obs, action)
            vals.append(float(info.get("prediction")==nxt))
    return statistics.mean(vals) if vals else 0.0

def run(seed):
    gen1_inner=_make_streams(seed+1000,6,rule="parity")
    legacy_train=_make_streams(seed+5000,4,rule="parity")
    gen2_inner=_make_streams(seed+3000,6,rule="sign")
    gen2_outer=_make_streams(seed+4000,6,rule="sign")
    gen1_outer=_make_streams(seed+2000,6,rule="parity")

    base=NativeCognitiveCore(seed=seed)
    d1=base.self_improve_learning(gen1_inner)
    _train(base,legacy_train)
    protected=copy.deepcopy(base.hypotheses)
    pre_gen2_state=base.export_state()

    d2=base.self_improve_learning(gen2_inner)
    _train(base,gen2_inner)
    default_gen2=direct_score(base,gen2_outer)
    default_gen1=direct_score(base,gen1_outer)

    # Native candidate discovery: no hand-coded context/task/generation mapping.
    discovery=gen2_inner[:4]
    selection=gen2_inner[4:]
    programs=NativeCognitiveCore._context_programs_from_streams(discovery,max_programs=32)
    candidates=[]
    for index,program in enumerate(programs):
        candidate=LearningKernel(
            evidence_threshold=base.learning_kernel.evidence_threshold,
            min_support_count=base.learning_kernel.min_support_count,
            context_mode="adaptive_union",
            delayed_window=base.learning_kernel.delayed_window,
            prediction_mode="protected_overlay",
            contradiction_margin=base.learning_kernel.contradiction_margin,
            replay_limit=base.learning_kernel.replay_limit,
            context_program=program,
        )
        score=NativeCognitiveCore._evaluate_kernel(
            candidate,
            selection,
            seed=83000,
            include_retention=False,
            protected_hypotheses=protected,
        )
        candidates.append({"index":index,"program":program,"score":score})
    candidates.sort(key=lambda r:(float(r["score"][1]),float(r["score"][2]),float(r["score"][0]),-r["index"]),reverse=True)

    if not candidates:
        return {
            "schema":"ACSIE.l1-gen2-direct-materialization-h29.v1",
            "scientific_status":"FAILED",
            "seed":seed,
            "failure_boundary":"native context-program discovery produced no candidates",
        }

    selected=candidates[0]
    selected_kernel=LearningKernel(
        evidence_threshold=base.learning_kernel.evidence_threshold,
        min_support_count=base.learning_kernel.min_support_count,
        context_mode="adaptive_union",
        delayed_window=base.learning_kernel.delayed_window,
        prediction_mode="protected_overlay",
        contradiction_margin=base.learning_kernel.contradiction_margin,
        replay_limit=base.learning_kernel.replay_limit,
        context_program=selected["program"],
    )
    materialized=NativeCognitiveCore.from_state(pre_gen2_state)
    materialized.protected_hypotheses=copy.deepcopy(protected)
    materialized.learning_kernel=selected_kernel
    _train(materialized,gen2_inner)
    materialized_gen2=direct_score(materialized,gen2_outer)
    materialized_gen1=direct_score(materialized,gen1_outer)

    replay=NativeCognitiveCore.from_state(materialized.export_state())
    replay.protected_hypotheses=copy.deepcopy(getattr(materialized,"protected_hypotheses",{}))
    replay.learning_kernel=selected_kernel
    replay_gen2=direct_score(replay,gen2_outer)

    result={
      "schema":"ACSIE.l1-gen2-direct-materialization-h29.v1",
      "scientific_status":"PASSED" if materialized_gen2>=0.75 and materialized_gen1>=0.60 and replay_gen2==materialized_gen2 else "FAILED",
      "seed":seed,
      "hypothesis":"Native context-program discovery plus protected-overlay arbitration can materialize a learned Gen2 learning kernel into a directly reusable procedure.",
      "decisions":{"gen1":asdict(d1),"gen2":asdict(d2)},
      "baseline_direct":{"gen2":default_gen2,"gen1":default_gen1},
      "candidate_count":len(candidates),
      "selected":{"index":selected["index"],"program":selected["program"],"selection_score":selected["score"],"kernel":asdict(selected_kernel)},
      "materialized_direct":{"gen2":materialized_gen2,"gen1":materialized_gen1},
      "replay":{"gen2":replay_gen2,"deterministic":replay_gen2==materialized_gen2},
      "integrity":{"holdout_contamination":False,"task_routing":False,"external_model":False,"network_dependency":False,"manual_runtime_strategy":False},
      "scope":"One-seed mechanism screen only; not a four-generation qualification."
    }
    return result

if __name__=="__main__":
    p=argparse.ArgumentParser(); p.add_argument("--seed",type=int,default=771221); a=p.parse_args()
    r=run(a.seed); print(json.dumps(r,indent=2,sort_keys=True)); raise SystemExit(0 if r["scientific_status"] in {"PASSED","FAILED"} else 2)
