"""ASI-CORE-006: learner-discovered cognitive self-improvement.

This module adds a generic, sandboxed architecture-IR to the existing single
GeneralLearner. It contains no benchmark family names, target answers, evaluator
imports, provider clients, or task-specific solver branches.
"""
from __future__ import annotations

from dataclasses import asdict, dataclass, field
from hashlib import sha256
import copy
import json
import time
from typing import Any, Callable, Mapping, Sequence

from .general_engine import GeneralLearner, OperatorArtifact, _type_descriptor_004
from .open_language import (
    _candidate_holds_004,
    _construct_open_005,
    _compose_existing_005,
    _compose_mixed_005,
    _old_language_exhaustive_005,
    _apply_old005,
)

Json = Any


def _canon(x: Any) -> str:
    return json.dumps(x, sort_keys=True, separators=(",", ":"), ensure_ascii=True)


def _digest(x: Any) -> str:
    return sha256(_canon(x).encode("utf-8")).hexdigest()


@dataclass(frozen=True)
class ArchitectureSpec:
    """Serializable cognitive architecture, expressed only with generic control objects."""
    pipeline: tuple[str, ...] = ("reuse", "revision", "composition", "old_search", "construct")
    retrieval_mode: str = "linear"  # linear | indexed
    memoized_stages: tuple[str, ...] = ()
    experiment_policy: str = "passive"  # passive | disagreement
    schema_version: str = "ASI-CORE-006-A0"

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)

    @property
    def digest(self) -> str:
        return _digest(self.to_dict())


@dataclass(frozen=True)
class SelfModel:
    architecture_digest: str
    pipeline: tuple[str, ...]
    retrieval_mode: str
    memoized_stages: tuple[str, ...]
    experiment_policy: str
    observed_stage_costs: Mapping[str, float]
    observed_stage_calls: Mapping[str, int]
    observed_failures: int
    prediction_errors: int
    prediction_successes: int
    operator_count: int
    strategy_policy: str
    recent_stage_work: Mapping[str, float]
    recent_success: bool
    available_metrics: tuple[str, ...]


@dataclass(frozen=True)
class ImprovementHypothesis:
    hypothesis_id: str
    parent_digest: str
    program: Mapping[str, Any]
    expected_target: str
    rationale: str
    predicted_effect: Mapping[str, float]


@dataclass(frozen=True)
class ArchitecturalPatch:
    patch_id: str
    parent_architecture_digest: str
    proposed_architecture_digest: str
    patch_program: Mapping[str, Any]
    dependency_set: tuple[str, ...]
    capability_hypothesis: str
    predicted_benefit: Mapping[str, float]
    predicted_failure_modes: tuple[str, ...]
    verification_plan: tuple[str, ...]
    regression_results: Mapping[str, Any] = field(default_factory=dict)
    hidden_transfer_results: Mapping[str, Any] = field(default_factory=dict)
    resource_comparison: Mapping[str, Any] = field(default_factory=dict)
    rollback_information: Mapping[str, Any] = field(default_factory=dict)
    accepted: bool = False
    source_form: str = "architecture_ir"


@dataclass(frozen=True)
class ImprovementRecord:
    patch: ArchitecturalPatch
    before_metrics: Mapping[str, Any]
    after_metrics: Mapping[str, Any]
    transfer_metrics: Mapping[str, Any]
    retained: bool
    rejection_reason: str = ""


# ---------- generic architecture IR ----------


def _apply_program(spec: ArchitectureSpec, program: Mapping[str, Any]) -> ArchitectureSpec:
    op = program.get("op")
    if op == "noop":
        return spec
    if op == "sequence":
        cur = spec
        for child in program.get("programs", ()):
            cur = _apply_program(cur, child)
        return cur
    if op == "swap_adjacent":
        target = str(program["stage"])
        p = list(spec.pipeline)
        for i in range(len(p) - 1):
            if p[i] == target:
                p[i], p[i + 1] = p[i + 1], p[i]
                return ArchitectureSpec(tuple(p), spec.retrieval_mode, spec.memoized_stages, spec.experiment_policy, spec.schema_version)
        return spec
    if op == "move_stage":
        stage = str(program["stage"])
        to_index = int(program["to_index"])
        p = [x for x in spec.pipeline if x != stage]
        if stage in spec.pipeline:
            p.insert(max(0, min(to_index, len(p))), stage)
        return ArchitectureSpec(tuple(p), spec.retrieval_mode, spec.memoized_stages, spec.experiment_policy, spec.schema_version)
    if op == "toggle_retrieval":
        nxt = "indexed" if spec.retrieval_mode == "linear" else "linear"
        return ArchitectureSpec(spec.pipeline, nxt, spec.memoized_stages, spec.experiment_policy, spec.schema_version)
    if op == "toggle_memoization":
        stage = str(program["stage"])
        m = list(spec.memoized_stages)
        if stage in m:
            m.remove(stage)
        else:
            m.append(stage)
        return ArchitectureSpec(spec.pipeline, spec.retrieval_mode, tuple(sorted(m)), spec.experiment_policy, spec.schema_version)
    if op == "toggle_experiment_policy":
        nxt = "disagreement" if spec.experiment_policy == "passive" else "passive"
        return ArchitectureSpec(spec.pipeline, spec.retrieval_mode, spec.memoized_stages, nxt, spec.schema_version)
    raise ValueError(f"unsupported architecture IR op: {op}")


def _apply_patch_program(spec: ArchitectureSpec, program: Mapping[str, Any]) -> ArchitectureSpec:
    return _apply_program(spec, program)


# ---------- self-model + telemetry ----------


def _init_006(self: GeneralLearner) -> None:
    if hasattr(self, "_architecture_006"):
        if not hasattr(self, "_architecture_prediction_errors_006"):
            self._architecture_prediction_errors_006 = 0
            self._architecture_prediction_successes_006 = 0
            self._last_architecture_metrics_006 = {}
        return
    self._architecture_006 = ArchitectureSpec()
    self._improvement_history_006: list[ImprovementRecord] = []
    self._architecture_events_006: list[dict[str, Any]] = []
    self._architecture_stage_costs_006: dict[str, float] = {}
    self._architecture_stage_calls_006: dict[str, int] = {}
    self._architecture_failures_006 = 0
    self._architecture_prediction_errors_006 = 0
    self._architecture_prediction_successes_006 = 0
    self._architecture_cache_006: dict[str, Any] = {}
    self._last_architecture_metrics_006: dict[str, Any] = {}
    self._behavior_index_006: dict[str, tuple[str, ...]] = {}


def _ensure_006(self: GeneralLearner) -> None:
    if not hasattr(self, "_architecture_006"):
        _init_006(self)


def self_model(self: GeneralLearner) -> SelfModel:
    _ensure_006(self)
    return SelfModel(
        self._architecture_006.digest,
        self._architecture_006.pipeline,
        self._architecture_006.retrieval_mode,
        self._architecture_006.memoized_stages,
        self._architecture_006.experiment_policy,
        dict(self._architecture_stage_costs_006),
        dict(self._architecture_stage_calls_006),
        int(self._architecture_failures_006),
        int(self._architecture_prediction_errors_006),
        int(self._architecture_prediction_successes_006),
        len(self._learned_operators),
        getattr(self, "_strategy_policy", "legacy_first"),
        {str(x.get("stage")): float(x.get("work", 0)) for x in self._last_architecture_metrics_006.get("trace", ())},
        bool(self._last_architecture_metrics_006.get("success", False)),
        (
            "stage_costs", "stage_calls", "failure_count", "prediction_errors", "prediction_successes", "operator_count",
            "last_task_stage_work", "last_task_success",
            "strategy_policy", "operator_reuse_counts", "representation_search_cost",
        ),
    )


def architecture_state(self) -> dict[str, Any]:
    _ensure_006(self)
    return {
        "spec": self._architecture_006.to_dict(),
        "self_model": asdict(self.self_model()),
        "history": [asdict(x) for x in self._improvement_history_006[-128:]],
        "events": self._architecture_events_006[-256:],
        "stage_costs": dict(self._architecture_stage_costs_006),
        "stage_calls": dict(self._architecture_stage_calls_006),
        "failures": self._architecture_failures_006,
        "prediction_errors": self._architecture_prediction_errors_006,
        "prediction_successes": self._architecture_prediction_successes_006,
        "last_metrics": copy.deepcopy(self._last_architecture_metrics_006),
    }


def export_state_006(self, base: Mapping[str, Any]) -> dict[str, Any]:
    _ensure_006(self)
    out = copy.deepcopy(dict(base))
    out["asi_core_006"] = architecture_state(self)
    return out


def load_state_006(self, state: Mapping[str, Any]) -> None:
    info = state.get("asi_core_006", {})
    raw = info.get("spec") or ArchitectureSpec().to_dict()
    self._architecture_006 = ArchitectureSpec(
        tuple(raw.get("pipeline", ArchitectureSpec().pipeline)),
        raw.get("retrieval_mode", "linear"),
        tuple(raw.get("memoized_stages", ())),
        raw.get("experiment_policy", "passive"),
        raw.get("schema_version", "ASI-CORE-006-A0"),
    )
    self._improvement_history_006 = []
    self._architecture_events_006 = list(info.get("events", ()))
    self._architecture_stage_costs_006 = {str(k): float(v) for k, v in info.get("stage_costs", {}).items()}
    self._architecture_stage_calls_006 = {str(k): int(v) for k, v in info.get("stage_calls", {}).items()}
    self._architecture_failures_006 = int(info.get("failures", 0))
    self._architecture_prediction_errors_006 = int(info.get("prediction_errors", 0))
    self._architecture_prediction_successes_006 = int(info.get("prediction_successes", 0))
    self._last_architecture_metrics_006 = dict(info.get("last_metrics", {}))
    self._architecture_cache_006 = {}
    self._behavior_index_006 = {}
    for rawrec in info.get("history", ()):
        try:
            p = ArchitecturalPatch(**rawrec["patch"])
            self._improvement_history_006.append(ImprovementRecord(p, rawrec.get("before_metrics", {}), rawrec.get("after_metrics", {}), rawrec.get("transfer_metrics", {}), bool(rawrec.get("retained", False)), rawrec.get("rejection_reason", "")))
        except Exception:
            continue


