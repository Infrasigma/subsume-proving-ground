"""CORE-013: substrate-owned cognitive procedure evolution.

This module makes a bounded learning-process representation a first-class object of the
single GeneralLearner. It is deliberately opt-in: importing it does not alter the frozen
CORE-011 behavior unless a procedure is explicitly enabled.

The first procedure language is intentionally small. It can reconfigure generic cognitive
policies already exposed by CORE-006/009/010, retain provenance, and survive serialization.
It does not contain benchmark answers, evaluator code, external-model calls, or regime labels.
"""
from __future__ import annotations

from dataclasses import dataclass, asdict
from hashlib import sha256
import copy
import json
from typing import Any, Callable, Mapping, Sequence
from .meta_language import eval_expr, expr_size

from .general_engine import GeneralLearner

Json = Any


def _canon(x: Any) -> str:
    return json.dumps(x, sort_keys=True, separators=(",", ":"), ensure_ascii=True, default=str)


def _digest(x: Any) -> str:
    return sha256(_canon(x).encode("utf-8")).hexdigest()


@dataclass(frozen=True)
class CognitiveProcedure:
    procedure_id: str
    parent_id: str | None
    target_layer: str
    trigger: Mapping[str, Any]
    steps: tuple[Mapping[str, Any], ...]
    preconditions: tuple[str, ...] = ()
    predicted_effects: Mapping[str, float] = None  # type: ignore[assignment]
    resource_profile: Mapping[str, float] = None  # type: ignore[assignment]
    status: str = "candidate"
    generation: int = 0
    provenance: Mapping[str, Any] = None  # type: ignore[assignment]

    def __post_init__(self) -> None:
        if self.predicted_effects is None:
            object.__setattr__(self, "predicted_effects", {})
        if self.resource_profile is None:
            object.__setattr__(self, "resource_profile", {})
        if self.provenance is None:
            object.__setattr__(self, "provenance", {})

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)

    @property
    def digest(self) -> str:
        return _digest(self.to_dict())


@dataclass(frozen=True)
class CognitiveOperator:
    operator_id: str
    input_fields: tuple[str, ...]
    output_field: str
    body: Mapping[str, Any]
    source_examples: tuple[Mapping[str, Any], ...] = ()
    construction_trials: int = 0
    provenance: Mapping[str, Any] = None  # type: ignore[assignment]
    evaluation: Mapping[str, Any] = None  # type: ignore[assignment]
    status: str = "candidate"

    def __post_init__(self) -> None:
        if self.provenance is None:
            object.__setattr__(self, "provenance", {})

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)

    @property
    def digest(self) -> str:
        return _digest(self.to_dict())

    def apply(self, state: Mapping[str, Any], telemetry: Mapping[str, Any]) -> Any:
        return eval_expr(
            self.body["expression"],
            {"state": copy.deepcopy(dict(state)), "telemetry": copy.deepcopy(dict(telemetry))},
        )


@dataclass(frozen=True)
class ProcedureDecision:
    procedure_id: str
    decision: str
    evaluation: Mapping[str, Any]
    reason: str
    parent_id: str | None


_LAYERS = ("representation", "search", "planning", "learning", "acquisition", "meta")
_OPS = (
    "set_layer_policy",
    "toggle_retrieval",
    "toggle_memoization",
    "toggle_experiment_policy",
    "adjust_open_search",
    "adjust_planning",
    "toggle_primitive_invention",
    "toggle_representation_memory",
    "adjust_procedure_frontier",
    "apply_learned_cognitive_operator",
    "adopt_strategy_templates",
)


def _init_013(self: GeneralLearner) -> None:
    if hasattr(self, "_asi_core_013_state"):
        return
    self._asi_core_013_state = {
        "enabled": False,
        "library": {},
        "active_by_layer": {},
        "events": [],
        "failures": [],
        "evaluations": [],
        "generator": {
            "frontier_limit": 8,
            "generation": 0,
            "recursion_depth": 0,
            "learned": False,
            "strategy_templates": [],
            "experience_conditioned_generation": False,
        },
        "strategy_archive": {},
        "cognitive_operators": {},
        "semantic_operators": {},
        "semantic_language": {"operators": {}},
        "semantic_operator_classes": {},
        "semantic_class_language": {"classes": {}},
        "active_semantic_class": None,
        "latest_telemetry": {},
        "default_policy": {
            "representation": "existing",
            "search": "existing",
            "planning": "existing",
            "learning": "existing",
            "acquisition": "existing",
            "meta": "existing",
        },
    }


def _strategy_ucb(rec: Mapping[str, Any], total_trials: int, exploration: float = 1.0) -> float:
    n = max(0, int(rec.get("trials", 0)))
    mean_u = float(rec.get("mean_utility", 0.0))
    if n == 0:
        return float("inf")
    return mean_u + exploration * ((max(1, total_trials) / n) ** 0.5)


def _mutate_strategy_template(
    template: Mapping[str, Any],
    *,
    layer: str,
    trigger: Mapping[str, Any],
    variant_index: int,
) -> dict[str, Any]:
    source_id = str(template.get("procedure_id"))
    steps: list[dict[str, Any]] = []
    mutated_search = False
    for step in template.get("steps", ()):
        q = dict(step)
        if q.get("op") == "adjust_open_search":
            mutated_search = True
            factor = float(q.get("budget_factor", 1.0))
            depth = int(q.get("depth_delta", 0))
            beam = int(q.get("beam_delta", 0))
            if variant_index % 2 == 0:
                q["budget_factor"] = min(4.0, max(1.25, factor * 1.5))
                q["depth_delta"] = depth + 1
                q["beam_delta"] = beam + 16
            else:
                q["budget_factor"] = min(4.0, max(1.0, factor))
                q["depth_delta"] = depth + 2
                q["beam_delta"] = beam + 32
        steps.append(q)
    if not mutated_search:
        steps.append({
            "op": "set_layer_policy",
            "layer": layer,
            "mode": "experience_conditioned_search",
        })
    return {
        "source_procedure_id": source_id,
        "source_digest": str(template.get("digest") or _digest(template)),
        "steps": tuple(steps),
        "variant_index": int(variant_index),
        "trigger": copy.deepcopy(dict(trigger)),
    }


def _update_strategy_archive(self: GeneralLearner, procedure_id: str, evaluation: Mapping[str, Any]) -> None:
    _ensure_013(self)
    rec = self._asi_core_013_state["library"].get(str(procedure_id))
    if rec is None or str(rec.get("target_layer")) == "meta":
        return
    utility = float(evaluation.get("transfer_gain", 0.0)) - float(evaluation.get("resource_cost", 0.0))
    archive = self._asi_core_013_state.setdefault("strategy_archive", {})
    row = archive.setdefault(str(procedure_id), {
        "procedure_id": str(procedure_id),
        "target_layer": str(rec.get("target_layer")),
        "diagnosis": str((rec.get("trigger") or {}).get("diagnosis", "")),
        "trials": 0,
        "successes": 0,
        "mean_utility": 0.0,
        "utilities": [],
    })
    n = int(row.get("trials", 0))
    row["trials"] = n + 1
    row["successes"] = int(row.get("successes", 0)) + int(utility > 0.0)
    old_mean = float(row.get("mean_utility", 0.0))
    row["mean_utility"] = old_mean + (utility - old_mean) / max(1, n + 1)
    row["utilities"] = (list(row.get("utilities", ())) + [utility])[-32:]


def _ensure_013(self: GeneralLearner) -> None:
    if not hasattr(self, "_asi_core_013_state"):
        _init_013(self)


def cognitive_evolution_enable(self: GeneralLearner, enabled: bool = True) -> dict[str, Any]:
    _ensure_013(self)
    self._asi_core_013_state["enabled"] = bool(enabled)
    self._asi_core_013_state["events"].append({
        "event": "EVOLUTION_ENABLED" if enabled else "EVOLUTION_DISABLED"
    })
    return {"enabled": bool(enabled)}


def cognitive_evolution_state(self: GeneralLearner) -> dict[str, Any]:
    _ensure_013(self)
    return copy.deepcopy(self._asi_core_013_state)


