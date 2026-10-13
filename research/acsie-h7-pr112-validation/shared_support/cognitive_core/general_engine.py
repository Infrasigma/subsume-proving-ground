"""ASI-CORE unified learning loop with executable representation search.

One learner state owns representation proposals, hypothesis search, executable skills,
transfer, composition, error records, bottleneck diagnosis, and bounded strategy change.
Representation choice is behavioral: a selected representation transforms the input before
hypothesis search and execution.  This is still a bounded CPU-first research system.
"""
from __future__ import annotations

from dataclasses import asdict, dataclass
from hashlib import sha256
import copy
import itertools
import json
import statistics
import time
from typing import Any, Callable, Mapping, Sequence

from .representation_engine import (
    RepresentationCandidate,
    representation_search,
    _compose_chain,
)
from .primitive_invention import GeneratedPrimitive, construct_with_evidence, construct_open_primitive
from .program_synthesis import apply_program, synthesize_program

Json = Any


def canon(x: Json) -> str:
    return json.dumps(x, sort_keys=True, separators=(",", ":"), ensure_ascii=True)


def digest(x: Json) -> str:
    return sha256(canon(x).encode()).hexdigest()


def _leaf_paths(x: Json, prefix: tuple[Any, ...] = ()):
    if isinstance(x, dict):
        for k in sorted(x):
            yield from _leaf_paths(x[k], prefix + ("d", str(k)))
    elif isinstance(x, list):
        for i, v in enumerate(x):
            yield from _leaf_paths(v, prefix + ("i", i))
    else:
        yield prefix, x


def numeric_paths(x: Json) -> tuple[tuple[tuple[Any, ...], float], ...]:
    return tuple((p, float(v)) for p, v in _leaf_paths(x) if isinstance(v, (int, float)) and not isinstance(v, bool))


def replace_path(x: Json, path: tuple[Any, ...], value: Any) -> Json:
    out = copy.deepcopy(x)
    node = out
    pairs = list(zip(path[::2], path[1::2]))
    if not pairs:
        return value
    for _, k in pairs[:-1]:
        node = node[k]
    node[pairs[-1][1]] = value
    return out


def structural_signature(x: Json) -> Any:
    if isinstance(x, dict):
        return ("dict", tuple((str(k), structural_signature(x[k])) for k in sorted(x)))
    if isinstance(x, list):
        return ("list", len(x), tuple(structural_signature(v) for v in x))
    return type(x).__name__


def flatten_atoms(x: Json) -> tuple[Any, ...]:
    out = []
    for p, v in _leaf_paths(x):
        out.append((p[-2:] if len(p) >= 2 else p, type(v).__name__, v))
    return tuple(out)


def _map_scalars(x: Json, fn: Callable[[Any], Any]) -> Json:
    if isinstance(x, dict):
        return {k: _map_scalars(v, fn) for k, v in x.items()}
    if isinstance(x, list):
        return [_map_scalars(v, fn) for v in x]
    return fn(x)


def _flatten_values(x: Json) -> list[Any]:
    return [v for _, v in _leaf_paths(x)]


@dataclass(frozen=True)
class RepresentationCredit:
    representation_id: str
    predictive_error: float
    sample_efficiency: float
    search_cost: float
    transfer_gain: float
    composition_gain: float
    robustness_surface: float
    robustness_generator: float
    reconstruction_integrity: float
    retention_value: float
    downstream_capability_gain: float
    accepted: bool


@dataclass(frozen=True)
class Skill:
    skill_id: str
    signature: str
    program_steps: tuple[str, ...]
    validation_score: float
    representation_steps: tuple[str, ...] = ("identity",)
    generated_primitives: tuple[GeneratedPrimitive, ...] = ()
    representation_search_trials: int = 0
    representation_construction_trace: tuple[str, ...] = ()

    def apply(self, x: Json) -> Json | None:
        represented = GeneralLearner.apply_representation_chain(x, self.representation_steps)
        if represented is None:
            return None
        out = copy.deepcopy(represented)
        for step in self.program_steps:
            if step.startswith("prim:"):
                primitive = next((p for p in self.generated_primitives if p.primitive_id == step), None)
                out = primitive.apply(out) if primitive is not None else None
            else:
                out = GeneralLearner.apply_step(out, step)
            if out is None:
                return None
        return out

    def inspect(self) -> tuple[str, ...]:
        return self.program_steps

    def test(self, examples: Sequence[tuple[Json, Json]]) -> bool:
        return all(self.apply(x) == y for x, y in examples)


@dataclass(frozen=True)
class LearningRecord:
    task_id: str
    representation: str
    representation_steps: tuple[str, ...]
    strategy: str
    prediction_error: float
    hypotheses_tested: int
    elapsed_ms: float
    learned_skill_id: str | None
    bottleneck: str | None
    representation_search_trials: int = 0
    primitive_search_trials: int = 0
    representation_memory_hit: bool = False


@dataclass(frozen=True)
class Bottleneck:
    task_id: str
    observation_digest: str
    representation: str
    strategy: str
    prediction_digest: str
    actual_digest: str
    error: float
    failed_assumption: str
    category: str
    resource: Mapping[str, float]


@dataclass(frozen=True)
class CognitiveState:
    skills: tuple[Skill, ...] = ()
    preferred_representations: tuple[str, ...] = (
        "identity",
        "numeric_flat",
        "numeric_surface_order",
        "record_numeric_multiset",
        "relational_roles",
    )
    strategy_stats: Mapping[str, tuple[int, float]] | None = None
    failures: tuple[Bottleneck, ...] = ()
    tasks_seen: int = 0
    representation_credits: tuple[RepresentationCredit, ...] = ()
    representation_library: tuple[tuple[str, ...], ...] = ()