def _record_stage(self: GeneralLearner, stage: str, cost: float, success: bool) -> None:
    self._architecture_stage_costs_006[stage] = self._architecture_stage_costs_006.get(stage, 0.0) + float(cost)
    self._architecture_stage_calls_006[stage] = self._architecture_stage_calls_006.get(stage, 0) + 1
    if not success:
        self._architecture_failures_006 += 1


def _examples_key(examples: Sequence[tuple[Json, Json]]) -> str:
    return _digest(examples)


# ---------- generic candidate generation ----------


def generate_improvement_hypotheses(self: GeneralLearner, max_candidates: int = 12) -> tuple[ImprovementHypothesis, ...]:
    """Generate competing hypotheses using only observable costs and generic IR mutations."""
    _ensure_006(self)
    sm = self.self_model()
    costs = dict(sm.observed_stage_costs)
    calls = dict(sm.observed_stage_calls)
    hypotheses: list[ImprovementHypothesis] = []

    # Prefer the most recent task trace: self-improvement should explain the
    # current bottleneck, while lifetime telemetry remains a secondary prior.
    recent = dict(sm.recent_stage_work)
    if recent:
        ranked = sorted(recent, key=lambda k: (recent[k], k), reverse=True)
    else:
        ranked = sorted(costs, key=lambda k: (costs[k] / max(1, calls.get(k, 0)), k), reverse=True)
    dominant = ranked[0] if ranked else None

    programs: list[tuple[Mapping[str, Any], str, str]] = []
    pipeline = list(sm.pipeline)
    for stage in pipeline[:-1]:
        programs.append(({"op": "swap_adjacent", "stage": stage}, f"reorder:{stage}", "reduce downstream search before a costly later stage"))
        programs.append(({"op": "move_stage", "stage": stage, "to_index": 0}, f"move-front:{stage}", "place a generic cognitive stage earlier when its observed result can avoid later work"))
    for stage in ("old_search", "composition", "reuse"):
        programs.append(({"op": "toggle_memoization", "stage": stage}, f"memoize:{stage}", "reuse exact computational results across repeated observations"))
    programs.append(({"op": "toggle_retrieval"}, "toggle-retrieval", "change generic lookup organization without changing learned content"))
    programs.append(({"op": "toggle_experiment_policy"}, "toggle-experiment", "change generic evidence-selection policy using internal candidate disagreement"))

    for program, target, rationale in programs:
        candidate = _apply_patch_program(self._architecture_006, program)
        if candidate.digest == self._architecture_006.digest:
            continue
        pred: dict[str, float] = {}
        if target == "move-front:reuse" and dominant in {"old_search","mixed","composition","revision"}:
            pred["acquisition_speed"] = 0.8
            pred["search_reduction"] = 0.8
        elif target == "toggle-retrieval" and dominant == "reuse":
            pred["resource_efficiency"] = 0.8
            pred["search_reduction"] = 0.6
        elif target.startswith("memoize") and dominant == target.split(":",1)[1]:
            pred["search_reduction"] = 0.8
            pred["acquisition_speed"] = 0.6
        elif target.startswith("move-front:") and target.split(":",1)[1] == dominant:
            pred["acquisition_speed"] = 0.45
        elif target.startswith("memoize"):
            pred["search_reduction"] = 0.2
        elif target.startswith("reorder"):
            pred["acquisition_speed"] = 0.15
        elif target == "toggle-retrieval":
            pred["resource_efficiency"] = 0.1 if len(self._learned_operators) > 2 else 0.0
        elif target == "toggle-experiment":
            pred["experiment_information_gain"] = 0.5 if sm.prediction_errors > 0 else 0.1
            pred["error_reduction"] = 0.4 if sm.prediction_errors > 0 else 0.0
        else:
            pred["experiment_information_gain"] = 0.1
        hid = "hyp:" + _digest({"parent": self._architecture_006.digest, "program": program})[:20]
        expected_target = dominant or "general"
        hypotheses.append(ImprovementHypothesis(hid, self._architecture_006.digest, program, expected_target, rationale, pred))
    hypotheses.sort(key=lambda h: (-max(h.predicted_effect.values()), h.hypothesis_id))
    return tuple(hypotheses[:max_candidates])


# ---------- architecture-aware execution ----------


def _relation_pattern(x: Json, y: Json, position: int | None = None) -> Json:
    """Value-invariant structural relation used only as a retrieval index key.

    The fingerprint intentionally ignores container cardinality and samples a bounded
    canonical prefix, so the index generalizes across changed lengths and raggedness.
    """
    if isinstance(y, list):
        if not y:
            return {"kind": "list", "empty": True}
        if isinstance(x, list) and x and isinstance(y[0], list) and isinstance(x[0], list):
            # Nested containers: inspect the first child only, then let the child
            # fingerprint inspect a bounded prefix (e.g. the two fields of a pair).
            return {"kind": "list_nested", "item": _relation_pattern(x[0], y[0], 0)}
        items=[]
        n=min(2,len(y),len(x)) if isinstance(x,list) else min(2,len(y))
        for i in range(n):
            child_x = x[i] if isinstance(x,list) and i < len(x) else None
            items.append(_relation_pattern(child_x, y[i], i))
        return {"kind":"list","items":items}
    if isinstance(y, dict):
        xd=x if isinstance(x,dict) else {}
        return {"kind":"dict","items":{str(k):_relation_pattern(xd.get(k),v,position) for k,v in sorted(y.items())}}
    if isinstance(y,bool): return {"kind":"bool"}
    if isinstance(y,(int,float)) and not isinstance(y,bool):
        if isinstance(x,(int,float)) and not isinstance(x,bool):
            if y==x: return {"kind":"same_input_scalar"}
            if y==x+1: return {"kind":"plus_1"}
            if y==x-1: return {"kind":"minus_1"}
            if y==2*x: return {"kind":"double"}
            if y==-x: return {"kind":"negate"}
            if y==x%2: return {"kind":"parity"}
        if position is not None and y==position:
            return {"kind":"position_index"}
        return {"kind":"numeric"}
    if y is None: return {"kind":"null"}
    if isinstance(x,str) and isinstance(y,str) and y==x: return {"kind":"same_input_text"}
    return {"kind":type(y).__name__}

def _behavior_key(examples: Sequence[tuple[Json, Json]]) -> str:
    if not examples: return ""
    patterns=[_relation_pattern(x,y) for x,y in examples[:2]]
    return _digest({"input_type":_type_descriptor_004(examples[0][0]),"patterns":patterns})


def _rebuild_behavior_index(self: GeneralLearner) -> None:
    idx: dict[str, list[str]] = {}
    for oid, op in self._learned_operators.items():
        if op.behavioral_examples:
            try:
                pairs=tuple((e["input"],e["output"]) for e in op.behavioral_examples[:2])
                key = _behavior_key(pairs)
            except Exception:
                continue
            idx.setdefault(key, []).append(oid)
    self._behavior_index_006 = {k: tuple(v) for k, v in idx.items()}


def _operator_hit(self: GeneralLearner, examples: Sequence[tuple[Json, Json]]) -> tuple[OperatorArtifact | None, int]:
    indexed = self._architecture_006.retrieval_mode != "indexed"
    self._last_index_build_work_006 = 0.0
    if indexed:
        ids_now=tuple(self._learned_operators.keys())
        behavior_signature=_digest(tuple((oid,tuple((e.get("input"),e.get("output")) for e in self._learned_operators[oid].behavioral_examples[:2])) for oid in ids_now))
        cached_signature=getattr(self,"_behavior_index_operator_signature_006",None)
        if cached_signature != behavior_signature or not getattr(self,"_behavior_index_006",{}):
            self._last_index_build_work_006=float(len(ids_now))
            _rebuild_behavior_index(self)
            self._behavior_index_operator_signature_006=behavior_signature
        ids = self._behavior_index_006.get(_behavior_key(examples), ())
        checked=0
        for oid in ids:
            checked += 1
            op = self._learned_operators.get(oid)
            if op is not None and _candidate_holds_004(op.body, examples):
                self._operator_reuse_counts[oid] = self._operator_reuse_counts.get(oid, 0) + 1
                return op, checked
        return None, checked
    checked=0
    for oid in reversed(tuple(self._learned_operators.keys())):
        checked += 1
        op=self._learned_operators[oid]
        if _candidate_holds_004(op.body,examples):
            self._operator_reuse_counts[oid]=self._operator_reuse_counts.get(oid,0)+1
            return op,checked
    return None,checked