def record_cognitive_failure(
    self: GeneralLearner,
    *,
    layer: str,
    diagnosis: str,
    evidence: Mapping[str, Any] | None = None,
    trace: Sequence[Mapping[str, Any]] = (),
    context: Mapping[str, Any] | None = None,
) -> dict[str, Any]:
    _ensure_013(self)
    layer = str(layer)
    if layer not in _LAYERS:
        raise ValueError(f"unknown cognitive layer: {layer}")
    rec = {
        "failure_id": "failure:" + _digest((layer, diagnosis, evidence or {}, trace))[:20],
        "layer": layer,
        "diagnosis": str(diagnosis),
        "evidence": copy.deepcopy(dict(evidence or {})),
        "trace": copy.deepcopy(list(trace)),
        "context": copy.deepcopy(dict(context or {})),
        "step": len(self._asi_core_013_state["failures"]),
    }
    self._asi_core_013_state["latest_telemetry"] = copy.deepcopy(dict(evidence or {}))
    self._asi_core_013_state["failures"].append(rec)
    self._asi_core_013_state["failures"] = self._asi_core_013_state["failures"][-128:]
    self._asi_core_013_state["events"].append({"event": "COGNITIVE_FAILURE", **rec})
    self._asi_core_013_state["events"] = self._asi_core_013_state["events"][-256:]
    return copy.deepcopy(rec)


def _base_candidate(self: GeneralLearner, layer: str, trigger: Mapping[str, Any]) -> CognitiveProcedure:
    st = self._asi_core_013_state
    parent = st["active_by_layer"].get(layer)
    gen = int(st["generator"].get("generation", 0))
    return CognitiveProcedure(
        procedure_id="proc:" + _digest((parent, layer, trigger, gen))[:20],
        parent_id=parent,
        target_layer=layer,
        trigger=copy.deepcopy(dict(trigger)),
        steps=(),
        preconditions=("no_external_evaluator_access", "no_hidden_regime_labels"),
        predicted_effects={},
        resource_profile={"extra_work": 0.0},
        generation=gen,
        provenance={
            "origin": "substrate_generated",
            "parent_id": parent,
            "failure_digest": _digest(trigger),
            "human_authored_solution": False,
            "external_model_solution": False,
        },
    )



def _operator_leaf(source: str, field: str) -> dict[str, Any]:
    return {"op": "get", "base": {"op": "var", "name": source}, "key": str(field)}


def _operator_expr_candidates(state_keys: Sequence[str], telemetry_keys: Sequence[str]) -> tuple[dict[str, Any], ...]:
    leaves = (
        [_operator_leaf("state", k) for k in sorted(set(state_keys))]
        + [_operator_leaf("telemetry", k) for k in sorted(set(telemetry_keys))]
        + [{"op": "const", "value": v} for v in (-4, -2, -1, 0, 1, 2, 4, 8)]
    )
    level1 = list(leaves)
    level2 = []
    for a in level1:
        for b in level1:
            for op in ("add", "sub", "mul"):
                level2.append({"op": op, "left": a, "right": b})
    pool = level1 + level2
    out = list(pool)
    for a in pool:
        for b in level1:
            for op in ("add", "sub", "mul"):
                out.append({"op": op, "left": a, "right": b})
    uniq = {_canon(e): e for e in out}
    return tuple(sorted(uniq.values(), key=lambda e: (expr_size(e), _canon(e))))


def _operator_matches(expr: Mapping[str, Any], rows: Sequence[Mapping[str, Any]], field: str) -> bool:
    for row in rows:
        try:
            got = eval_expr(expr, {
                "state": dict(row["state"]),
                "telemetry": dict(row.get("telemetry", {})),
            })
        except Exception:
            return False
        if got != row["after"].get(field):
            return False
    return True


def _construct_cognitive_operator(
    self: GeneralLearner,
    rows: Sequence[Mapping[str, Any]],
    *,
    output_field: str,
    holdout_fraction: float = 0.34,
) -> CognitiveOperator | None:
    if len(rows) < 6:
        return None
    split = max(3, min(len(rows) - 2, int(round(len(rows) * (1.0 - holdout_fraction)))))
    train, holdout = tuple(rows[:split]), tuple(rows[split:])
    state_keys = sorted({str(k) for row in rows for k in dict(row.get("state", {})).keys()})
    telemetry_keys = sorted({str(k) for row in rows for k in dict(row.get("telemetry", {})).keys()})
    for trials, expr in enumerate(_operator_expr_candidates(state_keys, telemetry_keys), start=1):
        if _operator_matches(expr, train, output_field) and _operator_matches(expr, holdout, output_field):
            return CognitiveOperator(
                operator_id="cop:" + _digest((output_field, expr))[:24],
                input_fields=tuple(state_keys + telemetry_keys),
                output_field=str(output_field),
                body={"kind": "state_transform", "expression": copy.deepcopy(expr)},
                source_examples=tuple(copy.deepcopy(x) for x in rows[:8]),
                construction_trials=trials,
                provenance={
                    "origin": "substrate_synthesized_operator",
                    "human_authored_solution": False,
                    "external_model_solution": False,
                    "held_out_validation": True,
                },
            )
    return None


def construct_cognitive_operator(
    self: GeneralLearner,
    rows: Sequence[Mapping[str, Any]],
    *,
    output_field: str,
) -> dict[str, Any] | None:
    _ensure_013(self)
    op = _construct_cognitive_operator(self, rows, output_field=output_field)
    if op is None:
        self._asi_core_013_state["events"].append({
            "event": "COGNITIVE_OPERATOR_CONSTRUCTION_FAILED",
            "output_field": str(output_field),
        })
        return None
    row = op.to_dict()
    self._asi_core_013_state["cognitive_operators"][op.operator_id] = row
    self._asi_core_013_state["events"].append({
        "event": "COGNITIVE_OPERATOR_CONSTRUCTED",
        "operator_id": op.operator_id,
        "construction_trials": op.construction_trials,
    })
    return copy.deepcopy(row)


def validate_and_retain_cognitive_operator(
    self: GeneralLearner,
    operator_id: str,
    evaluation: Mapping[str, Any],
) -> bool:
    _ensure_013(self)
    raw = self._asi_core_013_state["cognitive_operators"].get(str(operator_id))
    if raw is None:
        return False
    accepted = bool(
        evaluation.get("held_out_passed")
        and evaluation.get("fresh_process_passed")
        and evaluation.get("regression_passed")
        and float(evaluation.get("generalization_score", 0.0)) >= 0.90
    )
    raw["evaluation"] = copy.deepcopy(dict(evaluation))
    raw["status"] = "validated" if accepted else "rejected"
    self._asi_core_013_state["events"].append({
        "event": "COGNITIVE_OPERATOR_DECISION",
        "operator_id": str(operator_id),
        "decision": "ACCEPTED" if accepted else "REJECTED",
    })
    return accepted


def execute_learned_cognitive_operator(
    self: GeneralLearner,
    operator_id: str,
    *,
    telemetry: Mapping[str, Any] | None = None,
    dry_run: bool = True,
) -> dict[str, Any]:
    _ensure_013(self)
    raw = self._asi_core_013_state["cognitive_operators"].get(str(operator_id))
    if raw is None or str(raw.get("status")) != "validated":
        return {"used": False, "reason": "operator_not_validated"}
    op = CognitiveOperator(**copy.deepcopy(raw))
    projection = {
        "frontier_limit": int(self._asi_core_013_state["generator"].get("frontier_limit", 8)),
    }
    obs = dict(telemetry or self._asi_core_013_state.get("latest_telemetry", {}))
    proposed = op.apply(projection, obs)
    result = {
        "used": True,
        "operator_id": op.operator_id,
        "output_field": op.output_field,
        "state_before": copy.deepcopy(projection),
        "telemetry": copy.deepcopy(obs),
        "proposed": proposed,
        "dry_run": bool(dry_run),
    }
    if not dry_run:
        if op.output_field != "frontier_limit":
            raise ValueError("unsupported learned operator output field")
        self._asi_core_013_state["generator"]["frontier_limit"] = max(
            1, min(64, int(round(float(proposed))))
        )
        result["state_after"] = {
            "frontier_limit": int(self._asi_core_013_state["generator"]["frontier_limit"])
        }
    return result


