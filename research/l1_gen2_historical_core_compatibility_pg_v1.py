from __future__ import annotations
import argparse, hashlib, json, statistics
from pathlib import Path
from cognitive_core.native_independent_core import NativeCognitiveCore, digest
from research.native_self_learning_kernel_gen2_corrected_retention_v1 import run as historical_gen2_run
try:
    from research.native_self_learning_kernel_v2 import _make_streams
except Exception:
    from research.native_self_learning_kernel_gen2_corrected_retention_v1 import _make_streams

CANONICAL_SEED=771221
GEN2_PARENT="b31109864ae547320d877e6c1575b805d950edf9"
GEN2_STATE_SHA256="0b599e436c4578115cc0312dd6545f5d56c14777fcb55562592c5d8f4e8f80a9"

def score(core,streams):
    vals=[]
    for stream in streams:
        for obs,action,nxt in stream:
            vals.append(float(core.predict(obs,action).get("prediction")==nxt))
    return {"accuracy":statistics.mean(vals) if vals else 0.0,"rows":len(vals)}

def run(export_path,seed,source_commit):
    export=json.loads(Path(export_path).read_text())
    state_path=Path(export_path).with_name("verified-gen2-state.json")
    state=export["frozen_state"]
    if export.get("scientific_status")!="PASSED": return {"scientific_status":"BLOCKED","reason":"export_not_passed"}
    if export.get("scientific_parent_commit")!=GEN2_PARENT: return {"scientific_status":"BLOCKED","reason":"wrong_parent"}
    if hashlib.sha256(state_path.read_bytes()).hexdigest()!=GEN2_STATE_SHA256: return {"scientific_status":"BLOCKED","reason":"wrong_state_sha256"}
    core=NativeCognitiveCore.from_state(state)
    roundtrip=core.export_state()
    return {
      "historical_reexecution": historical_gen2_run(seed=771221),
      "schema":"ACSIE.l1-gen2-historical-core-compatibility.v1",
      "scientific_status":"COMPLETED",
      "seed":seed,
      "runtime_core_source":source_commit,
      "state":{"raw_digest":digest(state),"roundtrip_digest":digest(roundtrip),"raw_equal":digest(state)==digest(roundtrip)},
      "canonical_outer":score(core,_make_streams(CANONICAL_SEED+4000,6,rule="sign")),
      "current_outer":score(core,_make_streams(seed+4000,6,rule="sign")),
      "scope":"Historical core compatibility diagnostic; not a four-generation gate."
    }

if __name__=="__main__":
    p=argparse.ArgumentParser(); p.add_argument("export_path"); p.add_argument("--seed",type=int,required=True); p.add_argument("--source-commit",required=True); a=p.parse_args()
    print(json.dumps(run(a.export_path,a.seed,a.source_commit),indent=2,sort_keys=True))