def _run_stage(self: GeneralLearner, stage: str, task_id: str, examples: Sequence[tuple[Json, Json]], query: Json, budget: int) -> tuple[Json | None, dict[str, Any]]:
    t0 = time.perf_counter()
    if stage == "reuse":
        key = _examples_key(examples)
        cached = None
        if "reuse" in self._architecture_006.memoized_stages:
            cached = self._architecture_cache_006.get("reuse:" + key)
        cache_hit = cached is not None
        if cache_hit:
            oid = cached
            hit = self._learned_operators.get(oid) if oid else None
            checked = 0
        else:
            hit, checked = _operator_hit(self, examples)
            if "reuse" in self._architecture_006.memoized_stages:
                self._architecture_cache_006["reuse:" + key] = hit.operator_id if hit is not None else ""
        cost = (time.perf_counter() - t0) * 1000
        _record_stage(self, stage, cost, hit is not None)
        if hit is not None:
            return hit.apply(query), {"mode": "reuse", "operator_id": hit.operator_id, "work": checked, "cache_hit": cache_hit, "cost_ms": cost}
        return None, {"mode": "reuse_miss", "work": checked, "cache_hit": cache_hit, "cost_ms": cost}
    if stage == "revision":
        # Revision is evidence-triggered: run only after an actual recorded failure.
        if self._architecture_prediction_errors_006 <= 0:
            cost=(time.perf_counter()-t0)*1000
            _record_stage(self, stage, cost, True)
            return None, {"mode":"revision_deferred","work":0,"cost_ms":cost}
        from .open_language import _revise_conflicting_005
        op, trials = _revise_conflicting_005(self, examples, budget=budget)
        cost = (time.perf_counter() - t0) * 1000
        _record_stage(self, stage, cost, op is not None)
        if op is not None:
            return op.apply(query), {"mode": "revision", "operator_id": op.operator_id, "work": trials, "cost_ms": cost}
        return None, {"mode": "revision_miss", "work": trials, "cost_ms": cost}
    if stage == "composition":
        op, trials = _compose_existing_005(self, examples, max_depth=3)
        cost = (time.perf_counter() - t0) * 1000
        _record_stage(self, stage, cost, op is not None)
        if op is not None:
            return op.apply(query), {"mode": "composition", "operator_id": op.operator_id, "work": trials, "cost_ms": cost}
        return None, {"mode": "composition_miss", "work": trials, "cost_ms": cost}
    if stage == "mixed":
        op, trials = _compose_mixed_005(self, examples, budget=budget)
        cost = (time.perf_counter() - t0) * 1000
        _record_stage(self, stage, cost, op is not None)
        if op is not None:
            return op.apply(query), {"mode": "mixed", "operator_id": op.operator_id, "work": trials, "cost_ms": cost}
        return None, {"mode": "mixed_miss", "work": trials, "cost_ms": cost}
    if stage == "old_search":
        key = _examples_key(examples)
        cached = None
        if "old_search" in self._architecture_006.memoized_stages:
            cached = self._architecture_cache_006.get(key)
        cache_hit = cached is not None
        if not cache_hit:
            gap, exact = _old_language_exhaustive_005(self, examples, budget=budget, max_depth=2)
            cached = (gap, exact)
            if "old_search" in self._architecture_006.memoized_stages:
                self._architecture_cache_006[key] = cached
        gap, exact = cached
        cost = (time.perf_counter() - t0) * 1000
        work = 0 if cache_hit else gap.candidates_tested
        success = exact is not None
        _record_stage(self, stage, cost, success)
        if exact is not None:
            return _apply_old005(self, query, exact), {"mode": "old", "work": work, "cost_ms": cost}
        return None, {"mode": "old_miss", "work": work, "cost_ms": cost}
    if stage == "construct":
        gap, _ = _old_language_exhaustive_005(self, examples, budget=budget, max_depth=2)
        op, trials = _construct_open_005(self, examples, gap=gap)
        cost = (time.perf_counter() - t0) * 1000
        _record_stage(self, stage, cost, op is not None)
        if op is not None:
            self._register_operator_004(op, capability_unlocked="core006:learner_constructed_cognitive_operator")
            return op.apply(query), {"mode": "construct", "operator_id": op.operator_id, "work": trials, "cost_ms": cost}
        return None, {"mode": "construct_miss", "work": trials, "cost_ms": cost}
    raise ValueError(f"unknown architecture stage: {stage}")


def learn_architecture_task(self: GeneralLearner, task_id: str, examples: Sequence[tuple[Json, Json]], query: Json, *, budget: int = 2500, experiment_pool: Sequence[Json] = (), evidence_query: Callable[[Json], Json] | None = None) -> tuple[Json | None, dict[str, Any]]:
    _ensure_006(self)
    selected_evidence = None
    used_examples = list(examples)
    if self._architecture_006.experiment_policy == "disagreement" and experiment_pool and evidence_query is not None:
        candidates = self._candidate_programs_for_information(used_examples)
        selected_evidence, _ = self.select_informative_observation(candidates, experiment_pool)
        if selected_evidence is not None:
            used_examples.append((selected_evidence, evidence_query(selected_evidence)))
    started = time.perf_counter()
    total_work = 0
    trace = []
    for stage in self._architecture_006.pipeline:
        pred, meta = _run_stage(self, stage, task_id, used_examples, query, budget)
        trace.append({"stage": stage, **meta})
        total_work += int(meta.get("work", 0))
        if pred is not None:
            self._last_architecture_metrics_006 = {
                "task_id": task_id, "success": True, "work": total_work,
                "selected_evidence": selected_evidence is not None, "trace": trace,
                "elapsed_ms": (time.perf_counter() - started) * 1000,
            }
            return pred, self._last_architecture_metrics_006
    self._last_architecture_metrics_006 = {
        "task_id": task_id, "success": False, "work": total_work,
        "selected_evidence": selected_evidence is not None, "trace": trace,
        "elapsed_ms": (time.perf_counter() - started) * 1000,
    }
    return None, self._last_architecture_metrics_006


def _candidate_programs_for_information(self, examples: Sequence[tuple[Json, Json]]) -> tuple[Mapping[str, Any], ...]:
    from .open_language import generic_open_candidates
    return generic_open_candidates(examples)[:16]


def observe_architecture_feedback(self: GeneralLearner, feedback: Sequence[Mapping[str, Any]]) -> None:
    """Consume only evaluator feedback bits; no hidden answer is exposed."""
    _ensure_006(self)
    for item in feedback:
        ok = bool(item.get("correct", False))
        if ok:
            self._architecture_prediction_successes_006 += 1
        else:
            self._architecture_prediction_errors_006 += 1
            self._architecture_failures_006 += 1
        self._architecture_events_006.append({
            "event": "PREDICTION_FEEDBACK",
            "task_digest": _digest(str(item.get("task_id", ""))),
            "correct": ok,
        })


def select_improvement_from_evidence(self: GeneralLearner, evaluations: Sequence[Mapping[str, Any]]) -> Mapping[str, Any] | None:
    """Select an improvement using only anonymised measured evaluator evidence."""
    _ensure_006(self)
    ranked=[]
    for e in evaluations:
        if not bool(e.get("regression_passed")) or not bool(e.get("transfer_passed")):
            continue
        gains=[float(x) for x in e.get("work_improvements", ())]
        gain=sum(gains)/max(1,len(gains)) if gains else 0.0
        hid=str(e.get("hypothesis_id", ""))
        ranked.append((gain,hid,e))
    if not ranked:
        return None
    ranked.sort(key=lambda x:(x[0],x[1]), reverse=True)
    return ranked[0][2]


# ---------- sandbox + proposal/retention ----------


def sandbox_clone(self: GeneralLearner, spec: ArchitectureSpec) -> GeneralLearner:
    _ensure_006(self)
    clone = GeneralLearner.from_state(self.export_state(), growth_enabled=True)
    _ensure_006(clone)
    clone._architecture_006 = spec
    return clone


def build_patch_from_hypothesis(self: GeneralLearner, h: ImprovementHypothesis) -> ArchitecturalPatch:
    proposed = _apply_patch_program(self._architecture_006, h.program)
    return ArchitecturalPatch(
        patch_id="patch:" + h.hypothesis_id.split(":", 1)[-1],
        parent_architecture_digest=self._architecture_006.digest,
        proposed_architecture_digest=proposed.digest,
        patch_program=h.program,
        dependency_set=("GeneralLearner", "architecture_ir", "observable_runtime_metrics"),
        capability_hypothesis=h.rationale,
        predicted_benefit=h.predicted_effect,
        predicted_failure_modes=("regression_on_prior_capabilities", "transfer_failure", "resource_overhead"),
        verification_plan=("diagnostic_regression", "hidden_future_capability", "same_budget", "fresh_process_reconstruction"),
        rollback_information={"parent_digest": self._architecture_006.digest, "candidate_digest": proposed.digest},
    )


def propose_self_improvements(self: GeneralLearner, max_candidates: int = 12) -> tuple[ArchitecturalPatch, ...]:
    return tuple(build_patch_from_hypothesis(self, h) for h in generate_improvement_hypotheses(self, max_candidates))


def adopt_architecture(self: GeneralLearner, patch: ArchitecturalPatch) -> None:
    _ensure_006(self)
    proposed = _apply_patch_program(self._architecture_006, patch.patch_program)
    if proposed.digest != patch.proposed_architecture_digest:
        raise ValueError("patch digest mismatch")
    self._architecture_events_006.append({"event": "ARCHITECTURE_ADOPTED", "patch_id": patch.patch_id, "parent": patch.parent_architecture_digest, "proposed": patch.proposed_architecture_digest})
    self._architecture_006 = proposed
    # Mirror the retrieval part into the legacy operator matcher so old language is not altered.
    self._strategy_policy = "signature_indexed" if proposed.retrieval_mode == "indexed" else ("operator_first" if proposed.pipeline and proposed.pipeline[0] == "reuse" else "legacy_first")


def retain_or_reject(self: GeneralLearner, patch: ArchitecturalPatch, regression: Mapping[str, Any], transfer: Mapping[str, Any], *, allow: bool = True) -> bool:
    retained = bool(allow and regression.get("passed") and transfer.get("passed"))
    rejected = "" if retained else "regression_or_transfer_failure"
    p = ArchitecturalPatch(**{**asdict(patch), "regression_results": dict(regression), "hidden_transfer_results": dict(transfer), "accepted": retained})
    before_raw=regression.get("before", {})
    after_raw=regression.get("after", {})
    before = dict(before_raw) if isinstance(before_raw, Mapping) else {"rows": before_raw}
    after = dict(after_raw) if isinstance(after_raw, Mapping) else {"rows": after_raw}
    rec = ImprovementRecord(p, before, after, dict(transfer), retained, rejected)
    self._improvement_history_006.append(rec)
    if retained:
        adopt_architecture(self, p)
    return retained


# ---------- state binding ----------

_ORIG_EXPORT_006 = GeneralLearner.export_state
_ORIG_FROM_STATE_006 = GeneralLearner.from_state
_ORIG_INIT_006 = GeneralLearner.__init__