def diagnose_cognitive_failure(
    self: GeneralLearner,
    *,
    layer: str,
    telemetry: Mapping[str, Any],
) -> tuple[dict[str, Any], ...]:
    """Infer competing process-level diagnoses from substrate-owned telemetry only."""
    _ensure_013(self)
    layer = str(layer)
    t = dict(telemetry)
    diagnoses: list[tuple[str, float, str]] = []

    prediction_error = float(t.get("prediction_error", 0.0))
    search_work = float(t.get("search_work", 0.0))
    search_budget = max(1.0, float(t.get("search_budget", 0.0)))
    transfer_failure = bool(t.get("transfer_failure", False))
    planning_failure = bool(t.get("planning_failure", False))
    uncertainty = float(t.get("uncertainty", 0.0))
    repeated_work = float(t.get("repeated_work", 0.0))
    context_ambiguity = float(t.get("context_ambiguity", 0.0))
    candidate_count = float(t.get("candidate_count", 0.0))
    procedure_frontier = max(1.0, float(self._asi_core_013_state["generator"].get("frontier_limit", 8)))

    if transfer_failure:
        diagnoses.append(("NEGATIVE_TRANSFER", 0.34, "old cognitive process succeeds on a prior surface but fails after reuse on a new surface"))
    if planning_failure:
        diagnoses.append(("PLANNING_HORIZON_INSUFFICIENT", 0.34, "goal-directed prediction fails before the planning horizon reaches a solution"))
    if context_ambiguity > 0.0 or uncertainty >= 0.5:
        diagnoses.append(("CONTEXT_AMBIGUITY", 0.30 + min(0.25, context_ambiguity), "multiple process explanations remain observationally plausible"))
    if search_work >= 0.85 * search_budget and search_budget > 1.0:
        diagnoses.append(("SEARCH_SATURATION", 0.42, "search consumes most available cognitive budget without resolving the failure"))
    if repeated_work > 0.0:
        diagnoses.append(("RETRIEVAL_REDUNDANCY", 0.24, "the telemetry shows repeated work that a retained retrieval process might avoid"))
    if prediction_error > 0.0:
        diagnoses.append(("PROCESS_MODEL_ERROR", 0.28, "the currently deployed cognitive process predicts an outcome inconsistent with observed feedback"))
    if layer == "meta" and candidate_count >= procedure_frontier:
        diagnoses.append(("PROCEDURE_SEARCH_SATURATION", 0.50, "the procedure search frontier is fully consumed without a validated improvement"))

    if not diagnoses:
        diagnoses.append(("INSUFFICIENT_EVIDENCE", 0.60, "available telemetry does not distinguish a process failure from insufficient information"))
        diagnoses.append(("CURRENT_PROCESS_APPEARS_SUFFICIENT", 0.40, "available telemetry contains no strong failure signal"))

    total = sum(score for _, score, _ in diagnoses) or 1.0
    out = []
    for kind, score, rationale in diagnoses:
        out.append({
            "diagnosis_id": "dx:" + _digest((layer, kind, t))[:20],
            "kind": kind,
            "confidence": float(score) / total,
            "layer": layer,
            "evidence": copy.deepcopy(t),
            "rationale": rationale,
        })
    out.sort(key=lambda x: (-float(x["confidence"]), x["kind"]))
    self._asi_core_013_state["events"].append({
        "event": "PROCESS_DIAGNOSIS",
        "layer": layer,
        "diagnoses": copy.deepcopy(out),
    })
    self._asi_core_013_state["events"] = self._asi_core_013_state["events"][-256:]
    return tuple(out)



def experimentally_select_and_retain_best_procedure(
    self: GeneralLearner,
    procedures: Sequence[Mapping[str, Any]],
    trial_runner: Callable[[GeneralLearner, Mapping[str, Any]], Mapping[str, Any]],
) -> dict[str, Any]:
    """Evaluate competing procedures on isolated clones and let the substrate choose the winner.

    The trial runner is an experimental boundary: it may measure outcomes but it must not
    choose or mutate the live learner. Only one procedure is eligible for activation.
    """
    _ensure_013(self)
    evaluations: list[dict[str, Any]] = []
    # Freeze one parent snapshot for the entire competition. Live state is not mutated until
    # every arm has been evaluated, preventing order effects and winner leakage.
    parent_state = self.export_state()

    for proc in procedures:
        pid = str(proc.get("procedure_id"))
        clone = GeneralLearner.from_state(parent_state, growth_enabled=True)
        _init_013(clone)
        try:
            result = dict(trial_runner(clone, copy.deepcopy(proc)))
        except Exception as exc:
            result = {
                "regression_passed": False,
                "transfer_passed": False,
                "fresh_process_passed": False,
                "resource_matched": False,
                "evaluator_isolated": False,
                "hidden_family_count": 0,
                "transfer_gain": 0.0,
                "future_reuse_gain": 0.0,
                "resource_cost": float("inf"),
                "exception": type(exc).__name__,
            }
        result["procedure_id"] = pid
        result["procedure_digest"] = str(proc.get("digest") or _digest(proc))
        evaluations.append(result)
        self._asi_core_013_state["library"].setdefault(pid, copy.deepcopy(dict(proc)))

    eligible = [
        e for e in evaluations
        if bool(e.get("regression_passed"))
        and bool(e.get("transfer_passed"))
        and bool(e.get("fresh_process_passed"))
        and bool(e.get("resource_matched"))
        and bool(e.get("evaluator_isolated"))
        and int(e.get("hidden_family_count", 0)) >= 2
        and float(e.get("transfer_gain", 0.0)) > 0.0
        and (float(e.get("transfer_gain", 0.0)) - float(e.get("resource_cost", 0.0))) > 0.0
    ]
    eligible.sort(key=lambda e: (
        float(e.get("transfer_gain", 0.0)),
        float(e.get("future_reuse_gain", 0.0)),
        -float(e.get("resource_cost", 0.0)),
        -float(e.get("candidate_work", 0.0)),
        str(e.get("procedure_digest")),
    ), reverse=True)

    winner = eligible[0] if eligible else None
    decisions = []
    for evaluation in evaluations:
        keep = winner is not None and evaluation["procedure_id"] == winner["procedure_id"]
        decision_eval = dict(evaluation)
        if not keep:
            decision_eval["transfer_gain"] = min(0.0, float(decision_eval.get("transfer_gain", 0.0)))
        decisions.append(self.record_cognitive_procedure_evaluation(
            evaluation["procedure_id"], decision_eval
        ))

    selection = {
        "candidate_count": len(evaluations),
        "eligible_count": len(eligible),
        "selected_procedure_id": winner["procedure_id"] if winner else None,
        "selection_basis": (
            "heldout_transfer_gain_then_future_reuse_then_resource_cost_then_candidate_work"
            if winner else "no_candidate_met_independent_evidence_gate"
        ),
        "independent_selection": True,
        "winner_evaluation": copy.deepcopy(winner) if winner else None,
    }
    self._asi_core_013_state["events"].append({"event": "PROCEDURE_SELECTION", **selection})
    self._asi_core_013_state["events"] = self._asi_core_013_state["events"][-256:]
    self._asi_core_013_state["evaluations"] = self._asi_core_013_state["evaluations"][-256:]
    return {"selection": selection, "evaluations": evaluations, "decisions": [asdict(x) for x in decisions]}


def experimentally_test_cognitive_procedures(
    self: GeneralLearner,
    procedures: Sequence[Mapping[str, Any]],
    trial_runner: Callable[[GeneralLearner, Mapping[str, Any]], Mapping[str, Any]],
) -> tuple[ProcedureDecision, ...]:
    """Run competing procedures on isolated learner clones and retain only measured winners.

    The environment/trial runner is an experimental boundary. It supplies outcomes only;
    the candidate generator, lifecycle, and decision logic remain substrate-owned.
    """
    _ensure_013(self)
    decisions: list[ProcedureDecision] = []
    for proc in procedures:
        pid = str(proc.get("procedure_id"))
        clone = GeneralLearner.from_state(self.export_state(), growth_enabled=True)
        _init_013(clone)
        clone._asi_core_013_state["enabled"] = bool(self._asi_core_013_state.get("enabled"))
        try:
            clone.execute_cognitive_procedure(proc, dry_run=False)
            evaluation = dict(trial_runner(clone, copy.deepcopy(proc)))
        except Exception as exc:
            evaluation = {
                "regression_passed": False,
                "transfer_passed": False,
                "fresh_process_passed": False,
                "transfer_gain": 0.0,
                "resource_cost": float("inf"),
                "exception": type(exc).__name__,
            }
        self._asi_core_013_state["library"].setdefault(pid, copy.deepcopy(dict(proc)))
        decisions.append(self.record_cognitive_procedure_evaluation(pid, evaluation))
    return tuple(decisions)


