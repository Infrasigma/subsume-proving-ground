#!/usr/bin/env python3
import json, os
from pathlib import Path
from lab.acsie_open_ended_self_extending_proving import run_seed

SEED=2026100305
PROVING_SHA="33259fc5c69d87b99c604f0050508c433a279daf"
ACSIE_REF="9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be"

out=Path("evidence")
out.mkdir(exist_ok=True)
cp=out/"B-checkpoint-current.json"
result=run_seed(
    SEED,
    6,
    checkpoint_path=str(cp),
    checkpoint_metadata={"proving_sha":PROVING_SHA,"acsie_ref":ACSIE_REF},
)
summary={
 "schema":"ACSIE.PR110.B.repaired-neutrality.v1",
 "seed":SEED,
 "generations_requested":6,
 "runtime_sha":ACSIE_REF,
 "proving_sha":PROVING_SHA,
 "result":result,
 "checkpoint_bytes":cp.stat().st_size if cp.exists() else None,
}
Path(out/"B-summary.json").write_text(json.dumps(summary,sort_keys=True,indent=2)+"\n")
print(json.dumps(summary,sort_keys=True,indent=2))