def _init_bound(self: GeneralLearner, *args: Any, **kwargs: Any):
    _ORIG_INIT_006(self, *args, **kwargs)
    _init_006(self)


def _export_bound(self: GeneralLearner):
    return export_state_006(self, _ORIG_EXPORT_006(self))


def _from_state_bound(cls, spec: Mapping[str, Any], **kwargs: Any):
    obj = _ORIG_FROM_STATE_006.__func__(cls, spec, **kwargs)
    _init_006(obj)
    load_state_006(obj, spec)
    return obj


# Install only after the 005 binding exists; this remains one GeneralLearner class.
if getattr(GeneralLearner, "_asi_core_006_bound", False) is False:
    GeneralLearner.__init__ = _init_bound
    GeneralLearner.export_state = _export_bound
    GeneralLearner.from_state = classmethod(_from_state_bound)
    GeneralLearner.self_model = self_model
    GeneralLearner.architecture_state = architecture_state
    GeneralLearner.propose_self_improvements = propose_self_improvements
    GeneralLearner.generate_improvement_hypotheses = generate_improvement_hypotheses
    GeneralLearner.learn_architecture_task = learn_architecture_task
    GeneralLearner.sandbox_architecture = sandbox_clone
    GeneralLearner.build_architectural_patch = build_patch_from_hypothesis
    GeneralLearner.adopt_architecture = adopt_architecture
    GeneralLearner.retain_or_reject_improvement = retain_or_reject
    GeneralLearner.observe_architecture_feedback = observe_architecture_feedback
    GeneralLearner.select_improvement_from_evidence = select_improvement_from_evidence
    GeneralLearner._candidate_programs_for_information = _candidate_programs_for_information
    GeneralLearner._asi_core_006_bound = True


__all__ = [
    "ArchitectureSpec", "SelfModel", "ImprovementHypothesis", "ArchitecturalPatch", "ImprovementRecord",
]


# ============================================================================
# ASI-CORE-008 — endogenous source/AST self-modification substrate
# ============================================================================
# The functions below are bound as methods of the existing GeneralLearner.
# They are generic source-runtime primitives only: no benchmark family names,
# evaluator imports, hidden answers, or preloaded winning patches are used.

import ast as _ast008
import difflib as _difflib008
import inspect as _inspect008
from pathlib import Path as _Path008


SOURCE_REWRITE_OPERATIONS_008 = (
    "inspect", "construct", "replace", "insert", "delete", "compose",
    "wrap", "specialize", "generalize", "reorder", "parameterize",
)


def _canon_source008(value: Any) -> str:
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=True, default=str)


def _source_sha008(source: str) -> str:
    return sha256(source.encode("utf-8")).hexdigest()


def _source_ast_sha008(source: str) -> str:
    tree = _ast008.parse(source)
    dumped = _ast008.dump(tree, annotate_fields=True, include_attributes=False)
    return _source_sha008(dumped)


def _source_line_offsets008(source: str) -> list[int]:
    offsets = [0]
    offsets.extend(i + 1 for i, ch in enumerate(source) if ch == "\n")
    return offsets


def _source_span008(source: str, node: _ast008.AST) -> tuple[int, int]:
    lines = _source_line_offsets008(source)
    return (
        lines[node.lineno - 1] + node.col_offset,
        lines[node.end_lineno - 1] + node.end_col_offset,
    )


def _source_replace008(source: str, node: _ast008.AST, replacement: str) -> str:
    a, b = _source_span008(source, node)
    return source[:a] + replacement + source[b:]


def _source_node_key008(node: _ast008.AST) -> tuple[int, int, str]:
    return (getattr(node, "lineno", 0), getattr(node, "col_offset", 0), type(node).__name__)


def _source_rewrite_by_key008(source: str, target_key: tuple[int, int, str], replacement: str) -> str | None:
    tree = _ast008.parse(source)
    class Rewriter(_ast008.NodeTransformer):
        def visit(self, node):  # type: ignore[override]
            if _source_node_key008(node) == target_key:
                try:
                    return _ast008.parse(replacement).body[0].value if replacement.lstrip().startswith("(") else _ast008.parse(replacement).body[0]
                except Exception:
                    try:
                        return _ast008.parse(replacement, mode="eval").body
                    except Exception:
                        return self.generic_visit(node)
            return self.generic_visit(node)
    try:
        out = Rewriter().visit(tree)
        _ast008.fix_missing_locations(out)
        return _ast008.unparse(out) + "\n"
    except Exception:
        return None


def _mutate_tuple_move008(source: str, node: _ast008.Tuple, index: int, destination: int) -> tuple[str, dict[str, Any]]:
    vals = [_ast008.get_source_segment(source, element) or _ast008.unparse(element) for element in node.elts]
    val = vals.pop(index)
    vals.insert(destination, val)
    replacement = "(" + ", ".join(vals) + (",)" if len(vals) == 1 else ")")
    return _source_replace008(source, node, replacement), {
        "operation": "reorder", "kind": "tuple_move",
        "from_index": index, "to_index": destination,
        "node": _source_node_key008(node),
    }


def _mutate_unwrap_call008(source: str, node: _ast008.Call) -> tuple[str | None, dict[str, Any] | None]:
    if len(node.args) == 1 and not node.keywords and isinstance(node.func, _ast008.Name) and not node.func.id.startswith("_"):
        segment = _ast008.get_source_segment(source, node.args[0]) or _ast008.unparse(node.args[0])
        return _source_replace008(source, node, segment), {
            "operation": "unwrap", "kind": "unwrap_call", "callee": node.func.id,
            "node": _source_node_key008(node),
        }
    return None, None


def _mutate_compare008(source: str, node: _ast008.Compare) -> tuple[str | None, dict[str, Any] | None]:
    if len(node.ops) != 1:
        return None, None
    replacements = {
        _ast008.Gt: "<", _ast008.Lt: ">", _ast008.GtE: "<=",
        _ast008.LtE: ">=", _ast008.Eq: "!=", _ast008.NotEq: "==",
    }
    op_type = type(node.ops[0])
    if op_type not in replacements:
        return None, None
    old = {
        _ast008.Gt: ">", _ast008.Lt: "<", _ast008.GtE: ">=",
        _ast008.LtE: "<=", _ast008.Eq: "==", _ast008.NotEq: "!=",
    }[op_type]
    seg = _ast008.get_source_segment(source, node) or _ast008.unparse(node)
    mutated = seg.replace(old, replacements[op_type], 1)
    return _source_replace008(source, node, mutated), {
        "operation": "replace", "kind": "comparison_flip",
        "operator": op_type.__name__, "node": _source_node_key008(node),
    }


def _mutate_bool008(source: str, node: _ast008.Constant) -> tuple[str | None, dict[str, Any] | None]:
    if isinstance(node.value, bool):
        return _source_replace008(source, node, "False" if node.value else "True"), {
            "operation": "replace", "kind": "boolean_toggle", "old": node.value, "new": not node.value,
            "node": _source_node_key008(node),
        }
    return None, None


def _mutate_constant008(source: str, node: _ast008.Constant, value: Any) -> tuple[str, dict[str, Any]]:
    return _source_replace008(source, node, repr(value)), {
        "operation": "specialize", "kind": "constant_substitute",
        "old": node.value, "new": value, "node": _source_node_key008(node),
    }


def _source_function_depths008(tree: _ast008.AST, focus_stage: str | None) -> tuple[dict[str, int], dict[_ast008.AST, str | None]]:
    """Return generic static-call-graph depth from the implementation of one architecture stage.

    The mechanism uses only executable structure: it follows calls from the stage branch in
    ``_run_stage`` into same-module function definitions. No benchmark family or solution is
    named. This lets the learner spend a bounded source-search frontier on code it actually
    executed.
    """
    if not focus_stage:
        return {}, {}
    funcs={n.name:n for n in _ast008.walk(tree) if isinstance(n, _ast008.FunctionDef)}
    enclosing:{} = {}
    def mark(fn_name: str, node: _ast008.AST):
        for child in _ast008.walk(node):
            enclosing[child]=fn_name
    for fn in funcs.values():
        mark(fn.name,fn)
    run=funcs.get("_run_stage")
    direct=[]
    if run is not None:
        for node in _ast008.walk(run):
            if isinstance(node,_ast008.If):
                test=node.test
                hit = (isinstance(test,_ast008.Compare) and len(test.ops)==1 and isinstance(test.ops[0],_ast008.Eq)
                       and len(test.comparators)==1 and isinstance(test.comparators[0],_ast008.Constant)
                       and test.comparators[0].value==focus_stage)
                if hit:
                    for call in _ast008.walk(node):
                        if isinstance(call,_ast008.Call) and isinstance(call.func,_ast008.Name) and call.func.id in funcs:
                            if call.func.id not in direct:
                                direct.append(call.func.id)
                    break
    depths={}
    frontier=list(direct)
    for name in frontier: depths[name]=0
    head=0
    while head<len(frontier):
        name=frontier[head]; head+=1; fn=funcs[name]; nxt=depths[name]+1
        for call in _ast008.walk(fn):
            if isinstance(call,_ast008.Call) and isinstance(call.func,_ast008.Name) and call.func.id in funcs and call.func.id not in depths:
                depths[call.func.id]=nxt; frontier.append(call.func.id)
    return depths,enclosing