def procedure_generator_status(self: GeneralLearner) -> dict[str, Any]:
    _ensure_013(self)
    return {
        "frontier_limit": int(self._asi_core_013_state["generator"].get("frontier_limit", 8)),
        "generation": int(self._asi_core_013_state["generator"].get("generation", 0)),
        "recursion_depth": int(self._asi_core_013_state["generator"].get("recursion_depth", 0)),
        "learned": bool(self._asi_core_013_state["generator"].get("learned", False)),
        "library_size": len(self._asi_core_013_state.get("library", {})),
    }


def propose_cognitive_procedures(self: GeneralLearner, max_candidates: int | None = None) -> tuple[dict[str, Any], ...]:
    """Generate competing generic process candidates from the latest internal failure.

    This is the bounded first-generation procedure language. Candidate identity and target
    task answers are never supplied; proposals are derived from failure category and
    learner-owned telemetry.
    """
    _ensure_013(self)
    limit = int(max_candidates or self._asi_core_013_state["generator"]["frontier_limit"])
    failures = self._asi_core_013_state["failures"]
    if not failures:
        return ()
    failure = failures[-1]
    layer = str(failure["layer"])
    diagnosis = str(failure["diagnosis"]).upper()
    evidence = failure.get("evidence", {})

    candidates: list[CognitiveProcedure] = []

    def add(tag: str, steps: Sequence[Mapping[str, Any]], effects: Mapping[str, float], extra_work: float, rationale: str):
        base = _base_candidate(self, layer, {
            "diagnosis": diagnosis,
            "evidence_signature": _digest(evidence)[:16],
            "tag": tag,
        })
        p = CognitiveProcedure(
            procedure_id="proc:" + _digest((base.digest, tag, steps))[:20],
            parent_id=base.parent_id,
            target_layer=layer,
            trigger=base.trigger,
            steps=tuple(copy.deepcopy(list(steps))),
            preconditions=base.preconditions,
            predicted_effects=dict(effects),
            resource_profile={"extra_work": float(extra_work)},
            generation=int(base.generation),
            provenance={**dict(base.provenance), "rationale": rationale},
        )
        candidates.append(p)

    # Experience-conditioned candidates are synthesized from validated process genomes.
    # The genome is selected by empirical utility + exploration uncertainty, then mutated
    # within a bounded generic operator. This is the recursive component H002 was missing.
    if self._asi_core_013_state["generator"].get("experience_conditioned_generation"):
        archive = self._asi_core_013_state.get("strategy_archive", {})
        total_trials = sum(int(v.get("trials", 0)) for v in archive.values())
        scored_templates = []
        for pid in self._asi_core_013_state["generator"].get("strategy_templates", ()):
            rec = self._asi_core_013_state.get("library", {}).get(str(pid))
            stats = archive.get(str(pid))
            if rec is None or stats is None or str(rec.get("status")) != "validated":
                continue
            scored_templates.append((_strategy_ucb(stats, total_trials), str(pid), rec))
        scored_templates.sort(key=lambda x: (-x[0], x[1]))
        for _, pid, rec in scored_templates[:4]:
            for variant in (0, 1):
                mutation = _mutate_strategy_template(
                    rec,
                    layer=layer,
                    trigger={
                        "diagnosis": diagnosis,
                        "evidence_signature": _digest(evidence)[:16],
                        "tag": "experience_conditioned_mutation",
                    },
                    variant_index=variant,
                )
                add(
                    f"experience_template:{pid}:v{variant}",
                    mutation["steps"],
                    {"reusable_strategy_gain": 0.50, "search_recall": 0.55},
                    float((rec.get("resource_profile") or {}).get("extra_work", 1.0)) * (1.0 + 0.15 * variant),
                    "mutate a previously validated process genome using empirically retained utility",
                )

    # Generic process alternatives. The search space is intentionally explicit and small
    # for H001; later hypotheses are allowed to evolve this language itself.
    add(
        "retrieve_then_search",
        ({"op": "set_layer_policy", "layer": layer, "mode": "retrieve_first"},),
        {"retrieval_efficiency": 0.35, "transfer": 0.10},
        0.10,
        "attempt reuse of retained procedural knowledge before new search",
    )
    add(
        "evidence_before_construction",
        (
            {"op": "set_layer_policy", "layer": layer, "mode": "evidence_first"},
            {"op": "toggle_experiment_policy"},
        ),
        {"information_gain": 0.40, "error_reduction": 0.25},
        0.20,
        "seek discriminating evidence before committing to a single process",
    )

    if any(k in diagnosis for k in ("SEARCH", "OPERATOR", "COMPOSITION", "CAPABILITY")) or int(evidence.get("search_work", 0)) > 0:
        # Preserve competing resource-allocation hypotheses. The learner must select among
        # them experimentally rather than receiving a single hand-picked search multiplier.
        add(
            "expand_search_balanced",
            (
                {"op": "adjust_open_search", "budget_factor": 2.0, "depth_delta": 1, "beam_delta": 16},
                {"op": "set_layer_policy", "layer": layer, "mode": "deeper_search"},
            ),
            {"reachable_capability": 0.30, "search_recall": 0.45},
            1.0,
            "trade bounded additional search for a larger executable hypothesis frontier",
        )
        add(
            "expand_search_deep",
            (
                {"op": "adjust_open_search", "budget_factor": 2.0, "depth_delta": 2, "beam_delta": 32},
                {"op": "set_layer_policy", "layer": layer, "mode": "deeper_search"},
            ),
            {"reachable_capability": 0.45, "search_recall": 0.60},
            1.8,
            "prioritize deeper compositional process hypotheses after search saturation",
        )
        add(
            "expand_search_budget",
            (
                {"op": "adjust_open_search", "budget_factor": 4.0, "depth_delta": 1, "beam_delta": 16},
                {"op": "set_layer_policy", "layer": layer, "mode": "budget_expanded_search"},
            ),
            {"reachable_capability": 0.40, "search_recall": 0.55},
            2.2,
            "prioritize broader hypothesis enumeration when the search budget is the observed bottleneck",
        )

    if "CONTEXT" in diagnosis or "AMBIG" in diagnosis or "TRANSFER" in diagnosis:
        add(
            "context_disambiguation_first",
            (
                {"op": "toggle_experiment_policy"},
                {"op": "set_layer_policy", "layer": layer, "mode": "context_first"},
            ),
            {"context_selection": 0.45, "negative_transfer": 0.30},
            0.25,
            "use disagreement-sensitive evidence before reusing a context-dependent process",
        )

    if "RETRIEVAL" in diagnosis or int(evidence.get("repeated_work", 0)) > 0:
        add(
            "indexed_reuse",
            (
                {"op": "toggle_retrieval"},
                {"op": "set_layer_policy", "layer": layer, "mode": "indexed_reuse"},
            ),
            {"search_reduction": 0.45, "repeat_work": 0.40},
            0.25,
            "change generic retrieval organization while preserving learned content",
        )

    if layer == "meta" or "PROCEDURE" in diagnosis or "META" in diagnosis:
        add(
            "wider_procedure_search",
            (
                {"op": "adjust_procedure_frontier", "factor": 2.0},
                {"op": "set_layer_policy", "layer": "meta", "mode": "broader_process_search"},
            ),
            {"procedure_search_recall": 0.45, "recursive_gain": 0.25},
            0.50,
            "increase the substrate's own bounded search frontier when procedure discovery saturates",
        )

    if "REPRESENTATION" in diagnosis or layer == "representation":
        add(
            "representation_exploration",
            (
                {"op": "toggle_primitive_invention"},
                {"op": "toggle_representation_memory"},
                {"op": "set_layer_policy", "layer": "representation", "mode": "explore_and_retain"},
            ),
            {"representation_recall": 0.35, "novelty": 0.25},
            0.30,
            "change generic representation exploration and retention policy after representation failure",
        )

    if "PLAN" in diagnosis or "HORIZON" in diagnosis:
        add(
            "longer_horizon_replan",
            (
                {"op": "adjust_planning", "horizon_delta": 2},
                {"op": "set_layer_policy", "layer": "planning", "mode": "replan_on_residual"},
            ),
            {"planning_horizon": 0.35, "recovery": 0.20},
            0.35,
            "increase model-based planning horizon after observed planning failure",
        )

    # Always preserve a no-op control for causal attribution.
    add(
        "control",
        ({"op": "set_layer_policy", "layer": layer, "mode": "existing"},),
        {"control": 0.0},
        0.0,
        "same-process control with no cognitive-policy change",
    )

    # De-duplicate by procedure digest and keep bounded competition.
    seen: set[str] = set()
    out: list[dict[str, Any]] = []
    for p in candidates:
        if p.digest in seen:
            continue
        seen.add(p.digest)
        payload = p.to_dict()
        payload["digest"] = p.digest
        out.append(payload)
        if len(out) >= max(1, limit):
            break

    self._asi_core_013_state["generator"]["generation"] = int(self._asi_core_013_state["generator"].get("generation", 0)) + 1
    self._asi_core_013_state["events"].append({
        "event": "PROCEDURES_GENERATED",
        "failure_id": failure["failure_id"],
        "candidate_ids": [x["procedure_id"] for x in out],
    })
    self._asi_core_013_state["events"] = self._asi_core_013_state["events"][-256:]
    for p in out:
        self._asi_core_013_state["library"].setdefault(p["procedure_id"], {**p, "status": "candidate"})
    return tuple(out)


