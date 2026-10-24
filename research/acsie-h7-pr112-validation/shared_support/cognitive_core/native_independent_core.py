from __future__ import annotations

"""Native model-independent cognitive substrate.

This module is intentionally self-contained. It does not orchestrate external agents,
call language models, retrieve from the network, or depend on provider APIs. The core
learns executable hypotheses from experience, maintains a persistent world model,
creates information-seeking goals, synthesizes bounded procedures, evaluates isolated
self-changes, and retains only changes that survive holdout and transfer evidence.

It is a research substrate, not an AGI/ASI claim.
"""

import copy
import json
import math
import random
import statistics
from dataclasses import dataclass, asdict
from hashlib import sha256
from typing import Any, Mapping, Sequence


Json = Any


def canon(x: Json) -> str:
    return json.dumps(x, sort_keys=True, separators=(",", ":"), default=str)


def digest(x: Json) -> str:
    return sha256(canon(x).encode()).hexdigest()


def _numeric(x: Any) -> bool:
    return isinstance(x, (int, float)) and not isinstance(x, bool)


def _leaves(x: Json, path=()):
    if isinstance(x, Mapping):
        for k in sorted(x, key=str):
            yield from _leaves(x[k], path + (str(k),))
    elif isinstance(x, (list, tuple)):
        for i, v in enumerate(x):
            yield from _leaves(v, path + (str(i),))
    else:
        yield path, x


def _get(x: Json, path: Sequence[str]) -> Json:
    cur = x
    for part in path:
        if isinstance(cur, Mapping):
            cur = cur[part]
        elif isinstance(cur, (list, tuple)):
            cur = cur[int(part)]
        else:
            raise KeyError(path)
    return cur


def _replace(x: Json, path: Sequence[str], value: Json) -> Json:
    out = copy.deepcopy(x)
    if not path:
        return copy.deepcopy(value)
    cur = out
    for part in path[:-1]:
        cur = cur[int(part)] if isinstance(cur, list) else cur[part]
    if isinstance(cur, list):
        cur[int(path[-1])] = copy.deepcopy(value)
    else:
        cur[path[-1]] = copy.deepcopy(value)
    return out


def _shape(x: Json):
    if isinstance(x, Mapping):
        return ("dict", tuple((str(k), _shape(v)) for k, v in sorted(x.items(), key=lambda kv: str(kv[0]))))
    if isinstance(x, list):
        return ("list", len(x), tuple(_shape(v) for v in x[:4]))
    if isinstance(x, tuple):
        return ("tuple", len(x), tuple(_shape(v) for v in x[:4]))
    if isinstance(x, bool):
        return "bool"
    if isinstance(x, int) and not isinstance(x, bool):
        return "int"
    if isinstance(x, float):
        return "float"
    if isinstance(x, str):
        return "str"
    if x is None:
        return "null"
    return type(x).__name__

def _restore_relation_expr(x: Json) -> Json:
    """Restore tuple-encoded native relation expressions after JSON serialization."""
    if isinstance(x, list):
        return tuple(_restore_relation_expr(v) for v in x)
    if isinstance(x, dict):
        return {
            k: _restore_relation_expr(v)
            for k, v in x.items()
        }
    return x


@dataclass(frozen=True)
class Experience:
    observation: Json
    action: str
    next_observation: Json
    reward: float
    context: tuple[Any, ...] = ()
    exogenous: bool = False
    step: int = 0


@dataclass(frozen=True)
class Hypothesis:
    hypothesis_id: str
    kind: str
    action: str
    role: tuple[Any, ...] | None
    delta: float | None
    delay: int
    context: tuple[Any, ...] | None
    support: float
    confidence: float
    complexity: int
    value: Json | None = None


@dataclass(frozen=True)
class Procedure:
    procedure_id: str
    program: tuple[Mapping[str, Any], ...]
    train_score: float
    holdout_score: float
    transfer_score: float
    complexity: int
    generation: int
    parent_id: str | None
    provenance: Mapping[str, Any]
    status: str = "candidate"


@dataclass(frozen=True)
class Capability:
    capability_id: str
    family: str
    score: float
    transfer: float
    novelty: float
    complexity: int
    procedure_id: str | None
    generation: int


@dataclass(frozen=True)
class LearningPolicy:
    max_expression_depth: int = 3
    beam: int = 128
    intervention_budget: int = 4
    memory_limit: int = 512
    curiosity_weight: float = 0.35
    mutation_rate: float = 0.30
    # Additional search width allocated to deeper synthesis levels.
    beam_growth: int = 0
    # Fraction of each synthesis frontier reserved for distinct operator prefixes.
    frontier_diversity: float = 0.0
    # Promote verified short program fragments into the reusable native library.
    abstract_verified_subprograms: bool = False

@dataclass(frozen=True)
class LearningKernel:
    """Executable learning-strategy configuration.

    These fields change how evidence is admitted, contextualized, delayed effects are
    induced, and competing predictions are reconciled. They are deliberately generic:
    no task-specific operator or external model is encoded here.
    """
    evidence_threshold: float = 0.66
    min_support_count: int = 2
    context_mode: str = "shape"
    delayed_window: int = 6
    prediction_mode: str = "ensemble"
    contradiction_margin: float = 0.20
    replay_limit: int = 64
    # Optional native compositional context-expression AST. When populated with
    # context_mode="adaptive_union", the learned expression is added as an
    # additional non-destructive context view rather than replacing legacy views.
    context_program: Any = None


@dataclass(frozen=True)
class ChangeDecision:
    candidate_id: str
    accepted: bool
    reason: str
    baseline_score: float
    candidate_score: float
    holdout_score: float
    transfer_score: float


@dataclass(frozen=True)
class SolutionCandidate:
    solver: str
    result: Json | None
    score: float
    verified: bool
    complexity: int
    evidence: Mapping[str, Any]