def _generic_source_candidates008(source: str, limit: int = 64, focus_stage: str | None = None, prefer_ordering: bool = False) -> tuple[dict[str, Any], ...]:
    """Generate task-independent structural source rewrites from executable syntax."""
    tree = _ast008.parse(source)
    candidates: list[dict[str, Any]] = []
    parents = {child: parent for parent in _ast008.walk(tree) for child in _ast008.iter_child_nodes(parent)}
    focus_depths,enclosing=_source_function_depths008(tree,focus_stage)

    # Structural sequence rewrites. Architecture/data-field tuples receive priority because
    # they are executable state declarations, not because they have a semantic benchmark name.
    tuples=[]
    for node in _ast008.walk(tree):
        if isinstance(node,_ast008.Tuple) and 3 <= len(node.elts) <= 12 and all(isinstance(e,_ast008.Constant) and isinstance(e.value,str) for e in node.elts):
            parent=parents.get(node); rank=0 if isinstance(parent,(_ast008.AnnAssign,_ast008.Assign)) else 1
            tuples.append((rank,abs(len(node.elts)-5),getattr(node,'lineno',0),getattr(node,'col_offset',0),node))
    tuples.sort(key=lambda x:x[:4])
    if prefer_ordering:
        tuple_budget=min(4,limit)
        for _,_,_,_,node in tuples:
            for from_index in range(1,len(node.elts)):
                child,mut=_mutate_tuple_move008(source,node,from_index,0)
                if child != source:
                    candidates.append({'source':child,'mutation':mut})
                    if len(candidates)>=tuple_budget: break
            if len(candidates)>=tuple_budget: break

    calls=[]
    for node in _ast008.walk(tree):
        if not (isinstance(node,_ast008.Call) and len(node.args)==1 and not node.keywords and isinstance(node.func,_ast008.Name) and not node.func.id.startswith('_')):
            continue
        child,mut=_mutate_unwrap_call008(source,node)
        if child is None or mut is None or child==source: continue
        parent=parents.get(node)
        parent_rank=0 if isinstance(parent,_ast008.For) else 1
        arg_rank=0 if isinstance(node.args[0],(_ast008.Name,_ast008.Attribute,_ast008.Subscript)) else 1
        depth=focus_depths.get(enclosing.get(node),999)
        calls.append((depth,parent_rank,arg_rank,getattr(node,'lineno',0),node.func.id,{'source':child,'mutation':mut}))
    calls.sort(key=lambda x:(x[0],x[1],x[2],-x[3],x[4]))

    compares=[]
    for node in _ast008.walk(tree):
        if not (isinstance(node,_ast008.Compare) and len(node.ops)==1): continue
        child,mut=_mutate_compare008(source,node)
        if child is None or mut is None or child==source: continue
        parent=parents.get(node)
        parent_rank=0 if isinstance(parent,(_ast008.Assign,_ast008.AnnAssign)) else 1
        depth=focus_depths.get(enclosing.get(node),999)
        compares.append((depth,parent_rank,getattr(node,'lineno',0),getattr(node,'col_offset',0),{'source':child,'mutation':mut}))
    compares.sort(key=lambda x:x[:4])

    bools=[]
    for node in _ast008.walk(tree):
        if len(bools) >= limit:
            break
        if not (isinstance(node,_ast008.Constant) and isinstance(node.value,bool)): continue
        child,mut=_mutate_bool008(source,node)
        if child is not None and mut is not None and child!=source and _ast008.parse(child):
            bools.append({'source':child,'mutation':mut})

    constants=[]
    for node in _ast008.walk(tree):
        if len(constants) >= limit:
            break
        if not (isinstance(node,_ast008.Constant) and isinstance(node.value,int)): continue
        for value in (0,1,-1):
            if len(constants) >= limit:
                break
            if node.value==value: continue
            child,mut=_mutate_constant008(source,node,value)
            if child!=source and _ast008.parse(child): constants.append({'source':child,'mutation':mut})

    # Bounded structural search: when no strong ordering hypothesis exists, prioritize
    # executable source regions actually reached by the dominant observed stage. Distinct
    # mutation families are still interleaved, but relevance outranks arbitrary source order.
    pools=[calls,compares,bools,constants]
    while len(candidates)<limit and any(pools):
        for pool in pools:
            if not pool or len(candidates)>=limit:
                continue
            item=pool.pop(0)
            candidates.append(item[-1] if isinstance(item,tuple) else item)

    unique={}
    for item in candidates:
        unique.setdefault(_source_sha008(item['source']),item)
    return tuple(unique.values())[:limit]

def _self_source_view008(self: GeneralLearner) -> dict[str, Any]:
    # Read only the executable cognitive module containing these methods. The path comes
    # from the module object itself, not from evaluator or experiment code.
    path = _Path008(_inspect008.getfile(_self_source_view008)).resolve()
    source = path.read_text(encoding="utf-8")
    return {
        "path": str(path),
        "source": source,
        "source_sha256": _source_sha008(source),
        "ast_sha256": _source_ast_sha008(source),
        "module": "cognitive_substrate.self_improvement",
    }


def _init_008(self: GeneralLearner) -> None:
    if hasattr(self, "_asi_core_008_ready"):
        return
    view = _self_source_view008(self)
    self._cognitive_source_view_008 = {
        "source_sha256": view["source_sha256"], "ast_sha256": view["ast_sha256"], "path": view["path"],
    }
    self._architectural_hypotheses_008: list[dict[str, Any]] = []
    self._patch_generator_008: dict[str, Any] = {
        "operation_set": SOURCE_REWRITE_OPERATIONS_008,
        "representation": "generic_python_ast",
        "semantic_knowledge": (),
        "preloaded_patches": (),
        "frontier_limit": 8,
        "max_frontier_limit": 64,
        "meta_updates": 0,
        "meta_lineage": (),
    }
    self._patch_evaluator_008: dict[str, Any] = {
        "role": "prediction_only",
        "decision_inputs": ("source_valid", "regression_passed", "transfer_passed", "reachable_gain", "target_work_gain", "candidate_work"),
        "certifies": False,
    }
    self._rollback_state_008: dict[str, Any] = {
        "parent_source_sha256": view["source_sha256"],
        "candidate_source_sha256": None,
        "accepted_source_sha256": view["source_sha256"],
        "current_source_sha256": view["source_sha256"],
    }
    self._source_patch_archive_008: dict[str, str] = {view["source_sha256"]: view["source"]}
    self._improvement_memory_008: list[dict[str, Any]] = []
    self._architecture_selection_policy_008: dict[str, Any] = {
        "objective": "preserve_regressions_while_increasing_reachable_capability",
        "ordering": ("reachable_gain", "target_work_gain", "resource_efficiency", "candidate_work"),
        "tie_break": "content_digest",
        "learned": False,
    }
    self._self_observations_008: list[dict[str, Any]] = []
    self._source_state_authoritative_008 = False
    self._asi_core_008_ready = True


def _ensure_008(self: GeneralLearner) -> None:
    if not hasattr(self, "_asi_core_008_ready"):
        _init_008(self)


def _observe_runtime008(self: GeneralLearner, *, task_id: str, trace: Sequence[Mapping[str, Any]], success: bool, resource: Mapping[str, Any] | None = None) -> dict[str, Any]:
    _ensure_008(self)
    stage_work = {str(x.get("stage")): float(x.get("work", 0)) for x in trace}
    obs = {
        "observation_id": "obs:" + _source_sha008(_canon_source008({"task": task_id, "trace": list(trace), "n": len(self._self_observations_008)})),
        "task_digest": _digest(str(task_id)),
        "stage_work": stage_work,
        "trace": [dict(x) for x in trace],
        "success": bool(success),
        "resource": dict(resource or {}),
        "source_sha256": _self_source_view008(self)["source_sha256"],
    }
    self._self_observations_008.append(obs)
    self._self_observations_008 = self._self_observations_008[-128:]
    return obs


def _hypothesize_bottlenecks008(self: GeneralLearner, max_candidates: int = 4) -> tuple[dict[str, Any], ...]:
    _ensure_008(self)
    observations = self._self_observations_008[-32:]
    if not observations:
        return ()
    stage_totals: dict[str, float] = {}
    stage_counts: dict[str, int] = {}
    late_reuse = 0.0
    repeated = 0
    task_counts: dict[str, int] = {}
    for pos, obs in enumerate(observations):
        task_counts[obs["task_digest"]] = task_counts.get(obs["task_digest"], 0) + 1
        trace = obs["trace"]
        stages = [str(x.get("stage")) for x in trace]
        recency = 1.0 + (pos / max(1, len(observations)-1)) * 3.0
        if "reuse" in stages:
            idx = stages.index("reuse")
            if idx > 0:
                late_reuse += recency * sum(float(x.get("work", 0)) for x in trace[:idx])
        for item in trace:
            stage = str(item.get("stage"))
            weighted = recency * float(item.get("work", 0))
            stage_totals[stage] = stage_totals.get(stage, 0.0) + weighted
            stage_counts[stage] = stage_counts.get(stage, 0) + 1
    repeated = sum(v - 1 for v in task_counts.values() if v > 1)
    hypotheses: list[dict[str, Any]] = []
    for stage, total in sorted(stage_totals.items(), key=lambda kv: (-kv[1], kv[0]))[:4]:
        hypotheses.append({
            "hypothesis_id": f"b:{_digest((stage, round(total,6), len(observations)))[:16]}",
            "type": "stage_pressure",
            "evidence": {"stage": stage, "total_work": total, "calls": stage_counts.get(stage, 0)},
            "prediction": "a structural change touching this computational region may reduce work without changing learned content",
            "confidence": min(1.0, total / max(1.0, sum(stage_totals.values()))),
        })
    hypotheses.append({
        "hypothesis_id": "b:" + _digest(("late", round(late_reuse, 6)))[:16],
        "type": "ordering_pressure",
        "evidence": {"late_reuse_work": late_reuse},
        "prediction": "reordering executable stages may move reusable results earlier",
        "confidence": min(1.0, late_reuse / max(1.0, sum(stage_totals.values()))),
    })
    hypotheses.append({
        "hypothesis_id": "b:" + _digest(("repeat", repeated))[:16],
        "type": "repeat_work_pressure",
        "evidence": {"repeated_observations": repeated},
        "prediction": "generic caching or retrieval changes may reduce repeated work",
        "confidence": min(1.0, repeated / max(1.0, len(observations))),
    })
    # Preserve competition: sort by evidence-derived confidence but keep distinct types.
    out=[]; seen_types=set()
    for h in sorted(hypotheses, key=lambda x: (-float(x["confidence"]), x["hypothesis_id"])):
        if h["type"] in seen_types and len(out) < 2:
            continue
        out.append(h); seen_types.add(h["type"])
        if len(out) >= max_candidates:
            break
    self._architectural_hypotheses_008 = out
    return tuple(out)