def _apply_step(self: GeneralLearner, step: Mapping[str, Any]) -> dict[str, Any]:
    op = str(step.get("op"))
    s = self._asi_core_013_state
    if op == "set_layer_policy":
        layer = str(step.get("layer"))
        mode = str(step.get("mode"))
        if layer not in _LAYERS:
            raise ValueError(f"unknown layer: {layer}")
        s["default_policy"][layer] = mode
        return {"op": op, "layer": layer, "mode": mode}

    if op == "toggle_retrieval":
        if hasattr(self, "_architecture_006"):
            before = self._architecture_006.retrieval_mode
            # Use the existing architecture IR rather than creating a second retrieval system.
            new_mode = "indexed" if before == "linear" else "linear"
            from .self_improvement import _apply_program
            self._architecture_006 = _apply_program(self._architecture_006, {"op": "toggle_retrieval"})
            return {"op": op, "before": before, "after": self._architecture_006.retrieval_mode}
        return {"op": op, "skipped": "architecture_ir_unavailable"}

    if op == "toggle_memoization":
        stage = str(step.get("stage", "reuse"))
        if hasattr(self, "_architecture_006"):
            from .self_improvement import _apply_program
            before = tuple(getattr(self._architecture_006, "memoized_stages", ()))
            self._architecture_006 = _apply_program(self._architecture_006, {"op": "toggle_memoization", "stage": stage})
            return {"op": op, "stage": stage, "before": before, "after": self._architecture_006.memoized_stages}
        return {"op": op, "skipped": "architecture_ir_unavailable"}

    if op == "toggle_experiment_policy":
        if hasattr(self, "_architecture_006"):
            from .self_improvement import _apply_program
            before = self._architecture_006.experiment_policy
            self._architecture_006 = _apply_program(self._architecture_006, {"op": "toggle_experiment_policy"})
            return {"op": op, "before": before, "after": self._architecture_006.experiment_policy}
        return {"op": op, "skipped": "architecture_ir_unavailable"}

    if op == "adjust_open_search":
        st = getattr(self, "_asi_core_009_state", None)
        if st is None:
            return {"op": op, "skipped": "open_capability_unavailable"}
        policy = st.setdefault("search_policy", {"max_depth": 6, "beam": 64, "budget": 3500})
        before = dict(policy)
        policy["budget"] = min(20000, max(1, int(policy.get("budget", 3500) * float(step.get("budget_factor", 1.0)))))
        policy["max_depth"] = max(1, min(12, int(policy.get("max_depth", 6)) + int(step.get("depth_delta", 0))))
        policy["beam"] = max(4, min(256, int(policy.get("beam", 64)) + int(step.get("beam_delta", 0))))
        return {"op": op, "before": before, "after": dict(policy)}

    if op == "adjust_planning":
        s.setdefault("planning_policy", {})
        before = dict(s["planning_policy"])
        s["planning_policy"]["horizon"] = max(1, min(16, int(s["planning_policy"].get("horizon", 4)) + int(step.get("horizon_delta", 0))))
        return {"op": op, "before": before, "after": dict(s["planning_policy"])}

    if op == "toggle_primitive_invention":
        before = bool(getattr(self, "enable_primitive_invention", False))
        if hasattr(self, "enable_primitive_invention"):
            self.enable_primitive_invention = not before
            return {"op": op, "before": before, "after": bool(self.enable_primitive_invention)}
        return {"op": op, "skipped": "primitive_invention_unavailable"}

    if op == "toggle_representation_memory":
        before = bool(getattr(self, "retain_representation_memory", False))
        if hasattr(self, "retain_representation_memory"):
            self.retain_representation_memory = not before
            return {"op": op, "before": before, "after": bool(self.retain_representation_memory)}
        return {"op": op, "skipped": "representation_memory_unavailable"}

    if op == "adopt_strategy_templates":
        requested = [str(x) for x in step.get("procedure_ids", ())]
        archive = self._asi_core_013_state.get("strategy_archive", {})
        activated = []
        for pid in requested:
            rec = self._asi_core_013_state.get("library", {}).get(pid)
            if rec is not None and str(rec.get("status")) == "validated" and pid in archive:
                activated.append(pid)
        self._asi_core_013_state["generator"]["strategy_templates"] = activated[:8]
        self._asi_core_013_state["generator"]["experience_conditioned_generation"] = bool(activated)
        return {"op": op, "requested": requested, "activated": activated[:8]}

    if op == "apply_learned_cognitive_operator":
        result = self.execute_learned_cognitive_operator(
            str(step.get("operator_id")),
            telemetry=step.get("telemetry"),
            dry_run=False,
        )
        return {"op": op, **result}

    if op == "adjust_procedure_frontier":
        gen = s["generator"]
        before = int(gen.get("frontier_limit", 8))
        factor = float(step.get("factor", 1.0))
        after = max(before + 1, int(round(before * factor)))
        gen["frontier_limit"] = min(64, after)
        return {"op": op, "before": before, "after": int(gen["frontier_limit"])}

    raise ValueError(f"unsupported cognitive procedure operation: {op}")



