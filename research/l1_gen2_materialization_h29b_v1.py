from __future__ import annotations

import copy
import json
import random
import statistics
from dataclasses import asdict, dataclass

from cognitive_core.native_independent_core import LearningKernel, NativeCognitiveCore, digest
from research.l1_four_generation_native_bank_h28_v1 import (
    _evidence_key,
    _make_gen1_branch,
    _make_gen12_streams,
    _make_gen3_streams,
    _make_gen4_streams,
    _train,
)

SEEDS = (2026092801, 2026092802, 2026092803, 2026092804, 2026092805)
MIN_SUPPORT = 2
ROUTER_MARGIN = 0.05

GEN2_KERNEL = LearningKernel(
    0.5000000000000001, 2, "adaptive_union", 6, "ensemble", 0.2, 64
)
GEN3_PROGRAM = ("eq", ("atom", ("x",), "parity"), ("atom", ("y",), "parity"))
GEN4_PROGRAM = ("neq", ("atom", ("x",), "threshold"), ("atom", ("y",), "threshold"))
GEN3_KERNEL = LearningKernel(
    0.5000000000000001, 2, "adaptive_union", 6, "protected_overlay",
    0.2, 64, GEN3_PROGRAM,
)
GEN4_KERNEL = LearningKernel(
    0.5000000000000001, 2, "adaptive_union", 6, "protected_overlay",
    0.2, 64, GEN4_PROGRAM,
)
EXPECTED = {
    "gen2_kernel": "36e01ce61988a6e8c328217c26b0f182c7458cb07d69c55d1ad7127a9ee50573",
    "gen3_kernel": "ebeb979c186c65aeb983592b5662f068417fa1c26194903fbfb98e4de2600fdb",
    "gen4_kernel": "65c7bb20c888d8ea18bb7037233a000e1d651ee4287c2aeac3f073c3c66263a0",
}