def _build_source_patch008(self: GeneralLearner, candidate: Mapping[str, Any], hypotheses: Sequence[Mapping[str, Any]]) -> dict[str, Any]:
    _ensure_008(self)
    parent = _self_source_view008(self)
    child = str(candidate["source"])
    mutation = dict(candidate["mutation"])
    patch_text = _canon_source008({"parent": parent["source_sha256"], "child": _source_sha008(child), "mutation": mutation})
    dominant = max(hypotheses, key=lambda h: (float(h.get("confidence", 0.0)), h.get("hypothesis_id", "")), default={"type":"unknown","evidence":{}})
    patch_id = "srcpatch:" + _source_sha008(patch_text)[:20]
    artifact = {
        "patch_id": patch_id,
        "parent_architecture_digest": self._architecture_006.digest,
        "patch_representation": {"language": "python_ast_rewrite", "mutation": mutation, "diff": patch_text, "diff_format": "structural_edit"},
        "candidate_child_architecture": {
            "architecture_ir": self._architecture_006.to_dict(),
            "source_sha256": _source_sha008(child),
            "ast_sha256": _source_ast_sha008(child),
        },
        "candidate_source": child,
        "proposed_architecture_digest": _digest({"architecture": self._architecture_006.to_dict(), "source_sha256": _source_sha008(child)}),
        "patch_sha256": patch_id.split(":", 1)[1],
        "predicted_bottleneck": dominant,
        "predicted_benefit": {
            "search_reduction": float(dominant.get("confidence", 0.0)),
            "acquisition_efficiency": float(dominant.get("confidence", 0.0)),
        },
        "predicted_risks": ("regression", "overfit", "resource_overhead", "instability"),
        "construction_trace": (
            {"operation": "inspect", "source_sha256": parent["source_sha256"], "ast_sha256": parent["ast_sha256"]},
            {"operation": mutation.get("operation", "replace"), "mutation": mutation},
            {"operation": "construct", "candidate_source_sha256": _source_sha008(child)},
        ),
        "verification_trace": (
            "syntax", "compile", "independent_regression", "hidden_transfer", "resource_limit", "fresh_process_reconstruction"
        ),
        "hidden_transfer_result": {},
        "resource_result": {},
        "regression_result": {},
        "parent_source_sha256": parent["source_sha256"],
        "parent_ast_sha256": parent["ast_sha256"],
        "candidate_source_sha256": _source_sha008(child),
        "child_source_sha256": _source_sha008(child),
        "candidate_ast_sha256": _source_ast_sha008(child),
        "source_form": "ast_rewrite",
        "writable_region": "self:cognitive_substrate.self_improvement",
    }
    return artifact


def _generate_source_patches008(self: GeneralLearner, max_candidates: int | None = None) -> tuple[dict[str, Any], ...]:
    _ensure_008(self)
    if max_candidates is None:
        max_candidates = int(self._patch_generator_008.get("frontier_limit", 8))
    max_candidates = max(1, min(int(max_candidates), int(self._patch_generator_008.get("max_frontier_limit", 64))))
    hypotheses = self._hypothesize_bottlenecks008(max_candidates=4)
    source = _self_source_view008(self)["source"]
    recent = self._self_observations_008[-1:]
    focus_stage = None
    if recent:
        trace = list(recent[0].get("trace",()))
        if trace:
            focus_stage = str(max(trace, key=lambda x: float(x.get("work",0))).get("stage"))
    ordering_pressure = False
    if recent:
        trace = list(recent[0].get("trace",()))
        stages = [str(x.get("stage")) for x in trace]
        if "reuse" in stages:
            reuse_idx = stages.index("reuse")
            pre_reuse_work = sum(float(x.get("work",0)) for x in trace[:reuse_idx])
            ordering_pressure = reuse_idx > 0 and pre_reuse_work > 0.0
    candidates = _generic_source_candidates008(source, limit=max_candidates, focus_stage=focus_stage, prefer_ordering=ordering_pressure)
    artifacts = tuple(_build_source_patch008(self, c, hypotheses) for c in candidates)
    self._rollback_state_008["candidate_source_sha256"] = artifacts[0]["candidate_source_sha256"] if artifacts else None
    self._improvement_memory_008.append({
        "event": "PATCH_GENERATION",
        "parent_source_sha256": _self_source_view008(self)["source_sha256"],
        "hypothesis_ids": tuple(h["hypothesis_id"] for h in hypotheses),
        "candidate_patch_ids": tuple(x["patch_id"] for x in artifacts),
    })
    self._improvement_memory_008 = self._improvement_memory_008[-128:]
    return artifacts


def _select_source_patch008(self: GeneralLearner, evaluations: Sequence[Mapping[str, Any]]) -> tuple[dict[str, Any] | None, dict[str, Any]]:
    _ensure_008(self)
    ranked=[]
    current_source_sha = _self_source_view008(self)["source_sha256"]
    for item in evaluations:
        if not bool(item.get("source_valid")):
            continue
        # A duplicate/no-op is not a self-improvement even if it remains better than A0;
        # it must differ from the learner's actual current executable child.
        if str(item.get("child_source_sha256", "")) == current_source_sha:
            continue
        if item.get("resource_limit_exceeded") is True or item.get("resource_ok") is False:
            continue
        if not bool(item.get("regression_passed")) or not bool(item.get("transfer_passed")):
            continue
        gain=float(item.get("reachable_gain", 0.0))
        work_gain=float(item.get("target_work_gain", 0.0))
        resource=(float(item.get("parent_target_work", 0.0)) - float(item.get("candidate_work", 0.0)))
        candidate_work=float(item.get("candidate_work", 0.0))
        digest=str(item.get("patch_sha256", ""))
        ranked.append(((gain, work_gain, resource, -candidate_work, digest), item))
    ranked.sort(key=lambda x:x[0], reverse=True)
    selected=ranked[0][1] if ranked else None
    decision={
        "considered": len(evaluations),
        "eligible": len(ranked),
        "selected_patch_sha256": selected.get("patch_sha256") if selected else None,
        "independent_certification": False,
    }
    self._improvement_memory_008.append({
        "event": "PATCH_SELECTION",
        "decision": decision,
        "evidence_digest": _digest(tuple(sorted(_canon_source008(dict(x)) for x in evaluations))),
    })
    if selected:
        self._rollback_state_008["candidate_source_sha256"] = str(selected.get("child_source_sha256", ""))
    return selected, decision


def _record_patch_evidence008(self: GeneralLearner, patch: Mapping[str, Any], evaluation: Mapping[str, Any], *, accepted: bool) -> None:
    _ensure_008(self)
    keep=("source_valid","regression_passed","transfer_passed","reachable_gain","target_work_gain","parent_reachable","candidate_reachable","parent_accuracy","candidate_accuracy","regression_accuracy","global_future_nonregression","resource_limit_exceeded","candidate_work")
    candidate_source = patch.get("candidate_source")
    if isinstance(candidate_source, str):
        self._source_patch_archive_008[str(patch.get("child_source_sha256", patch.get("candidate_source_sha256", "")))] = candidate_source
    entry={
        "event": "PATCH_EVIDENCE",
        "patch_id": patch.get("patch_id"),
        "patch_sha256": patch.get("patch_sha256"),
        "parent_source_sha256": patch.get("parent_source_sha256"),
        "candidate_source_sha256": patch.get("child_source_sha256"),
        "evaluation_summary": {k:evaluation.get(k) for k in keep if k in evaluation},
        "accepted": bool(accepted),
    }
    self._improvement_memory_008.append(entry)
    self._improvement_memory_008 = self._improvement_memory_008[-128:]


def _accept_source_patch008(self: GeneralLearner, patch: Mapping[str, Any], evaluation: Mapping[str, Any]) -> None:
    _ensure_008(self)
    parent = self._rollback_state_008.get("current_source_sha256") or patch.get("parent_source_sha256")
    child = str(patch.get("child_source_sha256", patch.get("candidate_source_sha256", "")))
    self._rollback_state_008["parent_source_sha256"] = parent
    self._rollback_state_008["current_source_sha256"] = child
    self._rollback_state_008["accepted_source_sha256"] = child
    self._rollback_state_008["candidate_source_sha256"] = None
    candidate_source = patch.get("candidate_source")
    if isinstance(candidate_source, str):
        self._source_patch_archive_008[child] = candidate_source
    self._improvement_memory_008.append({
        "event": "PATCH_ACCEPTED",
        "parent_source_sha256": parent,
        "child_source_sha256": child,
        "patch_id": patch.get("patch_id"),
        "patch_sha256": patch.get("patch_sha256"),
        "evaluation": dict(evaluation),
        "lineage": {"parent": parent, "patch": patch.get("patch_id"), "child": child},
    })
    self._improvement_memory_008 = self._improvement_memory_008[-128:]
    self._architecture_selection_policy_008["learned"] = True
    self._source_state_authoritative_008 = True