def propose_meta_cognitive_procedures(self: GeneralLearner) -> tuple[dict[str, Any], ...]:
    """Generate competing second-order process hypotheses from endogenous experience."""
    _ensure_013(self)
    gen = self._asi_core_013_state["generator"]
    limit = int(gen.get("frontier_limit", 8))
    ceiling = 64
    events = self._asi_core_013_state.get("events", ())[-64:]
    saturated = any(
        e.get("event") == "PROCEDURES_GENERATED"
        and len(e.get("candidate_ids", ())) >= limit
        for e in events
    )
    if not saturated or limit >= ceiling:
        return ()

    parent = self._asi_core_013_state.get("active_by_layer", {}).get("meta")
    archive = self._asi_core_013_state.get("strategy_archive", {})
    total_trials = sum(int(v.get("trials", 0)) for v in archive.values())
    ranked = []
    for pid, stats in archive.items():
        rec = self._asi_core_013_state.get("library", {}).get(pid)
        if rec is None or str(rec.get("target_layer")) == "meta" or str(rec.get("status")) != "validated":
            continue
        ranked.append((_strategy_ucb(stats, total_trials), str(pid)))
    ranked.sort(key=lambda x: (-x[0], x[1]))

    out = []

    frontier_steps = (
        {"op": "adjust_procedure_frontier", "factor": 2.0},
        {"op": "set_layer_policy", "layer": "meta", "mode": "broader_process_search"},
    )
    out.append(CognitiveProcedure(
        procedure_id="proc:" + _digest(("meta_frontier", parent, limit, frontier_steps))[:20],
        parent_id=parent,
        target_layer="meta",
        trigger={"diagnosis": "PROCEDURE_SEARCH_SATURATION", "frontier_limit": limit},
        steps=frontier_steps,
        preconditions=("no_external_evaluator_access", "no_hidden_regime_labels"),
        predicted_effects={"procedure_search_recall": 0.45, "recursive_gain": 0.15},
        resource_profile={"extra_work": float(limit)},
        generation=int(gen.get("generation", 0)),
        provenance={
            "origin": "substrate_generated",
            "second_order": True,
            "meta_strategy": "frontier_only_generation",
            "human_authored_solution": False,
            "external_model_solution": False,
        },
    ))

    if ranked:
        source_ids = [pid for _, pid in ranked[:4]]
        experience_steps = (
            {"op": "adopt_strategy_templates", "procedure_ids": source_ids},
            {"op": "adjust_procedure_frontier", "factor": 2.0},
            {"op": "set_layer_policy", "layer": "meta", "mode": "experience_conditioned_generation"},
        )
        out.append(CognitiveProcedure(
            procedure_id="proc:" + _digest(("meta_experience", parent, limit, source_ids, experience_steps))[:20],
            parent_id=parent,
            target_layer="meta",
            trigger={"diagnosis": "PROCEDURE_SEARCH_SATURATION", "frontier_limit": limit},
            steps=experience_steps,
            preconditions=("no_external_evaluator_access", "no_hidden_regime_labels"),
            predicted_effects={"procedure_search_recall": 0.65, "recursive_gain": 0.40},
            resource_profile={"extra_work": float(limit + len(source_ids))},
            generation=int(gen.get("generation", 0)),
            provenance={
                "origin": "substrate_generated",
                "second_order": True,
                "meta_strategy": "experience_conditioned_generation",
                "source_procedure_ids": source_ids,
                "human_authored_solution": False,
                "external_model_solution": False,
            },
        ))

    rows = []
    for p in out:
        row = p.to_dict()
        row["digest"] = p.digest
        self._asi_core_013_state["library"].setdefault(p.procedure_id, {**row, "status": "candidate"})
        rows.append(row)
    self._asi_core_013_state["events"].append({
        "event": "META_PROCEDURES_GENERATED",
        "procedure_ids": [r["procedure_id"] for r in rows],
        "source_procedure_ids": [pid for _, pid in ranked[:4]],
    })
    self._asi_core_013_state["events"] = self._asi_core_013_state["events"][-256:]
    return tuple(copy.deepcopy(r) for r in rows)


def propose_meta_cognitive_procedure(self: GeneralLearner) -> dict[str, Any] | None:
    candidates = propose_meta_cognitive_procedures(self)
    return copy.deepcopy(candidates[0]) if candidates else None



def run_cognitive_procedure_task(
    self: GeneralLearner,
    layer: str,
    *,
    task_id: str,
    examples: Sequence[tuple[Json, Json]] = (),
    query: Json | None = None,
    actions: Sequence[str] = (),
    goal: Mapping[str, Any] | None = None,
    budget: int = 4000,
) -> tuple[Json | None, dict[str, Any]]:
    """Execute the deployed procedure through an opt-in cognitive boundary."""
    _ensure_013(self)
    layer = str(layer)
    proc = self.get_active_cognitive_procedure(layer) if self._asi_core_013_state.get("enabled") else None
    if proc is not None:
        execution = self.execute_cognitive_procedure(proc, dry_run=False)
    else:
        execution = {"used": False, "reason": "no_active_cognitive_procedure"}

    mode = str(self._asi_core_013_state.get("default_policy", {}).get(layer, "existing"))
    if layer in {"representation", "learning", "search"}:
        if not examples:
            raise ValueError("examples required for representation/learning/search execution")
        pred, rec = self.learn_architecture_task(task_id, examples, query, budget=budget) if layer == "search" else self.learn_task(task_id, examples, query, budget=budget)
        record = asdict(rec) if hasattr(rec, "__dataclass_fields__") else copy.deepcopy(rec)
        return pred, {"layer": layer, "mode": mode, "procedure": execution, "record": record}

    if layer == "acquisition":
        pred, rec = self.attempt_open_capability(task_id, examples, query, budget=budget)
        return pred, {"layer": layer, "mode": mode, "procedure": execution, "record": rec}

    if layer == "planning":
        if not actions or goal is None:
            raise ValueError("actions and goal required for planning execution")
        policy = self._asi_core_013_state.get("planning_policy", {})
        horizon = int(policy.get("horizon", 4))
        rec = self.interactive_plan(query, actions, goal, horizon=horizon)
        return rec.get("predicted_final"), {"layer": layer, "mode": mode, "procedure": execution, "plan": rec}

    return None, {"layer": layer, "mode": mode, "procedure": execution}


def execute_cognitive_procedure(
    self: GeneralLearner,
    procedure: Mapping[str, Any] | CognitiveProcedure,
    *,
    dry_run: bool = False,
) -> dict[str, Any]:
    _ensure_013(self)
    raw = procedure.to_dict() if isinstance(procedure, CognitiveProcedure) else dict(procedure)
    steps = list(raw.get("steps", ()))
    clone = copy.deepcopy(self)
    _init_013(clone)
    trace = []
    for step in steps:
        trace.append(_apply_step(clone, step))
    result = {
        "procedure_id": str(raw.get("procedure_id")),
        "target_layer": str(raw.get("target_layer")),
        "dry_run": bool(dry_run),
        "steps": trace,
        "state_digest_before": _digest({
            "architecture": getattr(self, "_architecture_006", None).to_dict() if hasattr(getattr(self, "_architecture_006", None), "to_dict") else None,
            "open_policy": copy.deepcopy(getattr(self, "_asi_core_009_state", {}).get("search_policy", {})),
            "planning_policy": copy.deepcopy(self._asi_core_013_state.get("planning_policy", {})),
            "layer_policy": copy.deepcopy(self._asi_core_013_state.get("default_policy", {})),
        }),
        "state_digest_after": _digest({
            "architecture": getattr(clone, "_architecture_006", None).to_dict() if hasattr(getattr(clone, "_architecture_006", None), "to_dict") else None,
            "open_policy": copy.deepcopy(getattr(clone, "_asi_core_009_state", {}).get("search_policy", {})),
            "planning_policy": copy.deepcopy(clone._asi_core_013_state.get("planning_policy", {})),
            "layer_policy": copy.deepcopy(clone._asi_core_013_state.get("default_policy", {})),
        }),
    }
    if not dry_run:
        # Apply the already-validated mechanics to the live learner only when explicitly requested.
        for step in steps:
            result.setdefault("live_steps", []).append(_apply_step(self, step))
        self._asi_core_013_state["events"].append({
            "event": "PROCEDURE_EXECUTED",
            "procedure_id": str(raw.get("procedure_id")),
            "state_digest_after": result["state_digest_after"],
        })
    return result


def record_cognitive_procedure_evaluation(
    self: GeneralLearner,
    procedure_id: str,
    evaluation: Mapping[str, Any],
) -> ProcedureDecision:
    _ensure_013(self)
    pid = str(procedure_id)
    rec = self._asi_core_013_state["library"].get(pid)
    if rec is None:
        raise KeyError(f"unknown procedure: {pid}")

    regression = bool(evaluation.get("regression_passed"))
    transfer = bool(evaluation.get("transfer_passed"))
    rebuild = bool(evaluation.get("fresh_process_passed", True))
    utility = float(evaluation.get("transfer_gain", 0.0)) - float(evaluation.get("resource_cost", 0.0))
    explicit_recursion = bool(evaluation.get("recursive_gain", False))
    accepted = bool(regression and transfer and rebuild and utility > 0.0)
    decision = "ACCEPTED" if accepted else "REJECTED"
    reason = (
        "independent regression+transfer+reconstruction passed with positive utility"
        if accepted else
        "candidate failed regression/transfer/reconstruction or produced non-positive utility"
    )

    rec["evaluation"] = copy.deepcopy(dict(evaluation))
    rec["status"] = "validated" if accepted else "rejected"
    rec["decision"] = decision
    self._asi_core_013_state["evaluations"].append({
        "procedure_id": pid,
        "decision": decision,
        "evaluation": copy.deepcopy(dict(evaluation)),
        "utility": utility,
    })
    self._asi_core_013_state["evaluations"] = self._asi_core_013_state["evaluations"][-256:]

    if accepted:
        _update_strategy_archive(self, pid, evaluation)
        layer = str(rec["target_layer"])
        self._asi_core_013_state["active_by_layer"][layer] = pid
        self._asi_core_013_state["default_policy"][layer] = "procedure:" + pid
        self._asi_core_013_state["generator"]["learned"] = True
        if explicit_recursion:
            self._asi_core_013_state["generator"]["recursion_depth"] = int(self._asi_core_013_state["generator"].get("recursion_depth", 0)) + 1
    self._asi_core_013_state["events"].append({
        "event": "PROCEDURE_DECISION",
        "procedure_id": pid,
        "decision": decision,
        "reason": reason,
    })
    self._asi_core_013_state["events"] = self._asi_core_013_state["events"][-256:]
    return ProcedureDecision(pid, decision, copy.deepcopy(dict(evaluation)), reason, rec.get("parent_id"))


