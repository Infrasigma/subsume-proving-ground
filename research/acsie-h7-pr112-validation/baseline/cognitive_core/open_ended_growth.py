from __future__ import annotations

"""Generic self-extending recursive capability search.

This layer does not define named capabilities or map task identities to
procedures. It only lets empirically retained executable artifacts expand the
future search language, with depth/resources adapted from observed transfer
evidence.

Literal unboundedness cannot be proven by a finite run; the runtime therefore
has no fixed depth ceiling and the evaluator measures sustained growth under
finite resource horizons.
"""

import copy
import json
from dataclasses import asdict, dataclass
from typing import Mapping, Sequence

from .behavioral_search import find_exact_expression
from .recursive_cognitive_compiler import (
    ArchiveRecord,
    CognitivePrimitive,
    Evaluation,
    ProcessCandidate,
    RecursiveCognitiveCompiler,
    ast_depth,
    ast_nodes,
    digest,
    eval_expr,
)


@dataclass(frozen=True)
class GrowthEvidence:
    accepted: bool
    transfer_error: float
    ood_error: float
    novelty: float
    resource_cost: float
    growth_credit: float
    resulting_max_depth: int
    retained_capability_count: int


class OpenEndedRecursiveCognitiveCompiler(RecursiveCognitiveCompiler):
    """Recursive compiler with self-extending search depth and archive credit.

    The controller is deliberately generic:
    - no task family names are consumed;
    - no target capability labels are consumed;
    - retained executable primitives are the only new search vocabulary;
    - search depth is a mutable resource, not a fixed maximum.
    """

    SCHEMA = "ACSIE.open-ended-recursive-compiler.v1"
    PROCESS_HISTORY_LIMIT = 128

    def __init__(self, max_depth: int = 2, population: int = 24, seed: int = 0):
        super().__init__(max_depth=max_depth, population=population, seed=seed)
        self.growth_credit = 0.0
        self.max_observed_depth = int(max_depth)
        self.growth_history: list[GrowthEvidence] = []
        self.last_search_stats: dict[str, object] = {}
        self.last_primitive_candidate_frontier: tuple[CognitivePrimitive, ...] = ()
        self.last_process_candidate_frontier: tuple[ProcessCandidate, ...] = ()
        self.last_process_validation_summary: dict[str, object] = {}

    def propose_search_policies(self):
        p = self.meta_policy
        d = max(1, int(p["max_depth"]))
        n = float(p["novelty_weight"])
        e = float(p["exploration"])
        m = float(p["mutation_rate"])
        return (
            {"max_depth": d, "novelty_weight": n, "exploration": e, "mutation_rate": m},
            {"max_depth": d + 1, "novelty_weight": n, "exploration": min(1.0, e + 0.1), "mutation_rate": m},
            {"max_depth": d + 2, "novelty_weight": min(0.95, n + 0.1), "exploration": e, "mutation_rate": min(0.95, m + 0.1)},
        )

    def _expression_stream(
        self,
        keys: Sequence[str],
        primitive_ids: Sequence[str],
        max_depth: int,
    ):
        raw_atoms = tuple(self._expr_atoms(keys, primitive_ids))
        # Once a capability exists, prioritize expressions that actually reuse
        # retained executable primitives. This is generic archive-driven search
        # ordering, not task-specific routing.
        macro_atoms = tuple(
            atom for atom in raw_atoms if atom.get("op") == "macro"
        )
        other_atoms = tuple(
            atom for atom in raw_atoms if atom.get("op") != "macro"
        )
        atoms = macro_atoms + other_atoms if primitive_ids else raw_atoms
        levels: dict[int, tuple[Mapping[str, object], ...]] = {1: atoms}
        seen: set[str] = set()

        for expression in atoms:
            key = json.dumps(expression, sort_keys=True, separators=(",", ":"))
            if key not in seen:
                seen.add(key)
                yield expression

        for depth in range(2, int(max_depth) + 1):
            current: list[Mapping[str, object]] = []

            for expression in levels.get(depth - 1, ()):
                current.append({"op": "abs", "arg": expression})
                current.append({"op": "neg", "arg": expression})

            pair_specs = [(depth - 1, depth - 1)]
            for other_depth in range(1, depth - 1):
                pair_specs.append((depth - 1, other_depth))
                pair_specs.append((other_depth, depth - 1))

            for left_depth, right_depth in pair_specs:
                for left in levels.get(left_depth, ()):
                    for right in levels.get(right_depth, ()):
                        for op in ("add", "sub", "mul", "max", "min"):
                            current.append(
                                {
                                    "op": op,
                                    "left": left,
                                    "right": right,
                                }
                            )

            unique: list[Mapping[str, object]] = []
            layer_seen: set[str] = set()
            for expression in current:
                if ast_depth(expression) != depth:
                    continue
                key = json.dumps(expression, sort_keys=True, separators=(",", ":"))
                if key in layer_seen:
                    continue
                layer_seen.add(key)
                unique.append(expression)

            levels[depth] = tuple(unique)
            for expression in unique:
                key = json.dumps(expression, sort_keys=True, separators=(",", ":"))
                if key in seen:
                    continue
                seen.add(key)
                yield expression

    def invent_primitive(
        self,
        train: Sequence,
        holdout: Sequence,
        transfer: Sequence,
        semantics: str = "delta",
    ) -> CognitivePrimitive | None:
        if len(train) < 6:
            return None

        discovery_rows = tuple(train)
        max_depth = max(3, int(self.meta_policy["max_depth"]) + 1)
        result = find_exact_expression(
            self,
            discovery_rows,
            max_depth,
            require_macro=bool(self.generation > 0 and self.primitives),
            max_alternatives=8,
        )
        if result is None:
            self.last_primitive_candidate_frontier = ()
            return None

        def error(rows: Sequence, expression: Mapping[str, object]) -> float:
            if not rows:
                return float("inf")
            vals = []
            pmap = self._pmap()
            for trace in rows:
                try:
                    vals.append(abs(eval_expr(expression, trace.inputs, pmap) - trace.target))
                except Exception:
                    return float("inf")
            return sum(vals) / len(vals)

        specs = list(result.alternatives) or [(result.expression, result.used_primitives)]
        frontier: list[CognitivePrimitive] = []
        for expression, used in specs[:8]:
            pid = "prim:" + digest((expression, semantics))[:20]
            if pid in self.primitives:
                continue
            primitive = CognitivePrimitive(
                pid,
                copy.deepcopy(expression),
                tuple(sorted(set().union(*(t.inputs.keys() for t in discovery_rows)))),
                semantics,
                self.generation,
                tuple(used),
                tuple(sorted({t.family for t in list(train) + list(transfer)})),
                error(train, expression),
                error(holdout, expression),
                error(transfer, expression),
                ast_nodes(expression),
                {
                    "origin": "open_ended_behavioral_dp_search",
                    "schema_given": False,
                    "external_model": False,
                    "selection_source": "discovery_and_selection_only",
                    "ood_used_for_selection": False,
                    "validation_evaluated_after_selection": True,
                    "validation_role": "admission_only",
                    "candidate_frontier_size": min(8, len(specs)),
                    "direct_parent_ids": list(used),
                    "search_stats": dict(result.stats),
                },
            )
            self.primitives[pid] = primitive
            self.archive[pid] = ArchiveRecord(primitive)
            frontier.append(primitive)

        self.last_search_stats = dict(result.stats)
        self.last_primitive_candidate_frontier = tuple(frontier)
        return frontier[0] if frontier else None

    def accept_primitive_frontier(
        self,
        *,
        transfer_rows: Sequence,
        ood_rows: Sequence,
        novelty: float,
        resource_cost: float,
    ) -> dict[str, object]:
        frontier = tuple(self.last_primitive_candidate_frontier)
        if not frontier:
            return {"accepted": False, "primary_primitive": None, "candidate_count": 0, "validated_candidate_count": 0}

        def error(rows: Sequence, expression: Mapping[str, object]) -> float:
            if not rows:
                return float("inf")
            vals = []
            pmap = self._pmap()
            for trace in rows:
                try:
                    vals.append(abs(eval_expr(expression, trace.inputs, pmap) - trace.target))
                except Exception:
                    return float("inf")
            return sum(vals) / len(vals)

        passing: list[CognitivePrimitive] = []
        validation: list[dict[str, object]] = []
        for primitive in frontier:
            oo = error(ood_rows, primitive.expression)
            ok = primitive.transfer_error <= 1e-9 and oo <= 1e-9
            validation.append({"primitive_id": primitive.primitive_id, "transfer_error": primitive.transfer_error, "ood_error": oo, "passed": ok})
            if ok:
                passing.append(primitive)
            else:
                self.primitives.pop(primitive.primitive_id, None)
                self.archive.pop(primitive.primitive_id, None)

        self.events.append({
            "event": "PRIMITIVE_POST_SELECTION_VALIDATION",
            "candidate_count": len(frontier),
            "validated_candidate_count": len(passing),
            "selection_source": "discovery_and_selection_only",
            "validation_role": "admission_only",
            "validation": validation,
        })
        if not passing:
            self.last_primitive_candidate_frontier = ()
            return {"accepted": False, "primary_primitive": None, "candidate_count": len(frontier), "validated_candidate_count": 0, "validation": validation}

        primary = passing[0]
        oo = next(x["ood_error"] for x in validation if x["primitive_id"] == primary.primitive_id)
        evaluation = Evaluation(
            float(primary.train_error), float(primary.holdout_error), float(primary.transfer_error), float(oo),
            1.0, max(1.0, float(resource_cost)), max(0.0, float(novelty)), True,
        )
        self.observe_growth_evidence(evaluation, novelty=novelty, resource_cost=resource_cost)
        self.last_primitive_candidate_frontier = tuple(passing)
        return {
            "accepted": True,
            "primary_primitive": primary,
            "candidate_count": len(frontier),
            "validated_candidate_count": len(passing),
            "accepted_candidate_ids": [p.primitive_id for p in passing],
            "validation": validation,
        }

    def _remember_process(self, process: ProcessCandidate) -> None:
        """Keep temporary synthesized-process state bounded without touching the active process.

        Process candidates are evaluated immediately; only retained capabilities need
        long-lived executable state. This bounds stale temporary objects without changing
        candidate generation, selection, evaluator thresholds, or admission semantics.
        """
        self.processes[process.process_id] = process
        if len(self.processes) <= self.PROCESS_HISTORY_LIMIT:
            return
        protected = {self.active, process.process_id}
        for pid in tuple(self.processes):
            if len(self.processes) <= self.PROCESS_HISTORY_LIMIT:
                break
            if pid in protected:
                continue
            self.processes.pop(pid, None)

    def synthesize_process(
        self,
        rows: Sequence,
        transfer: Sequence,
        ood: Sequence | None = None,
    ) -> ProcessCandidate | None:
        if not rows or not transfer:
            self.last_process_candidate_frontier = ()
            return None

        discovery_rows = tuple(rows)
        primitive_ids = tuple(self.primitives)
        frontier: list[ProcessCandidate] = []
        seen: set[str] = set()
        policies = self.propose_search_policies()

        for policy in policies:
            result = find_exact_expression(
                self,
                discovery_rows,
                max(1, int(policy["max_depth"])),
                require_macro=bool(self.generation > 0 and primitive_ids),
                max_alternatives=8,
            )
            if result is None:
                continue
            self.last_search_stats = dict(result.stats)
            specs = list(result.alternatives) or [(result.expression, result.used_primitives)]
            for expression, used in specs[:8]:
                pid = "proc:" + digest((expression, policy, self.generation))[:20]
                if pid in seen:
                    continue
                seen.add(pid)
                process = ProcessCandidate(
                    pid, copy.deepcopy(expression), tuple(used), self.generation,
                    tuple(used), dict(policy),
                    tuple(sorted({t.family for t in discovery_rows})),
                    ast_nodes(expression),
                    {
                        "origin": "open_ended_behavioral_dp_search",
                        "external_model": False,
                        "retained_macro_count": len(used),
                        "search_depth": int(policy["max_depth"]),
                        "ood_used_for_selection": False,
                        "selection_source": "discovery_and_selection_only",
                        "post_selection_validation_role": "admission_only",
                        "candidate_frontier_size": min(8, len(specs)),
                        "search_stats": dict(result.stats),
                    },
                )
                self._remember_process(process)
                frontier.append(process)
            if frontier:
                break

        self.last_process_candidate_frontier = tuple(frontier)
        if not frontier:
            self.process_frontier = []
            return None

        frontier.sort(key=lambda p: (-len(p.used_primitives), p.complexity, json.dumps(p.expression, sort_keys=True), p.process_id))
        self.process_frontier = [p.process_id for p in frontier]
        return frontier[0]

    def validate_process_frontier(
        self,
        *,
        train: Sequence,
        holdout: Sequence,
        transfer: Sequence,
        ood: Sequence,
        conflict: Sequence = (),
    ) -> tuple[ProcessCandidate | None, Evaluation | None, dict[str, object]]:
        frontier = tuple(self.last_process_candidate_frontier)
        evaluations: list[dict[str, object]] = []
        passing: list[tuple[ProcessCandidate, Evaluation]] = []
        for proc in frontier:
            ev = self.evaluate(proc, train, holdout, transfer, ood, conflict)
            ok = bool(ev.accepted and ev.ood_error <= 1e-9)
            evaluations.append({"process_id": proc.process_id, "transfer_error": ev.transfer_error, "ood_error": ev.ood_error, "accepted": ok})
            if ok:
                passing.append((proc, ev))
        chosen = passing[0] if passing else (None, None)
        summary = {
            "candidate_count": len(frontier),
            "validated_candidate_count": len(passing),
            "selection_source": "discovery_and_selection_only",
            "validation_role": "admission_only",
            "validation": evaluations,
        }
        self.last_process_validation_summary = dict(summary)
        self.events.append({"event": "PROCESS_POST_SELECTION_VALIDATION", **summary})
        return chosen[0], chosen[1], summary

    def observe_growth_evidence(
        self,
        evaluation: Evaluation,
        *,
        novelty: float,
        resource_cost: float,
    ) -> GrowthEvidence:
        safe_novelty = max(0.0, float(novelty))
        cost = max(1.0, float(resource_cost))
        transfer_gain = max(0.0, 1.0 - float(evaluation.transfer_error))
        ood_gain = max(0.0, 1.0 - float(evaluation.ood_error))
        generic_gain = (transfer_gain + ood_gain + safe_novelty) / cost

        if evaluation.accepted and generic_gain > 0.0:
            self.growth_credit += generic_gain
            # No fixed ceiling: each productive retained capability can buy
            # another unit of future search depth.
            increments = max(1, int(self.growth_credit))
            self.growth_credit -= increments
            self.meta_policy["max_depth"] = int(self.meta_policy["max_depth"]) + increments

        self.max_observed_depth = max(
            self.max_observed_depth,
            int(self.meta_policy["max_depth"]),
        )

        evidence = GrowthEvidence(
            accepted=bool(evaluation.accepted),
            transfer_error=float(evaluation.transfer_error),
            ood_error=float(evaluation.ood_error),
            novelty=safe_novelty,
            resource_cost=cost,
            growth_credit=float(self.growth_credit),
            resulting_max_depth=int(self.meta_policy["max_depth"]),
            retained_capability_count=len(self.primitives),
        )
        self.growth_history.append(evidence)
        self.growth_history = self.growth_history[-512:]
        return evidence

    def accept_primitive(
        self,
        primitive_id: str,
        *,
        transfer_error: float,
        ood_error: float,
        novelty: float,
        resource_cost: float,
    ) -> bool:
        if primitive_id not in self.primitives:
            return False
        accepted = (
            float(transfer_error) <= 1e-9
            and float(ood_error) <= 1e-9
        )
        if not accepted:
            self.primitives.pop(primitive_id, None)
            self.archive.pop(primitive_id, None)
            return False
        evaluation = Evaluation(
            train_error=0.0,
            holdout_error=0.0,
            transfer_error=float(transfer_error),
            ood_error=float(ood_error),
            interference_harm=1.0,
            resource_cost=max(1.0, float(resource_cost)),
            novelty=max(0.0, float(novelty)),
            accepted=True,
        )
        self.observe_growth_evidence(
            evaluation,
            novelty=novelty,
            resource_cost=resource_cost,
        )
        return True

    def retain(
        self,
        proc: ProcessCandidate,
        evaluation: Evaluation,
        *,
        novelty: float = 0.0,
    ) -> bool:
        accepted = super().retain(proc, evaluation)
        if not accepted:
            return False
        self.observe_growth_evidence(
            evaluation,
            novelty=novelty,
            resource_cost=max(1.0, float(proc.complexity)),
        )
        return True

    def growth_state(self) -> dict:
        return {
            "schema": self.SCHEMA,
            "max_depth": int(self.meta_policy["max_depth"]),
            "max_observed_depth": int(self.max_observed_depth),
            "growth_credit": float(self.growth_credit),
            "retained_capability_count": len(self.primitives),
            "active_process": self.active,
            "last_search_stats": dict(self.last_search_stats),
            "history": [asdict(x) for x in self.growth_history[-64:]],
        }


__all__ = [
    "GrowthEvidence",
    "OpenEndedRecursiveCognitiveCompiler",
]