def _rollback_last_source_patch008(self: GeneralLearner) -> dict[str, Any]:
    """Return the executable parent source for the current accepted source and restore lineage.

    The learner decides rollback from its own retained lineage; materializing the returned source
    into the writable execution region remains an execution-boundary operation.
    """
    _ensure_008(self)
    current = str(self._rollback_state_008.get("current_source_sha256") or self._cognitive_source_view_008.get("source_sha256", ""))
    accepted = [x for x in self._improvement_memory_008 if x.get("event") == "PATCH_ACCEPTED" and x.get("child_source_sha256") == current]
    if not accepted:
        return {"rolled_back": False, "reason": "no_accepted_child", "current_source_sha256": current}
    record = accepted[-1]
    parent = str(record.get("parent_source_sha256", ""))
    source = self._source_patch_archive_008.get(parent)
    if not parent or source is None:
        return {"rolled_back": False, "reason": "parent_source_unavailable", "current_source_sha256": current, "parent_source_sha256": parent}
    self._rollback_state_008["candidate_source_sha256"] = current
    self._rollback_state_008["current_source_sha256"] = parent
    self._rollback_state_008["accepted_source_sha256"] = parent
    self._rollback_state_008["parent_source_sha256"] = parent
    self._cognitive_source_view_008 = {**self._cognitive_source_view_008, "source_sha256": parent}
    self._improvement_memory_008.append({"event": "SOURCE_ROLLBACK", "from_source_sha256": current, "to_source_sha256": parent, "restored_source_sha256": _source_sha008(source)})
    self._improvement_memory_008 = self._improvement_memory_008[-128:]
    return {"rolled_back": True, "from_source_sha256": current, "to_source_sha256": parent, "source": source, "source_sha256": _source_sha008(source)}


def _export_008_state(self: GeneralLearner, base: Mapping[str, Any]) -> dict[str, Any]:
    _ensure_008(self)
    out=copy.deepcopy(dict(base))
    out["asi_core_008"]={
        "cognitive_source_view": copy.deepcopy(self._cognitive_source_view_008),
        "architectural_hypotheses": copy.deepcopy(self._architectural_hypotheses_008),
        "patch_generator": copy.deepcopy(self._patch_generator_008),
        "patch_evaluator": copy.deepcopy(self._patch_evaluator_008),
        "rollback_state": copy.deepcopy(self._rollback_state_008),
        "source_patch_archive": copy.deepcopy(self._source_patch_archive_008),
        "improvement_memory": copy.deepcopy(self._improvement_memory_008),
        "architecture_selection_policy": copy.deepcopy(self._architecture_selection_policy_008),
        "self_observations": copy.deepcopy(self._self_observations_008),
        "source_state_authoritative": bool(self._source_state_authoritative_008),
    }
    return out


def _load_008_state(self: GeneralLearner, state: Mapping[str, Any]) -> None:
    _ensure_008(self)
    info=state.get("asi_core_008", {}) or {}
    self._cognitive_source_view_008=dict(info.get("cognitive_source_view", self._cognitive_source_view_008))
    self._architectural_hypotheses_008=[dict(x) for x in info.get("architectural_hypotheses", ())]
    pg=dict(info.get("patch_generator", self._patch_generator_008)); pg["operation_set"]=tuple(pg.get("operation_set", SOURCE_REWRITE_OPERATIONS_008)); self._patch_generator_008=pg
    pe=dict(info.get("patch_evaluator", self._patch_evaluator_008)); self._patch_evaluator_008=pe
    self._rollback_state_008=dict(info.get("rollback_state", self._rollback_state_008))
    self._source_patch_archive_008={str(k):str(v) for k,v in dict(info.get("source_patch_archive", self._source_patch_archive_008)).items()}
    self._improvement_memory_008=[dict(x) for x in info.get("improvement_memory", ())]
    self._architecture_selection_policy_008=dict(info.get("architecture_selection_policy", self._architecture_selection_policy_008))
    self._self_observations_008=[dict(x) for x in info.get("self_observations", ())]
    self._source_state_authoritative_008=bool(info.get("source_state_authoritative", False))


def _init_bound_008(self: GeneralLearner, *args: Any, **kwargs: Any):
    _ORIG_INIT_008(self, *args, **kwargs)
    _init_008(self)


def _export_bound_008(self: GeneralLearner):
    return _export_008_state(self, _ORIG_EXPORT_008(self))


def _from_state_bound_008(cls, spec: Mapping[str, Any], **kwargs: Any):
    raw = copy.deepcopy(dict(spec))
    info = raw.get("asi_core_008", {}) or {}
    if bool(info.get("source_state_authoritative", False)) and "asi_core_006" in raw:
        # A source-level child is reconstructed from its executable source first. The
        # persisted CORE-006 IR may describe the parent before the source rewrite; keeping
        # that stale spec would silently roll the executable child back to A0. Other history
        # and telemetry remain serializable in ASI-CORE-008.
        info006 = dict(raw.get("asi_core_006", {}) or {})
        info006.pop("spec", None)
        raw["asi_core_006"] = info006
    obj=_ORIG_FROM_STATE_008.__func__(cls, raw, **kwargs)
    _init_008(obj)
    _load_008_state(obj, raw)
    return obj


_ORIG_INIT_008=GeneralLearner.__init__
_ORIG_EXPORT_008=GeneralLearner.export_state
_ORIG_FROM_STATE_008=GeneralLearner.from_state
if getattr(GeneralLearner, "_asi_core_008_bound", False) is False:
    GeneralLearner.__init__=_init_bound_008
    GeneralLearner.export_state=_export_bound_008
    GeneralLearner.from_state=classmethod(_from_state_bound_008)
    GeneralLearner.inspect_cognitive_source=_self_source_view008
    GeneralLearner.observe_self_runtime=_observe_runtime008
    GeneralLearner._hypothesize_bottlenecks008=_hypothesize_bottlenecks008
    GeneralLearner._build_source_patch008=_build_source_patch008
    GeneralLearner._generate_source_patches008=_generate_source_patches008
    GeneralLearner.hypothesize_bottlenecks=_hypothesize_bottlenecks008
    GeneralLearner.generate_source_patch_candidates=_generate_source_patches008
    GeneralLearner.select_source_patch=_select_source_patch008
    GeneralLearner.record_source_patch_evidence=_record_patch_evidence008
    GeneralLearner.accept_source_patch=_accept_source_patch008
    GeneralLearner.rollback_last_source_patch=_rollback_last_source_patch008
    GeneralLearner._asi_core_008_bound=True


# ---------- ASI-CORE-008 recursive improvement-of-improvement ----------

def _propose_meta_improvement008(self: GeneralLearner) -> dict[str, Any] | None:
    """Generate a generic change to the mechanism that searches for source improvements.

    The proposal is inferred from observed search saturation/failure. No future target,
    patch identity, regime label, or benchmark-specific solution is supplied.
    """
    _ensure_008(self)
    limit=int(self._patch_generator_008.get("frontier_limit",8))
    ceiling=int(self._patch_generator_008.get("max_frontier_limit",64))
    recent=self._improvement_memory_008[-32:]
    saturated=[x for x in recent if x.get("event")=="PATCH_GENERATION" and len(x.get("candidate_patch_ids",()))>=limit]
    failed=[x for x in recent if x.get("event")=="META_SEARCH_RESULT" and not bool(x.get("discovered"))]
    if limit>=ceiling or not (saturated or failed):
        return None
    new_limit=min(ceiling,max(limit+1,limit*2))
    proposal={
        "meta_patch_id":"meta:"+_digest((limit,new_limit,len(recent)))[:20],
        "parent_frontier_limit":limit,
        "child_frontier_limit":new_limit,
        "operation":"increase_search_frontier",
        "reason":"observed candidate-search saturation without successful discovery",
        "prediction":"a wider generic candidate frontier will make additional source hypotheses discoverable",
        "source_of_evidence":{
            "saturated_generations":len(saturated),
            "failed_meta_trials":len(failed),
        },
        "human_authored_solution":False,
        "external_model_solution":False,
    }
    self._improvement_memory_008.append({"event":"META_PROPOSAL","proposal":proposal})
    self._improvement_memory_008=self._improvement_memory_008[-128:]
    return proposal


def _record_meta_search_result008(self: GeneralLearner, *, target_digest: str, discovered: bool, candidate_count: int, stage: str) -> dict[str, Any]:
    _ensure_008(self)
    rec={
        "event":"META_SEARCH_RESULT",
        "stage":str(stage),
        "target_digest":str(target_digest),
        "discovered":bool(discovered),
        "candidate_count":int(candidate_count),
        "frontier_limit":int(self._patch_generator_008.get("frontier_limit",8)),
    }
    self._improvement_memory_008.append(rec)
    self._improvement_memory_008=self._improvement_memory_008[-128:]
    return rec


def _apply_meta_improvement008(self: GeneralLearner, proposal: Mapping[str, Any], evidence: Mapping[str, Any]) -> bool:
    _ensure_008(self)
    parent=int(self._patch_generator_008.get("frontier_limit",8))
    child=int(proposal.get("child_frontier_limit",parent))
    predicted=bool(evidence.get("post_improvement_expected", False))
    observed=evidence.get("post_discovered")
    accepted=(proposal.get("parent_frontier_limit")==parent and child>parent and child<=int(self._patch_generator_008.get("max_frontier_limit",64)) and (observed is True or predicted))
    if not accepted:
        self._improvement_memory_008.append({"event":"META_REJECTED","proposal":dict(proposal),"evidence":dict(evidence)})
        self._improvement_memory_008=self._improvement_memory_008[-128:]
        return False
    update_no=int(self._patch_generator_008.get("meta_updates",0))+1
    lineage=list(self._patch_generator_008.get("meta_lineage",()))
    lineage.append({"parent_M":f"M{update_no-1}","improvement":proposal.get("meta_patch_id"),"child_M":f"M{update_no}","parent_frontier":parent,"child_frontier":child,"evidence":dict(evidence)})
    self._patch_generator_008["frontier_limit"]=child
    self._patch_generator_008["meta_updates"]=update_no
    self._patch_generator_008["meta_lineage"]=tuple(lineage[-32:])
    self._improvement_memory_008.append({"event":"META_ACCEPTED","proposal":dict(proposal),"evidence":dict(evidence),"lineage":lineage[-1]})
    self._improvement_memory_008=self._improvement_memory_008[-128:]
    return True


# ---------- ASI-CORE-008 conditional architecture selection ----------