def get_active_cognitive_procedure(self: GeneralLearner, layer: str) -> dict[str, Any] | None:
    _ensure_013(self)
    pid = self._asi_core_013_state["active_by_layer"].get(str(layer))
    if not pid:
        return None
    return copy.deepcopy(self._asi_core_013_state["library"].get(pid))


def retrieve_cognitive_procedures(self: GeneralLearner, layer: str, *, status: str = "validated") -> tuple[dict[str, Any], ...]:
    _ensure_013(self)
    rows = []
    for rec in self._asi_core_013_state["library"].values():
        if str(rec.get("target_layer")) == str(layer) and str(rec.get("status")) == status:
            rows.append(rec)
    rows.sort(key=lambda r: (
        -float((r.get("evaluation") or {}).get("transfer_gain", 0.0)),
        float((r.get("evaluation") or {}).get("resource_cost", 0.0)),
        str(r.get("procedure_id")),
    ))
    return tuple(copy.deepcopy(rows))


def cognitive_procedure_reuse(self: GeneralLearner, layer: str) -> dict[str, Any] | None:
    """Return the retained process to use for a later task; never returns target answers."""
    return get_active_cognitive_procedure(self, layer)


def execute_reused_cognitive_procedure(self: GeneralLearner, layer: str, *, dry_run: bool = True) -> dict[str, Any]:
    proc = get_active_cognitive_procedure(self, layer)
    if proc is None:
        return {"used": False, "reason": "no_validated_procedure"}
    result = execute_cognitive_procedure(self, proc, dry_run=dry_run)
    result["used"] = True
    result["reuse"] = True
    return result


def _export_013(self: GeneralLearner, base: Mapping[str, Any]) -> dict[str, Any]:
    _ensure_013(self)
    out = copy.deepcopy(dict(base))
    out["asi_core_013"] = copy.deepcopy(self._asi_core_013_state)
    return out


def _load_013(self: GeneralLearner, state: Mapping[str, Any]) -> None:
    info = dict(state.get("asi_core_013", {}) or {})
    _init_013(self)
    self._asi_core_013_state = {
        "enabled": bool(info.get("enabled", False)),
        "library": {str(k): dict(v) for k, v in dict(info.get("library", {})).items()},
        "active_by_layer": {str(k): str(v) for k, v in dict(info.get("active_by_layer", {})).items()},
        "events": list(info.get("events", ())),
        "failures": list(info.get("failures", ())),
        "evaluations": list(info.get("evaluations", ())),
        "generator": dict(info.get("generator", {})) or copy.deepcopy(self._asi_core_013_state["generator"]),
        "strategy_archive": {str(k): dict(v) for k, v in dict(info.get("strategy_archive", {})).items()},
        "cognitive_operators": {str(k): dict(v) for k, v in dict(info.get("cognitive_operators", {})).items()},
        "semantic_operators": {str(k): dict(v) for k, v in dict(info.get("semantic_operators", {})).items()},
        "semantic_language": dict(info.get("semantic_language", {})) or {"operators": {}},
        "semantic_operator_classes": {str(k): dict(v) for k, v in dict(info.get("semantic_operator_classes", {})).items()},
        "semantic_class_language": dict(info.get("semantic_class_language", {})) or {"classes": {}},
        "active_semantic_class": info.get("active_semantic_class"),
        "latest_telemetry": dict(info.get("latest_telemetry", {})),
        "default_policy": dict(info.get("default_policy", {})) or copy.deepcopy(self._asi_core_013_state["default_policy"]),
    }


_ORIG_INIT_013 = GeneralLearner.__init__
_ORIG_EXPORT_013 = GeneralLearner.export_state
_ORIG_FROM_STATE_013 = GeneralLearner.from_state

def _init_bound_013(self: GeneralLearner, *args: Any, **kwargs: Any):
    _ORIG_INIT_013(self, *args, **kwargs)
    _init_013(self)

def _export_bound_013(self: GeneralLearner):
    return _export_013(self, _ORIG_EXPORT_013(self))

def _from_state_bound_013(cls, spec: Mapping[str, Any], **kwargs: Any):
    obj = _ORIG_FROM_STATE_013.__func__(cls, copy.deepcopy(dict(spec)), **kwargs)
    _init_013(obj)
    _load_013(obj, spec)
    return obj

if not getattr(GeneralLearner, "_asi_core_013_bound", False):
    GeneralLearner.__init__ = _init_bound_013
    GeneralLearner.export_state = _export_bound_013
    GeneralLearner.from_state = classmethod(_from_state_bound_013)
    GeneralLearner.cognitive_evolution_enable = cognitive_evolution_enable
    GeneralLearner.cognitive_evolution_state = cognitive_evolution_state
    GeneralLearner.record_cognitive_failure = record_cognitive_failure
    GeneralLearner.diagnose_cognitive_failure = diagnose_cognitive_failure
    GeneralLearner.experimentally_test_cognitive_procedures = experimentally_test_cognitive_procedures
    GeneralLearner.experimentally_select_and_retain_best_procedure = experimentally_select_and_retain_best_procedure
    GeneralLearner.procedure_generator_status = procedure_generator_status
    GeneralLearner.propose_cognitive_procedures = propose_cognitive_procedures
    GeneralLearner.construct_cognitive_operator = construct_cognitive_operator
    GeneralLearner.execute_learned_cognitive_operator = execute_learned_cognitive_operator
    GeneralLearner.validate_and_retain_cognitive_operator = validate_and_retain_cognitive_operator
    GeneralLearner.execute_cognitive_procedure = execute_cognitive_procedure
    GeneralLearner.propose_meta_cognitive_procedures = propose_meta_cognitive_procedures
    GeneralLearner.propose_meta_cognitive_procedure = propose_meta_cognitive_procedure
    GeneralLearner.run_cognitive_procedure_task = run_cognitive_procedure_task
    GeneralLearner.record_cognitive_procedure_evaluation = record_cognitive_procedure_evaluation
    GeneralLearner.get_active_cognitive_procedure = get_active_cognitive_procedure
    GeneralLearner.retrieve_cognitive_procedures = retrieve_cognitive_procedures
    GeneralLearner.cognitive_procedure_reuse = cognitive_procedure_reuse
    GeneralLearner.execute_reused_cognitive_procedure = execute_reused_cognitive_procedure
    GeneralLearner._asi_core_013_bound = True

__all__ = ["CognitiveProcedure", "ProcedureDecision"]


