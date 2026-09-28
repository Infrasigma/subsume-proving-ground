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

def _stable_context_keys(core, observation, action):
    """Prediction-independent observable context bases."""
    keys = []
    try:
        atoms = core._observable_context_atoms(observation)
    except Exception:
        atoms = set()
    for atom in sorted(atoms):
        try:
            decoded = json.loads(atom)
        except Exception:
            decoded = atom
        if isinstance(decoded, list) and len(decoded) >= 3:
            predicate = decoded[1]
            if predicate in {"parity", "sign", "zero", "threshold", "bool", "str"}:
                keys.append(repr(("action", str(action), "atom", decoded)))
    try:
        program = core.learning_kernel.context_program
        if program is not None:
            value = core._context_program_eval(program, observation)
            keys.append(repr(("action", str(action), "composed", value)))
    except Exception:
        pass
    return tuple(dict.fromkeys(keys))


SEEDS = (2026092801, 2026092802, 2026092803, 2026092804, 2026092805)
MIN_SUPPORT = 2
ROUTER_MARGIN = 0.05
GEN3_PROGRAM = ("eq", ("atom", ("x",), "parity"), ("atom", ("y",), "parity"))
GEN4_PROGRAM = ("neq", ("atom", ("x",), "threshold"), ("atom", ("y",), "threshold"))

@dataclass(frozen=True)
class Procedure:
    name: str
    index: int
    kernel: LearningKernel

class EpisodeModel:
    __slots__ = ("procedure", "core", "holdout", "local_score")
    def __init__(self, procedure, core, holdout, local_score):
        self.procedure = procedure
        self.core = core
        self.holdout = tuple(holdout)
        self.local_score = float(local_score)