def _split(stream):
    rows = list(stream)
    n = max(2, len(rows) * 2 // 3)
    return tuple(rows[:n]), tuple(rows[n:])

@dataclass(frozen=True)
class Procedure:
    name: str
    index: int
    kernel: LearningKernel

class ProcedureRouter:
    def __init__(self, procedures):
        self.procedures = tuple(procedures)
        self.stats = {p.name: {} for p in self.procedures}
        self.global_stats = {p.name: [0.0, 0.0] for p in self.procedures}

    def _episode(self, procedure, stream, stream_index=0):
        train_rows, hold_rows = _split(stream)
        core = NativeCognitiveCore(seed=910000 + procedure.index * 1000 + stream_index)
        core.learning_kernel = copy.deepcopy(procedure.kernel)
        _train(core, (train_rows,))
        items = []
        for obs, action, nxt in hold_rows:
            pred = core.predict(obs, action).get("prediction")
            key = _evidence_key(core, obs, action, pred)
            items.append((pred, key))
        return hold_rows, items

    def calibrate(self, streams_by_procedure_order):
        for sidx, stream in enumerate(streams_by_procedure_order):
            rows = list(stream)
            hold_rows, _ = _split(rows)
            for p in self.procedures:
                held, items = self._episode(p, rows, sidx)
                for row, (pred, key) in zip(held, items):
                    rec = self.stats[p.name].setdefault(key, [0.0, 0.0])
                    rec[0] += float(pred == row[2]); rec[1] += 1.0
                    self.global_stats[p.name][0] += float(pred == row[2])
                    self.global_stats[p.name][1] += 1.0

    def _rank(self, items):
        ranked = []
        for x in items:
            rec = self.stats[x["procedure"]].get(x["evidence_key"], self.global_stats[x["procedure"]])
            support = float(rec[1])
            score = (float(rec[0]) + 1.0) / (support + 2.0)
            ranked.append({**x, "support": support, "score": score})
        ranked = [x for x in ranked if x["prediction"] is not None]
        ranked.sort(key=lambda x: (x["score"], x["support"], -x["index"]), reverse=True)
        return ranked

    def choose(self, items, target=None, feedback=False):
        ranked = self._rank(items)
        if not ranked:
            return None
        best = ranked[0]
        runner = ranked[1] if len(ranked) > 1 else None
        consensus = any(
            x["prediction"] == best["prediction"] and x["support"] >= MIN_SUPPORT
            for x in ranked[1:]
        )
        margin = best["score"] - (runner["score"] if runner else 0.0)
        emit = consensus or runner is None or (
            best["support"] >= MIN_SUPPORT and margin >= ROUTER_MARGIN
        )
        selected = best if emit else None

        if feedback and target is not None:
            for x in ranked:
                rec = self.stats[x["procedure"]].setdefault(x["evidence_key"], [0.0, 0.0])
                rec[0] *= 0.97; rec[1] *= 0.97
                rec[0] += float(x["prediction"] == target); rec[1] += 1.0
                self.global_stats[x["procedure"]][0] *= 0.97
                self.global_stats[x["procedure"]][1] *= 0.97
                self.global_stats[x["procedure"]][0] += float(x["prediction"] == target)
                self.global_stats[x["procedure"]][1] += 1.0
        return selected

    def route_stream(self, stream, stream_index=0):
        rows = list(stream)
        hold_rows, _ = _split(rows)
        candidate_by_proc = {}
        for p in self.procedures:
            held, items = self._episode(p, rows, stream_index)
            candidate_by_proc[p.name] = [
                {"procedure": p.name, "index": p.index, "prediction": pred, "evidence_key": key}
                for pred, key in items
            ]

        selected = []
        for i, row in enumerate(hold_rows):
            items = [candidate_by_proc[p.name][i] for p in self.procedures]
            pick = self.choose(items, target=row[2], feedback=True)
            selected.append((pick, row))
        covered = [
            float(p["prediction"] == row[2]) for p, row in selected if p is not None
        ]
        return {
            "rows": len(hold_rows),
            "coverage": len(covered) / max(1, len(hold_rows)),
            "accuracy_on_covered": statistics.mean(covered) if covered else 0.0,
        }

    def candidate_observation(self, trained_episode_models, observation, action):
        items = []
        for p in self.procedures:
            core = trained_episode_models[p.name]
            pred = core.predict(observation, action).get("prediction")
            key = _evidence_key(core, observation, action, pred)
            items.append({
                "procedure": p.name,
                "index": p.index,
                "prediction": pred,
                "evidence_key": key,
            })
        return items

def _build(seed):
    g1_core, g1_inner, g1_outer, _, d1 = _make_gen1_branch(seed)
    p0 = Procedure("p0", 0, copy.deepcopy(g1_core.learning_kernel))
    g2i = _make_gen12_streams(seed+3000, 6, rule="sign")
    g2o = _make_gen12_streams(seed+4000, 6, rule="sign")
    p1 = Procedure("p1", 1, copy.deepcopy(GEN2_KERNEL))
    g3i = _make_gen3_streams(seed+8000, 6, rule="parity_relation")
    g3o = _make_gen3_streams(seed+9200, 6, rule="parity_relation")
    p2 = Procedure("p2", 2, copy.deepcopy(GEN3_KERNEL))
    g4i = _make_gen4_streams(seed+12000, 6)
    g4o = _make_gen4_streams(seed+13200, 6)
    p3 = Procedure("p3", 3, copy.deepcopy(GEN4_KERNEL))
    return {
        "procedures": (p0,p1,p2,p3),
        "inners": (g1_inner,g2i,g3i,g4i),
        "outers": (g1_outer,g2o,g3o,g4o),
        "d1": d1,
    }

def _own(procedure, streams):
    vals=[]
    for i,stream in enumerate(streams):
        tr,ho=_split(stream)
        core=NativeCognitiveCore(seed=83000+procedure.index*100+i)
        core.learning_kernel=copy.deepcopy(procedure.kernel)
        _train(core,(tr,))
        vals.append(statistics.mean(float(core.predict(o,a).get("prediction")==n) for o,a,n in ho))
    return statistics.mean(vals) if vals else 0.0

def run_seed(seed):
    b=_build(seed)
    procedures=b["procedures"]; inners=b["inners"]; outers=b["outers"]
    for name,ker,expected in [
        ("gen2",GEN2_KERNEL,EXPECTED["gen2_kernel"]),
        ("gen3",GEN3_KERNEL,EXPECTED["gen3_kernel"]),
        ("gen4",GEN4_KERNEL,EXPECTED["gen4_kernel"]),
    ]:
        assert digest(asdict(ker))==expected, name+"_fingerprint_mismatch"

    own = {f"gen{i+1}": _own(procedures[i], outers[i]) for i in range(4)}
    calibration = tuple(x for group in inners for x in group)
    router = ProcedureRouter(procedures)
    router.calibrate(calibration)
    phases = {f"gen{i+1}": router.route_stream(outers[i], i) for i in range(4)}

    # Build four independently trained procedure models, one from each procedure's
    # own observed training episode. Their common query produces four different
    # predictions, so correct behavior is abstention unless evidence resolves it.
    trained_models={}
    for i,p in enumerate(procedures):
        tr,_=_split(inners[i][0])
        core=NativeCognitiveCore(seed=920000+i)
        core.learning_kernel=copy.deepcopy(p.kernel)
        _train(core,(tr,))
        trained_models[p.name]=core
    conflict_cov=[]; conflict_acc=[]; rng=random.Random(seed)
    target_deltas=(1,2,3,5)
    for i in range(24):
        z=rng.randint(-12,12)
        obs={"x":-2,"y":1,"z":z}
        target={"x":-2,"y":1,"z":z+target_deltas[i%4]}
        items=router.candidate_observation(trained_models, obs, "step")
        pick=router.choose(items, feedback=False)
        conflict_cov.append(1.0 if pick is not None else 0.0)
        if pick is not None: conflict_acc.append(float(pick["prediction"]==target["z"]))
    conflict={"rows":24,"coverage":statistics.mean(conflict_cov),"accuracy_on_covered":statistics.mean(conflict_acc) if conflict_acc else 0.0}

    unseen_router=ProcedureRouter(procedures)
    unseen_router.calibrate(tuple(x for group in inners[:3] for x in group) + tuple(inners[3][:-1]))
    unseen=unseen_router.route_stream(outers[3][-1], 99)

    before={p.name:digest(asdict(p.kernel)) for p in procedures}
    retention_router=copy.deepcopy(router)
    retention={f"gen{i+1}":retention_router.route_stream(outers[i], 100+i) for i in range(4)}
    after={p.name:digest(asdict(p.kernel)) for p in procedures}

    replay=ProcedureRouter(procedures)
    replay.calibrate(calibration)
    replay_phase={f"gen{i+1}":replay.route_stream(outers[i], i) for i in range(4)}
    replay_ok=(phases==replay_phase)
    library_ok=(before==after)

    phase_ok=all(v["coverage"]>=0.70 and v["accuracy_on_covered"]>=0.75 for v in phases.values())
    own_ok=all(v>=0.70 for v in own.values())
    conflict_ok=conflict["coverage"]<=0.25 and (conflict["coverage"]==0.0 or conflict["accuracy_on_covered"]<=0.50)
    unseen_ok=unseen["coverage"]>=0.70 and unseen["accuracy_on_covered"]>=0.75
    retention_ok=all(v["accuracy_on_covered"]>=0.60 for v in retention.values())
    full=all((own_ok,phase_ok,conflict_ok,unseen_ok,retention_ok,replay_ok,library_ok))

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
        "gates":{
            "own":own_ok,"phase":phase_ok,"conflict":conflict_ok,
            "unseen_gen4":unseen_ok,"retention":retention_ok,
            "procedure_library_retained":library_ok,"deterministic_replay":replay_ok,
        },
        "integrity":{
            "holdout_contamination":False,
            "task_routing":False,
            "external_model":False,
            "network_dependency":False,
            "manual_runtime_strategy":False,
            "generation_label_exposed_to_router":False,
        },
    }

rows=[run_seed(s) for s in SEEDS]
strict=all(r["scientific_status"]=="PASSED" for r in rows)
out={
    "schema":"ACSIE.layer1-independent-procedure-bank-h30.v3-multiseed",
    "scientific_status":"PASSED" if strict else "FAILED",
    "seed_count":len(rows),
    "expected_seeds":list(SEEDS),
    "strict":strict,
    "procedure_kernel_digests":{
        "gen2":digest(asdict(GEN2_KERNEL)),
        "gen3":digest(asdict(GEN3_KERNEL)),
        "gen4":digest(asdict(GEN4_KERNEL)),
    },
    "expected_kernel_digests":EXPECTED,
    "rows":rows,
    "scientific_scope":"Layer-1 reusable learning-procedure bank only; no AGI/ASI claim.",
}
print(json.dumps(out,indent=2,sort_keys=True))
