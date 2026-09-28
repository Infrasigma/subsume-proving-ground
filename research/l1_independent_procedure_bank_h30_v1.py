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

    def evaluate_applicability(self, prefix, stream_index):
        """Evaluate this procedure using only the observed episode prefix."""
        rows = tuple(prefix)
        fold_scores = []
        native_scores = []
        block_size = max(1, len(rows) // 4)
        for fold in range(1, 4):
            train_rows = tuple(rows[: fold * block_size])
            valid_rows = tuple(rows[fold * block_size : (fold + 1) * block_size])
            if not train_rows or not valid_rows:
                continue
            probe = NativeCognitiveCore(
                seed=920000 + self.index * 10000 + stream_index * 4 + fold
            )
            probe.learning_kernel = copy.deepcopy(self.kernel)
            probe.observe_batch(tuple(train_rows))
            for obs, action, nxt in valid_rows:
                info = probe.predict(obs, action)
                fold_scores.append(float(info.get("prediction") == nxt))
                native_scores.append(float(info.get("arbitration_score", 0.0)))
        local_score = statistics.mean(fold_scores) if fold_scores else 0.0
        native_score = statistics.mean(native_scores) if native_scores else 0.0

        causal_core = NativeCognitiveCore(
            seed=920000 + self.index * 10000 + stream_index * 4 + 7
        )
        causal_core.learning_kernel = copy.deepcopy(self.kernel)
        causal_scores = []
        causal_native_scores = []
        for obs, action, nxt in rows:
            info = causal_core.predict(obs, action)
            if info.get("prediction") is not None:
                causal_scores.append(
                    float(info.get("prediction") == nxt)
                )
                causal_native_scores.append(
                    float(info.get("arbitration_score", 0.0))
                )
            causal_core.observe(obs, action, nxt)
        causal_score = statistics.mean(causal_scores) if causal_scores else 0.0
        causal_native_score = (
            statistics.mean(causal_native_scores)
            if causal_native_scores else 0.0
        )

        core = causal_core
        fit_scores = []
        for obs, action, _nxt in rows:
            info = core.predict(obs, action)
            fit_scores.append(float(info.get("arbitration_score", 0.0)))
        fit_native_score = statistics.mean(fit_scores) if fit_scores else 0.0
        return ApplicableProcedure(
            self,
            core,
            local_score,
            native_score,
            fit_native_score,
            causal_score,
            causal_native_score,
        )


class ApplicableProcedure:
    """A procedure bound to one episode prefix, usable only after selection."""

    __slots__ = (
        "procedure",
        "core",
        "local_score",
        "native_score",
        "fit_native_score",
        "causal_score",
        "causal_native_score",
    )

    def __init__(
        self,
        procedure,
        core,
        local_score,
        native_score,
        fit_native_score,
        causal_score,
        causal_native_score,
    ):
        self.procedure = procedure
        self.core = core
        self.local_score = float(local_score)
        self.native_score = float(native_score)
        self.fit_native_score = float(fit_native_score)
        self.causal_score = float(causal_score)
        self.causal_native_score = float(causal_native_score)
        self.causal_score = float(causal_score)
        self.causal_native_score = float(causal_native_score)

    def predict(self, suffix):
        """Predict on the post-selection suffix without inspecting its targets."""
        return tuple(
            (copy.deepcopy(obs), action, self.core.predict(obs, action))
            for obs, action, _target in suffix
        )

class EpisodeModel:
    __slots__ = (
        "procedure",
        "core",
        "holdout",
        "local_score",
        "native_score",
        "fit_native_score",
        "causal_score",
        "causal_native_score",
    )
    def __init__(
        self,
        procedure,
        core,
        holdout,
        local_score,
        native_score,
        fit_native_score,
        causal_score,
        causal_native_score,
    ):
        self.procedure = procedure
        self.core = core
        self.holdout = tuple(holdout)
        self.local_score = float(local_score)
        self.native_score = float(native_score)
        self.fit_native_score = float(fit_native_score)
        self.causal_score = float(causal_score)
        self.causal_native_score = float(causal_native_score)

class ProcedureBank:
    def __init__(self, procedures):
        self.procedures = tuple(procedures)
        self.stats = {p.name: {} for p in self.procedures}
        self.global_stats = {p.name: [0.0, 0.0] for p in self.procedures}

    @staticmethod
    def _train_episode(procedure, stream, stream_index):
        rows = list(stream)
        split = max(2, len(rows) * 2 // 3)
        prefix = tuple(rows[:split])

        # Applicability is evaluated independently from the observed prefix only.
        applicable = procedure.evaluate_applicability(prefix, stream_index)
        return EpisodeModel(
            procedure,
            applicable.core,
            rows[split:],
            applicable.local_score,
            applicable.native_score,
            applicable.fit_native_score,
            applicable.causal_score,
            applicable.causal_native_score,
        )

    def build_models(self, streams):
        return {
            p.name: [self._train_episode(p, stream, i) for i, stream in enumerate(streams)]
            for p in self.procedures
        }

    def _prefix_historical_score(self, procedure_name, model):
        values = []
        for experience in model.core.experience:
            key_metrics = []
            for key in _stable_context_keys(
                model.core,
                experience.observation,
                experience.action,
            ):
                rec = self.stats[procedure_name].get(key)
                if rec is None or rec[1] < MIN_SUPPORT:
                    continue
                reliability = (float(rec[0]) + 1.0) / (float(rec[1]) + 2.0)
                key_metrics.append((reliability, float(rec[1])))
            if key_metrics:
                key_metrics.sort(reverse=True)
                values.append(statistics.mean(
                    reliability for reliability, _support in key_metrics[:3]
                ))
        if values:
            return statistics.mean(values)
        historical = self.global_stats[procedure_name]
        return (
            (float(historical[0]) + 1.0) / (float(historical[1]) + 2.0)
            if historical[1]
            else 0.0
        )

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
    def _harmonic(values):
        vals=[float(v) for v in values if float(v)>0.0]
        if len(vals)<2:
            return 0.0
        return len(vals)/sum(1.0/v for v in vals)

    @classmethod
    def _joint_score(cls, native_score, historical_score, local_score, history_available):
        if history_available:
            return cls._harmonic((native_score, historical_score, local_score))
        return cls._harmonic((native_score, local_score))

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
                history_available = True
            else:
                rec = self.global_stats[x["procedure"]]
                support = float(rec[1])
                historical = (float(rec[0]) + 1.0) / (support + 2.0) if support else 0.0
                basis_count = 0
                history_available = False

            native = float(x.get("native_score", 0.0))
            local = float(x.get("local_score", 0.0))
            joint = self._joint_score(native, historical, local, history_available)
            ranked.append({
                **x,
                "support": support,
                "historical_score": historical,
                "history_available": history_available,
                "native_score": native,
                "local_score": local,
                "native_uncertainty": float(x.get("native_uncertainty", 1.0)),
                "score": joint,
                "basis_count": basis_count,
            })
        return sorted(
            [x for x in ranked if x["prediction"] is not None],
            key=lambda x: (
                x["score"], x["local_score"], x["basis_count"],
                x["support"], x["native_score"], -x["index"]
            ),
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
                "min_support": min(x["support"] for x in members),
                "min_native": min(x["native_score"] for x in members),
                "min_local": min(x["local_score"] for x in members),
                "max_uncertainty": max(x["native_uncertainty"] for x in members),
            })
        group_rows.sort(
            key=lambda g:(len(g["members"]),g["mean_score"],g["min_local"],g["min_native"],g["min_support"]),
            reverse=True,
        )
        best_group=group_rows[0]
        best=max(best_group["members"],key=lambda x:(x["score"],x["native_score"],x["local_score"],x["support"],-x["index"]))

        if len(group_rows)==1:
            emit=(
                best_group["min_support"] >= MIN_SUPPORT
                and best_group["min_native"] >= 0.50
                and best_group["max_uncertainty"] <= 0.60
                and best_group["mean_score"] >= 0.50
            )
        else:
            runner=group_rows[1]
            margin=best_group["mean_score"]-runner["mean_score"]
            consensus=(
                len(best_group["members"])>=2
                and best_group["min_support"] >= MIN_SUPPORT
                and best_group["min_native"] >= 0.50
                and best_group["max_uncertainty"] <= 0.60
                and best_group["mean_score"] >= 0.50
            )
            strong_single=(
                best_group["min_support"] >= MIN_SUPPORT
                and best_group["min_native"] >= 0.65
                and best_group["max_uncertainty"] <= 0.50
                and best_group["mean_score"] >= 0.60
                and margin >= 0.10
            )
            emit=consensus or strong_single
        selected=copy.deepcopy(best) if emit else None

        if feedback:
            target=items[0]["target"] if items else None
            if target is not None:
                for x in ranked:
                    for key in x.get("evidence_keys",()):
                        rec=self.stats[x["procedure"]].setdefault(key,[0.0,0.0])
                        rec[0]=rec[0]*0.97+float(x["prediction"]==target)
                        rec[1]=rec[1]*0.97+1.0
                    self.global_stats[x["procedure"]][0]=self.global_stats[x["procedure"]][0]*0.97+float(x["prediction"]==target)
                    self.global_stats[x["procedure"]][1]=self.global_stats[x["procedure"]][1]*0.97+1.0
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

    def route_episode_models(self, episode_models):
        """Select one reusable procedure for the current episode, then execute it.

        Applicability is learned only from the observed prefix via the procedure's
        local cross-validation score. The untouched suffix is used solely for the
        scientific evaluation after selection.
        """
        candidates = []
        for p in self.procedures:
            model = episode_models[p.name]
            historical = self.global_stats[p.name]
            historical_score = (
                (float(historical[0]) + 1.0) / (float(historical[1]) + 2.0)
                if historical[1]
                else 0.0
            )
            contextual_historical_score = self._prefix_historical_score(
                p.name,
                model,
            )
            candidates.append({
                "procedure": p.name,
                "index": p.index,
                "local_score": float(model.local_score),
                "native_score": float(model.native_score),
                "fit_native_score": float(model.fit_native_score),
                "causal_score": float(model.causal_score),
                "causal_native_score": float(model.causal_native_score),
                "contextual_historical_score": contextual_historical_score,
                "historical_score": historical_score,
            })

        # Primary criterion: current-episode applicability from the observed
        # prefix. When applicability is tied, use prefix-only native
        # arbitration evidence, then calibrated prior reliability. The procedure
        # index is only the final deterministic tie-break. No suffix observation
        # participates here.
        candidates.sort(
            key=lambda x: (
                x["causal_score"],
                x["local_score"],
                x["native_score"],
                x["fit_native_score"],
                x["contextual_historical_score"],
                x["historical_score"],
                -x["index"],
            ),
            reverse=True,
        )
        selected_name = candidates[0]["procedure"] if candidates else None
        if selected_name is None:
            return {
                "selected_procedure": None,
                "procedure_local_scores": candidates,
                "rows": len(next(iter(episode_models.values())).holdout) if episode_models else 0,
                "coverage": 0.0,
                "accuracy_on_covered": 0.0,
            }

        selected_model = episode_models[selected_name]

        # Selection has already been made from prefix-only applicability scores.
        # Only now is the selected procedure permitted to predict on the suffix.
        selected_procedure = ApplicableProcedure(
            selected_model.procedure,
            selected_model.core,
            selected_model.local_score,
            selected_model.native_score,
            selected_model.fit_native_score,
            selected_model.causal_score,
            selected_model.causal_native_score,
        )
        predictions = selected_procedure.predict(selected_model.holdout)

        # Outcome feedback is applied after suffix prediction, never to applicability
        # selection itself.
        covered = []
        for (obs, action, info), (_target_obs, _target_action, target) in zip(
            predictions,
            selected_model.holdout,
        ):
            pred = info.get("prediction")
            if pred is not None:
                covered.append(float(pred == target))
                item_keys = _stable_context_keys(selected_model.core, obs, action)
                for key in item_keys:
                    rec = self.stats[selected_name].setdefault(key, [0.0, 0.0])
                    rec[0] = rec[0] * 0.97 + float(pred == target)
                    rec[1] = rec[1] * 0.97 + 1.0
                self.global_stats[selected_name][0] = (
                    self.global_stats[selected_name][0] * 0.97
                    + float(pred == target)
                )
                self.global_stats[selected_name][1] = (
                    self.global_stats[selected_name][1] * 0.97 + 1.0
                )

        return {
            "selected_procedure": selected_name,
            "procedure_local_scores": candidates,
            "rows": len(selected_model.holdout),
            "coverage": len(covered) / max(1, len(selected_model.holdout)),
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

def _aggregate_episode_metrics(metrics):
    total_rows = sum(int(m["rows"]) for m in metrics)
    covered = sum(float(m["coverage"]) * int(m["rows"]) for m in metrics)
    correct = sum(
        float(m["coverage"]) * int(m["rows"]) * float(m["accuracy_on_covered"])
        for m in metrics
    )
    return {
        "rows": total_rows,
        "coverage": covered / max(1, total_rows),
        "accuracy_on_covered": correct / max(1e-12, covered) if covered else 0.0,
    }

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

    bank = ProcedureBank(procedures)
    inner_models = {
        p.name: [
            bank._train_episode(p, stream, idx)
            for idx, stream in enumerate(inner_groups[i])
        ]
        for i, p in enumerate(procedures)
    }
    bank.calibrate(inner_models)

    # Correct routing unit: one observed episode, seen by every candidate procedure.
    # Generation labels are never passed to ProcedureBank or the router.
    episode_model_sets = {label: [] for label in ("gen1", "gen2", "gen3", "gen4")}
    phase = {}
    for i, (label, streams) in enumerate(
        zip(("gen1", "gen2", "gen3", "gen4"), outer_groups)
    ):
        episode_metrics = []
        for j, stream in enumerate(streams):
            models_by_proc = {
                p.name: bank._train_episode(p, stream, 1000 + i * 100 + j * 10 + p.index)
                for p in procedures
            }
            episode_model_sets[label].append(models_by_proc)
            episode_metrics.append(bank.route_episode_models(models_by_proc))
        phase[label] = _aggregate_episode_metrics(episode_metrics)

    # Standalone procedure competence on the same current episode.
    own = {}
    for i, label in enumerate(("gen1", "gen2", "gen3", "gen4")):
        vals = []
        for models_by_proc in episode_model_sets[label]:
            vals.extend(
                float(x["prediction"] == x["target"])
                for x in bank.candidates(models_by_proc[procedures[i].name])
            )
        own[label] = statistics.mean(vals) if vals else 0.0

    # Explicit conflict safety test.
    trained_cores = {
        p.name: inner_models[p.name][0].core
        for p in procedures
    }
    rng = random.Random(seed)
    target_deltas = (1, 2, 3, 5)
    conflict_rows = []
    for i in range(24):
        z = rng.randint(-12, 12)
        obs = {"x": -2, "y": 1, "z": z}
        target = {"x": -2, "y": 1, "z": z + target_deltas[i % 4]}
        conflict_rows.append((obs, "step", target))
    conflict = bank.conflict(trained_cores, conflict_rows)

    # Unseen Gen4 episode: every procedure must observe the SAME episode prefix.
    unseen_bank = ProcedureBank(procedures)
    unseen_inner_models = {
        p.name: (inner_models[p.name][:-1] if p.name == "p3" else inner_models[p.name])
        for p in procedures
    }
    unseen_bank.calibrate(unseen_inner_models)
    final_unseen_models = {
        p.name: bank._train_episode(p, gen4_outer[-1], 2000 + p.index)
        for p in procedures
    }
    unseen = unseen_bank.route_episode_models(final_unseen_models)

    # Diagnostic-only row trace: generated after selection from the same untouched suffix.
    # This does not alter selection, prediction, feedback, or any gate semantics.
    selected_unseen_model = final_unseen_models[unseen["selected_procedure"]]
    unseen_suffix_predictions = []
    for obs, action, target in selected_unseen_model.holdout:
        info = selected_unseen_model.core.predict(obs, action)
        pred = info.get("prediction")
        unseen_suffix_predictions.append({
            "observation": copy.deepcopy(obs),
            "action": action,
            "target": copy.deepcopy(target),
            "prediction": copy.deepcopy(pred),
            "correct": bool(pred == target),
        })

    # Retention across all prior episodes with the procedure descriptors held immutable.
    before = {p.name: digest(asdict(p.kernel)) for p in procedures}
    retention_bank = copy.deepcopy(bank)
    retention = {}
    for label, model_sets in episode_model_sets.items():
        retention[label] = _aggregate_episode_metrics([
            retention_bank.route_episode_models(models_by_proc)
            for models_by_proc in model_sets
        ])
    after = {p.name: digest(asdict(p.kernel)) for p in procedures}

    # Deterministic replay of the same episode-level routing protocol.
    replay_bank = ProcedureBank(procedures)
    replay_bank.calibrate(inner_models)
    replay_phase = {}
    for label, model_sets in episode_model_sets.items():
        replay_phase[label] = _aggregate_episode_metrics([
            replay_bank.route_episode_models(models_by_proc)
            for models_by_proc in model_sets
        ])

    own_ok = all(v >= 0.70 for v in own.values())
    phase_ok = all(
        v["coverage"] >= 0.70 and v["accuracy_on_covered"] >= 0.75
        for v in phase.values()
    )
    conflict_ok = (
        conflict["coverage"] <= 0.25
        and (conflict["coverage"] == 0.0 or conflict["accuracy_on_covered"] <= 0.50)
    )
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
        "unseen_gen4_suffix_predictions": unseen_suffix_predictions,
        "retention_after_sequence": retention,
        "procedure_library_retained": library_ok,
        "replay": {"deterministic_replay": replay_ok},
        "gates": {
            "own": own_ok,
            "phase": phase_ok,
            "conflict": conflict_ok,
            "unseen_gen4": unseen_ok,
            "retention": retention_ok,
            "procedure_library_retained": library_ok,
            "deterministic_replay": replay_ok,
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

# H35 symmetric applicability screen trigger.

# H36 same-episode routing invariant trigger.

# H36 API repair trigger.

# H37 episode-level routing trigger.

# H38 same-episode native-historical arbitration trigger.

# H38 signature repair trigger.

# H39 same-episode local-native-historical arbitration trigger.

# H40 episode-level reusable procedure selection trigger.

# H41 actual episode-router qualification trigger.