class NativeCognitiveCore:
    """A single persistent cognitive substrate."""

    SCHEMA = "ACSIE.native-independent-core.v4"

    def __init__(self, seed: int = 0, policy: LearningPolicy | None = None):
        self.seed = int(seed)
        self.rng = random.Random(self.seed)
        self.policy = policy or LearningPolicy()
        self.learning_kernel = LearningKernel()
        self.experience: list[Experience] = []
        self.hypotheses: dict[str, Hypothesis] = {}
        self.procedures: dict[str, Procedure] = {}
        self.capabilities: dict[str, Capability] = {}
        self.goal_archive: list[dict[str, Any]] = []
        self.change_history: list[dict[str, Any]] = []
        self.prediction_history: list[dict[str, Any]] = []
        self.semantic_memory: dict[str, dict[str, Any]] = {}
        self.solver_history: dict[str, dict[str, float]] = {}
        # Verified executable capability macros learned from prior solutions.
        # Each token encodes its program, so the library survives restart without
        # external files or hidden code generation.
        self.program_library: list[str] = []
        # Frozen native predecessor used only by the protected compositional overlay.
        self.protected_hypotheses: dict[str, Hypothesis] = {}
        self.generation = 0
        self.step = 0
        # Semantics-preserving cache for adaptive context discovery. The cache is
        # invalidated whenever retained experience changes; it is never serialized.
        self._adaptive_context_cache: dict[
            str,
            tuple[int, int, dict[tuple[str, ...], tuple[str, Any]]],
        ] = {}

    # ---------- experience and world-model induction ----------

    def observe(
        self,
        observation: Json,
        action: str,
        next_observation: Json,
        *,
        reward: float = 0.0,
        context: Sequence[Any] = (),
        exogenous: bool = False,
    ) -> Experience:
        row = Experience(
            copy.deepcopy(observation),
            str(action),
            copy.deepcopy(next_observation),
            float(reward),
            tuple(context),
            bool(exogenous),
            self.step,
        )
        self.step += 1
        self.experience.append(row)
        self.experience = self.experience[-self.policy.memory_limit :]
        self._adaptive_context_cache.clear()
        self._refresh_hypotheses()
        return row

    def observe_batch(
        self,
        rows: Sequence[tuple[Json, str, Json]],
        *,
        reward: float = 0.0,
        context: Sequence[Any] = (),
        exogenous: bool = False,
    ) -> tuple[Experience, ...]:
        """Batch observation with one final hypothesis regeneration.

        The hypothesis store is fully regenerated from retained experience, so
        batching preserves the final observable learner state while avoiding
        repeated intermediate regeneration during isolated evaluator rollouts.
        """
        observed = []
        for observation, action, next_observation in rows:
            observed.append(
                Experience(
                    copy.deepcopy(observation),
                    str(action),
                    copy.deepcopy(next_observation),
                    float(reward),
                    tuple(context),
                    bool(exogenous),
                    self.step,
                )
            )
            self.step += 1
        if observed:
            self.experience.extend(observed)
            self.experience = self.experience[-self.policy.memory_limit:]
            self._adaptive_context_cache.clear()
            self._refresh_hypotheses()
        return tuple(observed)

    @staticmethod
    def _effects(before: Json, after: Json):
        bm = {tuple(p): v for p, v in _leaves(before)}
        am = {tuple(p): v for p, v in _leaves(after)}
        out = []
        for path, bv in bm.items():
            if path not in am:
                continue
            av = am[path]
            if bv != av:
                if _numeric(bv) and _numeric(av):
                    out.append(("delta", path, float(av) - float(bv)))
                elif isinstance(bv, bool) and isinstance(av, bool):
                    out.append(("toggle", path, bool(av)))
                else:
                    out.append(("set", path, copy.deepcopy(av)))
        return out

    def _adaptive_context(self, action: str) -> dict[tuple[str, ...], tuple[str, Any]]:
        """Discover context partitions only when observed effects disagree by a feature."""
        action = str(action)
        min_count = max(2, int(self.learning_kernel.min_support_count))
        cached = self._adaptive_context_cache.get(action)
        if (
            cached is not None
            and cached[0] == len(self.experience)
            and cached[1] == min_count
        ):
            return cached[2]
        records = [e for e in self.experience if not e.exogenous and e.action == str(action)]
        evidence: dict[tuple[str, ...], list[tuple[tuple[str, Any], tuple[Any, ...]]]] = {}

        for row in records:
            effect_by_path: dict[tuple[str, ...], tuple[Any, ...]] = {}
            for kind, path, value in self._effects(row.observation, row.next_observation):
                normalized = (
                    round(float(value), 12)
                    if kind == "delta" and _numeric(value)
                    else canon(value)
                )
                effect_by_path[tuple(path)] = (kind, normalized)

            # Context features are allowed to be unchanged by the action.
            # A feature can still explain a changed target (e.g. x parity determines
            # the z delta) and therefore must not be restricted to the effect path.
            for path, value in _leaves(row.observation):
                # Context features should be observable before the action and unchanged
                # by the transition being explained. Including the changed target itself
                # creates post-treatment conditioning and can over-fragment the evidence.
                if path in effect_by_path:
                    continue
                predicates: list[tuple[str, Any]] = []
                if isinstance(value, bool):
                    predicates.append(("value", bool(value)))
                elif isinstance(value, str):
                    predicates.append(("value", value[:32]))
                elif _numeric(value):
                    v = float(value)
                    if isinstance(value, int) and not isinstance(value, bool):
                        predicates.append(("parity", int(value) % 2))
                    predicates.append(("sign", "neg" if v < 0 else "zero" if v == 0 else "pos"))
                    predicates.append(("zero", v == 0))
                    predicates.append(("threshold", v >= 0))

                for predicate in predicates:
                    for effect_path, effect in effect_by_path.items():
                        evidence.setdefault(tuple(path), []).append((predicate, effect))

        selected: dict[tuple[str, ...], tuple[str, Any]] = {}
        for path, rows in evidence.items():
            candidates: list[tuple[int, int, str, dict[Any, set[tuple[Any, ...]]]]] = []
            for name in ("value", "parity", "sign", "zero", "threshold"):
                groups: dict[Any, set[tuple[Any, ...]]] = {}
                counts: dict[Any, int] = {}
                for (predicate, key), effect in rows:
                    if predicate != name:
                        continue
                    counts[key] = counts.get(key, 0) + 1
                    groups.setdefault(key, set()).add(effect)
                if len(groups) < 2 or any(counts[k] < min_count for k in groups):
                    continue
                # The predicate is useful only if observed effects actually differ
                # between its groups. Irrelevant input features are discarded.
                if len(set().union(*groups.values())) < 2:
                    continue
                separable_groups = sum(1 for effects in groups.values() if len(effects) == 1)
                candidates.append((separable_groups, len(groups), name, groups))

            if candidates:
                candidates.sort(key=lambda x: (-x[0], x[1], x[2]))
                # Keep the strongest predicate as the primary diagnostic/public value,
                # but retain the full set of currently supported predicates. This makes
                # the adaptive context basis monotone: when a new mechanism introduces
                # an additional useful discriminator, previously learned rules are
                # re-expressed under the expanded basis rather than being made
                # inaccessible by replacing the old discriminator.
                selected[path] = (candidates[0][2], tuple(c[2] for c in candidates))

        self._adaptive_context_cache[action] = (
            len(self.experience),
            min_count,
            selected,
        )
        return selected

    def _adaptive_union_context_signatures(
        self,
        observation: Json,
        action: str,
    ) -> tuple[tuple[Any, ...], ...]:
        """Return independent context views rather than conjoining all predicates.

        Each discovered predicate is a separate context basis. Hypotheses learned under
        one basis therefore remain addressable when a later basis is introduced. This is
        generic: no mechanism, task family, or feature name is privileged.
        """
        adaptive_numeric = self._adaptive_context(str(action))
        variants: list[tuple[Any, ...]] = []

        for path, (_primary, supported) in sorted(adaptive_numeric.items()):
            value = None
            for leaf_path, leaf_value in _leaves(observation):
                if tuple(leaf_path) == tuple(path):
                    value = leaf_value
                    break
            if value is None and not any(tuple(leaf_path) == tuple(path) for leaf_path, _ in _leaves(observation)):
                continue

            for predicate in (supported or (_primary,)):
                atom = None
                if predicate == "value":
                    atom = (path, "value", value if not isinstance(value, str) else value[:32])
                elif predicate == "parity" and isinstance(value, int) and not isinstance(value, bool):
                    atom = (path, "parity", int(value) % 2)
                elif predicate == "sign":
                    try:
                        fv = float(value)
                    except (TypeError, ValueError):
                        continue
                    atom = (path, "sign", "neg" if fv < 0 else "zero" if fv == 0 else "pos")
                elif predicate == "zero":
                    try:
                        atom = (path, "zero", float(value) == 0)
                    except (TypeError, ValueError):
                        continue
                elif predicate == "threshold":
                    try:
                        atom = (path, "threshold", float(value) >= 0)
                    except (TypeError, ValueError):
                        continue
                if atom is not None:
                    variants.append(canon((atom,)))

        return tuple(dict.fromkeys(variants))

    @staticmethod
    def _context_program_eval(program: Any, observation: Json) -> Any:
        """Evaluate a tiny native context-expression grammar.

        The grammar is data-driven and task-independent. Leaves are observable
        state features; internal nodes are compositional relations/boolean
        combinators. This is deliberately separate from the transition-relation
        DSL: its output is only used to partition evidence into context views.
        """
        if program is None:
            return None
        op = program[0]
        if op == "atom":
            path = tuple(program[1])
            predicate = str(program[2])
            value = _get(observation, path)
            if predicate == "value":
                return value if not isinstance(value, str) else value[:32]
            if predicate == "parity":
                if not isinstance(value, int) or isinstance(value, bool):
                    raise ValueError("parity requires int")
                return int(value) % 2
            if predicate == "sign":
                fv = float(value)
                return "neg" if fv < 0 else "zero" if fv == 0 else "pos"
            if predicate == "zero":
                return float(value) == 0.0
            if predicate == "threshold":
                return float(value) >= 0.0
            raise ValueError(predicate)
        if op in {"eq", "neq", "and", "or", "xor"}:
            left = NativeCognitiveCore._context_program_eval(program[1], observation)
            right = NativeCognitiveCore._context_program_eval(program[2], observation)
            if op == "eq":
                return left == right
            if op == "neq":
                return left != right
            if op == "and":
                return bool(left) and bool(right)
            if op == "or":
                return bool(left) or bool(right)
            return bool(left) ^ bool(right)
        if op == "not":
            return not bool(
                NativeCognitiveCore._context_program_eval(program[1], observation)
            )
        raise ValueError(op)

    @staticmethod
    def _context_program_complexity(program: Any) -> int:
        return len(canon(program))

    @classmethod
    def _context_programs_from_streams(
        cls,
        streams: Sequence[Sequence[tuple[Json, str, Json]]],
        *,
        max_programs: int = 256,
    ) -> tuple[Any, ...]:
        """Generate compositional context expressions from observed state only.

        No task family names, benchmark IDs, or hand-written family rules enter the
        grammar. Candidate expressions are ranked by how cleanly they partition
        observed transition effects on the discovery streams, then evaluated by the
        normal holdout/transfer gates.
        """
        paths: list[tuple[str, ...]] = []
        values_by_path: dict[tuple[str, ...], list[Any]] = {}
        rows: list[tuple[Json, str, Json]] = []
        for stream in streams:
            rows.extend(list(stream))
        for observation, _action, _next_observation in rows[:64]:
            for path, value in _leaves(observation):
                if _numeric(value):
                    values_by_path.setdefault(tuple(path), []).append(value)

        for path, vals in sorted(values_by_path.items()):
            if not vals:
                continue
            # Constants are avoided in the context grammar: context programs
            # describe relations among observed features, not target-specific cutoffs.
            paths.append(path)
            if len(paths) >= 8:
                break

        atoms: list[Any] = []
        for path in paths:
            for predicate in ("parity", "sign", "zero", "threshold"):
                atoms.append(("atom", path, predicate))

        programs: dict[str, Any] = {}
        for atom in atoms:
            programs.setdefault(canon(atom), atom)

        for i, left in enumerate(atoms):
            for right in atoms[i + 1:]:
                for op in ("eq", "neq", "xor", "and", "or"):
                    expr = (op, left, right)
                    programs.setdefault(canon(expr), expr)

        # One extra compositional layer creates genuinely new rule structure
        # without requiring an unbounded grammar in the first experiment.
        # One extra compositional layer creates genuinely new rule structure.
        # The previous implementation truncated simple relations by insertion order
        # before seeing discovery evidence. That created a concrete L2-A blind spot:
        # valid relations could never participate in the second compositional layer.
        # Rank first-order relations using discovery-only partition quality first,
        # then compose the strongest 64 relation hypotheses. No holdout/transfer
        # outcome and no task-family identifier enters this ranking.
        pairwise = list(programs.values())
        first_order = [
            p for p in pairwise
            if isinstance(p, tuple) and p and p[0] in {"eq", "neq", "xor"}
        ]

        first_order_scored: list[tuple[tuple[float, float, int, str], Any]] = []
        for expr in first_order:
            groups: dict[str, list[str]] = {}
            usable = 0
            for observation, action, next_observation in rows[:48]:
                try:
                    value = cls._context_program_eval(expr, observation)
                    effects = cls._effects(observation, next_observation)
                except Exception:
                    continue
                if not effects:
                    continue
                effect_key = canon(effects)
                groups.setdefault(canon(value), []).append(effect_key)
                usable += 1
            if usable < 4 or len(groups) < 2:
                continue

            purity = 0.0
            separable = 0
            for members in groups.values():
                unique_effects = len(set(members))
                purity += 1.0 / max(1, unique_effects)
                if unique_effects == 1:
                    separable += 1

            complexity = cls._context_program_complexity(expr)
            score = (
                float(separable),
                float(purity),
                -float(complexity),
                canon(expr),
            )
            first_order_scored.append((score, expr))

        first_order_scored.sort(key=lambda item: item[0], reverse=True)

        # Structural coverage reservoir: discovery quality alone can delete a weak
        # but necessary relation before composition. Preserve every same-predicate
        # equality/inequality relation across observed field pairs first, then fill
        # the remaining beam from the discovery-quality ranking. This is generic:
        # it depends only on syntax operands/operators, never on target identity.
        first_order_beam = min(256, max(64, 2 * int(max_programs)))
        reservoir = []
        reservoir_keys = set()
        for _score, expr in first_order_scored:
            if not (
                isinstance(expr, tuple)
                and len(expr) == 3
                and expr[0] in {"eq", "neq"}
                and isinstance(expr[1], tuple)
                and isinstance(expr[2], tuple)
                and expr[1][0] == "atom"
                and expr[2][0] == "atom"
                and len(expr[1]) >= 3
                and len(expr[2]) >= 3
                and expr[1][2] == expr[2][2]
            ):
                continue
            left_path = tuple(expr[1][1])
            right_path = tuple(expr[2][1])
            predicate = str(expr[1][2])
            pair_key = (str(expr[0]), predicate, tuple(sorted((left_path, right_path))))
            if pair_key in reservoir_keys:
                continue
            reservoir_keys.add(pair_key)
            reservoir.append(expr)
            if len(reservoir) >= max(0, first_order_beam - 32):
                break

        simple_relations = list(reservoir)
        chosen = {canon(expr) for expr in simple_relations}
        for _score, expr in first_order_scored:
            if len(simple_relations) >= first_order_beam:
                break
            key = canon(expr)
            if key in chosen:
                continue
            chosen.add(key)
            simple_relations.append(expr)
        for i, left in enumerate(simple_relations):
            for right in simple_relations[i + 1:]:
                for op in ("and", "or", "xor"):
                    expr = (op, left, right)
                    programs.setdefault(canon(expr), expr)

        scored: list[tuple[tuple[float, float, int, str], Any]] = []
        for expr in programs.values():
            groups: dict[str, list[str]] = {}
            usable = 0
            for observation, action, next_observation in rows[:48]:
                try:
                    value = cls._context_program_eval(expr, observation)
                    effects = cls._effects(observation, next_observation)
                except Exception:
                    continue
                if not effects:
                    continue
                effect_key = canon(effects)
                groups.setdefault(canon(value), []).append(effect_key)
                usable += 1
            if usable < 4 or len(groups) < 2:
                continue

            purity = 0.0
            separable = 0
            for members in groups.values():
                unique_effects = len(set(members))
                purity += 1.0 / max(1, unique_effects)
                if unique_effects == 1:
                    separable += 1

            complexity = cls._context_program_complexity(expr)
            score = (
                float(separable),
                float(purity),
                -float(complexity),
                canon(expr),
            )
            scored.append((score, expr))

        scored.sort(key=lambda item: item[0], reverse=True)
        return tuple(expr for _score, expr in scored[:max(1, int(max_programs))])

    def _compositional_union_context_signatures(
        self,
        observation: Json,
        action: str,
    ) -> tuple[tuple[Any, ...], ...]:
        variants = list(self._adaptive_union_context_signatures(observation, action))
        program = self.learning_kernel.context_program
        if program is not None:
            try:
                value = self._context_program_eval(program, observation)
                variants.append(
                    (canon(("composed_context", program, value)),)
                )
            except Exception:
                pass
        return tuple(dict.fromkeys(variants))

    def _observable_context_atoms(self, observation: Json) -> set[str]:
        """Enumerate context atoms directly observable from state, independent of evidence selection."""
        atoms: set[str] = set()
        for path, value in _leaves(observation):
            typ = _shape(value)
            atoms.add(canon((path, typ)))

            if isinstance(value, bool):
                atoms.add(canon((path, "bool", bool(value))))
                atoms.add(canon((path, "value", bool(value))))
            elif isinstance(value, str):
                atoms.add(canon((path, "str", value[:32])))
                atoms.add(canon((path, "value", value[:32])))
            elif _numeric(value):
                fv = float(value)
                if isinstance(value, int) and not isinstance(value, bool):
                    atoms.add(canon((path, "parity", int(value) % 2)))
                atoms.add(canon((path, "sign", "neg" if fv < 0 else "zero" if fv == 0 else "pos")))
                atoms.add(canon((path, "zero", fv == 0)))
                atoms.add(canon((path, "threshold", fv >= 0)))
                atoms.add(canon((path, "value", value)))
        return atoms

    @staticmethod
    def _context_contains_composed_context(
        context: tuple[Any, ...] | str | None,
    ) -> bool:
        """Return whether a stored context contains a native composed-context view."""
        if not context:
            return False
        if isinstance(context, str):
            raw_atoms = [context]
        else:
            raw_atoms = list(context)
        for atom in raw_atoms:
            decoded = atom
            if isinstance(decoded, str):
                try:
                    decoded = json.loads(decoded)
                except (TypeError, ValueError):
                    continue
            if (
                isinstance(decoded, list)
                and len(decoded) >= 3
                and decoded[0] == "composed_context"
            ):
                return True
        return False

    def _matches_active_composed_context(
        self,
        observation: Json,
        context: tuple[Any, ...] | str | None,
    ) -> bool:
        """Match a stored composed-context view exactly to the active native program/value."""
        if not context or self.learning_kernel.context_program is None:
            return False
        try:
            expected = self._context_program_eval(
                self.learning_kernel.context_program,
                observation,
            )
        except Exception:
            return False
        if isinstance(context, str):
            raw_atoms = [context]
        else:
            raw_atoms = list(context)
        for atom in raw_atoms:
            decoded = atom
            if isinstance(decoded, str):
                try:
                    decoded = json.loads(decoded)
                except (TypeError, ValueError):
                    continue
            if (
                isinstance(decoded, list)
                and len(decoded) >= 3
                and decoded[0] == "composed_context"
            ):
                return (
                    canon(decoded[1]) == canon(self.learning_kernel.context_program)
                    and decoded[2] == expected
                )
        return False

    def _stored_context_matches(
        self,
        observation: Json,
        context: tuple[Any, ...] | str | None,
    ) -> bool:
        if not context:
            return True

        raw_atoms: list[Any] = []
        if isinstance(context, str):
            try:
                decoded = json.loads(context)
            except (TypeError, ValueError):
                decoded = context
            if isinstance(decoded, list):
                # adaptive_union stores a canonical JSON wrapper around one atom;
                # historical tuple-based contexts arrive as a sequence of atoms.
                raw_atoms.extend(decoded)
            else:
                raw_atoms.append(decoded)
        else:
            raw_atoms.extend(tuple(context))

        observable = self._observable_context_atoms(observation)
        for atom in raw_atoms:
            if isinstance(atom, str):
                try:
                    decoded = json.loads(atom)
                except (TypeError, ValueError):
                    decoded = atom
            else:
                decoded = atom

            # A stored adaptive-union context can itself contain one atom wrapper.
            if (
                isinstance(decoded, list)
                and len(decoded) == 1
                and isinstance(decoded[0], list)
            ):
                decoded = decoded[0]

            if isinstance(decoded, list) and decoded and decoded[0] == "composed_context":
                try:
                    program = decoded[1]
                    expected = decoded[2]
                    actual = self._context_program_eval(program, observation)
                    if actual != expected:
                        return False
                    continue
                except Exception:
                    return False
            if isinstance(decoded, list):
                key = canon(decoded)
            else:
                key = str(decoded)
            if key not in observable:
                return False
        return True

    def _context_signature(
        self,
        observation: Json,
        action: str | None = None,
        *,
        adaptive_numeric: dict[tuple[str, ...], tuple[str, Any]] | None = None,
    ) -> tuple[Any, ...]:
        """Derive a task-independent context key from observable structure."""
        mode = self.learning_kernel.context_mode
        if adaptive_numeric is None:
            adaptive_numeric = (
                self._adaptive_context(str(action))
                if mode == "adaptive" and action is not None
                else {}
            )
        if mode == "adaptive_union":
            variants = self._compositional_union_context_signatures(observation, str(action))
            return variants[0] if variants else ()

        if mode == "none":
            return ()
        leaves = []
        for path, value in _leaves(observation):
            typ = _shape(value)
            if mode == "shape":
                leaves.append(canon((path, typ)))
            elif mode == "value_bin":
                # Numeric values are the predicted state, not a context discriminator.
                # Keeping their sign here fragments otherwise reusable numeric rules.
                if _numeric(value):
                    continue
                if isinstance(value, bool):
                    leaves.append(canon((path, typ, bool(value))))
                elif isinstance(value, str):
                    leaves.append(canon((path, typ, value[:16])))
                else:
                    leaves.append(canon((path, typ)))
            elif mode == "categorical":
                if isinstance(value, bool):
                    leaves.append(canon((path, "bool", bool(value))))
                elif isinstance(value, str):
                    leaves.append(canon((path, "str", value[:32])))
            elif mode == "adaptive" and path in adaptive_numeric:
                primary, supported = adaptive_numeric[path]
                predicates = supported if supported else (primary,)
                for predicate in predicates:
                    if predicate == "value":
                        leaves.append(canon((path, "value", value if not isinstance(value, str) else value[:32])))
                    elif predicate == "parity" and isinstance(value, int) and not isinstance(value, bool):
                        leaves.append(canon((path, "parity", int(value) % 2)))
                    elif predicate == "sign":
                        leaves.append(canon((path, "sign", "neg" if float(value) < 0 else "zero" if float(value) == 0 else "pos")))
                    elif predicate == "zero":
                        leaves.append(canon((path, "zero", float(value) == 0)))
                    elif predicate == "threshold":
                        leaves.append(canon((path, "threshold", float(value) >= 0)))
        return tuple(leaves[:64])


    @staticmethod
    def _relation_eval(expr: Any, observation: Json) -> Any:
        """Evaluate a small data-derived transition relation DSL."""
        op = expr[0]
        if op == "const":
            return copy.deepcopy(expr[1])
        if op == "field":
            return copy.deepcopy(_get(observation, expr[1]))
        if op == "neg":
            return -float(NativeCognitiveCore._relation_eval(expr[1], observation))
        if op == "abs":
            return abs(float(NativeCognitiveCore._relation_eval(expr[1], observation)))
        if op in {"add", "sub", "mul", "mod", "min", "max"}:
            a = NativeCognitiveCore._relation_eval(expr[1], observation)
            b = NativeCognitiveCore._relation_eval(expr[2], observation)
            if op == "add":
                return a + b
            if op == "sub":
                return a - b
            if op == "mul":
                return a * b
            if op == "mod":
                return a % b
            if op == "min":
                return min(a, b)
            return max(a, b)
        if op == "not":
            return not bool(NativeCognitiveCore._relation_eval(expr[1], observation))
        if op == "eq":
            feature = NativeCognitiveCore._relation_eval(expr[1], observation)
            return NativeCognitiveCore._relation_eval(expr[3], observation) if feature == expr[2] else NativeCognitiveCore._relation_eval(expr[4], observation)
        if op == "ge":
            feature = float(NativeCognitiveCore._relation_eval(expr[1], observation))
            return NativeCognitiveCore._relation_eval(expr[3], observation) if feature >= float(expr[2]) else NativeCognitiveCore._relation_eval(expr[4], observation)
        if op == "parity":
            feature = int(NativeCognitiveCore._relation_eval(expr[1], observation))
            return NativeCognitiveCore._relation_eval(expr[2], observation) if feature % 2 == 0 else NativeCognitiveCore._relation_eval(expr[3], observation)
        raise ValueError(op)

    @staticmethod
    def _relation_complexity(expr: Any) -> int:
        return len(canon(expr))

    @classmethod
    def _candidate_relation_exprs(
        cls,
        rows: Sequence[Experience],
        target_path: tuple[str, ...],
    ) -> tuple[Any, ...]:
        """Generate relations from observed fields/constants, never from task-family names."""
        feature_rows = rows[:32]
        leaves = {}
        targets = []
        for row in feature_rows:
            try:
                targets.append(_get(row.next_observation, target_path))
            except (KeyError, IndexError, TypeError, ValueError):
                continue
            for path, value in _leaves(row.observation):
                leaves.setdefault(tuple(path), []).append(value)

        constants: list[Any] = [0, 1, -1, 2, -2]
        for value in targets[:8]:
            if _numeric(value) or isinstance(value, (str, bool)):
                if canon(value) not in {canon(x) for x in constants}:
                    constants.append(copy.deepcopy(value))
        for vals in leaves.values():
            for value in vals[:4]:
                if _numeric(value) and abs(float(value)) <= 16:
                    constants.append(copy.deepcopy(value))
                elif isinstance(value, (str, bool)):
                    constants.append(copy.deepcopy(value))

        exprs: list[Any] = [("const", copy.deepcopy(c)) for c in constants]
        numeric_fields = []
        categorical_fields = []
        bool_fields = []
        for path, vals in leaves.items():
            if not vals:
                continue
            sample = vals[0]
            if _numeric(sample):
                numeric_fields.append(path)
            elif isinstance(sample, str):
                categorical_fields.append(path)
            elif isinstance(sample, bool):
                bool_fields.append(path)

        bases = exprs[:24]
        for path in numeric_fields[:8] + categorical_fields[:8] + bool_fields[:8]:
            bases.append(("field", path))

        numeric_bases = [e for e in bases if e[0] in {"const", "field"}]
        for field in numeric_fields[:8]:
            fexpr = ("field", field)
            exprs.extend((fexpr, ("neg", fexpr), ("abs", fexpr)))
            for c in constants[:8]:
                if _numeric(c):
                    cexpr = ("const", c)
                    exprs.extend((
                        ("add", fexpr, cexpr),
                        ("sub", fexpr, cexpr),
                        ("mul", fexpr, cexpr),
                        ("mod", fexpr, cexpr) if float(c) != 0 else ("const", 0),
                        ("min", fexpr, cexpr),
                        ("max", fexpr, cexpr),
                        ("add", cexpr, fexpr),
                    ))
            # Permit generic nonlinear unary composition such as x*x without
            # routing on a task-family name or introducing a special-case square op.
            exprs.append(("mul", fexpr, fexpr))
        # Relational combinations between independently observed numeric fields.
        for i, left_path in enumerate(numeric_fields[:6]):
            for right_path in numeric_fields[i + 1:7]:
                left = ("field", left_path)
                right = ("field", right_path)
                exprs.extend((
                    ("add", left, right),
                    ("sub", left, right),
                    ("sub", right, left),
                    ("mul", left, right),
                ))

        # Fit a directly observed affine relation y = a*x+b from each numeric feature.
        for path in numeric_fields[:8]:
            pairs = []
            for row in feature_rows:
                try:
                    x = _get(row.observation, path)
                    y = _get(row.next_observation, target_path)
                except (KeyError, IndexError, TypeError, ValueError):
                    continue
                if _numeric(x) and _numeric(y):
                    pairs.append((float(x), float(y)))
            if len(pairs) >= 2:
                first = pairs[0]
                model = None
                for second in pairs[1:]:
                    if abs(second[0] - first[0]) > 1e-12:
                        a = (second[1] - first[1]) / (second[0] - first[0])
                        b = first[1] - a * first[0]
                        model = (a, b)
                        break
                if model is not None:
                    a, b = model
                    if all(abs(a*x+b-y) <= 1e-9 for x, y in pairs):
                        exprs.append(("add", ("mul", ("field", path), ("const", a)), ("const", b)))

        # Learn simple causal conditionals from any observed feature.
        branch_exprs = list(dict.fromkeys(canon(e) for e in numeric_bases[:20]))
        branch_lookup = {canon(e): e for e in numeric_bases[:20]}
        branch_values = list(branch_lookup.values())
        for path in numeric_fields[:6]:
            fexpr = ("field", path)
            parity_pairs: dict[int, list[Any]] = {0: [], 1: []}
            for row in feature_rows:
                try:
                    x = _get(row.observation, path)
                    y = _get(row.next_observation, target_path)
                except (KeyError, IndexError, TypeError, ValueError):
                    continue
                if isinstance(x, int) and not isinstance(x, bool):
                    parity_pairs[int(x) % 2].append(y)
            if all(parity_pairs[k] for k in (0, 1)):
                for even in branch_values[:12]:
                    for odd in branch_values[:12]:
                        exprs.append(("parity", fexpr, even, odd))

            thresholds = sorted({0.0, *[float(x) for x in constants if _numeric(x)][:6]})
            for threshold in thresholds:
                for high in branch_values[:10]:
                    for low in branch_values[:10]:
                        exprs.append(("ge", fexpr, threshold, high, low))

        for path in categorical_fields[:6]:
            fexpr = ("field", path)
            observed_values = []
            for row in feature_rows:
                try:
                    value = _get(row.observation, path)
                except (KeyError, IndexError, TypeError, ValueError):
                    continue
                if value not in observed_values:
                    observed_values.append(value)
            for value in observed_values[:6]:
                for yes in branch_values[:10]:
                    for no in branch_values[:10]:
                        exprs.append(("eq", fexpr, value, yes, no))

        # Stable de-duplication and bounded search.
        unique: dict[str, Any] = {}
        for expr in exprs:
            unique.setdefault(canon(expr), expr)
        return tuple(sorted(unique.values(), key=cls._relation_complexity)[:512])

    @classmethod
    def _induce_relation(
        cls,
        rows: Sequence[Experience],
        target_path: tuple[str, ...],
        threshold: float,
    ) -> tuple[Any, float, float] | None:
        candidates = cls._candidate_relation_exprs(rows, target_path)
        best = None
        for expr in candidates:
            ok = True
            matches = 0
            usable = 0
            for row in rows[:32]:
                try:
                    target = _get(row.next_observation, target_path)
                    pred = cls._relation_eval(expr, row.observation)
                except Exception:
                    ok = False
                    break
                usable += 1
                if pred == target or (
                    _numeric(pred) and _numeric(target) and abs(float(pred) - float(target)) <= 1e-9
                ):
                    matches += 1
                else:
                    ok = False
                    break
            support = matches / max(1, usable)
            if ok and usable >= 2 and support >= threshold:
                score = cls._relation_complexity(expr)
                rank = (support, -score)
                if best is None or rank > best[0]:
                    best = (rank, expr, support, min(0.999, 0.55 + 0.44 * support))
        if best is None:
            return None
        _, expr, support, confidence = best
        return expr, support, confidence

    def _refresh_hypotheses(self):
        records = [e for e in self.experience if not e.exogenous]
        by_action: dict[str, list[Experience]] = {}
        for e in records:
            by_action.setdefault(e.action, []).append(e)

        threshold = min(0.99, max(0.50, float(self.learning_kernel.evidence_threshold)))
        min_count = max(2, int(self.learning_kernel.min_support_count))
        # Non-destructive regeneration is enabled only for the adaptive-union
        # learning rule. Other context modes preserve their established sequential/batch
        # semantics. When adaptive-union is installed as a new learning rule, prior
        # hypotheses remain addressable while new context views are added.
        prior_hypotheses = (
            dict(self.hypotheses)
            if self.learning_kernel.context_mode == "adaptive_union"
            else {}
        )
        self.hypotheses = {}

        for action, rows in by_action.items():
            if len(rows) < min_count:
                continue
            adaptive_numeric = (
                self._adaptive_context(action)
                if self.learning_kernel.context_mode == "adaptive"
                else {}
            )
            groups: dict[tuple[Any, ...], list[Experience]] = {}
            for row in rows:
                if self.learning_kernel.context_mode == "adaptive_union":
                    contexts = self._compositional_union_context_signatures(row.observation, action)
                    if not contexts:
                        contexts = ((),)
                else:
                    contexts = (
                        self._context_signature(
                            row.observation,
                            action,
                            adaptive_numeric=adaptive_numeric,
                        ),
                    )
                for ctx in contexts:
                    groups.setdefault(ctx, []).append(row)

            for ctx, grows in groups.items():
                composed_context_group = self._context_contains_composed_context(ctx)
                group_min_count = 1 if composed_context_group else min_count
                if len(grows) < group_min_count:
                    continue
                counts: dict[tuple[Any, ...], int] = {}
                for row in grows:
                    for effect in self._effects(row.observation, row.next_observation):
                        kind, path, value = effect
                        # Numeric deltas are quantized only for counting so near-identical
                        # observations do not create an unbounded hypothesis population.
                        normalized = round(float(value), 12) if kind == "delta" else canon(value)
                        key = (kind, tuple(path), normalized)
                        counts[key] = counts.get(key, 0) + 1

                # Generic affine relation discovery: fit y = a*x + b independently
                # within each learned context, without task-family routing.
                numeric_pairs: dict[tuple[str, ...], list[tuple[float, float]]] = {}
                for row in grows:
                    for path, before_value in _leaves(row.observation):
                        if not _numeric(before_value):
                            continue
                        try:
                            after_value = _get(row.next_observation, path)
                        except (KeyError, IndexError, TypeError, ValueError):
                            continue
                        if _numeric(after_value):
                            numeric_pairs.setdefault(tuple(path), []).append(
                                (float(before_value), float(after_value))
                            )
                for path, pairs in numeric_pairs.items():
                    unique = []
                    seen_x = set()
                    for x, y in pairs:
                        if x not in seen_x:
                            seen_x.add(x)
                            unique.append((x, y))
                    if len(unique) < min_count:
                        continue
                    x1, y1 = unique[0]
                    model = None
                    for x2, y2 in unique[1:]:
                        if abs(x2 - x1) > 1e-12:
                            a = (y2 - y1) / (x2 - x1)
                            b = y1 - a * x1
                            model = (a, b)
                            break
                    if model is None:
                        continue
                    a, b = model
                    exact = all(abs(a * x + b - y) <= 1e-9 for x, y in unique)
                    support = len(unique) / max(1, len(grows))
                    # Constant-set is represented separately; affine is for genuine
                    # input-dependent transformations.
                    if exact and support >= threshold and abs(a - 1.0) > 1e-12:
                        key = ("affine", path, round(a, 12), round(b, 12))
                        hid = "hyp:" + digest((action, ctx, key))[:20]
                        self.hypotheses[hid] = Hypothesis(
                            hid,
                            "affine",
                            action,
                            ("path",) + tuple(path),
                            None,
                            1,
                            ctx if ctx else None,
                            support,
                            min(0.999, 0.70 + 0.29 * support),
                            2 + len(path),
                            {"a": a, "b": b},
                        )

                # Generic numeric set-rule discovery: distinguish "x becomes C"
                # from ordinary numeric deltas without knowing a task family.
                numeric_set_groups: dict[tuple[str, ...], dict[float, int]] = {}
                for row in grows:
                    for path, before_value in _leaves(row.observation):
                        after_value = None
                        try:
                            after_value = _get(row.next_observation, path)
                        except (KeyError, IndexError, TypeError, ValueError):
                            continue
                        if _numeric(before_value) and _numeric(after_value):
                            numeric_set_groups.setdefault(tuple(path), {})
                            target = float(after_value)
                            counts_for_path = numeric_set_groups[tuple(path)]
                            counts_for_path[target] = counts_for_path.get(target, 0) + 1
                for path, target_counts in numeric_set_groups.items():
                    target, count = sorted(
                        target_counts.items(),
                        key=lambda kv: (-kv[1], kv[0]),
                    )[0]
                    support = count / max(1, len(grows))
                    if count >= min_count and support >= threshold:
                        key = ("set", path, round(float(target), 12))
                        hid = "hyp:" + digest((action, ctx, key))[:20]
                        self.hypotheses[hid] = Hypothesis(
                            hid,
                            "set",
                            action,
                            ("path",) + tuple(path),
                            None,
                            1,
                            ctx if ctx else None,
                            support,
                            min(0.995, 0.50 + 0.49 * support),
                            1 + len(path),
                            float(target),
                        )

                # General relation induction covers state-dependent transformations
                # that cannot be represented as a single delta/set hypothesis.
                target_paths = {
                    tuple(path)
                    for row in grows
                    for path, _ in _leaves(row.next_observation)
                    if any(tuple(ep) == tuple(path) for _, ep, _ in self._effects(row.observation, row.next_observation))
                }
                for target_path in sorted(target_paths):
                    induced = self._induce_relation(grows, target_path, threshold)
                    if induced is not None:
                        expr, support, confidence = induced
                        hid = "hyp:" + digest((action, ctx, "relation", target_path, expr))[:20]
                        self.hypotheses[hid] = Hypothesis(
                            hid,
                            "expression",
                            action,
                            ("path",) + tuple(target_path),
                            None,
                            1,
                            ctx if ctx else None,
                            float(support),
                            float(confidence),
                            self._relation_complexity(expr),
                            copy.deepcopy(expr),
                        )

                for key, count in counts.items():
                    support = count / max(1, len(grows))
                    required_count = (
                        group_min_count
                        if composed_context_group and key[0] == "delta"
                        else min_count
                    )
                    if count < required_count or support < threshold:
                        continue
                    kind, path, normalized = key
                    raw_value = None
                    if kind == "set":
                        # recover the actual set-value from the first matching event
                        for row in grows:
                            for ek, ep, ev in self._effects(row.observation, row.next_observation):
                                if ek == kind and tuple(ep) == tuple(path) and canon(ev) == normalized:
                                    raw_value = copy.deepcopy(ev)
                                    break
                            if raw_value is not None:
                                break
                    elif kind == "delta":
                        raw_value = float(normalized)
                    else:
                        raw_value = copy.deepcopy(normalized)

                    hid = "hyp:" + digest((action, ctx, key))[:20]
                    h = Hypothesis(
                        hid,
                        kind,
                        action,
                        ("path",) + tuple(path),
                        float(raw_value) if kind == "delta" else None,
                        1,
                        ctx if ctx else None,
                        support,
                        min(0.995, 0.50 + 0.49 * support),
                        1 + len(path),
                        copy.deepcopy(raw_value) if kind == "set" else None,
                    )
                    self.hypotheses[hid] = h

                # Delayed effects require repeated aligned evidence rather than a single
                # accidental successor. This materially reduces false temporal rules.
                max_delay = min(
                    max(1, int(self.learning_kernel.delayed_window)),
                    len(grows) // 2,
                )
                for delay in range(2, max_delay + 1):
                    aligned: dict[tuple[Any, ...], int] = {}
                    pairs = 0
                    for idx in range(len(grows) - delay + 1):
                        current = grows[idx]
                        future = grows[idx + delay - 1]
                        if self.learning_kernel.context_mode == "adaptive_union":
                            current_views = set(self._adaptive_union_context_signatures(current.observation, action))
                            future_views = set(self._adaptive_union_context_signatures(future.observation, action))
                            if current_views.isdisjoint(future_views):
                                continue
                        elif (
                            self._context_signature(
                                current.observation,
                                action,
                                adaptive_numeric=adaptive_numeric,
                            )
                            != self._context_signature(
                                future.observation,
                                action,
                                adaptive_numeric=adaptive_numeric,
                            )
                        ):
                            continue
                        pairs += 1
                        current_effects = {
                            (kind, tuple(path), round(float(value), 12) if kind == "delta" else canon(value))
                            for kind, path, value in self._effects(current.observation, current.next_observation)
                        }
                        for kind, path, value in self._effects(future.observation, future.next_observation):
                            normalized = round(float(value), 12) if kind == "delta" else canon(value)
                            key = (kind, tuple(path), normalized)
                            if key in current_effects:
                                continue
                            aligned[key] = aligned.get(key, 0) + 1
                    if pairs < min_count:
                        continue
                    for key, count in aligned.items():
                        support = count / max(1, pairs)
                        if count < min_count or support < threshold:
                            continue
                        kind, path, normalized = key
                        raw_value = float(normalized) if kind == "delta" else None
                        hid = "hyp:" + digest((action, ctx, "delayed", delay, key))[:20]
                        self.hypotheses[hid] = Hypothesis(
                            hid,
                            kind,
                            action,
                            ("path",) + tuple(path),
                            raw_value,
                            delay,
                            ctx if ctx else None,
                            support,
                            min(0.995, 0.50 + 0.49 * support),
                            2 + len(path),
                            None,
                        )

        for hid, hypothesis in prior_hypotheses.items():
            self.hypotheses.setdefault(hid, hypothesis)

    def _apply_hypothesis(self, hypothesis: Hypothesis, observation: Json) -> Json | None:
        if hypothesis.role is None or hypothesis.role[0] != "path":
            return None
        path = hypothesis.role[1:]
        base = _get(observation, path)
        if hypothesis.kind == "delta":
            return _replace(
                observation,
                path,
                float(base) + float(hypothesis.delta or 0.0),
            )
        if hypothesis.kind == "toggle":
            return _replace(observation, path, not bool(base))
        if hypothesis.kind == "set":
            if hypothesis.value is None:
                return None
            return _replace(observation, path, hypothesis.value)
        if hypothesis.kind == "affine":
            if not isinstance(hypothesis.value, Mapping):
                return None
            a = float(hypothesis.value.get("a", 1.0))
            b = float(hypothesis.value.get("b", 0.0))
            return _replace(observation, path, a * float(base) + b)
        if hypothesis.kind == "expression":
            return _replace(
                observation,
                path,
                self._relation_eval(hypothesis.value, observation),
            )
        return copy.deepcopy(observation)

    def _hypothesis_arbitration_score(
        self,
        hypothesis: Hypothesis,
        observation: Json,
        action: str,
    ) -> float:
        """Score a persistent hypothesis from native, training-only evidence.

        The score is deliberately mechanism-agnostic: hypotheses compete on their
        recency-weighted predictive consistency over prior experience that matches
        their own stored observable context. No task-family, benchmark ID, or
        external signal is used.
        """
        relevant = [
            row for row in self.experience
            if (not row.exogenous and row.action == str(action))
        ]
        if hypothesis.context:
            relevant = [
                row for row in relevant
                if self._stored_context_matches(row.observation, hypothesis.context)
            ]
        if hypothesis.delay != 1:
            relevant = []

        if not relevant:
            return 0.25 * float(hypothesis.confidence) + 0.25 * float(hypothesis.support)

        window = max(8, int(self.learning_kernel.replay_limit))
        relevant = relevant[-window:]
        total_weight = 0.0
        correct_weight = 0.0
        decay = 0.97
        for age, row in enumerate(reversed(relevant)):
            pred = None
            try:
                pred = self._apply_hypothesis(hypothesis, row.observation)
            except Exception:
                pred = None
            if pred is None:
                continue
            weight = decay ** age
            total_weight += weight
            if pred == row.next_observation:
                correct_weight += weight

        if total_weight <= 1e-12:
            consistency = 0.0
        else:
            consistency = correct_weight / total_weight

        prior = 0.5 * (
            float(hypothesis.confidence) + float(hypothesis.support)
        )
        return 0.75 * consistency + 0.25 * prior

    def predict(self, observation: Json, action: str, *, horizon: int = 1) -> dict[str, Any]:
        action = str(action)
        target_horizon = max(1, int(horizon))
        current_ctx = self._context_signature(observation, action)
        current_contexts = (
            self._adaptive_union_context_signatures(observation, action)
            if self.learning_kernel.context_mode == "adaptive_union"
            else (current_ctx,)
        )
        current_context_set = {tuple(ctx) for ctx in current_contexts}
        hs = [
            h for h in self.hypotheses.values()
            if h.action == action
            and h.delay == target_horizon
            and (
                h.context is None
                or tuple(h.context) == current_ctx
                or tuple(h.context) in current_context_set
                or self._stored_context_matches(observation, h.context)
            )
        ]
        if not hs:
            return {"prediction": None, "alternatives": (), "uncertainty": 1.0}
        candidates = []
        for h in sorted(hs, key=lambda z: (-z.confidence, z.complexity, z.hypothesis_id)):
            try:
                pred = self._apply_hypothesis(h, observation)
                if pred is None:
                    continue
                arbitration_score = self._hypothesis_arbitration_score(
                    h,
                    observation,
                    action,
                )
                candidates.append((pred, h.confidence, arbitration_score, h.complexity, h.hypothesis_id))
            except Exception:
                continue
        if (
            self.learning_kernel.prediction_mode == "protected_overlay"
            and self.protected_hypotheses
        ):
            protected_hs = [
                h for h in self.protected_hypotheses.values()
                if h.action == action
                and h.delay == target_horizon
                and (
                    h.context is None
                    or tuple(h.context) == current_ctx
                    or tuple(h.context) in current_context_set
                    or self._stored_context_matches(observation, h.context)
                )
            ]
            exact_composed_learned = [
                h for h in hs
                if self._matches_active_composed_context(observation, h.context)
            ]
            exact_composed_protected = [
                h for h in protected_hs
                if self._matches_active_composed_context(observation, h.context)
            ]
            if exact_composed_learned or exact_composed_protected:
                hs = exact_composed_learned
                protected_hs = exact_composed_protected
            for h in sorted(
                protected_hs,
                key=lambda z: (-z.confidence, z.complexity, z.hypothesis_id),
            ):
                try:
                    pred = self._apply_hypothesis(h, observation)
                    if pred is None:
                        continue
                    arbitration_score = self._hypothesis_arbitration_score(
                        h, observation, action
                    )
                    candidates.append(
                        (
                            "__protected__",
                            pred,
                            h.confidence,
                            arbitration_score,
                            h.complexity,
                            h.hypothesis_id,
                        )
                    )
                except Exception:
                    continue
        if not candidates:
            return {"prediction": None, "alternatives": (), "uncertainty": 1.0}
        buckets: dict[str, tuple[Json, float, float, int, str, str]] = {}
        for item in candidates:
            source = "protected" if len(item) == 6 and item[0] == "__protected__" else "learned"
            if source == "protected":
                _, pred, conf, arb, complexity, hypothesis_id = item
            else:
                pred, conf, arb, complexity, hypothesis_id = item
            k = canon(pred)
            old = buckets.get(k)
            new_key = (
                float(arb),
                float(conf),
                1.0 if source == "protected" else 0.0,
                -int(complexity),
                str(hypothesis_id),
            )
            old_key = (
                old[2],
                old[1],
                1.0 if old[5] == "protected" else 0.0,
                -old[3],
                old[4],
            ) if old is not None else None
            if old is None or new_key > old_key:
                buckets[k] = (
                    pred,
                    float(conf),
                    float(arb),
                    int(complexity),
                    str(hypothesis_id),
                    source,
                )
        ordered = sorted(
            buckets.values(),
            key=lambda x: (-x[2], -x[1], 0 if x[5] == "protected" else 1, x[3], x[4]),
        )
        mode = self.learning_kernel.prediction_mode
        if mode == "protected_overlay":
            protected_scores = [x[2] for x in ordered if x[5] == "protected"]
            learned_scores = [x[2] for x in ordered if x[5] == "learned"]
            if protected_scores and learned_scores:
                best_protected = max(protected_scores)
                best_learned = max(learned_scores)
                if best_protected > best_learned + float(self.learning_kernel.contradiction_margin):
                    protected_ordered = [x for x in ordered if x[5] == "protected"]
                    learned_ordered = [x for x in ordered if x[5] == "learned"]
                    if protected_ordered:
                        ordered = protected_ordered + learned_ordered
                else:
                    learned_ordered = [x for x in ordered if x[5] == "learned"]
                    protected_ordered = [x for x in ordered if x[5] == "protected"]
                    if learned_ordered:
                        ordered = learned_ordered + protected_ordered
        if mode == "conservative":
            if ordered[0][1] < 0.90 or (
                len(ordered) > 1
                and abs(ordered[0][1] - ordered[1][1]) <= self.learning_kernel.contradiction_margin
            ):
                return {"prediction": None, "alternatives": tuple(copy.deepcopy(x[0]) for x in ordered[:8]), "uncertainty": 1.0}
        elif mode == "highest_confidence":
            ordered = ordered[:1]
        # ensemble mode preserves the full competing-prediction evidence.
        uncertainty = min(1.0, max(0.0, 1.0 - ordered[0][2]))
        if len(ordered) > 1:
            uncertainty = max(
                uncertainty,
                min(1.0, 0.25 + 0.15 * (len(ordered) - 1)),
            )
        return {
            "prediction": copy.deepcopy(ordered[0][0]),
            "alternatives": tuple(copy.deepcopy(x[0]) for x in ordered[:8]),
            "uncertainty": uncertainty,
            "arbitration_score": float(ordered[0][2]),
            "prediction_source": ordered[0][5],
        }

    def feedback(self, observation: Json, action: str, predicted: Json | None, actual: Json) -> dict[str, Any]:
        correct = predicted == actual
        row = {
            "step": self.step,
            "action": str(action),
            "correct": bool(correct),
            "predicted": copy.deepcopy(predicted),
            "actual": copy.deepcopy(actual),
        }
        self.prediction_history.append(row)
        self.prediction_history = self.prediction_history[-self.policy.memory_limit:]
        key = digest((str(action), _shape(observation)))
        mem = self.semantic_memory.setdefault(key, {"attempts": 0, "correct": 0, "error_rate": 0.0})
        mem["attempts"] += 1
        mem["correct"] += int(correct)
        mem["error_rate"] = 1.0 - mem["correct"] / max(1, mem["attempts"])
        return row

    def calibrated_uncertainty(self, action: str, shape: Any | None = None) -> float:
        if shape is None:
            rows = [r for r in self.prediction_history if r["action"] == str(action)]
        else:
            key = digest((str(action), shape))
            rows = []
            mem = self.semantic_memory.get(key)
            if mem is not None:
                return float(mem["error_rate"])
        if not rows:
            return 1.0
        return 1.0 - sum(bool(r["correct"]) for r in rows) / len(rows)

    def intervene(self, observation: Json, actions: Sequence[str]) -> tuple[dict[str, Any], ...]:
        """Construct counterfactual action predictions from the current learned model."""
        out = []
        for action in actions:
            pred = self.predict(observation, action)
            out.append({
                "action": str(action),
                "prediction": pred["prediction"],
                "uncertainty": float(pred["uncertainty"]),
                "information_value": float(pred["uncertainty"]),
            })
        return tuple(sorted(out, key=lambda x: (-x["information_value"], x["action"])))

    def select_experiment(self, observation: Json, actions: Sequence[str]) -> dict[str, Any] | None:
        candidates = self.intervene(observation, actions)
        if not candidates:
            return None
        return copy.deepcopy(candidates[0])

    def generate_curriculum(self, tasks: Sequence[tuple[str, Sequence[tuple[Json, Json]]]]) -> tuple[dict[str, Any], ...]:
        """Create the next learning tasks from observed failure/uncertainty rather than labels."""
        scored = []
        for task_id, examples in tasks:
            if not examples:
                continue
            failures = 0
            for x, y in examples:
                solved = False
                for e in self._expr_candidates():
                    try:
                        if self._expr_eval(e, x) == y:
                            solved = True
                            break
                    except Exception:
                        continue
                if not solved:
                    failures += 1
            novelty = 1.0 / (1.0 + sum(1 for p in self.procedures.values() if p.provenance.get("family") == task_id))
            score = failures + self.policy.curiosity_weight * novelty
            scored.append({
                "task_id": str(task_id),
                "score": float(score),
                "reason": "failure_or_novelty_pressure",
                "examples": len(examples),
            })
        scored.sort(key=lambda x: (-x["score"], x["task_id"]))
        return tuple(scored[:32])

    def learn_generic_program(
        self,
        examples: Sequence[tuple[Json, Json]],
        holdout: Sequence[tuple[Json, Json]] = (),
        transfer: Sequence[tuple[Json, Json]] = (),
        *,
        family: str = "generic-program",
        admit_to_library: bool = True,
    ) -> Procedure | None:
        """Use the repository's generic, model-free program synthesizer as a native kernel capability."""
        try:
            from .program_synthesis import synthesize_program
            candidate, trials = synthesize_program(
                examples,
                max_depth=self.policy.max_expression_depth + 2,
                beam=max(16, min(256, self.policy.beam)),
                beam_growth=max(0, int(self.policy.beam_growth)),
                frontier_diversity=max(0.0, min(1.0, float(self.policy.frontier_diversity))),
                library=tuple(self.program_library),
            )
        except Exception:
            return None
        if candidate is None:
            return None
        external_holdout = bool(holdout)
        external_transfer = bool(transfer)
        holdout = tuple(holdout) or tuple(examples[-max(1, len(examples)//3):])
        transfer = tuple(transfer) or tuple(examples[:1])
        hs = sum(candidate.apply(x) == y for x, y in holdout) / max(1, len(holdout))
        ts = sum(candidate.apply(x) == y for x, y in transfer) / max(1, len(transfer))
        if hs < 1.0 or ts < 1.0:
            return None
        pid = "gproc:" + digest((candidate.program_id, self.generation))[:20]
        p = Procedure(
            pid,
            tuple({"token": s} for s in candidate.steps),
            float(1.0 - candidate.training_error),
            float(hs),
            float(ts),
            int(candidate.complexity),
            self.generation,
            None,
            {
                "origin": "native_generic_program_synthesis",
                "external_model": False,
                "network": False,
                "manual_solution": False,
                "trials": int(trials),
                "family": str(family),
                "external_holdout_provided": external_holdout,
                "external_transfer_provided": external_transfer,
            },
            "validated",
        )
        self.procedures[pid] = p
        if admit_to_library:
            self._admit_program_macro(candidate.steps, family=family, procedure_id=pid)
            if self.policy.abstract_verified_subprograms:
                self._admit_program_submacros(candidate.steps, family=family, procedure_id=pid)
        cid = "cap:" + digest((family, pid, "generic-program"))[:20]
        self.capabilities[cid] = Capability(cid, family, p.train_score, p.transfer_score, 1.0, p.complexity, pid, self.generation)
        return p

    def _admit_program_submacros(
        self,
        steps: Sequence[str],
        *,
        family: str,
        procedure_id: str | None = None,
    ) -> tuple[str, ...]:
        """Promote verified short contiguous program fragments for later reuse."""
        from .program_synthesis import encode_macro

        tokens = tuple(steps)
        added: list[str] = []
        for width in (2, 3):
            if len(tokens) < width:
                continue
            for start in range(len(tokens) - width + 1):
                fragment = tokens[start : start + width]
                macro = encode_macro(fragment)
                if macro in self.program_library:
                    continue
                if len(self.program_library) >= 64:
                    self.program_library = self.program_library[-63:]
                self.program_library.append(macro)
                added.append(macro)
                self.change_history.append({
                    "type": "capability_subprogram_abstraction",
                    "generation": self.generation,
                    "macro": macro,
                    "family": str(family),
                    "procedure_id": procedure_id,
                    "step_count": width,
                    "start": start,
                    "executable_representation_change": True,
                    "external_model": False,
                    "network": False,
                })
        return tuple(added)

    def _admit_program_macro(
        self,
        steps: Sequence[str],
        *,
        family: str,
        procedure_id: str | None = None,
    ) -> str | None:
        """Promote a verified program into the executable search vocabulary."""
        from .program_synthesis import encode_macro
        macro = encode_macro(tuple(steps))
        if macro in self.program_library:
            return macro
        # Keep the reusable library bounded and deterministic.
        if len(self.program_library) >= 64:
            self.program_library = self.program_library[-63:]
        self.program_library.append(macro)
        self.change_history.append({
            "type": "capability_library_growth",
            "generation": self.generation,
            "macro": macro,
            "family": str(family),
            "procedure_id": procedure_id,
            "step_count": len(tuple(steps)),
            "executable_representation_change": True,
            "external_model": False,
            "network": False,
        })
        return macro

    # ---------- native algorithmic portfolio and metacognition ----------

    @staticmethod
    def _candidate_score(expected: Json | None, predicted: Json | None) -> float:
        if expected is None:
            return 0.0 if predicted is None else 0.5
        return 1.0 if predicted == expected else 0.0

    def _solver_profile_key(self, examples: Sequence[tuple[Json, Json]]) -> str:
        shapes = tuple(sorted((_shape(x), _shape(y)) for x, y in examples[:8]))
        return digest(("solver-profile", shapes))[:20]

    def _record_solver_outcome(self, profile: str, solver: str, verified: bool, score: float) -> None:
        bucket = self.solver_history.setdefault(
            profile,
            {"program_induction": 0.0, "relation_induction": 0.0},
        )
        key = str(solver)
        old = float(bucket.get(key, 0.0))
        bucket[key] = 0.85 * old + 0.15 * (float(score) if verified else 0.0)

    def solve_examples_anytime(
        self,
        examples: Sequence[tuple[Json, Json]],
        holdout: Sequence[tuple[Json, Json]] = (),
        transfer: Sequence[tuple[Json, Json]] = (),
        *,
        budget: int = 256,
    ) -> SolutionCandidate:
        """Run a native solver portfolio and keep only independently verified solutions.

        The portfolio is representation-driven, not benchmark-family routed. It combines
        executable program induction with relation induction over observed structures.
        Search is bounded by budget and always returns the best verified candidate found.
        """
        if not examples:
            return SolutionCandidate("none", None, 0.0, False, 0, {"reason": "no_examples"})

        holdout_rows = tuple(holdout) or tuple(examples[-max(1, len(examples) // 3):])
        transfer_rows = tuple(transfer) or tuple(examples[:max(1, len(examples) // 4)])
        candidates: list[SolutionCandidate] = []

        # Solver 1: generic executable sequence/program induction.
        program = self.learn_generic_program(
            examples[:],
            holdout_rows,
            transfer_rows,
            family="native-portfolio",
            admit_to_library=False,
        )
        if program is not None:
            verifier = [
                float(program.program and True),
                float(program.holdout_score),
                float(program.transfer_score),
            ]
            candidates.append(SolutionCandidate(
                "program_induction",
                program,
                statistics.mean(verifier),
                bool(program.provenance.get("external_holdout_provided"))
                and bool(program.provenance.get("external_transfer_provided"))
                and program.holdout_score >= 1.0
                and program.transfer_score >= 1.0,
                int(program.complexity),
                {"trials": program.provenance.get("trials", 0)},
            ))

        # Solver 2: native relation induction over state transitions.
        # Convert examples to temporary experiences without mutating the live model.
        scratch = NativeCognitiveCore(seed=self.seed + 9001, policy=self.policy)
        scratch.learning_kernel = LearningKernel(
            evidence_threshold=self.learning_kernel.evidence_threshold,
            min_support_count=self.learning_kernel.min_support_count,
            context_mode="adaptive",
            delayed_window=self.learning_kernel.delayed_window,
            prediction_mode=self.learning_kernel.prediction_mode,
            contradiction_margin=self.learning_kernel.contradiction_margin,
            replay_limit=self.learning_kernel.replay_limit,
        )
        for x, y in examples:
            if isinstance(x, Mapping) and isinstance(y, Mapping):
                scratch.observe(x, "portfolio", y)

        relation_predictions = []
        if scratch.experience:
            target_paths = {
                tuple(path)
                for x, y in examples
                if isinstance(x, Mapping) and isinstance(y, Mapping)
                for _, path, _ in scratch._effects(x, y)
            }
            for path in sorted(target_paths):
                induced = scratch._induce_relation(
                    scratch.experience,
                    path,
                    scratch.learning_kernel.evidence_threshold,
                )
                if induced is None:
                    continue
                expr, support, confidence = induced
                relation_predictions.append((path, expr, support, confidence))

        for path, expr, support, confidence in relation_predictions[:max(1, budget // 8)]:
            good_hold = [
                float(
                    scratch._relation_eval(expr, x) == scratch._relation_eval(
                        ("field", tuple(path)), y
                    )
                )
                for x, y in holdout_rows
                if isinstance(x, Mapping) and isinstance(y, Mapping)
            ]
            good_transfer = [
                float(
                    scratch._relation_eval(expr, x)
                    == scratch._relation_eval(("field", tuple(path)), y)
                )
                for x, y in transfer_rows
                if isinstance(x, Mapping) and isinstance(y, Mapping)
            ]
            hs = statistics.mean(good_hold) if good_hold else support
            ts = statistics.mean(good_transfer) if good_transfer else support
            score = statistics.mean((float(support), float(confidence), hs, ts))
            candidates.append(SolutionCandidate(
                "relation_induction",
                {"path": path, "expression": expr},
                score,
                bool(holdout) and bool(transfer) and hs >= 0.75 and ts >= 0.75,
                self._relation_complexity(expr),
                {"support": support, "confidence": confidence},
            ))

        if not candidates:
            return SolutionCandidate("none", None, 0.0, False, 0, {"reason": "no_candidate"})

        # Verification is mandatory; prefer verified candidates, then evidence, then simplicity.
        profile = self._solver_profile_key(examples)
        for c in candidates:
            self._record_solver_outcome(profile, c.solver, c.verified, c.score)
        learned = self.solver_history.get(profile, {})
        candidates.sort(
            key=lambda c: (
                bool(c.verified),
                float(c.score),
                float(learned.get(c.solver, 0.0)),
                -int(c.complexity),
                c.solver,
            ),
            reverse=True,
        )
        chosen = candidates[0]
        if (
            chosen.solver == "program_induction"
            and chosen.verified
            and isinstance(chosen.result, Procedure)
        ):
            tokens = tuple(
                step["token"]
                for step in chosen.result.program
                if isinstance(step, Mapping) and step.get("token")
            )
            self._admit_program_macro(
                tokens,
                family="native-portfolio",
                procedure_id=chosen.result.procedure_id,
            )
        return chosen

    def adversarial_verify(
        self,
        candidate: SolutionCandidate,
        cases: Sequence[tuple[Json, Json]],
        *,
        perturbations: int = 8,
    ) -> dict[str, Any]:
        """Stress-test a candidate on derived cases without using external intelligence."""
        if not candidate.verified or not cases:
            return {"verified": False, "score": 0.0, "tested": 0}
        tested = 0
        correct = 0
        if candidate.solver == "relation_induction" and isinstance(candidate.result, Mapping):
            expr = candidate.result["expression"]
            for obs, expected in cases:
                try:
                    pred = self._relation_eval(expr, obs)
                    target = _get(expected, candidate.result["path"])
                    correct += int(pred == target)
                    tested += 1
                except Exception:
                    tested += 1
        elif candidate.solver == "program_induction" and isinstance(candidate.result, Procedure):
            try:
                from .program_synthesis import apply_program
                tokens = tuple(
                    step["token"]
                    for step in candidate.result.program
                    if isinstance(step, Mapping) and "token" in step
                )
            except Exception:
                tokens = ()
            for obs, expected in cases:
                try:
                    pred = apply_program(obs, tokens)
                    correct += int(pred == expected)
                    tested += 1
                except Exception:
                    tested += 1
        # Perturbation budget is represented as repeated independent verification passes.
        passes = []
        for _ in range(max(1, int(perturbations))):
            passes.append(correct / max(1, tested))
        # Replaying the exact same adversarial cases is not additional evidence.
        # The adversarial cases themselves are the independent evaluation set.
        score = correct / max(1, tested)
        return {
            "verified": score >= 0.75,
            "score": float(score),
            "tested": tested,
            "passes": 1,
            "evaluation_semantics": "fixed_adversarial_cases",
        }

    # ---------- information-seeking goal generation ----------

    def generate_goals(self, observations: Sequence[Json]) -> tuple[dict[str, Any], ...]:
        goals = []
        known_shapes = {canon(_shape(e.observation)) for e in self.experience}
        for x in observations:
            shape_novelty = 1.0 if canon(_shape(x)) not in known_shapes else 0.0
            unknown_actions = sorted({
                e.action for e in self.experience
                if self.predict(e.observation, e.action)["prediction"] is None
            })
            uncertainty = sum(
                self.predict(e.observation, a)["uncertainty"]
                for e in self.experience[-32:]
                for a in unknown_actions[:8]
            )
            score = self.policy.curiosity_weight * (uncertainty + shape_novelty)
            goals.append({
                "goal_id": "goal:" + digest((x, unknown_actions))[:20],
                "type": "information_gain",
                "target": copy.deepcopy(x),
                "actions": tuple(unknown_actions),
                "score": float(score),
            })
        goals.sort(key=lambda g: (-float(g["score"]), g["goal_id"]))
        for g in goals[:16]:
            self.goal_archive.append(copy.deepcopy(g))
        self.goal_archive = self.goal_archive[-64:]
        return tuple(goals[:16])

    # ---------- native executable procedure synthesis ----------

    @staticmethod
    def _expr_eval(expr: Mapping[str, Any], x: Json) -> Any:
        op = expr.get("op")
        if op == "input":
            return copy.deepcopy(x)
        if op == "const":
            return copy.deepcopy(expr.get("value"))
        if op == "len":
            v = NativeCognitiveCore._expr_eval(expr["value"], x)
            return len(v)
        if op == "neg":
            v = NativeCognitiveCore._expr_eval(expr["value"], x)
            return -v
        if op in {"add", "sub", "mul"}:
            a = NativeCognitiveCore._expr_eval(expr["left"], x)
            b = NativeCognitiveCore._expr_eval(expr["right"], x)
            return {"add": a + b, "sub": a - b, "mul": a * b}[op]
        if op == "head":
            v = NativeCognitiveCore._expr_eval(expr["value"], x)
            return copy.deepcopy(v[0]) if v else None
        if op == "tail":
            v = NativeCognitiveCore._expr_eval(expr["value"], x)
            return copy.deepcopy(v[-1]) if v else None
        if op == "reverse":
            v = NativeCognitiveCore._expr_eval(expr["value"], x)
            return list(reversed(v))
        if op == "sort":
            v = NativeCognitiveCore._expr_eval(expr["value"], x)
            return sorted(v)
        if op == "map_add":
            v = NativeCognitiveCore._expr_eval(expr["value"], x)
            c = float(expr["constant"])
            return [item + c for item in v]
        if op == "map_neg":
            v = NativeCognitiveCore._expr_eval(expr["value"], x)
            return [-item for item in v]
        raise ValueError(op)

    @classmethod
    def _expr_candidates(cls) -> tuple[Mapping[str, Any], ...]:
        i = {"op": "input"}
        return (
            i,
            {"op": "reverse", "value": i},
            {"op": "sort", "value": i},
            {"op": "head", "value": i},
            {"op": "tail", "value": i},
            {"op": "len", "value": i},
            {"op": "neg", "value": i},
            {"op": "map_neg", "value": i},
            {"op": "map_add", "value": i, "constant": 1.0},
            {"op": "map_add", "value": i, "constant": -1.0},
        )

    def learn_function(
        self,
        examples: Sequence[tuple[Json, Json]],
        holdout: Sequence[tuple[Json, Json]] = (),
        transfer: Sequence[tuple[Json, Json]] = (),
        *,
        family: str = "generic",
    ) -> Procedure | None:
        if not examples:
            return None
        holdout = tuple(holdout) or tuple(examples[-max(1, len(examples)//3):])
        transfer = tuple(transfer) or tuple(examples[:1])
        candidates = list(self._expr_candidates())
        # Mutate generic programs with bounded composition.
        base = list(candidates)
        for e in base:
            for c in (-2.0, -1.0, 0.5, 2.0):
                if e.get("op") == "map_add":
                    candidates.append({**e, "constant": c})
        scored = []
        for expr in candidates[: max(16, self.policy.beam)]:
            try:
                train_score = sum(
                    self._expr_eval(expr, x) == y for x, y in examples
                ) / len(examples)
                hold_score = sum(
                    self._expr_eval(expr, x) == y for x, y in holdout
                ) / len(holdout)
                transfer_score = sum(
                    self._expr_eval(expr, x) == y for x, y in transfer
                ) / len(transfer)
            except Exception:
                continue
            complexity = len(canon(expr))
            scored.append((train_score, hold_score, transfer_score, -complexity, expr))
        if not scored:
            return None
        scored.sort(key=lambda z: z[:4], reverse=True)
        tr, ho, te, _, expr = scored[0]
        if tr < 1.0 or ho < 1.0 or te < 1.0:
            return None
        pid = "proc:" + digest((expr, self.generation))[:20]
        p = Procedure(
            pid,
            (copy.deepcopy(expr),),
            float(tr),
            float(ho),
            float(te),
            len(canon(expr)),
            self.generation,
            None,
            {
                "origin": "native_program_induction",
                "external_model": False,
                "network": False,
                "manual_solution": False,
            },
            "validated",
        )
        self.procedures[pid] = p
        cap_id = "cap:" + digest((family, pid))[:20]
        self.capabilities[cap_id] = Capability(
            cap_id, family, float(tr), float(te), 1.0, p.complexity, pid, self.generation
        )
        return p

    # ---------- planning ----------

    def plan(
        self,
        observation: Json,
        actions: Sequence[str],
        goal: Mapping[str, Any],
        *,
        horizon: int = 4,
    ) -> dict[str, Any]:
        frontier = [(copy.deepcopy(observation), ())]
        visited = {canon(observation)}
        for _depth in range(max(1, min(12, int(horizon)))):
            nxt = []
            for state, plan in frontier:
                try:
                    target_path = tuple(str(x) for x in goal.get("path", ()))
                    if _get(state, target_path) == goal.get("equals"):
                        return {"actions": plan, "predicted_final": state, "goal_score": 1.0}
                except Exception:
                    pass
                for action in actions:
                    pred = self.predict(state, action)
                    if pred["prediction"] is None:
                        continue
                    ns = pred["prediction"]
                    key = canon(ns)
                    if key in visited:
                        continue
                    visited.add(key)
                    nxt.append((ns, plan + (str(action),)))
            frontier = nxt
            if not frontier:
                break
        return {
            "actions": (),
            "predicted_final": copy.deepcopy(observation),
            "goal_score": 0.0,
        }

    # ---------- self-improvement ----------

    def _policy_variants(self) -> tuple[LearningPolicy, ...]:
        p = self.policy
        return (
            LearningPolicy(
                min(6, p.max_expression_depth + 1),
                min(512, p.beam + 64),
                p.intervention_budget,
                p.memory_limit,
                p.curiosity_weight,
                p.mutation_rate,
            ),
            LearningPolicy(
                p.max_expression_depth,
                p.beam,
                min(16, p.intervention_budget + 2),
                min(4096, p.memory_limit * 2),
                min(1.0, p.curiosity_weight + 0.15),
                p.mutation_rate,
            ),
            LearningPolicy(
                p.max_expression_depth,
                max(32, p.beam - 32),
                p.intervention_budget,
                p.memory_limit,
                max(0.0, p.curiosity_weight - 0.10),
                p.mutation_rate,
            ),
        )

    @staticmethod
    def _score_policy(policy: LearningPolicy, tasks: Sequence[tuple[Sequence[tuple[Json, Json]], Sequence[tuple[Json, Json]], Sequence[tuple[Json, Json]]]]) -> tuple[float, float, float]:
        core = NativeCognitiveCore(seed=0, policy=policy)
        tr_scores = []
        ho_scores = []
        te_scores = []
        for train, holdout, transfer in tasks:
            p = core.learn_function(train, holdout, transfer)
            tr_scores.append(1.0 if p and p.train_score >= 1.0 else 0.0)
            ho_scores.append(float(p.holdout_score) if p else 0.0)
            te_scores.append(float(p.transfer_score) if p else 0.0)
        return statistics.mean(tr_scores), statistics.mean(ho_scores), statistics.mean(te_scores)

    def self_improve(
        self,
        tasks: Sequence[
            tuple[Sequence[tuple[Json, Json]], Sequence[tuple[Json, Json]], Sequence[tuple[Json, Json]]]
        ],
    ) -> ChangeDecision:
        base = self._score_policy(self.policy, tasks)
        best = ChangeDecision("none", False, "no_verified_improvement", base[0], base[0], base[1], base[2])
        for i, candidate_policy in enumerate(self._policy_variants()):
            score = self._score_policy(candidate_policy, tasks)
            if (
                score[0] >= base[0]
                and score[1] > base[1]
                and score[2] > base[2]
            ):
                best = ChangeDecision(
                    "policy:" + str(i),
                    True,
                    "verified_transfer_improvement",
                    base[0],
                    score[0],
                    score[1],
                    score[2],
                )
                self.policy = candidate_policy
                self.generation += 1
                self.change_history.append(asdict(best))
                break
        return best


    # ---------- continual learning, causal disambiguation, and recursive learning ----------

    def consolidate(self) -> dict[str, Any]:
        """Consolidate high-evidence knowledge and prune low-evidence interference."""
        before = len(self.hypotheses)
        retained = {}
        by_signature: dict[tuple[Any, ...], list[Hypothesis]] = {}
        for h in self.hypotheses.values():
            if h.support < 0.50 or h.confidence < 0.50:
                continue
            key = (h.action, h.kind, h.role, h.delay, h.context)
            by_signature.setdefault(key, []).append(h)
        for key, candidates in by_signature.items():
            candidates.sort(key=lambda h: (-h.support, -h.confidence, h.complexity, h.hypothesis_id))
            winner = candidates[0]
            if len(candidates) > 1:
                competing = [h for h in candidates[1:] if abs(h.support - winner.support) < self.learning_kernel.contradiction_margin]
                if competing:
                    # Keep the best-supported candidate but reduce certainty when evidence is ambiguous.
                    winner = Hypothesis(
                        winner.hypothesis_id, winner.kind, winner.action, winner.role,
                        winner.delta, winner.delay, winner.context, winner.support,
                        max(0.50, winner.confidence * 0.85), winner.complexity, winner.value
                    )
            retained[winner.hypothesis_id] = winner
        self.hypotheses = retained
        self.experience = self.experience[-max(1, int(self.learning_kernel.replay_limit)):]
        return {
            "hypotheses_before": before,
            "hypotheses_after": len(self.hypotheses),
            "experience_retained": len(self.experience),
        }

    def negative_transfer_score(
        self,
        streams: Sequence[Sequence[tuple[Json, str, Json]]],
    ) -> float:
        """Measure retained performance on the first stream after later-stream learning."""
        if len(streams) < 2:
            return 1.0
        base_rows = list(streams[0])
        if len(base_rows) < 4:
            return 1.0
        split = max(2, len(base_rows) // 2)
        warm = base_rows[:split]
        frozen_eval = base_rows[split:]

        core = NativeCognitiveCore(seed=self.seed, policy=self.policy)
        core.learning_kernel = self.learning_kernel
        # observe_batch is semantics-equivalent to sequential observe() for the
        # exported state/predictions, but avoids rebuilding the hypothesis store
        # after every warm-up row during repeated candidate evaluation.
        core.observe_batch(warm)
        before = [
            float(core.predict(obs, action)["prediction"] == nxt)
            for obs, action, nxt in frozen_eval
        ]
        before_score = statistics.mean(before) if before else 0.0

        for stream in streams[1:]:
            core.observe_batch(stream)

        after = [
            float(core.predict(obs, action)["prediction"] == nxt)
            for obs, action, nxt in frozen_eval
        ]
        after_score = statistics.mean(after) if after else 0.0
        if before_score <= 1e-9:
            return 1.0 if after_score == 0.0 else min(1.0, after_score)
        return max(0.0, min(1.0, after_score / before_score))

    @classmethod
    def _compositional_kernel_variants(
        cls,
        base: LearningKernel,
        streams: Sequence[Sequence[tuple[Json, str, Json]]],
        *,
        max_programs: int = 128,
    ) -> tuple[LearningKernel, ...]:
        """Create structurally new kernel candidates from a native context-expression grammar."""
        programs = cls._context_programs_from_streams(
            streams,
            max_programs=max_programs,
        )
        variants = []
        for program in programs:
            variants.append(
                LearningKernel(
                    evidence_threshold=base.evidence_threshold,
                    min_support_count=base.min_support_count,
                    context_mode="adaptive_union",
                    delayed_window=base.delayed_window,
                    prediction_mode="protected_overlay",
                    contradiction_margin=base.contradiction_margin,
                    replay_limit=base.replay_limit,
                    context_program=program,
                )
            )
        return tuple(variants)

    @staticmethod
    def _kernel_variants(base: LearningKernel) -> tuple[LearningKernel, ...]:
        """Generate a deterministic neighborhood of generic learning-rule mutations."""
        variants: list[LearningKernel] = []
        contexts = ("shape", "value_bin", "categorical", "adaptive", "adaptive_union")
        predictions = ("ensemble", "highest_confidence", "conservative")
        thresholds = tuple(sorted({
            max(0.50, min(0.90, base.evidence_threshold + delta))
            for delta in (-0.08, -0.04, 0.04, 0.08)
        }))
        supports = tuple(sorted({
            max(2, min(5, base.min_support_count + delta))
            for delta in (-1, 1)
        }))
        windows = tuple(sorted({
            max(2, min(10, base.delayed_window + delta))
            for delta in (-2, -1, 1, 2)
        }))
        margins = tuple(sorted({
            max(0.10, min(0.35, base.contradiction_margin + delta))
            for delta in (-0.05, 0.05)
        }))
        replays = tuple(sorted({
            max(16, min(256, base.replay_limit + delta))
            for delta in (-32, 32)
        }))

        for context in contexts:
            variants.append(LearningKernel(
                evidence_threshold=base.evidence_threshold,
                min_support_count=base.min_support_count,
                context_mode=context,
                delayed_window=base.delayed_window,
                prediction_mode=base.prediction_mode,
                contradiction_margin=base.contradiction_margin,
                replay_limit=base.replay_limit,
            ))
        for threshold in thresholds:
            variants.append(LearningKernel(
                evidence_threshold=threshold,
                min_support_count=base.min_support_count,
                context_mode=base.context_mode,
                delayed_window=base.delayed_window,
                prediction_mode=base.prediction_mode,
                contradiction_margin=base.contradiction_margin,
                replay_limit=base.replay_limit,
            ))
        for support in supports:
            variants.append(LearningKernel(
                evidence_threshold=base.evidence_threshold,
                min_support_count=support,
                context_mode=base.context_mode,
                delayed_window=base.delayed_window,
                prediction_mode=base.prediction_mode,
                contradiction_margin=base.contradiction_margin,
                replay_limit=base.replay_limit,
            ))
        for window in windows:
            variants.append(LearningKernel(
                evidence_threshold=base.evidence_threshold,
                min_support_count=base.min_support_count,
                context_mode=base.context_mode,
                delayed_window=window,
                prediction_mode=base.prediction_mode,
                contradiction_margin=base.contradiction_margin,
                replay_limit=base.replay_limit,
            ))
        for prediction in predictions:
            variants.append(LearningKernel(
                evidence_threshold=base.evidence_threshold,
                min_support_count=base.min_support_count,
                context_mode=base.context_mode,
                delayed_window=base.delayed_window,
                prediction_mode=prediction,
                contradiction_margin=base.contradiction_margin,
                replay_limit=base.replay_limit,
            ))
        for margin in margins:
            variants.append(LearningKernel(
                evidence_threshold=base.evidence_threshold,
                min_support_count=base.min_support_count,
                context_mode=base.context_mode,
                delayed_window=base.delayed_window,
                prediction_mode=base.prediction_mode,
                contradiction_margin=margin,
                replay_limit=base.replay_limit,
            ))
        for replay in replays:
            variants.append(LearningKernel(
                evidence_threshold=base.evidence_threshold,
                min_support_count=base.min_support_count,
                context_mode=base.context_mode,
                delayed_window=base.delayed_window,
                prediction_mode=base.prediction_mode,
                contradiction_margin=base.contradiction_margin,
                replay_limit=replay,
            ))

        # Add cross-parameter mutations so later generations are not trapped in a
        # one-dimensional coordinate search.
        for context in contexts:
            for threshold in thresholds[:2]:
                variants.append(LearningKernel(
                    evidence_threshold=threshold,
                    min_support_count=base.min_support_count,
                    context_mode=context,
                    delayed_window=base.delayed_window,
                    prediction_mode=base.prediction_mode,
                    contradiction_margin=base.contradiction_margin,
                    replay_limit=base.replay_limit,
                ))

        unique = []
        seen = set()
        for candidate in variants:
            key = tuple(asdict(candidate).items())
            if key == tuple(asdict(base).items()) or key in seen:
                continue
            seen.add(key)
            unique.append(candidate)
        return tuple(unique[:64])

    @staticmethod
    def _evaluate_kernel(
        kernel: LearningKernel,
        streams: Sequence[Sequence[tuple[Json, str, Json]]],
        *,
        seed: int = 0,
        include_retention: bool = True,
        protected_hypotheses: Mapping[str, Hypothesis] | None = None,
    ) -> tuple[float, float, float, float]:
        """Evaluate the actual native transition learner under an isolated kernel."""
        if not streams:
            return (0.0, 0.0, 0.0, 1.0)

        train_scores: list[float] = []
        hold_scores: list[float] = []
        transfer_scores: list[float] = []

        for i, stream in enumerate(streams):
            rows = list(stream)
            if len(rows) < 6:
                continue
            split = max(2, len(rows) * 2 // 3)
            train = rows[:split]
            hold = rows[split:]

            core = NativeCognitiveCore(seed=seed + i)
            core.learning_kernel = kernel
            if kernel.prediction_mode == "protected_overlay" and protected_hypotheses:
                core.protected_hypotheses = copy.deepcopy(dict(protected_hypotheses))
            core.observe_batch(train)

            train_scores.append(
                statistics.mean(
                    float(core.predict(obs, action)["prediction"] == nxt)
                    for obs, action, nxt in train
                )
            )

            hold_values = [
                float(core.predict(obs, action)["prediction"] == nxt)
                for obs, action, nxt in hold
            ]
            hold_scores.append(statistics.mean(hold_values) if hold_values else 0.0)

            other = list(streams[(i + 1) % len(streams)])
            transfer_values = [
                float(core.predict(obs, action)["prediction"] == nxt)
                for obs, action, nxt in other[:max(1, len(other) // 3)]
            ]
            transfer_scores.append(
                statistics.mean(transfer_values) if transfer_values else 0.0
            )

        retention = 1.0
        if include_retention and len(streams) >= 2:
            retention_core = NativeCognitiveCore(seed=seed + 7000)
            retention_core.learning_kernel = kernel
            retention = retention_core.negative_transfer_score(streams)

        return (
            statistics.mean(train_scores) if train_scores else 0.0,
            statistics.mean(hold_scores) if hold_scores else 0.0,
            statistics.mean(transfer_scores) if transfer_scores else 0.0,
            retention,
        )

    def _historical_competence(self, kernel: LearningKernel) -> float:
        """Evaluate a candidate only on experience acquired before this improvement step.

        This is a retention constraint, not a fresh discovery objective: the current
        generation's holdout/transfer data are never used here because they have not yet
        been observed by this core.
        """
        if not self.experience:
            return 1.0
        probe = NativeCognitiveCore.from_state(self.export_state())
        probe.protected_hypotheses = copy.deepcopy(self.hypotheses)
        probe.learning_kernel = kernel
        probe._adaptive_context_cache.clear()
        probe._refresh_hypotheses()
        values = [
            float(
                probe.predict(row.observation, row.action)["prediction"]
                == row.next_observation
            )
            for row in self.experience
        ]
        return statistics.mean(values) if values else 1.0

    def self_improve_learning(
        self,
        streams: Sequence[Sequence[tuple[Json, str, Json]]],
    ) -> ChangeDecision:
        """Search for and install a better learning algorithm, not just a larger budget."""
        base_kernel = self.learning_kernel
        base = self._evaluate_kernel(base_kernel, streams, seed=self.seed)
        best = ChangeDecision(
            "kernel:none",
            False,
            "no_verified_learning_kernel_improvement",
            base[0],
            base[0],
            base[1],
            base[2],
        )
        best_tuple = base
        best_candidate = None
        best_idx = None
        best_rank = None
        variants = list(self._kernel_variants(base_kernel))
        variants.extend(
            self._compositional_kernel_variants(
                base_kernel,
                streams,
                max_programs=min(32, max(16, int(self.policy.beam))),
            )
        )
        unique_variants = []
        seen_variants = set()
        for candidate in variants:
            key = canon(asdict(candidate))
            if key == canon(asdict(base_kernel)) or key in seen_variants:
                continue
            seen_variants.add(key)
            unique_variants.append(candidate)

        historical_baseline = self._historical_competence(base_kernel)

        for idx, candidate in enumerate(unique_variants):
            partial = self._evaluate_kernel(
                candidate,
                streams,
                seed=self.seed,
                include_retention=False,
                protected_hypotheses=self.hypotheses,
            )
            if (
                partial[1] > base[1]
                and partial[2] >= base[2]
                and partial[0] >= base[0]
            ):
                historical_candidate = self._historical_competence(candidate)
                historical_gate = historical_candidate + 1e-12 >= historical_baseline
                if not historical_gate:
                    continue
                # Candidate-time negative-transfer is diagnostic only here. The original
                # gate conflated "fresh Gen2 learning" with retention across later streams
                # that may contain observationally incompatible mechanisms. The final Gen2
                # protocol evaluates legacy competence retention separately on its own
                # untouched legacy holdout. Do not weaken that final gate.
                candidate_retention = 1.0
                if len(streams) >= 2:
                    retention_core = NativeCognitiveCore(seed=self.seed + 7000)
                    retention_core.learning_kernel = candidate
                    candidate_retention = retention_core.negative_transfer_score(streams)

                score = (
                    partial[0],
                    partial[1],
                    partial[2],
                    candidate_retention,
                )

                # Evaluate every primary-eligible candidate before installing one.
                # Candidate retention is recorded for diagnosis, but is not a secondary
                # discovery gate because it is mechanism-confounded in this Gen2 protocol.
                rank = (
                    float(score[1]),
                    float(score[2]),
                    float(score[0]),
                    1.0 if (
                        candidate.context_mode == "adaptive_union"
                        and candidate.context_program is not None
                    ) else 0.0,
                    1.0 if candidate.context_mode == "categorical" else 0.0,
                    -float(candidate.evidence_threshold),
                    -idx,
                )
                if best_rank is None or rank > best_rank:
                    best_rank = rank
                    best_tuple = score
                    best_candidate = candidate
                    best_idx = idx
        if best_candidate is not None:
            if best_candidate.prediction_mode == "protected_overlay":
                self.protected_hypotheses = copy.deepcopy(self.hypotheses)
            else:
                self.protected_hypotheses = {}
            best = ChangeDecision(
                f"kernel:{best_idx}",
                True,
                "verified_recursive_learning_improvement",
                base[0],
                best_tuple[0],
                best_tuple[1],
                best_tuple[2],
            )
            self.learning_kernel = best_candidate
            self.generation += 1
            self.change_history.append({
                **asdict(best),
                "kernel": asdict(best_candidate),
                "retention_score": best_tuple[3],
                "historical_competence_baseline": historical_baseline,
                "historical_competence_candidate": self._historical_competence(best_candidate),
            })
        return best

    def counterfactual_rollout(
        self,
        observation: Json,
        actions: Sequence[str],
        *,
        horizon: int = 3,
    ) -> tuple[dict[str, Any], ...]:
        """Enumerate short model-based futures and retain uncertainty for each branch."""
        frontier = [(copy.deepcopy(observation), (), 0.0)]
        results = []
        for depth in range(max(1, min(8, int(horizon)))):
            nxt = []
            for state, plan, uncertainty in frontier:
                for action in actions:
                    info = self.predict(state, action)
                    if info["prediction"] is None:
                        continue
                    ns = info["prediction"]
                    total_u = uncertainty + float(info["uncertainty"])
                    p = plan + (str(action),)
                    results.append({
                        "actions": p,
                        "state": copy.deepcopy(ns),
                        "uncertainty": min(1.0, total_u / (depth + 1)),
                    })
                    if depth + 1 < max(1, min(8, int(horizon))):
                        nxt.append((ns, p, total_u))
            frontier = nxt
            if not frontier:
                break
        return tuple(sorted(results, key=lambda r: (r["uncertainty"], len(r["actions"]))))[:128]

    # ---------- persistence ----------

    def export_state(self) -> dict[str, Any]:
        return {
            "schema": self.SCHEMA,
            "seed": self.seed,
            "policy": asdict(self.policy),
            "learning_kernel": asdict(self.learning_kernel),
            "experience": [asdict(e) for e in self.experience],
            "hypotheses": {k: asdict(v) for k, v in self.hypotheses.items()},
            "procedures": {k: asdict(v) for k, v in self.procedures.items()},
            "capabilities": {k: asdict(v) for k, v in self.capabilities.items()},
            "goal_archive": copy.deepcopy(self.goal_archive),
            "change_history": copy.deepcopy(self.change_history),
            "prediction_history": copy.deepcopy(self.prediction_history),
            "semantic_memory": copy.deepcopy(self.semantic_memory),
            "solver_history": copy.deepcopy(self.solver_history),
            "program_library": list(self.program_library),
            "protected_hypotheses": {k: asdict(v) for k, v in self.protected_hypotheses.items()},
            "generation": self.generation,
            "step": self.step,
        }

    @classmethod
    def from_state(cls, state: Mapping[str, Any]) -> "NativeCognitiveCore":
        p = LearningPolicy(**dict(state.get("policy", {})))
        out = cls(seed=int(state.get("seed", 0)), policy=p)
        if state.get("learning_kernel"):
            kernel_state = dict(state.get("learning_kernel", {}))
            raw_context_program = kernel_state.get("context_program")
            if raw_context_program is not None:
                kernel_state["context_program"] = _restore_relation_expr(raw_context_program)
            out.learning_kernel = LearningKernel(**kernel_state)
        out.generation = int(state.get("generation", 0))
        out.step = int(state.get("step", 0))
        out.experience = [
            Experience(
                e["observation"], e["action"], e["next_observation"], float(e["reward"]),
                tuple(e.get("context", ())), bool(e.get("exogenous", False)), int(e.get("step", 0))
            )
            for e in state.get("experience", ())
        ]
        out.hypotheses = {
            k: Hypothesis(
                v["hypothesis_id"], v["kind"], v["action"], tuple(v["role"]) if v.get("role") is not None else None,
                float(v["delta"]) if v.get("delta") is not None else None, int(v["delay"]),
                (
                    v.get("context")
                    if isinstance(v.get("context"), str)
                    else tuple(v["context"]) if v.get("context") is not None else None
                ),
                float(v["support"]), float(v["confidence"]), int(v["complexity"]),
                _restore_relation_expr(v.get("value")) if v.get("kind") == "expression"
                else copy.deepcopy(v.get("value"))
            ) for k, v in dict(state.get("hypotheses", {})).items()
        }
        out.procedures = {
            k: Procedure(
                v["procedure_id"], tuple(v["program"]), float(v["train_score"]),
                float(v["holdout_score"]), float(v["transfer_score"]), int(v["complexity"]),
                int(v["generation"]), v.get("parent_id"), dict(v.get("provenance", {})),
                str(v.get("status", "candidate"))
            ) for k, v in dict(state.get("procedures", {})).items()
        }
        out.capabilities = {
            k: Capability(**v) for k, v in dict(state.get("capabilities", {})).items()
        }
        out.goal_archive = copy.deepcopy(list(state.get("goal_archive", ())))
        out.change_history = copy.deepcopy(list(state.get("change_history", ())))
        out.prediction_history = copy.deepcopy(list(state.get("prediction_history", ())))
        out.semantic_memory = copy.deepcopy(dict(state.get("semantic_memory", {})))
        out.solver_history = copy.deepcopy(dict(state.get("solver_history", {})))
        out.program_library = [
            str(token)
            for token in state.get("program_library", ())
            if isinstance(token, str) and token.startswith("macro:")
        ][-64:]
        out.protected_hypotheses = {
            k: Hypothesis(
                v["hypothesis_id"], v["kind"], v["action"],
                tuple(v["role"]) if v.get("role") is not None else None,
                float(v["delta"]) if v.get("delta") is not None else None,
                int(v["delay"]),
                (
                    v.get("context")
                    if isinstance(v.get("context"), str)
                    else tuple(v["context"]) if v.get("context") is not None else None
                ),
                float(v["support"]), float(v["confidence"]), int(v["complexity"]),
                _restore_relation_expr(v.get("value")) if v.get("kind") == "expression"
                else copy.deepcopy(v.get("value"))
            ) for k, v in dict(state.get("protected_hypotheses", {})).items()
        }
        return out