class GeneralLearner:
    """Single general learner used by ASI-CORE-001/002."""

    REPRESENTATIONS = (
        "identity",
        "canonical",
        "numeric_flat",
        "numeric_surface_order",
        "record_numeric_multiset",
        "sequence_tokens",
        "relational_roles",
        "length_multiset",
        "pair_normalize",
        "sort",
    )
    STRATEGIES = ("skill_first", "search_wide", "search_deep")

    def __init__(
        self,
        state: CognitiveState | None = None,
        max_depth: int = 2,
        beam: int = 48,
        *,
        fixed_representation: str | None = None,
        frozen_representation_steps: tuple[str, ...] | None = None,
        retain_skills: bool = True,
        retain_representation_memory: bool = True,
        enable_representation_learning: bool = True,
        legacy_representation_fallback: bool = True,
        representation_depth: int = 2,
        enable_primitive_invention: bool = True,
        enable_open_representation_synthesis: bool = True,
        open_representation_depth: int = 5,
        enable_open_primitive_synthesis: bool = True,
        open_primitive_depth: int = 5,
    ) -> None:
        if max_depth < 1:
            raise ValueError("max_depth must be positive")
        self.max_depth = max_depth
        self.beam = beam
        self.fixed_representation = fixed_representation
        self.frozen_representation_steps = frozen_representation_steps
        self.retain_skills = retain_skills
        self.retain_representation_memory = retain_representation_memory
        self.enable_representation_learning = enable_representation_learning
        self.legacy_representation_fallback = legacy_representation_fallback
        self.representation_depth = representation_depth
        self.enable_primitive_invention = enable_primitive_invention
        self.enable_open_representation_synthesis = enable_open_representation_synthesis
        self.open_representation_depth = open_representation_depth
        self.enable_open_primitive_synthesis = enable_open_primitive_synthesis
        self.open_primitive_depth = open_primitive_depth
        self._state = state or CognitiveState(strategy_stats={})
        self._records: list[LearningRecord] = []
        self._last_representation_search_trials = 0
        self._last_primitive_search_trials = 0
        self._last_representation_memory_hit = False

    @property
    def state(self) -> CognitiveState:
        return self._state

    @property
    def skills(self) -> tuple[Skill, ...]:
        return self._state.skills

    @staticmethod
    def rep_distance(rep: str, a: Json, b: Json) -> float:
        if rep in ("identity", "canonical"):
            return 0.0 if a == b else 1.0
        if rep == "sort":
            try:
                return 0.0 if sorted(a) == sorted(b) else 1.0
            except Exception:
                return 1.0
        if rep == "numeric_flat":
            return 0.0 if sorted(numeric_paths(a)) == sorted(numeric_paths(b)) else 1.0
        return 0.5

    @staticmethod
    def _representation_program_runner(program: tuple[str, ...], x: Json) -> Json | None:
        out = copy.deepcopy(x)
        for step in program:
            if step.startswith("__invent__:"):
                kind = step.split(":",1)[1]
                temp = GeneratedPrimitive("temporary", kind)
                out = temp.apply(out)
            elif step.startswith("g:"):
                out = apply_program(out, (step[2:],))
            elif step.startswith("prim:"):
                return None
            else:
                out = GeneralLearner.apply_step(out, step)
            if out is None:
                return None
        return out

    @staticmethod
    def apply_representation_chain(x: Json, steps: Sequence[str]) -> Json | None:
        out = copy.deepcopy(x)
        for step in steps:
            if step.startswith("g:"):
                out = apply_program(out, (step[2:],))
            else:
                # Preserve the earlier CS representation kernel for legacy candidates.
                out = _compose_chain(out, (step,))
            if out is None:
                return None
        return out

    def _program_finder(self, examples: Sequence[tuple[Json, Json]]) -> tuple[tuple[str, ...] | None, int]:
        chosen, tested = self._search(examples, "search_wide", min(1200, max(64, self.beam * 16)))
        if chosen is None and self.max_depth > 1:
            chosen2, tested2 = self._search(examples, "search_deep", min(1200, max(64, self.beam * 16)))
            tested += tested2
            chosen = chosen2
        if chosen is None and self.enable_primitive_invention:
            primitive, tested_primitive = construct_with_evidence(examples)
            tested += tested_primitive
            if primitive is not None:
                # Temporary token for representation scoring; the concrete primitive is
                # reconstructed later and serialized into the retained Skill.
                chosen = ("__invent__:" + primitive.kind,)
        return chosen, tested

    def propose_representations(self, examples: Sequence[tuple[Json, Json]]) -> tuple[RepresentationCandidate, ...]:
        if self.frozen_representation_steps is not None:
            steps = tuple(self.frozen_representation_steps)
            return (RepresentationCandidate(
                "frozen:" + "+".join(steps), steps, 0.0, 0.0, len(steps), True
            ),)
        if self.fixed_representation is not None:
            return (
                RepresentationCandidate(
                    "r:" + self.fixed_representation,
                    (self.fixed_representation,),
                    0.0,
                    0.0,
                    1,
                    True,
                ),
            )
        if not self.enable_representation_learning:
            return (RepresentationCandidate("r:identity", ("identity",), 0.0, 0.0, 1, True),)
        return representation_search(
            examples,
            self._program_finder,
            self._representation_program_runner,
            max_depth=self.representation_depth,
            beam=max(10, min(16, self.beam // 2)),
        )

    @staticmethod
    def apply_step(x: Json, step: str) -> Json | None:
        if step == "identity":
            return copy.deepcopy(x)
        if step == "head" and isinstance(x, (list, tuple)) and x:
            return copy.deepcopy(x[0])
        if step == "tail" and isinstance(x, (list, tuple)) and x:
            return copy.deepcopy(x[-1])
        if step == "len" and isinstance(x, (list, tuple, dict, str)):
            return len(x)
        if step in {"sum", "min", "max"} and isinstance(x, list) and all(isinstance(v, (int, float)) and not isinstance(v, bool) for v in x):
            if not x:
                return None
            if step == "sum": return sum(x)
            if step == "min": return min(x)
            return max(x)
        if step == "rev" and isinstance(x, list):
            return list(reversed(x))
        if step == "rot1" and isinstance(x, list) and x:
            return x[1:] + x[:1]
        if step == "sort" and isinstance(x, list):
            try:
                return sorted(x)
            except TypeError:
                return None
        if step == "neg":
            ns = numeric_paths(x)
            y = copy.deepcopy(x)
            for p, v in ns:
                y = replace_path(y, p, -v)
            return y if ns else None
        if step == "not":
            bs = [(p, v) for p, v in _leaf_paths(x) if isinstance(v, bool)]
            y = copy.deepcopy(x)
            for p, v in bs:
                y = replace_path(y, p, not v)
            return y if bs else None
        if step.startswith("affine_to_scalar:"):
            _, a, b = step.split(":")
            a = float(a)
            b = float(b)
            ns = numeric_paths(x)
            return a * ns[0][1] + b if len(ns) == 1 else None
        if step.startswith("affine:"):
            _, a, b = step.split(":")
            a = float(a)
            b = float(b)
            ns = numeric_paths(x)
            y = copy.deepcopy(x)
            for p, v in ns:
                y = replace_path(y, p, a * v + b)
            return y if ns else None
        if step.startswith("bool:") and isinstance(x, list) and len(x) == 2 and all(isinstance(v, bool) for v in x):
            op = step.split(":", 1)[1]
            a, b = x
            return (a != b) if op == "xor" else ((a and b) if op == "and" else (a or b))
        if step == "grid_mirror" and isinstance(x, list) and x and all(isinstance(r, list) for r in x):
            return [list(reversed(r)) for r in x]
        if step == "grid_rotate" and isinstance(x, list) and x and all(isinstance(r, list) for r in x):
            return [list(row) for row in zip(*x[::-1])]
        if step == "tokens_reverse" and isinstance(x, dict) and isinstance(x.get("tokens"), list):
            return list(reversed(x["tokens"]))
        if step == "inc_all":
            return _map_scalars(x, lambda v: v + 1 if isinstance(v, int) and not isinstance(v, bool) else v)
        if step == "reachable" and isinstance(x, dict) and isinstance(x.get("edges"), list) and "from" in x and "to" in x:
            src, tgt = x["from"], x["to"]
            seen = {src}
            stack = [src]
            while stack:
                u = stack.pop()
                for e in x["edges"]:
                    if isinstance(e, list) and len(e) == 2 and e[0] == u and e[1] not in seen:
                        seen.add(e[1])
                        stack.append(e[1])
            return tgt in seen
        if step == "line_plan" and isinstance(x, dict) and isinstance(x.get("start"), (int, float)) and isinstance(x.get("goal"), (int, float)):
            s, g = int(x["start"]), int(x["goal"])
            return ["R"] * max(0, g - s) + ["L"] * max(0, s - g)
        if step == "select_best" and isinstance(x, dict) and isinstance(x.get("items"), list):
            try:
                return max(x["items"], key=lambda z: (z["v"] / z["w"], z["v"]))["name"]
            except Exception:
                return None
        return None

    def _step_candidates(self, examples: Sequence[tuple[Json, Json]]) -> tuple[str, ...]:
        steps = {
            "identity", "head", "tail", "len", "sum", "min", "max", "rev", "rot1", "sort", "neg", "not",
            "grid_mirror", "grid_rotate", "tokens_reverse", "inc_all", "reachable", "line_plan", "select_best",
        }
        for op in ("xor", "and", "or"):
            steps.add(f"bool:{op}")
        for x, y in examples:
            xp, yp = numeric_paths(x), numeric_paths(y)
            if len(xp) == len(yp) > 0:
                xs = [v for _, v in xp]
                ys = [v for _, v in yp] if len(yp) == len(xp) else []
                for a in (-2, -1, -0.5, 0.5, 1, 2, 3, 4):
                    b = ys[0] - a * xs[0]
                    if all(abs(a * u + b - v) < 1e-9 for u, v in zip(xs, ys)):
                        steps.add(f"affine:{a}:{b}")
                if len(xp) == 1:
                    for a in (-2, -1, -0.5, 0.5, 1, 2, 3, 4):
                        b = ys[0] - a * xs[0]
                        steps.add(f"affine_to_scalar:{a}:{b}")
        return tuple(sorted(steps))

    def _fit(self, program: tuple[str, ...], examples: Sequence[tuple[Json, Json]]) -> float:
        good = 0
        for x, y in examples:
            out = copy.deepcopy(x)
            for step in program:
                out = self.apply_step(out, step)
                if out is None:
                    break
            good += int(out == y)
        return good / max(1, len(examples))

    def _skill_hit(self, examples: Sequence[tuple[Json, Json]], representation_steps: tuple[str, ...]) -> Skill | None:
        # A prior skill may be reusable even when representation search proposes a different
        # candidate. The skill itself carries the representation needed to execute it.
        for skill in reversed(self.skills):
            if skill.test(examples):
                return skill
        return None

    def _search(self, examples: Sequence[tuple[Json, Json]], strategy: str, budget: int) -> tuple[tuple[str, ...] | None, int]:
        steps = self._step_candidates(examples)
        tested = 0
        max_depth = min(self.max_depth, 3)
        for depth in range(1, max_depth + 1):
            if strategy != "search_deep" and depth > 1:
                break
            for parts in itertools.product(steps, repeat=depth):
                if "identity" in parts and depth > 1:
                    continue
                tested += 1
                if tested > budget:
                    return None, tested
                if self._fit(parts, examples) >= 1.0:
                    return parts, tested
        return None, tested

    @staticmethod
    def _apply_program(x: Json, program: Sequence[str], generated: Sequence[GeneratedPrimitive] = ()) -> Json | None:
        out = copy.deepcopy(x)
        for step in program:
            if step.startswith("prim:"):
                primitive = next((p for p in generated if p.primitive_id == step), None)
                out = primitive.apply(out) if primitive is not None else None
            else:
                out = GeneralLearner.apply_step(out, step)
            if out is None:
                return None
        return out

    def _representation_candidates(self, examples: Sequence[tuple[Json, Json]]) -> tuple[RepresentationCandidate, ...]:
        candidates = []
        self._last_representation_search_trials = 0
        self._last_representation_memory_hit = False
        if self.retain_representation_memory:
            for steps in reversed(self._state.representation_library):
                try:
                    if all(self.apply_representation_chain(x, steps) is not None for x, _ in examples):
                        fit = all(self.apply_representation_chain(x, steps) == y for x, y in examples)
                        if fit:
                            candidates.append(RepresentationCandidate(
                                "memory:" + "+".join(steps), steps, 1.0, 1.0, len(steps), True
                            ))
                            self._last_representation_memory_hit = True
                            break
                except Exception:
                    continue
        if self.enable_open_representation_synthesis and not candidates:
            synthesized, trials = synthesize_program(
                examples, max_depth=self.open_representation_depth, beam=max(32, min(96, self.beam * 2))
            )
            self._last_representation_search_trials = trials
            if synthesized is not None:
                candidates.append(RepresentationCandidate(
                    synthesized.program_id,
                    tuple("g:" + step for step in synthesized.steps),
                    1.0 - synthesized.training_error,
                    1.0 - synthesized.holdout_error,
                    synthesized.complexity,
                    True,
                ))
        candidates.extend(self.propose_representations(examples))
        return tuple(candidates) or (RepresentationCandidate("r:identity", ("identity",), 0.0, 0.0, 1, True),)

    def learn_task(self, task_id: str, examples: Sequence[tuple[Json, Json]], query: Json, *, budget: int = 4000):
        if not examples:
            raise ValueError("examples cannot be empty")
        start = time.perf_counter()
        hypotheses = 0
        self._last_representation_search_trials = 0
        self._last_primitive_search_trials = 0
        self._last_representation_memory_hit = False
        chosen: tuple[str, ...] | None = None
        used = "skill_first"
        generated: tuple[GeneratedPrimitive, ...] = ()
        # Reuse is tested before representation search. This is important both for resource
        # efficiency and because a retained skill carries the representation it needs.
        skill = self._skill_hit(examples, tuple()) if self.retain_skills else None
        if skill is not None:
            rep = RepresentationCandidate("r:" + "+".join(skill.representation_steps), skill.representation_steps, 1.0, 1.0, len(skill.representation_steps), True)
            chosen = skill.program_steps
            generated = skill.generated_primitives
        else:
            rep_candidates = self._representation_candidates(examples)
            rep = rep_candidates[0]
            transformed = []
            valid = True
            for x, y in examples:
                tx = self.apply_representation_chain(x, rep.steps)
                if tx is None:
                    valid = False
                    break
                transformed.append((tx, y))
            if valid:
                used = "search_deep" if self._state.failures else "search_wide"
                chosen, n = self._search(transformed, used, min(budget, 2500))
                hypotheses += n
                if chosen is None and used != "search_deep" and self.max_depth > 1:
                    used = "search_deep"
                    chosen, n = self._search(transformed, used, min(budget, 2500))
                    hypotheses += n

        if chosen is None and 'transformed' in locals() and transformed and (self.enable_primitive_invention or self.enable_open_primitive_synthesis):
            primitive = None
            if self.enable_primitive_invention:
                primitive, nprim = construct_with_evidence(transformed)
                hypotheses += nprim
                self._last_primitive_search_trials += nprim
            if primitive is None and self.enable_open_primitive_synthesis:
                primitive, nopen = construct_open_primitive(transformed, max_depth=self.open_primitive_depth, beam=max(32, min(96, self.beam * 2)))
                hypotheses += nopen
                self._last_primitive_search_trials += nopen
            if primitive is not None:
                generated = (primitive,)
                chosen = (primitive.primitive_id,)

        pred = None
        represented_query = self.apply_representation_chain(query, rep.steps)
        if chosen is not None and represented_query is not None:
            pred = copy.deepcopy(represented_query)
            pred = self._apply_program(pred, chosen, generated)
        validation_examples = [(self.apply_representation_chain(x, rep.steps), y) for x, y in examples]
        if chosen and generated:
            validation = sum(int(self._apply_program(x, chosen, generated)==y) for x,y in validation_examples if x is not None) / max(1,len(examples))
        else:
            validation = self._fit(chosen, [(x, y) for x, y in validation_examples if x is not None]) if chosen else 0.0

        newskills = list(self.skills) if self.retain_skills else []
        sid = None
        if self.retain_skills and chosen and validation == 1.0:
            sid = sha256(canon({"rep": rep.steps, "program": chosen}).encode()).hexdigest()[:16]
            trace = tuple(rep.steps) + ("validated_downstream",)
            sk = Skill(sid, digest(structural_signature(examples[0][0])), tuple(chosen), validation, tuple(rep.steps), generated, self._last_representation_search_trials, trace)
            if all(s.skill_id != sid for s in newskills):
                newskills.append(sk)

        elapsed = (time.perf_counter() - start) * 1000
        err = 0.0 if pred is not None else 1.0
        bottleneck = None if pred is not None else ("search_failure" if hypotheses >= budget else "representation_or_mechanism_failure")
        rec = LearningRecord(task_id, rep.name, tuple(rep.steps), used, err, hypotheses, elapsed, sid, bottleneck, self._last_representation_search_trials, self._last_primitive_search_trials, self._last_representation_memory_hit)
        self._records.append(rec)

        stats = dict(self._state.strategy_stats or {})
        cnt, loss = stats.get(used, (0, 0.0))
        stats[used] = (cnt + 1, loss + err)
        credits = list(self._state.representation_credits)
        rep_library = list(self._state.representation_library)
        if self.retain_representation_memory and any(str(x).startswith("g:") for x in rep.steps):
            rep_tuple = tuple(rep.steps)
            if rep_tuple not in rep_library and validation == 1.0:
                rep_library.append(rep_tuple)
        credits.append(
            RepresentationCredit(
                rep.name,
                err,
                1.0 / max(1, len(examples)),
                float(hypotheses),
                0.0,
                0.0,
                0.0,
                rep.score,
                0.0,
                validation,
                0.0,
                bool(pred is not None and validation == 1.0),
            )
        )
        self._state = CognitiveState(tuple(newskills), self._state.preferred_representations, stats, self._state.failures, self._state.tasks_seen + 1, tuple(credits[-512:]), tuple(rep_library[-128:]))
        return pred, rec

    def observe_outcome(self, record: LearningRecord, observation: Json, prediction: Json | None, actual: Json, *, cpu_ms: float = 0.0):
        success = prediction == actual
        if not success:
            b = Bottleneck(
                record.task_id,
                digest(observation),
                record.representation,
                record.strategy,
                digest(prediction),
                digest(actual),
                1.0,
                "current_hypothesis_inconsistent_with_observation",
                record.bottleneck or "reasoning_failure",
                {"cpu_ms": cpu_ms, "hypotheses_tested": float(record.hypotheses_tested)},
            )
            self._state = CognitiveState(self.skills, self._state.preferred_representations, self._state.strategy_stats, self._state.failures + (b,), self._state.tasks_seen, self._state.representation_credits, self._state.representation_library)
        credits = list(self._state.representation_credits)
        for i in range(max(0, len(credits) - 1), -1, -1):
            c = credits[i]
            if c.representation_id == record.representation and c.downstream_capability_gain == 0.0:
                credits[i] = RepresentationCredit(
                    c.representation_id, c.predictive_error, c.sample_efficiency, c.search_cost,
                    c.transfer_gain, c.composition_gain, c.robustness_surface, c.robustness_generator,
                    c.reconstruction_integrity, c.retention_value, 1.0 if success else -1.0,
                    c.accepted and success,
                )
                break
        self._state = CognitiveState(self.skills, self._state.preferred_representations, self._state.strategy_stats, self._state.failures, self._state.tasks_seen, tuple(credits[-512:]), self._state.representation_library)

    def repair_skill(self, skill_id: str, candidate: tuple[str, ...], validation: Sequence[tuple[Json, Json]]) -> bool:
        if not all(self.apply_step_chain(x, candidate) == y for x, y in validation):
            return False
        skills = []
        replaced = False
        for sk in self.skills:
            if sk.skill_id == skill_id:
                skills.append(Skill(skill_id, sk.signature, candidate, 1.0, sk.representation_steps))
                replaced = True
            else:
                skills.append(sk)
        if not replaced:
            return False
        self._state = CognitiveState(tuple(skills), self._state.preferred_representations, self._state.strategy_stats, self._state.failures, self._state.tasks_seen, self._state.representation_credits, self._state.representation_library)
        return True

    @classmethod
    def apply_step_chain(cls, x: Json, steps: Sequence[str]) -> Json | None:
        return cls._run_steps(x, steps)

    @classmethod
    def _run_steps(cls, x: Json, steps: Sequence[str]) -> Json | None:
        out = copy.deepcopy(x)
        for step in steps:
            out = cls.apply_step(out, step)
            if out is None:
                return None
        return out

    def diagnose(self) -> tuple[Bottleneck, ...]:
        return self._state.failures

    def runtime_records(self) -> tuple[LearningRecord, ...]:
        return tuple(self._records)

    def export_state(self) -> dict[str, Any]:
        return asdict(self._state)

    @classmethod
    def from_state(cls, spec: Mapping[str, Any], **kwargs: Any):
        skills = tuple(
            Skill(
                s["skill_id"],
                s["signature"],
                tuple(s["program_steps"]),
                float(s["validation_score"]),
                tuple(s.get("representation_steps", ("identity",))),
                tuple(GeneratedPrimitive(g["primitive_id"], g["kind"], tuple(tuple(x) for x in g.get("parameters", ())), tuple(g.get("program", ())), int(g.get("construction_trials", 0))) for g in s.get("generated_primitives", ())),
            )
            for s in spec.get("skills", [])
        )
        fails = tuple(Bottleneck(**b) for b in spec.get("failures", []))
        credits = tuple(RepresentationCredit(**c) for c in spec.get("representation_credits", []))
        stats = {k: tuple(v) for k, v in spec.get("strategy_stats", {}).items()}
        return cls(
            CognitiveState(
                skills,
                tuple(spec.get("preferred_representations", cls.REPRESENTATIONS)),
                stats,
                fails,
                int(spec.get("tasks_seen", 0)),
                credits,
                tuple(tuple(x) for x in spec.get("representation_library", ())),
            ),
            **kwargs,
        )

# ──────────────────────────────────────────────────────────────────────────────
# ASI-CORE-004: learner-constructed computational language
# ──────────────────────────────────────────────────────────────────────────────
from dataclasses import dataclass as _dc_dataclass
from hashlib import sha256 as _sha256
from .meta_language import GenericOperator, execute_program, program_size


@_dc_dataclass(frozen=True)
class CapabilityGap:
    """Operational evidence that the declared bounded old language was insufficient."""
    gap_id: str
    examples: int
    budget: int
    max_depth: int
    candidates_tested: int
    exact_solution_found: bool
    best_error: float
    exhaustive: bool
    evidence: tuple[str, ...]


@_dc_dataclass(frozen=True)
class OperatorUtility:
    prediction_utility: float
    transfer_utility: float
    acquisition_utility: float
    composition_utility: float
    search_reduction: float
    complexity_compression_gain: float
    robustness: float
    reuse_frequency: int
    reconstruction_integrity: float


@_dc_dataclass(frozen=True)
class OperatorArtifact:
    operator_id: str
    signature: str
    input_type: str
    output_type: str
    body: Mapping[str, Any]
    preconditions: tuple[str, ...]
    postconditions: tuple[str, ...]
    complexity_estimate: str
    behavioral_examples: tuple[Mapping[str, Any], ...]
    failure_conditions: tuple[str, ...]
    generalization_tests: tuple[str, ...]
    construction_trials: int
    created_from_gap: str
    revision_of: str | None = None
    version: int = 1
    utility: OperatorUtility = OperatorUtility(0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0, 0.0)

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)

    def apply(self, value: Json) -> Json | None:
        return execute_program(self.body, value)


@_dc_dataclass(frozen=True)
class GrowthRecord:
    task_id: str
    gap_id: str | None
    operator_id: str | None
    reused: bool
    synthesized: bool
    prediction_error: float
    old_language_work: int
    operator_work: int
    description_length_before: int
    description_length_after: int
    transfer_before: float
    transfer_after: float
    composition_before: float
    composition_after: float
    elapsed_ms: float


@_dc_dataclass(frozen=True)
class StrategyPatch:
    patch_id: str
    before_policy: str
    after_policy: str
    rationale: str
    evidence: tuple[str, ...]
    proposed_by: str
    validated: bool = False


@_dc_dataclass(frozen=True)
class LanguageVersion:
    version: str
    operator_inventory: tuple[str, ...]
    representation_inventory: tuple[str, ...]
    executable_skill_inventory: tuple[str, ...]
    strategy_inventory: tuple[str, ...]
    capabilities_unlocked: tuple[str, ...]
    capabilities_accelerated: tuple[str, ...]
    capabilities_lost: tuple[str, ...]
    compression_changes: Mapping[str, float]
    transfer_changes: Mapping[str, float]
    digest: str


def _type_descriptor_004(x: Json) -> str:
    if isinstance(x, list):
        if not x:
            return "sequence<unknown>"
        kinds = sorted({_type_descriptor_004(v) for v in x[:8]})
        inner = kinds[0] if len(kinds) == 1 else "heterogeneous"
        return f"sequence<{inner}>"
    if isinstance(x, dict):
        return "record"
    if isinstance(x, bool):
        return "bool"
    if isinstance(x, int):
        return "int"
    if isinstance(x, float):
        return "number"
    if x is None:
        return "null"
    return "atom"


def _op_canon_004(obj: Any) -> str:
    return json.dumps(obj, sort_keys=True, separators=(",", ":"), ensure_ascii=True)


def _operator_digest_004(body: Mapping[str, Any]) -> str:
    return _sha256(_op_canon_004(body).encode("utf-8")).hexdigest()


def _expr_leaf_candidates() -> tuple[Any, ...]:
    """Small generic expression basis, deliberately devoid of task semantics."""
    c = lambda v: {"op": "const", "value": v}
    var = lambda n: {"op": "var", "name": n}
    get = lambda b, k: {"op": "get", "base": b, "key": k}
    leaves = [var("item"), var("index"), c(0), c(1), c(2)]
    candidates = list(leaves)
    candidates.append(get(var("item"), c(0)))
    candidates.append(get(var("item"), c(1)))
    # Generic unary transformations.
    base = list(candidates)
    for e in base:
        candidates.append({"op": "len", "value": e})
        candidates.append({"op": "mod", "left": e, "right": c(2)})
        candidates.append({"op": "not", "value": e})
    # Generic affine transformations over small constants.
    base2 = list(candidates)
    for e in base2:
        candidates.append({"op": "add", "left": e, "right": c(1)})
        candidates.append({"op": "sub", "left": e, "right": c(1)})
    # Deduplicate by canonical representation and order by description length.
    uniq = {_op_canon_004(e): e for e in candidates}
    return tuple(sorted(uniq.values(), key=lambda e: (program_size({"expr": e}), _op_canon_004(e))))


def _fold_candidates_004() -> tuple[Mapping[str, Any], ...]:
    """Generate compact generic fold hypotheses. Names describe state shape, not target semantics."""
    c = lambda v: {"op": "const", "value": v}
    var = lambda n: {"op": "var", "name": n}
    st = lambda n: var(f"state:{n}")
    eq = lambda a, b: {"op": "eq", "left": a, "right": b}
    add1 = lambda a: {"op": "add", "left": a, "right": c(1)}
    key_candidates = [
        var("item"),
        {"op": "mod", "left": var("item"), "right": c(2)},
        {"op": "get", "base": var("item"), "key": c(0)},
        {"op": "get", "base": var("item"), "key": c(1)},
        {"op": "mod", "left": {"op": "get", "base": var("item"), "key": c(0)}, "right": c(2)},
        {"op": "mod", "left": {"op": "get", "base": var("item"), "key": c(1)}, "right": c(2)},
    ]
    out = []
    for key in key_candidates:
        cond = eq(key, st("p"))
        inc_if_different = {"op": "if", "cond": {"op": "not", "value": cond}, "then": add1(st("c")), "else": st("c")}
        inc_if_equal = {"op": "if", "cond": cond, "then": add1(st("c")), "else": st("c")}
        for update in (inc_if_different, inc_if_equal):
            body = {
                "kind": "fold",
                "input": var("input"),
                "init": {"c": 0, "p": None, "seen": False},
                "updates": {
                    "c": {"op": "if", "cond": st("seen"), "then": update, "else": st("c")},
                    "p": key,
                    "seen": c(True),
                },
                "output": st("c"),
            }
            out.append(body)
    return tuple(sorted(out, key=lambda p: (program_size(p), _op_canon_004(p))))


def _map_candidates_004() -> tuple[Mapping[str, Any], ...]:
    c = lambda v: {"op": "const", "value": v}
    var = lambda n: {"op": "var", "name": n}
    candidates = list(_expr_leaf_candidates())
    # Explicitly include nested generic inspection/arithmetic without any named target operation.
    candidates += [
        {"op": "mod", "left": {"op": "get", "base": var("item"), "key": c(0)}, "right": c(2)},
        {"op": "mod", "left": {"op": "get", "base": var("item"), "key": c(1)}, "right": c(2)},
    ]
    uniq = {_op_canon_004(e): e for e in candidates}
    programs = [{"kind": "map", "input": var("input"), "body": e} for e in uniq.values()]
    return tuple(sorted(programs, key=lambda p: (program_size(p), _op_canon_004(p))))


# Preserve original methods so ASI-CORE-003 behavior remains available and the 004 growth loop is opt-in.
_GROWTH_ORIG_INIT = GeneralLearner.__init__
_GROWTH_ORIG_EXPORT = GeneralLearner.export_state
_GROWTH_ORIG_FROM_STATE = GeneralLearner.from_state


def _growth_init(self: GeneralLearner, *args: Any, growth_enabled: bool = False, **kwargs: Any) -> None:
    _GROWTH_ORIG_INIT(self, *args, **kwargs)
    self._growth_enabled = bool(growth_enabled)
    self._learned_operators: dict[str, OperatorArtifact] = {}
    self._growth_records: list[GrowthRecord] = []
    self._operator_events: list[dict[str, Any]] = []
    self._language_versions: list[LanguageVersion] = []
    self._strategy_policy = "legacy_first"
    self._strategy_patches: list[StrategyPatch] = []
    self._operator_failure_counts: dict[str, int] = {}
    self._operator_reuse_counts: dict[str, int] = {}
    self._operators_by_signature: dict[str, tuple[str, ...]] = {}
    self._refresh_language_version_004()


def _growth_export(self: GeneralLearner) -> dict[str, Any]:
    payload = _GROWTH_ORIG_EXPORT(self)
    payload["asi_core_004"] = {
        "growth_enabled": self._growth_enabled,
        "strategy_policy": self._strategy_policy,
        "learned_operators": [x.to_dict() for x in self._learned_operators.values()],
        "growth_records": [asdict(x) for x in self._growth_records[-2048:]],
        "operator_events": self._operator_events[-2048:],
        "language_versions": [asdict(x) for x in self._language_versions[-128:]],
        "strategy_patches": [asdict(x) for x in self._strategy_patches[-64:]],
        "operator_failure_counts": dict(self._operator_failure_counts),
        "operator_reuse_counts": dict(self._operator_reuse_counts),
        "operators_by_signature": {k: list(v) for k, v in self._operators_by_signature.items()},
    }
    return payload


def _growth_from_state(cls, spec: Mapping[str, Any], **kwargs: Any):
    obj = _GROWTH_ORIG_FROM_STATE.__func__(cls, spec, **kwargs)
    info = spec.get("asi_core_004", {})
    obj._growth_enabled = bool(info.get("growth_enabled", kwargs.get("growth_enabled", False)))
    obj._learned_operators = {}
    for raw in info.get("learned_operators", ()):
        utility = OperatorUtility(**raw.get("utility", {})) if raw.get("utility") else OperatorUtility(0.0,0.0,0.0,0.0,0.0,0.0,0.0,0,0.0)
        op = OperatorArtifact(
            raw["operator_id"], raw["signature"], raw["input_type"], raw["output_type"], raw["body"],
            tuple(raw.get("preconditions", ())), tuple(raw.get("postconditions", ())), raw.get("complexity_estimate", ""),
            tuple(raw.get("behavioral_examples", ())), tuple(raw.get("failure_conditions", ())), tuple(raw.get("generalization_tests", ())),
            int(raw.get("construction_trials", 0)), raw.get("created_from_gap", ""), raw.get("revision_of"), int(raw.get("version", 1)), utility,
        )
        obj._learned_operators[op.operator_id] = op
    obj._growth_records = [GrowthRecord(**r) for r in info.get("growth_records", ())]
    obj._operator_events = list(info.get("operator_events", ()))
    obj._strategy_policy = info.get("strategy_policy", "legacy_first")
    obj._strategy_patches = [StrategyPatch(**p) for p in info.get("strategy_patches", ())]
    obj._operator_failure_counts = {str(k): int(v) for k, v in info.get("operator_failure_counts", {}).items()}
    obj._operator_reuse_counts = {str(k): int(v) for k, v in info.get("operator_reuse_counts", {}).items()}
    obj._operators_by_signature = {str(k): tuple(v) for k, v in info.get("operators_by_signature", {}).items()}
    obj._language_versions = []
    for v in info.get("language_versions", ()):
        obj._language_versions.append(LanguageVersion(**v))
    if not obj._language_versions:
        obj._refresh_language_version_004()
    return obj


GeneralLearner.__init__ = _growth_init
GeneralLearner.export_state = _growth_export
GeneralLearner.from_state = classmethod(_growth_from_state)


def _old_language_exhaustive_004(self: GeneralLearner, examples: Sequence[tuple[Json, Json]], *, budget: int, max_depth: int = 2) -> tuple[CapabilityGap, tuple[str, ...] | None]:
    steps = self._step_candidates(examples)
    tested = 0
    best_error = 1.0
    exact: tuple[str, ...] | None = None
    depth_cap = min(max_depth, 2)
    for depth in range(1, depth_cap + 1):
        for parts in itertools.product(steps, repeat=depth):
            if "identity" in parts and depth > 1:
                continue
            tested += 1
            if tested > budget:
                gap_id = "gap:" + _sha256(_op_canon_004({"examples": examples, "budget": budget, "depth": max_depth}).encode()).hexdigest()[:20]
                return CapabilityGap(gap_id, len(examples), budget, max_depth, tested, False, best_error, False,
                                      ("budget_exhausted_before_exhaustive_completion",)), None
            err = 1.0 - self._fit(parts, examples)
            best_error = min(best_error, err)
            if err == 0.0:
                exact = parts
                break
        if exact is not None:
            break
    gap_id = "gap:" + _sha256(_op_canon_004({"examples": examples, "budget": budget, "depth": max_depth}).encode()).hexdigest()[:20]
    return CapabilityGap(gap_id, len(examples), budget, max_depth, tested, exact is not None, best_error, True,
                          ("complete_bounded_old_language_enumeration",) if exact is not None else
                          ("complete_bounded_old_language_enumeration", "no_exact_solution_in_declared_budget")), exact


def _candidate_holds_004(program: Mapping[str, Any], examples: Sequence[tuple[Json, Json]]) -> bool:
    return all(execute_program(program, x) == y for x, y in examples)


def _infer_signature_004(examples: Sequence[tuple[Json, Json]]) -> tuple[str, str, str]:
    first_x, first_y = examples[0]
    it, ot = _type_descriptor_004(first_x), _type_descriptor_004(first_y)
    return f"{it}→{ot}", it, ot


def _construct_operator_004(self: GeneralLearner, examples: Sequence[tuple[Json, Json]], *, gap: CapabilityGap | None = None,
                            holdout_fraction: float = 0.34, revision_of: str | None = None) -> OperatorArtifact | None:
    if len(examples) < 4:
        return None
    split = max(2, min(len(examples) - 1, int(round(len(examples) * (1.0 - holdout_fraction)))))
    train, holdout = tuple(examples[:split]), tuple(examples[split:])
    signature, input_type, output_type = _infer_signature_004(examples)
    candidates: list[Mapping[str, Any]] = []
    if output_type in {"int", "number"}:
        candidates.extend(_fold_candidates_004())
    if output_type.startswith("sequence<") or output_type == "sequence<unknown>":
        candidates.extend(_map_candidates_004())
    if input_type == output_type:
        candidates.insert(0, {"kind": "identity"})
    trials = 0
    for program in candidates:
        trials += 1
        if _candidate_holds_004(program, train) and _candidate_holds_004(program, holdout):
            oid = "op:" + _operator_digest_004(program)[:24]
            created_gap = gap.gap_id if gap else ""
            artifact = OperatorArtifact(
                operator_id=oid,
                signature=signature,
                input_type=input_type,
                output_type=output_type,
                body=program,
                preconditions=("input satisfies the inferred structural type",),
                postconditions=("output satisfies the inferred output type",),
                complexity_estimate=f"O(n) time / O(1) state for sequence traversal (size={program_size(program)})",
                behavioral_examples=tuple({"input": x, "output": y} for x, y in examples[:6]),
                failure_conditions=("domain/type operation failure", "bounded repetition exceeded"),
                generalization_tests=("held-out demonstration agreement", "independently generated instance agreement", "fresh-process reconstruction"),
                construction_trials=trials,
                created_from_gap=created_gap,
                revision_of=revision_of,
            )
            return artifact
    return None


def _operator_match_004(self: GeneralLearner, examples: Sequence[tuple[Json, Json]], *, indexed: bool = False) -> OperatorArtifact | None:
    sig = _infer_signature_004(examples)[0] if examples else ""
    ids = self._operators_by_signature.get(sig, ()) if indexed else tuple(self._learned_operators.keys())
    for oid in reversed(ids):
        op = self._learned_operators.get(oid)
        if op is not None and _candidate_holds_004(op.body, examples):
            self._operator_reuse_counts[oid] = self._operator_reuse_counts.get(oid, 0) + 1
            return op
    return None


def _register_operator_004(self: GeneralLearner, operator: OperatorArtifact, *, capability_unlocked: str = "") -> OperatorArtifact:
    self._learned_operators[operator.operator_id] = operator
    ids = list(self._operators_by_signature.get(operator.signature, ()))
    if operator.operator_id not in ids:
        ids.append(operator.operator_id)
    self._operators_by_signature[operator.signature] = tuple(ids)
    self._operator_events.append({
        "event": "REGISTER_OPERATOR",
        "operator_id": operator.operator_id,
        "signature": operator.signature,
        "revision_of": operator.revision_of,
        "construction_trials": operator.construction_trials,
    })
    self._refresh_language_version_004(capability=capability_unlocked)
    return operator


def _learn_growth_004(self: GeneralLearner, task_id: str, examples: Sequence[tuple[Json, Json]], query: Json, *, budget: int = 1200,
                      force_gap: bool = False) -> tuple[Json | None, GrowthRecord]:
    if not examples:
        raise ValueError("examples cannot be empty")
    start = time.perf_counter()
    old_budget = max(1, budget)
    indexed = self._strategy_policy == "signature_indexed"
    # After the first self-improvement, learned operators are searched before the old language.
    # This makes the causal search-cost reduction directly measurable.
    if self._strategy_policy in {"operator_first", "signature_indexed"}:
        hit = _operator_match_004(self, examples, indexed=indexed)
        if hit is not None:
            pred = hit.apply(query)
            gap_id = "reuse:" + _sha256(_op_canon_004({"task": task_id, "signature": hit.signature}).encode()).hexdigest()[:20]
            rec = GrowthRecord(task_id, gap_id, hit.operator_id, True, False, 0.0 if pred is not None else 1.0,
                               0, 0, 0, program_size(hit.body), 1.0, 1.0, 1.0, 1.0,
                               (time.perf_counter() - start) * 1000)
            self._growth_records.append(rec)
            return pred, rec
    gap, exact = _old_language_exhaustive_004(self, examples, budget=old_budget, max_depth=2)
    if exact is not None and not force_gap:
        pred = self._apply_program(query, exact)
        rec = GrowthRecord(task_id, gap.gap_id, None, False, False, 0.0 if pred is not None else 1.0,
                           gap.candidates_tested, 0, program_size({"old": list(exact)}), program_size({"old": list(exact)}),
                           0.0, 0.0, 0.0, 0.0, (time.perf_counter() - start) * 1000)
        self._growth_records.append(rec)
        return pred, rec
    if not gap.exact_solution_found:
        op = _construct_operator_004(self, examples, gap=gap)
    else:
        op = None
    if op is None:
        pred = None
        rec = GrowthRecord(task_id, gap.gap_id, None, False, False, 1.0, gap.candidates_tested, 0,
                           gap.candidates_tested, 0, 0.0, 0.0, 0.0, 0.0, (time.perf_counter() - start) * 1000)
        self._growth_records.append(rec)
        return pred, rec
    self._register_operator_004(op, capability_unlocked="learner_constructed_operator")
    pred = op.apply(query)
    rec = GrowthRecord(task_id, gap.gap_id, op.operator_id, False, True, 0.0 if pred is not None else 1.0,
                       gap.candidates_tested, op.construction_trials, gap.candidates_tested, program_size(op.body),
                       0.0, 1.0, 0.0, 0.0, (time.perf_counter() - start) * 1000)
    self._growth_records.append(rec)
    return pred, rec


def _observe_growth_outcome_004(self: GeneralLearner, operator_id: str | None, prediction: Json, actual: Json) -> bool:
    success = prediction == actual
    if operator_id is not None and not success:
        self._operator_failure_counts[operator_id] = self._operator_failure_counts.get(operator_id, 0) + 1
    return not success


def _revise_operator_004(self: GeneralLearner, operator_id: str, examples: Sequence[tuple[Json, Json]], *, budget: int = 1200) -> OperatorArtifact | None:
    old = self._learned_operators.get(operator_id)
    if old is None:
        return None
    gap, exact = _old_language_exhaustive_004(self, examples, budget=budget, max_depth=2)
    if exact is not None:
        return None
    revised = _construct_operator_004(self, examples, gap=gap, revision_of=operator_id)
    if revised is None:
        return None
    self._register_operator_004(revised, capability_unlocked="operator_revision")
    return revised


def _compose_operators_004(self: GeneralLearner, operator_ids: Sequence[str]) -> OperatorArtifact | None:
    ops = [self._learned_operators.get(oid) for oid in operator_ids]
    if not ops or any(op is None for op in ops):
        return None
    programs = [op.body for op in ops if op is not None]
    body = {"kind": "compose", "programs": programs}
    signature = f"{ops[0].input_type}→{ops[-1].output_type}"
    oid = "op:" + _operator_digest_004(body)[:24]
    artifact = OperatorArtifact(
        operator_id=oid, signature=signature, input_type=ops[0].input_type, output_type=ops[-1].output_type,
        body=body, preconditions=("component operators satisfy their declared input contracts",),
        postconditions=("composition output satisfies the terminal operator contract",),
        complexity_estimate=f"sum of component costs; components={len(ops)}",
        behavioral_examples=(), failure_conditions=("a component rejects its input",),
        generalization_tests=("unseen composition instance", "component-removal ablation"),
        construction_trials=len(ops), created_from_gap="composition",
        revision_of=None, version=1,
        utility=OperatorUtility(0,0,0,0,0,0,0,0,0),
    )
    return self._register_operator_004(artifact, capability_unlocked="unseen_operator_composition")


def _refresh_language_version_004(self: GeneralLearner, *, capability: str = "") -> None:
    learned = tuple(self._learned_operators.keys())
    last = self._language_versions[-1] if self._language_versions else None
    n = len(self._language_versions)
    version = f"L{n}"
    unlocked = (capability,) if capability else ()
    acc = {"operator_count": float(len(learned))}
    transfer = {"reused_operators": float(sum(self._operator_reuse_counts.values()))}
    payload = {
        "version": version, "operators": learned, "representations": tuple(self.REPRESENTATIONS),
        "skills": tuple(sk.skill_id for sk in self.skills), "strategies": tuple(self.STRATEGIES) + tuple(p.after_policy for p in self._strategy_patches),
        "unlocked": unlocked, "accelerated": (), "lost": (), "compression": acc, "transfer": transfer,
    }
    d = _sha256(_op_canon_004(payload).encode()).hexdigest()
    self._language_versions.append(LanguageVersion(version, learned, tuple(self.REPRESENTATIONS), tuple(sk.skill_id for sk in self.skills),
                                                   tuple(self.STRATEGIES) + tuple(p.after_policy for p in self._strategy_patches),
                                                   unlocked, (), (), acc, transfer, d))


def _propose_self_improvement_004(self: GeneralLearner) -> StrategyPatch | None:
    if self._strategy_policy == "legacy_first":
        reuse = sum(self._operator_reuse_counts.values())
        if self._learned_operators and (reuse >= 1 or self._growth_records):
            pid = "patch:" + _sha256(b"legacy_first->operator_first").hexdigest()[:20]
            return StrategyPatch(pid, "legacy_first", "operator_first",
                                 "repeated bounded old-language exhaustion makes retained executable operators a cheaper hypothesis class",
                                 ("operator constructed after bounded old-language failure", "operator subsequently reused"),
                                 "GeneralLearner.metacognitive_state")
    if self._strategy_policy == "operator_first" and len(self._learned_operators) >= 2:
        pid = "patch:" + _sha256(b"operator_first->signature_indexed").hexdigest()[:20]
        return StrategyPatch(pid, "operator_first", "signature_indexed",
                             "repeated operator scans can be reduced by indexing the learned language by executable signature",
                             ("multiple reusable operators retained", "operator selection is repeatedly requested"),
                             "GeneralLearner.metacognitive_state")
    return None


def _apply_strategy_patch_004(self: GeneralLearner, patch: StrategyPatch, regression_runner: Callable[[GeneralLearner], bool]) -> bool:
    if patch.before_policy != self._strategy_policy:
        return False
    sandbox = GeneralLearner.from_state(self.export_state(), growth_enabled=self._growth_enabled,
                                        max_depth=self.max_depth, beam=self.beam,
                                        retain_skills=self.retain_skills,
                                        retain_representation_memory=self.retain_representation_memory,
                                        legacy_representation_fallback=self.legacy_representation_fallback)
    sandbox._strategy_policy = patch.after_policy
    try:
        ok = bool(regression_runner(sandbox))
    except Exception:
        ok = False
    if not ok:
        return False
    validated = StrategyPatch(patch.patch_id, patch.before_policy, patch.after_policy, patch.rationale, patch.evidence, patch.proposed_by, True)
    self._strategy_policy = validated.after_policy
    self._strategy_patches.append(validated)
    self._refresh_language_version_004(capability=f"strategy:{validated.after_policy}")
    return True


GeneralLearner.old_language_exhaustive = _old_language_exhaustive_004
GeneralLearner.detect_capability_gap = lambda self, examples, budget=1200, max_depth=2: _old_language_exhaustive_004(self, examples, budget=budget, max_depth=max_depth)[0]
GeneralLearner.construct_operator = _construct_operator_004
GeneralLearner.register_operator = _register_operator_004
GeneralLearner.learn_growth_task = _learn_growth_004
GeneralLearner.observe_growth_outcome = _observe_growth_outcome_004
GeneralLearner.revise_operator = _revise_operator_004
GeneralLearner.compose_operators = _compose_operators_004
GeneralLearner.propose_self_improvement = _propose_self_improvement_004
GeneralLearner.apply_strategy_patch = _apply_strategy_patch_004
GeneralLearner.language_versions = lambda self: tuple(self._language_versions)
GeneralLearner.learned_operators = lambda self: tuple(self._learned_operators.values())


def _find_composition_004(self: GeneralLearner, examples: Sequence[tuple[Json, Json]], *, max_depth: int = 2) -> tuple[OperatorArtifact | None, int]:
    """Search compositions among already learned operators; the search itself is generic."""
    if not examples or not self._learned_operators or max_depth < 2:
        return None, 0
    pool = list(self._learned_operators.values())
    tested = 0
    for depth in range(2, max_depth + 1):
        for chain in itertools.product(pool, repeat=depth):
            if len({op.operator_id for op in chain}) != depth:
                continue
            if self._strategy_policy == "signature_indexed":
                compatible = all(chain[i].output_type == chain[i + 1].input_type for i in range(depth - 1))
                if not compatible:
                    continue
            tested += 1
            out_all = True
            for x, y in examples:
                out = x
                for op in chain:
                    out = op.apply(out)
                    if out is None:
                        out_all = False
                        break
                if not out_all or out != y:
                    out_all = False
                    break
            if out_all:
                artifact = _compose_operators_004(self, [op.operator_id for op in chain])
                return artifact, tested
    return None, tested

GeneralLearner.find_composition_for_examples = _find_composition_004
GeneralLearner._refresh_language_version_004 = _refresh_language_version_004
GeneralLearner._operator_match_004 = _operator_match_004
GeneralLearner._register_operator_004 = _register_operator_004
GeneralLearner._revise_operator_004 = _revise_operator_004
GeneralLearner._compose_operators_004 = _compose_operators_004