# CORE-015: autonomous typed multi-effect semantic operator discovery.
def discover_semantic_operator(
    self: GeneralLearner,
    train: Sequence[Mapping[str, Any]],
    holdout: Sequence[Mapping[str, Any]],
    transfer: Sequence[Mapping[str, Any]],
    *,
    generation: int | None = None,
) -> dict[str, Any] | None:
    _ensure_013(self)
    from .semantic_operator_discovery import SemanticOperatorDiscovery
    engine = SemanticOperatorDiscovery(seed=0)
    engine.generation = int(self._asi_core_013_state["generator"].get("recursion_depth", 0) if generation is None else generation)
    op = engine.discover(train, holdout, transfer)
    if op is None:
        self._asi_core_013_state["events"].append({
            "event": "SEMANTIC_OPERATOR_DISCOVERY_FAILED",
            "generation": engine.generation,
        })
        return None
    raw = op.to_dict()
    self._asi_core_013_state["semantic_operators"][op.operator_id] = copy.deepcopy(raw)
    self._asi_core_013_state["semantic_language"]["operators"][op.operator_id] = copy.deepcopy(raw)
    self._asi_core_013_state["generator"]["learned"] = True
    self._asi_core_013_state["generator"]["recursion_depth"] = max(
        int(self._asi_core_013_state["generator"].get("recursion_depth", 0)),
        int(op.generation) + 1,
    )
    self._asi_core_013_state["events"].append({
        "event": "SEMANTIC_OPERATOR_INSTALLED",
        "operator_id": op.operator_id,
        "generation": int(op.generation),
        "effect_count": len(op.effects),
        "language_token": "sop:" + op.operator_id,
    })
    return copy.deepcopy(raw)


def execute_semantic_operator(
    self: GeneralLearner,
    operator_id: str,
    *,
    telemetry: Mapping[str, Any] | None = None,
    dry_run: bool = True,
) -> dict[str, Any]:
    _ensure_013(self)
    raw = self._asi_core_013_state["semantic_operators"].get(str(operator_id))
    if raw is None or str(raw.get("status")) != "validated":
        return {"used": False, "reason": "semantic_operator_not_validated"}
    from .semantic_operator_discovery import Effect, SemanticOperator, apply_to_general_learner
    op = SemanticOperator(
        operator_id=str(raw["operator_id"]),
        input_roles=tuple(raw.get("input_roles", ())),
        effects=tuple(Effect(**e) for e in raw.get("effects", ())),
        kind=str(raw["kind"]),
        generation=int(raw.get("generation", 0)),
        parent_operator_ids=tuple(raw.get("parent_operator_ids", ())),
        train_error=float(raw.get("train_error", 0.0)),
        holdout_error=float(raw.get("holdout_error", 0.0)),
        transfer_error=float(raw.get("transfer_error", 0.0)),
        complexity=int(raw.get("complexity", 0)),
        provenance=dict(raw.get("provenance", {})),
        status=str(raw.get("status", "candidate")),
    )
    result = apply_to_general_learner(self, op, dict(telemetry or self._asi_core_013_state.get("latest_telemetry", {})), dry_run=dry_run)
    result["language_token"] = "sop:" + op.operator_id
    result["installed"] = True
    self._asi_core_013_state["events"].append({
        "event": "SEMANTIC_OPERATOR_EXECUTED",
        "operator_id": op.operator_id,
        "dry_run": bool(dry_run),
        "changed_fields": sorted(k for k in result.get("before", {}) if result.get("after", {}).get(k) != result.get("before", {}).get(k)),
    })
    self._asi_core_013_state["events"] = self._asi_core_013_state["events"][-256:]
    return result


def semantic_operator_language(self: GeneralLearner) -> dict[str, Any]:
    _ensure_013(self)
    return copy.deepcopy(self._asi_core_013_state.get("semantic_language", {"operators": {}}))


if not getattr(GeneralLearner, "_asi_core_015_bound", False):
    GeneralLearner.discover_semantic_operator = discover_semantic_operator
    GeneralLearner.execute_semantic_operator = execute_semantic_operator
    GeneralLearner.semantic_operator_language = semantic_operator_language
    GeneralLearner._asi_core_015_bound = True


# CORE-016 bindings: discover and execute trace-induced stateful operator classes.
def discover_cognitive_operator_class(
    self: GeneralLearner,
    train: Sequence[Sequence[Mapping[str, Any]]],
    holdout: Sequence[Sequence[Mapping[str, Any]]],
    transfer: Sequence[Sequence[Mapping[str, Any]]],
    *,
    parent_operator_ids: Sequence[str] = (),
) -> dict[str, Any] | None:
    _ensure_013(self)
    from .open_semantic_substrate import OpenSemanticSubstrate, OperatorExperience
    substrate = OpenSemanticSubstrate(seed=0)
    substrate.generation = int(self._asi_core_013_state["generator"].get("recursion_depth", 0))
    def convert(episodes):
        out = []
        for episode in episodes:
            rows = []
            for row in episode:
                rows.append(OperatorExperience(
                    event=str(row.get("event")),
                    observed_outcome=tuple(row.get("observed_outcome", row.get("outcome", ()))),
                    chosen_policy=tuple(row.get("chosen_policy", row.get("policy", ()))),
                    reward=float(row.get("reward", 0.0)),
                    context_id=str(row.get("context_id", row.get("episode_id", ""))),
                    step=int(row.get("step", 0)),
                    decision=bool(row.get("decision", False)),
                    episode_id=str(row.get("episode_id", "")),
                ))
            out.append(rows)
        return out
    spec = substrate.discover_operator_class(
        convert(train), convert(holdout), convert(transfer),
        parent_operator_ids=parent_operator_ids,
    )
    if spec is None:
        self._asi_core_013_state["events"].append({"event": "SEMANTIC_OPERATOR_CLASS_DISCOVERY_FAILED"})
        return None
    self._asi_core_013_state["semantic_operator_classes"][spec.class_id] = copy.deepcopy(spec.to_dict())
    self._asi_core_013_state["semantic_class_language"]["classes"][spec.class_id] = substrate.language[spec.class_id]
    self._asi_core_013_state["active_semantic_class"] = spec.class_id
    self._asi_core_013_state["generator"]["learned"] = True
    self._asi_core_013_state["generator"]["recursion_depth"] = max(
        int(self._asi_core_013_state["generator"].get("recursion_depth", 0)), int(spec.generation) + 1
    )
    self._asi_core_013_state["events"].append({
        "event": "SEMANTIC_OPERATOR_CLASS_INSTALLED",
        "class_id": spec.class_id,
        "kind": spec.kind,
        "generation": spec.generation,
        "stateful": spec.stateful,
        "history_dependent": spec.history_dependent,
    })
    return copy.deepcopy(spec.to_dict())


def execute_cognitive_operator_class(
    self: GeneralLearner,
    class_id: str,
    *,
    event: str,
    observed_outcome: Sequence[Any],
    dry_run: bool = False,
) -> dict[str, Any]:
    _ensure_013(self)
    from .open_semantic_substrate import OperatorClassSpec, OpenSemanticSubstrate, StatefulHistoryRouter, apply_discovered_class_to_learner
    raw = self._asi_core_013_state["semantic_operator_classes"].get(str(class_id))
    if raw is None or str(raw.get("status")) != "validated":
        return {"used": False, "reason": "semantic_operator_class_not_validated"}
    spec = OperatorClassSpec(**copy.deepcopy(raw))
    substrate = OpenSemanticSubstrate(seed=0)
    substrate.generation = int(spec.generation)
    payload = self._asi_core_013_state.get("semantic_class_language", {}).get("classes", {}).get(str(class_id))
    if payload is None:
        return {"used": False, "reason": "semantic_operator_class_not_installed"}
    substrate.language[str(class_id)] = copy.deepcopy(payload)
    router = substrate.instantiate(spec)
    policy = router.step(str(event), tuple(observed_outcome))
    if policy is None:
        return {"used": False, "reason": "class_has_no_dispatch_for_observation"}
    result = apply_discovered_class_to_learner(self, policy, dry_run=dry_run)
    result.update({
        "used": True,
        "class_id": str(class_id),
        "class_kind": spec.kind,
        "stateful": spec.stateful,
        "history_dependent": spec.history_dependent,
        "policy": tuple(policy),
    })
    self._asi_core_013_state["events"].append({
        "event": "SEMANTIC_OPERATOR_CLASS_EXECUTED",
        "class_id": str(class_id),
        "dry_run": bool(dry_run),
        "stateful": spec.stateful,
    })
    return result


if not getattr(GeneralLearner, "_asi_core_016_bound", False):
    GeneralLearner.discover_cognitive_operator_class = discover_cognitive_operator_class
    GeneralLearner.execute_cognitive_operator_class = execute_cognitive_operator_class
    GeneralLearner._asi_core_016_bound = True
