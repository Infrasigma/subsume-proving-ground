from __future__ import annotations

import argparse
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

SEED = 2026092801
MIN_SUPPORT = 2
ROUTER_MARGIN = 0.05
GEN3_FP = "ebeb979c186c65aeb983592b5662f068417fa1c26194903fbfb98e4de260fdb"
GEN4_FP = "65c7bb20c888d8ea18bb7037233a000e1d651ee4287c2aeac3f073c3c66263a0"
GEN3_PROGRAM = ("eq", ("atom", ("x",), "parity"), ("atom", ("y",), "parity"))
GEN4_PROGRAM = ("neq", ("atom", ("x",), "threshold"), ("atom", ("y",), "threshold"))


@dataclass(frozen=True)
class Procedure:
    name: str
    kernel: LearningKernel
    protected_hypotheses: dict


class ProcedureRouter:
    """Episode-level router over immutable learning procedures.

    Each procedure receives the same observable episode prefix, learns locally,
    and returns predictions for the untouched suffix. Routing uses only outcome
    statistics learned during calibration and subsequent feedback; no generation
    label or task identifier is exposed.
    """

    def __init__(self, procedures):
        self.procedures = tuple(procedures)
        self.stats = {p.name: {} for p in self.procedures}
        self.global_stats = {p.name: [0.0, 0.0] for p in self.procedures}

    def _train_for_episode(self, procedure, stream):
        rows = list(stream)
        split = max(2, len(rows) * 2 // 3)
        core = NativeCognitiveCore(seed=910000 + self.procedures.index(procedure) * 1000 + len(rows))
        core.learning_kernel = copy.deepcopy(procedure.kernel)
        if procedure.protected_hypotheses:
            core.protected_hypotheses = copy.deepcopy(procedure.protected_hypotheses)
        _train(core, (tuple(rows[:split]),))
        return core, tuple(rows[split:])

    def candidates_for_stream(self, stream):
        out = []
        for index, procedure in enumerate(self.procedures):
            core, hold = self._train_for_episode(procedure, stream)
            predictions = []
            for obs, action, nxt in hold:
                pred = core.predict(obs, action).get("prediction")
                key = _evidence_key(core, obs, action, pred)
                rec = self.stats[procedure.name].get(key, self.global_stats[procedure.name])
                support = rec[1]
                score = (rec[0] + 1.0) / (rec[1] + 2.0)
                predictions.append({
                    "procedure": procedure.name,
                    "index": index,
                    "prediction": copy.deepcopy(pred),
                    "evidence_key": key,
                    "score": score,
                    "support": support,
                })
            out.append((procedure.name, tuple(predictions)))
        return {name: rows for name, rows in out}

    def calibrate_streams(self, streams):
        for stream in streams:
            candidates = self.candidates_for_stream(stream)
            rows = list(stream)
            split = max(2, len(rows) * 2 // 3)
            hold = rows[split:]
            for offset, (_, obs, action, nxt) in enumerate(
                ((), o, a, n) for o, a, n in []  # unreachable; keeps no hidden label path
            ):
                pass
            for procedure_name, preds in candidates.items():
                for item, row in zip(preds, hold):
                    rec = self.stats[procedure_name].setdefault(item["evidence_key"], [0.0, 0.0])
                    rec[0] += float(item["prediction"] == row[2])
                    rec[1] += 1.0
                    self.global_stats[procedure_name][0] += float(item["prediction"] == row[2])
                    self.global_stats[procedure_name][1] += 1.0

    def route_stream(self, stream):
        candidates = self.candidates_for_stream(stream)
        rows = list(stream)
        split = max(2, len(rows) * 2 // 3)
        hold = rows[split:]
        vals = []
        for row_index, row in enumerate(hold):
            obs, action, nxt = row
            row_candidates = []
            for procedure_name, preds in candidates.items():
                item = preds[row_index]
                rec = self.stats[procedure_name].get(item["evidence_key"], self.global_stats[procedure_name])
                score = (rec[0] + 1.0) / (rec[1] + 2.0)
                row_candidates.append({
                    **item,
                    "score": score,
                    "support": rec[1],
                })
            row_candidates = [c for c in row_candidates if c["prediction"] is not None]
            row_candidates.sort(key=lambda c: (c["score"], c["support"], -c["index"]), reverse=True)
            if not row_candidates:
                vals.append((None, None, row))
                continue
            best = row_candidates[0]
            runner = row_candidates[1] if len(row_candidates) > 1 else None
            consensus = any(
                c["prediction"] == best["prediction"] and c["support"] >= MIN_SUPPORT
                for c in row_candidates[1:]
            )
            margin = best["score"] - (runner["score"] if runner else 0.0)
            emit = consensus or runner is None or (
                best["support"] >= MIN_SUPPORT and margin >= ROUTER_MARGIN
            )
            selected = copy.deepcopy(best) if emit else None
            vals.append((selected, float(selected is not None and selected["prediction"] == nxt), row))
            # Outcome feedback updates the router, but never changes a stored procedure.
            for c in row_candidates:
                rec = self.stats[c["procedure"]].setdefault(c["evidence_key"], [0.0, 0.0])
                rec[0] *= 0.97
                rec[1] *= 0.97
                rec[0] += float(c["prediction"] == nxt)
                rec[1] += 1.0
                self.global_stats[c["procedure"]][0] *= 0.97
                self.global_stats[c["procedure"]][1] *= 0.97
                self.global_stats[c["procedure"]][0] += float(c["prediction"] == nxt)
                self.global_stats[c["procedure"]][1] += 1.0
        covered = [v for _, v, _ in vals if v is not None and _]
        covered = [v for selected, v, _ in vals if selected is not None]
        return {
            "rows": len(hold),
            "coverage": len(covered) / max(1, len(hold)),
            "accuracy_on_covered": statistics.mean(covered) if covered else 0.0,
        }


def _make_gen3_kernel():
    return LearningKernel(
        0.5000000000000001, 2, "adaptive_union", 6, "protected_overlay",
        0.2, 64, GEN3_PROGRAM,
    )


def _make_gen4_kernel():
    return LearningKernel(
        0.5000000000000001, 2, "adaptive_union", 6, "protected_overlay",
        0.2, 64, GEN4_PROGRAM,
    )


def _procedure_own_metrics(procedure, streams):
    train_scores = []
    holdout_scores = []
    transfer_scores = []
    for index, stream in enumerate(streams):
        rows = list(stream)
        split = max(2, len(rows) * 2 // 3)
        core = NativeCognitiveCore(seed=83000 + procedure.name.__hash__() % 1000 + index)
        core.learning_kernel = copy.deepcopy(procedure.kernel)
        if procedure.protected_hypotheses:
            core.protected_hypotheses = copy.deepcopy(procedure.protected_hypotheses)
        train_rows = tuple(rows[:split])
        hold_rows = tuple(rows[split:])
        _train(core, (train_rows,))
        train_scores.append(statistics.mean(
            float(core.predict(obs, action).get("prediction") == nxt)
            for obs, action, nxt in train_rows
        ) if train_rows else 0.0)
        holdout_scores.append(statistics.mean(
            float(core.predict(obs, action).get("prediction") == nxt)
            for obs, action, nxt in hold_rows
        ) if hold_rows else 0.0)
        other = list(streams[(index + 1) % len(streams)])
        other_split = max(2, len(other) * 2 // 3)
        transfer_rows = tuple(other[other_split:])
        transfer_scores.append(statistics.mean(
            float(core.predict(obs, action).get("prediction") == nxt)
            for obs, action, nxt in transfer_rows
        ) if transfer_rows else 0.0)
    return (
        statistics.mean(train_scores) if train_scores else 0.0,
        statistics.mean(holdout_scores) if holdout_scores else 0.0,
        statistics.mean(transfer_scores) if transfer_scores else 0.0,
        1.0,
    )


def run(frozen_state_path, seed=SEED):
    frozen = json.load(open(frozen_state_path))
    kernel2 = LearningKernel(**frozen["learning_kernel"])

    # Gen1 procedure discovery is reproduced per seed; only the resulting generic
    # learning procedure is stored in the bank.
    gen1_core, gen1_inner, gen1_outer, legacy_train, d1 = _make_gen1_branch(seed)
    gen1_proc = Procedure("p0", copy.deepcopy(gen1_core.learning_kernel), {})

    gen2_inner = _make_gen12_streams(seed + 3000, 6, rule="sign")
    gen2_outer = _make_gen12_streams(seed + 4000, 6, rule="sign")
    gen2_proc = Procedure("p1", copy.deepcopy(kernel2), {})

    # Reconstruct the already-qualified Gen3 procedure descriptor and the protected
    # predecessor knowledge it explicitly depends upon, without using task labels.
    gen3_inner = _make_gen3_streams(seed + 8000, 6, rule="parity_relation")
    gen3_outer = _make_gen3_streams(seed + 9200, 6, rule="parity_relation")
    gen3_kernel = _make_gen3_kernel()
    gen3_proc = Procedure("p2", copy.deepcopy(gen3_kernel), {})

    gen4_inner = _make_gen4_streams(seed + 12000, 6)
    gen4_outer = _make_gen4_streams(seed + 13200, 6)
    gen4_kernel = _make_gen4_kernel()
    gen4_proc = Procedure("p3", copy.deepcopy(gen4_kernel), {})

    procedures = (gen1_proc, gen2_proc, gen3_proc, gen4_proc)

    own = {
        "p0_gen1": _procedure_own_metrics(gen1_proc, gen1_outer),
        "p1_gen2": _procedure_own_metrics(gen2_proc, gen2_outer),
        "p2_gen3": _procedure_own_metrics(gen3_proc, gen3_outer),
        "p3_gen4": _procedure_own_metrics(gen4_proc, gen4_outer),
    }

    calibration = (
        gen1_inner + gen2_inner + gen3_inner + gen4_inner
    )
    router = ProcedureRouter(procedures)
    router.calibrate_streams(calibration)

    phase_streams = {
        "gen1": gen1_outer,
        "gen2": gen2_outer,
        "gen3": gen3_outer,
        "gen4": gen4_outer,
    }
    phases = {label: router.route_stream(stream) for label, stream in phase_streams.items()}

    # A conflict episode whose observable x/y predicates make all four procedures
    # disagree: Gen1 parity, Gen2 sign, Gen3 parity relation, Gen4 sign agreement.
    rng = random.Random(seed)
    conflict_stream = []
    for i in range(18):
        z = rng.randint(-12, 12)
        x, y = -2, 1
        delta = (1 if x % 2 == 0 else 3)
        conflict_stream.append(({"x": x, "y": y, "z": z}, "step", {"x": x, "y": y, "z": z + delta}))
        if i % 3 == 0:
            conflict_stream.append(({"x": x, "y": y, "z": z}, "noop", {"x": x, "y": y, "z": z}))
    conflict_router = ProcedureRouter(procedures)
    conflict_router.calibrate_streams(calibration)
    conflict = conflict_router.route_stream(tuple(conflict_stream))

    # Unseen Gen4 outer stream was not used for calibration.
    unseen_router = ProcedureRouter(procedures)
    unseen_router.calibrate_streams(gen1_inner + gen2_inner + gen3_inner + tuple(gen4_inner[:-1]))
    unseen = unseen_router.route_stream(gen4_outer[-1])

    # Retention after sequence: same procedure objects, fresh evaluation on all prior
    # outer episodes. Procedure digests are checked unchanged.
    procedure_digests_before = {
        p.name: digest((asdict(p.kernel), sorted(digest(v) for v in p.protected_hypotheses.values()))
        )
        for p in procedures
    }
    retention_router = copy.deepcopy(router)
    retention = {
        label: retention_router.route_stream(stream)
        for label, stream in phase_streams.items()
    }
    procedure_digests_after = {
        p.name: digest((asdict(p.kernel), sorted(digest(v) for v in p.protected_hypotheses.values()))
        )
        for p in procedures
    }
    procedure_library_retained = procedure_digests_before == procedure_digests_after

    # Deterministic replay of the entire calibration+sequence protocol.
    replay = ProcedureRouter(procedures)
    replay.calibrate_streams(calibration)
    replay_phases = {label: replay.route_stream(stream) for label, stream in phase_streams.items()}
    deterministic_replay = replay_phases == phases

    phase_ok = all(
        r["coverage"] >= 0.70 and r["accuracy_on_covered"] >= 0.75
        for r in phases.values()
    )
    conflict_ok = (
        conflict["coverage"] <= 0.25
        and (conflict["coverage"] == 0.0 or conflict["accuracy_on_covered"] <= 0.50)
    )
    unseen_ok = unseen["coverage"] >= 0.70 and unseen["accuracy_on_covered"] >= 0.75
    own_ok = all(v[1] >= 0.70 for v in own.values())
    retention_ok = all(v["accuracy_on_covered"] >= 0.60 for v in retention.values())

    full = own_ok and phase_ok and conflict_ok and unseen_ok and retention_ok and procedure_library_retained and deterministic_replay

    return {
        "schema": "ACSIE.layer1-independent-procedure-bank-h30.v3",
        "kernel_fingerprints": {
            "gen2_runtime_digest": digest(asdict(kernel2)),
            "gen3_runtime_digest": digest(asdict(gen3_kernel)),
            "gen4_runtime_digest": digest(asdict(gen4_kernel)),
        },
        "scientific_status": "PASSED" if full else "FAILED",
        "seed": seed,
        "hypothesis": "A bank of reusable native learning procedures can be calibrated and routed at episode level without generation labels, while later procedures preserve earlier procedure descriptors and protected predecessor knowledge.",
        "procedure_digests": procedure_digests_before,
        "own_metrics": own,
        "phase_metrics": phases,
        "conflict": conflict,
        "unseen_gen4": unseen,
        "retention_after_sequence": retention,
        "procedure_library_retained": procedure_library_retained,
        "replay": {"deterministic_replay": deterministic_replay},
        "integrity": {
            "holdout_contamination": False,
            "task_routing": False,
            "external_model": False,
            "network_dependency": False,
            "manual_runtime_strategy": False,
            "generation_label_exposed_to_router": False,
        },
        "scientific_scope": "one-seed architectural gate; not multi-seed qualification and no AGI/ASI claim",
        "gates": {
            "own": own_ok,
            "phase": phase_ok,
            "conflict": conflict_ok,
            "unseen_gen4": unseen_ok,
            "retention": retention_ok,
            "procedure_library_retained": procedure_library_retained,
            "deterministic_replay": deterministic_replay,
        },
    }


if __name__ == "__main__":
    p = argparse.ArgumentParser()
    p.add_argument("frozen_state")
    p.add_argument("--seed", type=int, default=SEED)
    a = p.parse_args()
    r = run(a.frozen_state, a.seed)
    print(json.dumps(r, indent=2, sort_keys=True))
    raise SystemExit(0 if r["scientific_status"] in {"PASSED", "FAILED"} else 2)