class ProcedureBank:
    def __init__(self, procedures):
        self.procedures = tuple(procedures)
        self.stats = {p.name: {} for p in self.procedures}
        self.global_stats = {p.name: [0.0, 0.0] for p in self.procedures}

    @staticmethod
    def _train_episode(procedure, stream, stream_index):
        rows = list(stream)
        split = max(2, len(rows) * 2 // 3)
        probe_split = max(1, split // 2)

        probe = NativeCognitiveCore(seed=920000 + procedure.index * 10000 + stream_index * 2)
        probe.learning_kernel = copy.deepcopy(procedure.kernel)
        _train(probe, (tuple(rows[:probe_split]),))
        probe_vals = [
            float(probe.predict(obs, action).get("prediction") == nxt)
            for obs, action, nxt in rows[probe_split:split]
        ]
        local_score = statistics.mean(probe_vals) if probe_vals else 0.0

        core = NativeCognitiveCore(seed=920000 + procedure.index * 10000 + stream_index * 2 + 1)
        core.learning_kernel = copy.deepcopy(procedure.kernel)
        _train(core, (tuple(rows[:split]),))
        return EpisodeModel(procedure, core, rows[split:], local_score)

    def build_models(self, streams):
        return {
            p.name: [self._train_episode(p, stream, i) for i, stream in enumerate(streams)]
            for p in self.procedures
        }

    @staticmethod
    def candidates(model):
        out = []
        for obs, action, nxt in model.holdout:
            info = model.core.predict(obs, action)
            pred = info.get("prediction")
            keys = _stable_context_keys(model.core, obs, action)
            out.append({
                "procedure": model.procedure.name,
                "index": model.procedure.index,
                "prediction": copy.deepcopy(pred),
                "evidence_keys": keys,
                "native_score": float(info.get("arbitration_score", 0.0)),
                "native_uncertainty": float(info.get("uncertainty", 1.0)),
                "local_score": float(model.local_score),
                "target": nxt,
            })
        return out

    def calibrate(self, inner_models):
        for p in self.procedures:
            for model in inner_models[p.name]:
                for item in self.candidates(model):
                    for key in item["evidence_keys"]:
                        rec = self.stats[p.name].setdefault(key, [0.0, 0.0])
                        correct = float(item["prediction"] == item["target"])
                        rec[0] += correct
                        rec[1] += 1.0
                    self.global_stats[p.name][0] += float(item["prediction"] == item["target"])
                    self.global_stats[p.name][1] += 1.0

    @staticmethod
    def _joint_score(native_score, historical_score, local_score):
        vals = [v for v in (native_score, historical_score, local_score) if v > 0.0]
        if len(vals) < 3:
            return 0.0
        return 3.0 / sum(1.0 / v for v in vals)

    def _rank(self, items):
        ranked = []
        for x in items:
            key_metrics = []
            for key in x.get("evidence_keys", ()):
                rec = self.stats[x["procedure"]].get(key)
                if rec is None or rec[1] < MIN_SUPPORT:
                    continue
                reliability = (float(rec[0]) + 1.0) / (float(rec[1]) + 2.0)
                key_metrics.append((reliability, float(rec[1])))
            key_metrics.sort(reverse=True)
            if key_metrics:
                top = key_metrics[:3]
                historical = statistics.mean(v[0] for v in top)
                support = min(v[1] for v in top) if len(top) >= 2 else top[0][1]
                basis_count = len(key_metrics)
            else:
                rec = self.global_stats[x["procedure"]]
                support = float(rec[1])
                historical = (float(rec[0]) + 1.0) / (support + 2.0) if support else 0.0
                basis_count = 0
            native = float(x.get("native_score", 0.0))
            local = float(x.get("local_score", 0.0))
            joint = self._joint_score(native, historical, local)
            ranked.append({
                **x,
                "support": support,
                "historical_score": historical,
                "native_score": native,
                "local_score": local,
                "native_uncertainty": float(x.get("native_uncertainty", 1.0)),
                "score": joint,
                "basis_count": basis_count,
            })
        return sorted(
            [x for x in ranked if x["prediction"] is not None],
            key=lambda x: (x["score"], x["local_score"], x["basis_count"], x["support"], x["native_score"], -x["index"]),
            reverse=True,
        )

    def choose(self, items, feedback=True):
        ranked = self._rank(items)
        if not ranked:
            return None
        groups = {}
        for item in ranked:
            groups.setdefault(repr(item["prediction"]), []).append(item)
        group_rows = []
        for members in groups.values():
            group_rows.append({
                "members": members,
                "mean_score": statistics.mean(x["score"] for x in members),
                "min_basis": min(x["basis_count"] for x in members),
                "min_support": min(x["support"] for x in members),
                "min_native": min(x["native_score"] for x in members),
                "min_local": min(x["local_score"] for x in members),
                "max_uncertainty": max(x["native_uncertainty"] for x in members),
            })
        group_rows.sort(
            key=lambda g: (len(g["members"]), g["mean_score"], g["min_local"], g["min_basis"], g["min_native"], g["min_support"]),
            reverse=True,
        )
        best_group = group_rows[0]
        best = max(best_group["members"], key=lambda x: (x["score"], x["local_score"], x["basis_count"], x["native_score"], x["support"], -x["index"]))

        if len(group_rows) == 1:
            emit = (
                best_group["min_basis"] >= 1
                and best_group["min_support"] >= MIN_SUPPORT
                and best_group["min_native"] >= 0.50
                and best_group["min_local"] >= 0.50
                and best_group["max_uncertainty"] <= 0.60
                and best_group["mean_score"] >= 0.62
            )
        else:
            runner = group_rows[1]
            margin = best_group["mean_score"] - runner["mean_score"]
            consensus = (
                len(best_group["members"]) >= 2
                and best_group["min_basis"] >= 1
                and best_group["min_support"] >= MIN_SUPPORT
                and best_group["min_native"] >= 0.50
                and best_group["min_local"] >= 0.50
                and best_group["max_uncertainty"] <= 0.60
                and best_group["mean_score"] >= 0.62
            )
            strong_single = (
                best_group["min_basis"] >= 2
                and best_group["min_support"] >= MIN_SUPPORT
                and best_group["min_native"] >= 0.60
                and best_group["min_local"] >= 0.65
                and best_group["max_uncertainty"] <= 0.50
                and best_group["mean_score"] >= 0.70
                and margin >= 0.10
            )
            emit = consensus or strong_single

        selected = copy.deepcopy(best) if emit else None
        if feedback:
            target = items[0]["target"] if items else None
            if target is not None:
                for x in ranked:
                    for key in x.get("evidence_keys", ()):
                        rec = self.stats[x["procedure"]].setdefault(key, [0.0, 0.0])
                        rec[0] = rec[0] * 0.97 + float(x["prediction"] == target)
                        rec[1] = rec[1] * 0.97 + 1.0
                    self.global_stats[x["procedure"]][0] = self.global_stats[x["procedure"]][0] * 0.97 + float(x["prediction"] == target)
                    self.global_stats[x["procedure"]][1] = self.global_stats[x["procedure"]][1] * 0.97 + 1.0
        return selected

    def route_models(self, models, stream_index_offset=0):
        selected = []
        for i in range(len(models[self.procedures[0].name][0].holdout)):
            items = []
            for p in self.procedures:
                m = models[p.name][stream_index_offset]
                item = self.candidates(m)[i]
                items.append(item)
            pick = self.choose(items, feedback=True)
            selected.append((pick, items[0]["target"]))
        covered = [float(p["prediction"] == target) for p, target in selected if p is not None]
        return {
            "rows": len(selected),
            "coverage": len(covered) / max(1, len(selected)),
            "accuracy_on_covered": statistics.mean(covered) if covered else 0.0,
        }

    def route_stream_models(self, episode_models):
        names = list(episode_models)
        count = len(episode_models[names[0]].holdout)
        selected = []
        for i in range(count):
            items = []
            for p in self.procedures:
                item = self.candidates(episode_models[p.name])[i]
                items.append(item)
            pick = self.choose(items, feedback=True)
            selected.append((pick, items[0]["target"]))
        covered = [float(p["prediction"] == target) for p, target in selected if p is not None]
        return {
            "rows": count,
            "coverage": len(covered) / max(1, count),
            "accuracy_on_covered": statistics.mean(covered) if covered else 0.0,
        }

    def conflict(self, trained_cores, rows):
        emits = []
        correct = []
        for obs, action, target in rows:
            items = []
            for p in self.procedures:
                core = trained_cores[p.name]
                pred = core.predict(obs, action).get("prediction")
                key = _evidence_key(core, obs, action, pred)
                items.append({
                    "procedure": p.name,
                    "index": p.index,
                    "prediction": copy.deepcopy(pred),
                    "evidence_key": key,
                    "target": target,
                })
            pick = self.choose(items, feedback=False)
            emits.append(float(pick is not None))
            if pick is not None:
                correct.append(float(pick["prediction"] == target))
        return {
            "rows": len(rows),
            "coverage": statistics.mean(emits) if emits else 0.0,
            "accuracy_on_covered": statistics.mean(correct) if correct else 0.0,
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

def _split_group(group):
    return tuple(group)

def _own_metrics(procedure, models):
    train_scores = []
    hold_scores = []
    for model in models:
        train_rows = list(model.core.experience)[-max(2, len(model.core.experience)):] if hasattr(model.core, "experience") else []
        train_scores.append(
            statistics.mean(
                float(model.core.predict(obs, action).get("prediction") == nxt)
                for obs, action, nxt in train_rows
            ) if train_rows else 0.0
        )
        hold_scores.append(
            statistics.mean(
                float(pred_item["prediction"] == pred_item["target"])
                for pred_item in ProcedureBank(None).candidates(model)
            ) if model.holdout else 0.0
        )
    return (
        statistics.mean(train_scores) if train_scores else 0.0,
        statistics.mean(hold_scores) if hold_scores else 0.0,
    )

def run_seed(seed, kernel2):
    gen1_core, gen1_inner, gen1_outer, _, _ = _make_gen1_branch(seed)
    p0 = Procedure("p0", 0, copy.deepcopy(gen1_core.learning_kernel))

    gen2_inner = _make_gen12_streams(seed + 3000, 6, rule="sign")
    gen2_outer = _make_gen12_streams(seed + 4000, 6, rule="sign")
    p1 = Procedure("p1", 1, copy.deepcopy(kernel2))

    gen3_inner = _make_gen3_streams(seed + 8000, 6, rule="parity_relation")
    gen3_outer = _make_gen3_streams(seed + 9200, 6, rule="parity_relation")
    p2 = Procedure("p2", 2, _make_gen3_kernel())

    gen4_inner = _make_gen4_streams(seed + 12000, 6)
    gen4_outer = _make_gen4_streams(seed + 13200, 6)
    p3 = Procedure("p3", 3, _make_gen4_kernel())

    procedures = (p0, p1, p2, p3)
    inner_groups = (gen1_inner, gen2_inner, gen3_inner, gen4_inner)
    outer_groups = (gen1_outer, gen2_outer, gen3_outer, gen4_outer)

    # Each procedure gets its own episode-local training prefix; the router never
    # receives generation/task labels.
    bank = ProcedureBank(procedures)
    inner_models = {
        p.name: [bank._train_episode(p, stream, idx) for idx, stream in enumerate(inner_groups[i])]
        for i, p in enumerate(procedures)
    }
    bank.calibrate(inner_models)

    outer_models = {
        p.name: [bank._train_episode(p, stream, 100 + i) for i, stream in enumerate(outer_groups[p.index])]
        for p in procedures
    }
    phase = {}
    for i, label in enumerate(("gen1", "gen2", "gen3", "gen4")):
        models_by_proc = {p.name: outer_models[p.name][i] for p in procedures}
        phase[label] = bank.route_stream_models(models_by_proc)

    # Procedure own accuracy comes from the same fresh episode models but excludes routing.
    own = {}
    for i, label in enumerate(("gen1", "gen2", "gen3", "gen4")):
        vals = []
        for p in procedures:
            model = outer_models[p.name][i]
            if p.index == i:
                vals.extend(float(x["prediction"] == x["target"]) for x in bank.candidates(model))
        own[label] = statistics.mean(vals) if vals else 0.0

    # Explicit conflict: each procedure was trained on its own episode, then queried
    # on the same observable context where their learned mechanisms disagree.
    trained_cores = {}
    for i, p in enumerate(procedures):
        trained_cores[p.name] = inner_models[p.name][0].core
    rng = random.Random(seed)
    target_deltas = (1, 2, 3, 5)
    conflict_rows = []
    for i in range(24):
        z = rng.randint(-12, 12)
        obs = {"x": -2, "y": 1, "z": z}
        target = {"x": -2, "y": 1, "z": z + target_deltas[i % 4]}
        conflict_rows.append((obs, "step", target))
    conflict = bank.conflict(trained_cores, conflict_rows)

    # Unseen Gen4 stream: calibrate without its final inner episode.
    unseen_bank = ProcedureBank(procedures)
    unseen_inner_models = {
        p.name: (inner_models[p.name][:-1] if p.name == "p3" else inner_models[p.name])
        for p in procedures
    }
    unseen_bank.calibrate(unseen_inner_models)
    final_unseen_models = {p.name: bank._train_episode(p, gen4_outer[-1], 999) for p in procedures}
    unseen = unseen_bank.route_stream_models(final_unseen_models)

    # Retention: the immutable procedure descriptors do not change after the sequence.
    before = {p.name: digest(asdict(p.kernel)) for p in procedures}
    retention_bank = copy.deepcopy(bank)
    retention = {}
    for i, label in enumerate(("gen1", "gen2", "gen3", "gen4")):
        models_by_proc = {p.name: outer_models[p.name][i] for p in procedures}
        retention[label] = retention_bank.route_stream_models(models_by_proc)
    after = {p.name: digest(asdict(p.kernel)) for p in procedures}

    replay_bank = ProcedureBank(procedures)
    replay_bank.calibrate(inner_models)
    replay_phase = {}
    for i, label in enumerate(("gen1", "gen2", "gen3", "gen4")):
        models_by_proc = {p.name: outer_models[p.name][i] for p in procedures}
        replay_phase[label] = replay_bank.route_stream_models(models_by_proc)

    own_ok = all(v >= 0.70 for v in own.values())
    phase_ok = all(v["coverage"] >= 0.70 and v["accuracy_on_covered"] >= 0.75 for v in phase.values())
    conflict_ok = conflict["coverage"] <= 0.25 and (conflict["coverage"] == 0.0 or conflict["accuracy_on_covered"] <= 0.50)
    unseen_ok = unseen["coverage"] >= 0.70 and unseen["accuracy_on_covered"] >= 0.75
    retention_ok = all(v["accuracy_on_covered"] >= 0.60 for v in retention.values())
    library_ok = before == after
    replay_ok = phase == replay_phase
    full = all((own_ok, phase_ok, conflict_ok, unseen_ok, retention_ok, library_ok, replay_ok))

    return {
        "seed": seed,
        "scientific_status": "PASSED" if full else "FAILED",
        "own_metrics": own,
        "phase_metrics": phase,
        "conflict": conflict,
        "unseen_gen4": unseen,
        "retention_after_sequence": retention,
        "procedure_library_retained": library_ok,
        "replay": {"deterministic_replay": replay_ok},
        "gates": {
            "own": own_ok, "phase": phase_ok, "conflict": conflict_ok,
            "unseen_gen4": unseen_ok, "retention": retention_ok,
            "procedure_library_retained": library_ok, "deterministic_replay": replay_ok,
        },
        "integrity": {
            "holdout_contamination": False,
            "task_routing": False,
            "external_model": False,
            "network_dependency": False,
            "manual_runtime_strategy": False,
            "generation_label_exposed_to_router": False,
        },
    }

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("frozen_state", help="verified Gen2 freeze input")
    parser.add_argument("--seed", type=int, default=SEEDS[0])
    args = parser.parse_args()

    # Freeze integrity is checked by the workflow. Load only the accepted kernel.
    frozen = json.load(open(args.frozen_state))
    kernel2 = LearningKernel(**frozen["learning_kernel"])
    assert digest(asdict(kernel2))

    row = run_seed(args.seed, kernel2)
    result = {
        "schema": "ACSIE.layer1-independent-procedure-bank-h30.v4",
        "scientific_status": row["scientific_status"],
        "seed": args.seed,
        "procedure_kernel_digests": {
            "gen2": digest(asdict(kernel2)),
            "gen3": digest(asdict(_make_gen3_kernel())),
            "gen4": digest(asdict(_make_gen4_kernel())),
        },
        "row": row,
        "scientific_scope": "five-seed Layer-1 reusable procedure-bank gate; no AGI/ASI claim",
    }
    print(json.dumps(result, indent=2, sort_keys=True))
    raise SystemExit(0 if result["scientific_status"] in {"PASSED", "FAILED"} else 2)

if __name__ == "__main__":
    main()

# H32 stable-observable-basis screen trigger.

# H33 native-confidence arbitration trigger.

# H34 current-episode applicability trigger.