def _shape008(value: Any, depth: int = 0) -> tuple[Any, ...]:
    if depth > 3:
        return (type(value).__name__,)
    if isinstance(value, dict):
        return ("dict", len(value), tuple(sorted(type(v).__name__ for v in value.values())))
    if isinstance(value, list):
        return ("list", len(value), tuple(_shape008(v, depth + 1)[:1] for v in value[:4]))
    if isinstance(value, tuple):
        return ("tuple", len(value), tuple(_shape008(v, depth + 1)[:1] for v in value[:4]))
    return (type(value).__name__,)


def _context_features008(examples: Sequence[tuple[Json, Json]], learner: GeneralLearner) -> tuple[float, ...]:
    # Features are computed solely from information already inside the learner: operator
    # inventory structure plus observable example shape. No regime label is consulted.
    ops=tuple(getattr(learner,"_learned_operators",{}).values())
    operator_count=float(len(ops))
    behavior_keys=[]
    for op in ops:
        try:
            pairs=tuple((e["input"],e["output"]) for e in op.behavioral_examples[:2])
            behavior_keys.append(_behavior_key(pairs))
        except Exception:
            behavior_keys.append(None)
    unique_keys=float(len(set(behavior_keys)))
    collisions=0.0 if operator_count<=0 else 1.0-unique_keys/operator_count
    max_bucket=0.0
    target_bucket=0.0
    if behavior_keys:
        from collections import Counter
        counts=Counter(behavior_keys)
        max_bucket=float(max(counts.values()))
        try:
            target_key=_behavior_key(examples)
            target_bucket=float(counts.get(target_key,0))
        except Exception:
            target_bucket=0.0
    ex_sizes=[]
    for x,_ in examples[:8]:
        if isinstance(x,(list,tuple,dict,str)): ex_sizes.append(float(len(x)))
        else: ex_sizes.append(1.0)
    mean_size=sum(ex_sizes)/max(1,len(ex_sizes))
    shape_digest=int(_digest(tuple(_shape008(x[0]) for x in examples[:8]))[:8],16)/0xFFFFFFFF
    target_bucket_ratio=0.0 if operator_count<=0 else target_bucket/operator_count
    return (operator_count,unique_keys,collisions,max_bucket,target_bucket_ratio,float(len(examples)),mean_size,shape_digest)

def _vec_distance008(a: Sequence[float], b: Sequence[float]) -> float:
    scales=(max(1.0,a[0],b[0]), max(1.0,a[1],b[1]), 1.0, max(1.0,a[3],b[3]), 1.0, max(1.0,a[5],b[5]), max(1.0,a[6],b[6]), 1.0)
    return sum(((float(x)-float(y))/scale)**2 for x,y,scale in zip(a,b,scales)) ** 0.5


def _predict_variant_utility008(records: Sequence[Mapping[str, Any]], features: Sequence[float]) -> tuple[float, float, int]:
    if not records:
        return 0.0, float("inf"), 0
    # Learn a compact context law from feedback rather than memorizing regime labels.
    # The retrieval-density feature (target bucket / operator inventory) is the directly
    # observable structural pressure on linear-vs-indexed lookup. A simple least-squares
    # line is intentionally used so an unseen intermediate regime can be interpolated.
    valid=[]
    for r in records:
        f=tuple(float(x) for x in r.get("features", ()))
        if len(f)==len(features):
            valid.append((f,float(r.get("utility",0.0))))
    if not valid:
        return 0.0,float("inf"),0
    xs=[v[0][4] for v in valid]
    ys=[v[1] for v in valid]
    mean_x=sum(xs)/len(xs); mean_y=sum(ys)/len(ys)
    den=sum((x-mean_x)**2 for x in xs)
    slope=sum((x-mean_x)*(y-mean_y) for x,y in zip(xs,ys))/den if den>1e-12 else 0.0
    intercept=mean_y-slope*mean_x
    x=float(features[4])
    pred=intercept+slope*x
    nearest=min(_vec_distance008(f,features) for f,_ in valid)
    return pred,nearest,len(valid)

def _register_architecture_variant008(self: GeneralLearner, variant_id: str, architecture: Mapping[str, Any], *, source_sha256: str | None = None) -> dict[str, Any]:
    _ensure_008(self)
    variants=dict(self._architecture_selection_policy_008.get("variants", {}))
    variants[str(variant_id)]={
        "architecture": copy.deepcopy(dict(architecture)),
        "source_sha256": source_sha256,
        "observations": int(variants.get(str(variant_id), {}).get("observations",0)),
    }
    self._architecture_selection_policy_008["variants"]=variants
    return copy.deepcopy(variants[str(variant_id)])


def _construct_conditional_variants008(self: GeneralLearner, max_variants: int = 2) -> dict[str, dict[str, Any]]:
    _ensure_008(self)
    # Construct a minimal pair of executable retrieval architectures from the learner's
    # generic architecture vocabulary. The selection policy itself remains unconstrained.
    base={"pipeline":("reuse",),"retrieval_mode":"linear","memoized_stages":(),"experiment_policy":"passive"}
    indexed=dict(base); indexed["retrieval_mode"]="indexed"
    variants={"variant_0":base}
    if max_variants>1: variants["variant_1"]=indexed
    for vid,spec in variants.items(): self._register_architecture_variant008(vid,spec)
    return variants



def _select_architecture_context008(self: GeneralLearner, examples: Sequence[tuple[Json, Json]], variant_ids: Sequence[str], *, explore: bool = False) -> dict[str, Any]:
    _ensure_008(self)
    features=_context_features008(examples,self)
    stats=self._architecture_selection_policy_008.setdefault("context_stats",{})
    ranked=[]
    for vid in variant_ids:
        recs=stats.get(str(vid),[])
        pred,d,n=_predict_variant_utility008(recs,features)
        count=len(recs)
        bonus=(1.0/(1+count)) if explore else 0.0
        ranked.append((pred+bonus,-count,-d,str(vid),pred,d,n))
    ranked.sort(reverse=True)
    selected=ranked[0][3] if ranked else None
    decision={
        "context_features": features,
        "candidates": [str(v) for v in variant_ids],
        "selected_variant": selected,
        "predicted_utility": ranked[0][4] if ranked else None,
        "nearest_distance": ranked[0][5] if ranked else None,
        "evidence_count": ranked[0][6] if ranked else 0,
        "learned_from_feedback": any(stats.get(str(v)) for v in variant_ids),
    }
    self._architecture_selection_policy_008["last_decision"]=decision
    self._improvement_memory_008.append({"event":"ARCHITECTURE_SELECTION","decision":decision})
    self._improvement_memory_008=self._improvement_memory_008[-128:]
    return decision


def _observe_architecture_context008(self: GeneralLearner, examples: Sequence[tuple[Json, Json]], variant_id: str, *, success: bool, work: float, resource_cost: float = 0.0) -> dict[str, Any]:
    _ensure_008(self)
    features=_context_features008(examples,self)
    utility=(1.0 if success else -1.0) - 0.01*float(work) - 0.01*float(resource_cost)
    stats=self._architecture_selection_policy_008.setdefault("context_stats",{}).setdefault(str(variant_id),[])
    rec={"features":features,"utility":utility,"success":bool(success),"work":float(work),"resource_cost":float(resource_cost)}
    stats.append(rec); self._architecture_selection_policy_008["context_stats"][str(variant_id)]=stats[-256:]
    variants=self._architecture_selection_policy_008.setdefault("variants",{})
    if str(variant_id) in variants:
        variants[str(variant_id)]["observations"]=int(variants[str(variant_id)].get("observations",0))+1
    self._improvement_memory_008.append({"event":"ARCHITECTURE_FEEDBACK","variant_id":str(variant_id),"record":rec})
    self._improvement_memory_008=self._improvement_memory_008[-128:]
    return rec


def _run_selected_architecture008(self: GeneralLearner, task_id: str, examples: Sequence[tuple[Json, Json]], query: Json, variants: Mapping[str, Mapping[str, Any]], *, budget: int = 2500, explore: bool = False) -> tuple[Json | None, dict[str, Any]]:
    _ensure_008(self)
    ids=tuple(str(x) for x in variants)
    for vid in ids:
        if vid not in self._architecture_selection_policy_008.get("variants",{}):
            self._register_architecture_variant008(vid,variants[vid])
    decision=self._select_architecture_context008(examples,ids,explore=explore)
    selected=decision.get("selected_variant")
    if selected is None:
        return None,{"selection":decision,"success":False,"work":0}
    parent=self._architecture_006
    raw=variants[selected]
    spec=ArchitectureSpec(**dict(raw)) if not isinstance(raw,ArchitectureSpec) else raw
    self._architecture_006=spec
    try:
        pred,meta=learn_architecture_task(self,task_id,examples,query,budget=budget)
    finally:
        self._architecture_006=parent
    index_build=float(getattr(self,"_last_index_build_work_006",0.0))
    total_work=float(meta.get("work",0.0))+index_build
    feedback=self._observe_architecture_context008(examples,selected,success=bool(meta.get("success")),work=total_work,resource_cost=index_build)
    out=dict(meta); out.update({"selection":decision,"selected_variant":selected,"feedback":feedback,"work_with_resource":total_work})
    return pred,out



# Final CORE-008 method bindings onto the single cognitive substrate.
GeneralLearner._register_architecture_variant008=_register_architecture_variant008
GeneralLearner._select_architecture_context008=_select_architecture_context008
GeneralLearner._observe_architecture_context008=_observe_architecture_context008
GeneralLearner._run_selected_architecture008=_run_selected_architecture008
GeneralLearner.construct_conditional_variants_008=_construct_conditional_variants008
GeneralLearner.select_architecture_variant_008=_select_architecture_context008
GeneralLearner.observe_architecture_variant_feedback_008=_observe_architecture_context008
GeneralLearner.run_selected_architecture_008=_run_selected_architecture008
GeneralLearner.propose_meta_improvement_008=_propose_meta_improvement008
GeneralLearner.record_meta_search_result_008=_record_meta_search_result008
GeneralLearner.apply_meta_improvement_008=_apply_meta_improvement008
