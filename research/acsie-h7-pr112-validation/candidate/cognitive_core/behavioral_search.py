from __future__ import annotations
import bisect

import json
from dataclasses import dataclass
from typing import Any, Mapping, Sequence

from .recursive_cognitive_compiler import ast_depth, ast_nodes

@dataclass(frozen=True)
class BehavioralSearchResult:
    expression: Mapping[str, Any]
    used_primitives: tuple[str, ...]
    stats: Mapping[str, Any]
    alternatives: tuple[
        tuple[Mapping[str, Any], tuple[str, ...]], ...
    ] = ()

def find_exact_expression(
    compiler: Any,
    rows: Sequence[Any],
    max_depth: int,
    *,
    require_macro: bool,
    selection_objective: str = "minimize_expanded_cost",
    max_alternatives: int = 8,
) -> BehavioralSearchResult | None:
    """Exact discovery-set synthesis with behavior/provenance quotienting.

    OOD is intentionally not part of this function. The caller must verify OOD
    only after a discovery-qualified candidate has been selected.
    """
    allowed_objectives = {"minimize_expanded_cost", "maximize_direct_vocabulary"}
    if selection_objective not in allowed_objectives:
        raise ValueError(
            f"unsupported selection_objective={selection_objective!r}; "
            f"expected one of {sorted(allowed_objectives)}"
        )

    discovery_rows = tuple(rows)
    if not discovery_rows:
        return None

    target = tuple(float(row.target) for row in discovery_rows)
    primitive_ids = tuple(sorted(compiler.primitives))
    pmap = compiler._pmap()

    vector_cache: dict[str, tuple[float, ...]] = {}
    free_cache: dict[str, tuple[str, ...]] = {}
    macro_vectors: dict[str, tuple[float, ...]] = {}
    macro_free: dict[str, tuple[str, ...]] = {}
    active_macros: set[str] = set()

    stats: dict[str, Any] = {
        "schema": "ACSIE.behavioral-search.v1",
        "status": "RUNNING",
        "rows": len(discovery_rows),
        "max_depth": int(max_depth),
        "primitive_count": len(primitive_ids),
        "generated_expressions": 0,
        "unique_states": 0,
        "duplicate_states": 0,
        "inverse_hits": 0,
        "target_matches": 0,
        "max_states_in_depth": 0,
        "max_lineage_coverage": 0,
        "best_target_depth": None,
        "selected_structural_lineage_ids": [],
        "selected_executable_lineage_ids": [],
        "max_structural_alias_coverage": 0,
        "dominance_pruned_states": 0,
        "dominance_frontier_max": 0,
        "exact_expression_duplicates_skipped": 0,
        "target_directed_candidates": 0,
        "generic_binary_pairs_evaluated": 0,
        "generic_binary_path_skipped_after_target": 0,
        "target_cutoff_depth": None,
        "target_cutoff_reason": None,
        "depths_completed": [],
        "alternative_candidate_count": 0,
        "target_candidate_count": 0,
        "target_candidate_lineage_signatures": [],
    }

    def free_keys(expr: Mapping[str, Any]) -> tuple[str, ...]:
        ident = json.dumps(expr, sort_keys=True, separators=(",", ":"))
        cached = free_cache.get(ident)
        if cached is not None:
            return cached

        op = str(expr["op"])
        if op == "get":
            result = (str(expr["key"]),)
        elif op == "const":
            result = ()
        elif op == "macro":
            mid = str(expr["id"])
            if mid in active_macros or mid not in pmap:
                result = ()
            else:
                result = macro_free.get(mid)
                if result is None:
                    active_macros.add(mid)
                    result = free_keys(pmap[mid])
                    active_macros.discard(mid)
                    macro_free[mid] = result
        elif op in {"abs", "neg", "threshold"}:
            result = free_keys(expr["arg"])
        else:
            result = tuple(
                sorted(
                    set(free_keys(expr["left"]))
                    | set(free_keys(expr["right"]))
                )
            )

        free_cache[ident] = tuple(result)
        return tuple(result)

    def eval_vector(expr: Mapping[str, Any]) -> tuple[float, ...]:
        ident = json.dumps(expr, sort_keys=True, separators=(",", ":"))
        cached = vector_cache.get(ident)
        if cached is not None:
            return cached

        op = str(expr["op"])
        if op == "get":
            key = str(expr["key"])
            values = tuple(float(row.inputs[key]) for row in discovery_rows)
        elif op == "const":
            value = float(expr["value"])
            values = tuple(value for _ in discovery_rows)
        elif op == "macro":
            mid = str(expr["id"])
            if mid in active_macros:
                raise ValueError("recursive macro cycle")
            values = macro_vectors.get(mid)
            if values is None:
                if mid not in pmap:
                    raise KeyError(mid)
                active_macros.add(mid)
                values = eval_vector(pmap[mid])
                active_macros.discard(mid)
                macro_vectors[mid] = values
        elif op in {"abs", "neg", "threshold"}:
            child = eval_vector(expr["arg"])
            if op == "abs":
                values = tuple(abs(v) for v in child)
            elif op == "neg":
                values = tuple(-v for v in child)
            else:
                lo = float(expr["lo"])
                values = tuple(1.0 if v >= lo else 0.0 for v in child)
        else:
            left = eval_vector(expr["left"])
            right = eval_vector(expr["right"])
            if op == "add":
                values = tuple(a + b for a, b in zip(left, right))
            elif op == "sub":
                values = tuple(a - b for a, b in zip(left, right))
            elif op == "mul":
                values = tuple(a * b for a, b in zip(left, right))
            elif op == "max":
                values = tuple(max(a, b) for a, b in zip(left, right))
            elif op == "min":
                values = tuple(min(a, b) for a, b in zip(left, right))
            else:
                raise ValueError(op)

        vector_cache[ident] = values
        return values

    # Exact memoization scoped to one search invocation.
    # The compiler's primitive vocabulary is treated as immutable while this
    # function executes. No frontier states or candidates are removed.
    provenance_lineage_cache: dict[tuple[str, ...], tuple[str, ...]] = {}
    executable_lineage_cache: dict[tuple[str, ...], tuple[str, ...]] = {}

    def _lineage_key(used_primitives: tuple[str, ...]) -> tuple[str, ...]:
        return tuple(sorted(set(used_primitives)))

    def provenance_lineage_ids(used_primitives: tuple[str, ...]) -> tuple[str, ...]:
        key = _lineage_key(used_primitives)
        cached = provenance_lineage_cache.get(key)
        if cached is not None:
            return cached
        if not key:
            return ()

        lineage: set[str] = set()
        primitive_lineage = getattr(compiler, "primitive_lineage", None)
        if primitive_lineage is None:
            lineage.update(key)
        else:
            for pid in key:
                lineage.update(str(x) for x in primitive_lineage(pid))

        result = tuple(sorted(lineage))
        provenance_lineage_cache[key] = result
        return result

    def executable_lineage_ids(used_primitives: tuple[str, ...]) -> tuple[str, ...]:
        key = _lineage_key(used_primitives)
        cached = executable_lineage_cache.get(key)
        if cached is not None:
            return cached
        if not key:
            return ()

        lineage: set[str] = set()
        executable_lineage = getattr(compiler, "executable_lineage", None)
        if executable_lineage is None:
            lineage.update(key)
        else:
            for pid in key:
                lineage.update(str(x) for x in executable_lineage(pid))

        result = tuple(sorted(lineage))
        executable_lineage_cache[key] = result
        return result

    structural_lineage_by_key: dict[str, set[str]] = {}
    executable_cost_cache: dict[str, tuple[int, int]] = {}

    def executable_cost(expr: Mapping[str, Any], active: tuple[str, ...] = ()) -> tuple[int, int]:
        ident = json.dumps(expr, sort_keys=True, separators=(",", ":"))
        if not active and ident in executable_cost_cache:
            return executable_cost_cache[ident]
        op = str(expr["op"])
        if op in {"get", "const"}:
            result = (1, 1)
        elif op == "macro":
            mid = str(expr["id"])
            if mid in active or mid not in pmap:
                result = (1, 1)
            else:
                result = executable_cost(pmap[mid], active + (mid,))
        elif op in {"abs", "neg", "threshold"}:
            child = executable_cost(expr["arg"], active)
            result = (1 + child[0], 1 + child[1])
        else:
            left = executable_cost(expr["left"], active)
            right = executable_cost(expr["right"], active)
            result = (1 + max(left[0], right[0]), 1 + left[1] + right[1])
        if not active:
            executable_cost_cache[ident] = result
        return result

    def structural_expand(
        expr: Mapping[str, Any],
        active: tuple[str, ...] = (),
    ) -> Mapping[str, Any]:
        op = str(expr["op"])
        if op in {"get", "const"}:
            return dict(expr)
        if op == "macro":
            mid = str(expr["id"])
            if mid in active or mid not in pmap:
                return {"op": "macro", "id": mid}
            return structural_expand(pmap[mid], active + (mid,))
        if op in {"abs", "neg", "threshold"}:
            return {
                "op": op,
                **(
                    {"arg": structural_expand(expr["arg"], active)}
                    if op != "threshold"
                    else {
                        "arg": structural_expand(expr["arg"], active),
                        "lo": expr["lo"],
                    }
                ),
            }
        left = structural_expand(expr["left"], active)
        right = structural_expand(expr["right"], active)
        if op in {"add", "mul", "max", "min"}:
            left_key = json.dumps(left, sort_keys=True, separators=(",", ":"))
            right_key = json.dumps(right, sort_keys=True, separators=(",", ":"))
            if left_key > right_key:
                left, right = right, left
        return {"op": op, "left": left, "right": right}

    def structural_key(expr: Mapping[str, Any]) -> str:
        return json.dumps(
            structural_expand(expr),
            sort_keys=True,
            separators=(",", ":"),
        )

    def structural_lineage(expr: Mapping[str, Any]) -> tuple[str, ...]:
        key = structural_key(expr)
        lineage = set(structural_lineage_by_key.get(key, set()))
        lineage.update(provenance_lineage_ids(used(expr)))
        return tuple(sorted(lineage))

    def used(expr: Mapping[str, Any]) -> tuple[str, ...]:
        return tuple(
            sorted(
                {
                    str(node["id"])
                    for node in compiler._walk(expr)
                    if node.get("op") == "macro"
                }
            )
        )

    def canonical(expr: Mapping[str, Any]) -> Mapping[str, Any]:
        op = str(expr["op"])
        if op in {"add", "mul", "max", "min"}:
            left_key = json.dumps(
                expr["left"], sort_keys=True, separators=(",", ":")
            )
            right_key = json.dumps(
                expr["right"], sort_keys=True, separators=(",", ":")
            )
            if left_key > right_key:
                return {
                    "op": op,
                    "left": expr["right"],
                    "right": expr["left"],
                }
        return expr

    def rank(state: tuple) -> tuple[Any, ...]:
        expr, _, state_used, state_keys, skey, _ = state
        lineage = set(structural_lineage_by_key.get(skey, set()))
        lineage.update(executable_lineage_ids(state_used))
        expanded_depth, expanded_nodes = executable_cost(expr)
        executable_lineage = executable_lineage_ids(state_used)
        # Expanded execution cost is the primary selection objective. Direct
        # vocabulary is preserved by the Pareto frontier, not rewarded without
        # bound. Required-macro searches may prefer richer vocabulary only on an
        # equal-cost tie; optional-macro searches prefer fewer dependencies.
        # Macro count is never rewarded. A macro-required call is an
        # eligibility constraint, not an instruction to maximize vocabulary.
        # Explicit vocabulary maximization belongs in a separate experiment.
        direct_count = len(set(state_used))
        if selection_objective == "maximize_direct_vocabulary":
            # Deliberately opt-in experiment. Cost and transfer/generalization
            # consequences must be evaluated separately; require_macro alone
            # never activates this cardinality-first objective.
            objective_key = (-direct_count, expanded_depth, expanded_nodes)
        else:
            # General/default behavior prioritizes expanded execution cost and
            # does not award irrelevant macro identifiers.
            objective_key = (expanded_depth, expanded_nodes, direct_count)
        return (
            *objective_key,
            -len(executable_lineage),
            -len(lineage),
            state_keys,
            ast_nodes(expr),
            json.dumps(expr, sort_keys=True, separators=(",", ":")),
        )

    levels: dict[
        int,
        dict[
            tuple[tuple[float, ...], tuple[str, ...], tuple[str, ...], int, int],
            tuple[Mapping[str, Any], tuple[float, ...], tuple[str, ...], tuple[str, ...]],
        ],
    ] = {}
    best_any: dict[
        tuple[float, ...],
        tuple[Mapping[str, Any], tuple[float, ...], tuple[str, ...], tuple[str, ...]],
    ] = {}
    best_macro: dict[
        tuple[float, ...],
        tuple[Mapping[str, Any], tuple[float, ...], tuple[str, ...], tuple[str, ...]],
    ] = {}

    dominance_frontier: dict[
        tuple[tuple[float, ...], tuple[str, ...]],
        list[
            tuple[
                Mapping[str, Any],
                tuple[float, ...],
                tuple[str, ...],
                tuple[str, ...],
            ]
        ],
    ] = {}
    dominated_state_tokens: set[int] = set()
    seen_expression_keys: set[str] = set()
    next_state_token = 0
    expression_key_by_state_token: dict[int, str] = {}
    ast_depth_by_state_token: dict[int, int] = {}
    target_candidate_found = False
    first_exact_target_depth: int | None = None
    target_continuation_budget = 2

    def expr_key(expr: Mapping[str, Any]) -> str:
        return json.dumps(expr, sort_keys=True, separators=(",", ":"))

    def is_dominated(state: tuple) -> bool:
        return state[5] in dominated_state_tokens

    def dominates(left: tuple, right: tuple) -> bool:
        # A composite alias may have a larger transitive lineage while
        # directly reusing fewer executable primitives. It must not dominate
        # an explicit multi-parent composition whose direct macro vocabulary
        # is richer. Transitive lineage remains necessary, but direct reuse is
        # also part of executable dominance.
        left_direct = set(left[2])
        right_direct = set(right[2])
        left_lineage = set(executable_lineage_ids(left[2]))
        right_lineage = set(executable_lineage_ids(right[2]))
        left_depth, left_nodes = executable_cost(left[0])
        right_depth, right_nodes = executable_cost(right[0])
        # Required-macro acquisition intentionally preserves direct-vocabulary
        # supersets. When macros are optional, a richer direct set is not by
        # itself better, so dominance is restricted to the same direct set;
        # caller-specific ranking then decides among retained alternatives.
        direct_coverage = (
            right_direct.issubset(left_direct)
            if selection_objective == "maximize_direct_vocabulary"
            else left_direct == right_direct
        )
        return (
            direct_coverage
            and right_lineage.issubset(left_lineage)
            and left_depth <= right_depth
            and left_nodes <= right_nodes
        )

    def discard_pruned_state(state: tuple) -> None:
        """Release storage that is already logically removed by dominance.

        Dominated states are not eligible for future composition. Removing them
        from their depth bucket and expression-local caches preserves the same
        logical dominance frontier while preventing pruned objects from staying
        resident until the entire search returns.
        """
        expr, _, _, _, _, _ = state
        ident = expr_key(expr)
        depth = ast_depth(expr)
        bucket = levels.get(depth)
        if bucket is not None:
            expanded_depth, expanded_nodes = executable_cost(expr)
            state_key = (
                state[1], state[2], state[3], expanded_depth, expanded_nodes
            )
            if bucket.get(state_key) is state:
                bucket.pop(state_key, None)
        vector_cache.pop(ident, None)
        free_cache.pop(ident, None)

        # Invocation-local exact index for the dynamic register() frontier.
    # Direct and lineage postings answer superset-existence queries; direct-set
    # buckets enumerate possible subsets for the reverse (new state dominates
    # old state) query. Only active frontier states are indexed.
    _H7_online_indexes: dict[tuple, dict] = {}
    _H7_online_metadata: dict[int, dict] = {}

    def _H7_meta(state: tuple) -> dict:
        token = state[5]
        cached = _H7_online_metadata.get(token)
        if cached is not None:
            return cached
        direct = frozenset(state[2])
        lineage = frozenset(executable_lineage_ids(state[2]))
        depth, nodes = executable_cost(state[0])
        result = {
            "state": state, "direct": direct, "lineage": lineage,
            "depth": depth, "nodes": nodes, "token": token,
        }
        _H7_online_metadata[token] = result
        return result

    def _H7_new_index() -> dict:
        return {"active": {}, "by_feature": {}, "by_direct_set": {}}

    def _H7_add(index: dict, state: tuple) -> None:
        meta = _H7_meta(state)
        token = state[5]
        index["active"][token] = state
        features = index["by_feature"]
        for pid in meta["direct"]:
            features.setdefault(("D", pid), set()).add(token)
        for pid in meta["lineage"]:
            features.setdefault(("L", pid), set()).add(token)
        index["by_direct_set"].setdefault(meta["direct"], set()).add(token)

    def _H7_remove(index: dict, state: tuple) -> None:
        token = state[5]
        meta = _H7_online_metadata.get(token)
        if meta is None:
            meta = _H7_meta(state)
        index["active"].pop(token, None)
        features = index["by_feature"]
        for pid in meta["direct"]:
            key = ("D", pid)
            posting = features.get(key)
            if posting is not None:
                posting.discard(token)
                if not posting:
                    features.pop(key, None)
        for pid in meta["lineage"]:
            key = ("L", pid)
            posting = features.get(key)
            if posting is not None:
                posting.discard(token)
                if not posting:
                    features.pop(key, None)
        direct_group = index["by_direct_set"].get(meta["direct"])
        if direct_group is not None:
            direct_group.discard(token)
            if not direct_group:
                index["by_direct_set"].pop(meta["direct"], None)
        _H7_online_metadata.pop(token, None)

    def _H7_find_dominator(index: dict, state: tuple):
        if not index["active"]:
            return None
        query = _H7_meta(state)
        if selection_objective == "maximize_direct_vocabulary":
            required = (
                [("D", pid) for pid in query["direct"]]
                + [("L", pid) for pid in query["lineage"]]
            )
            if required:
                features = index["by_feature"]
                postings = sorted(
                    (features.get(key, set()) for key in required),
                    key=len,
                )
                if not postings or not postings[0]:
                    return None
                candidate_tokens = set(postings[0])
                for posting in postings[1:]:
                    if not candidate_tokens:
                        return None
                    candidate_tokens.intersection_update(posting)
            else:
                candidate_tokens = index["active"].keys()
        else:
            candidate_tokens = index["by_direct_set"].get(query["direct"], ())

        matches = []
        for token in candidate_tokens:
            existing = index["active"].get(token)
            if existing is None:
                continue
            candidate = _H7_meta(existing)
            if candidate["depth"] > query["depth"] or candidate["nodes"] > query["nodes"]:
                continue
            if selection_objective == "maximize_direct_vocabulary":
                if not query["direct"].issubset(candidate["direct"]):
                    continue
                if not query["lineage"].issubset(candidate["lineage"]):
                    continue
            elif (
                query["direct"] != candidate["direct"]
                or query["lineage"] != candidate["lineage"]
            ):
                continue
            matches.append(existing)
        # Current linear implementation returns the earliest live frontier
        # representative. Tokens are assigned monotonically at registration.
        return min(matches, key=lambda candidate: candidate[5]) if matches else None

    def _H7_find_dominated(index: dict, state: tuple):
        query = _H7_meta(state)
        qdirect = query["direct"]
        if selection_objective != "maximize_direct_vocabulary":
            candidate_tokens = set(index["by_direct_set"].get(qdirect, ()))
        elif len(qdirect) <= 8:
            ids = sorted(qdirect)
            candidate_tokens = set()
            for subset_mask in range(1 << len(ids)):
                subset = frozenset(
                    ids[bit] for bit in range(len(ids))
                    if subset_mask & (1 << bit)
                )
                candidate_tokens.update(
                    index["by_direct_set"].get(subset, ())
                )
        else:
            # Exact fallback: all empty-direct states plus any state sharing a
            # direct ID with q. A direct subset of q must be in this union.
            candidate_tokens = set(index["by_direct_set"].get(frozenset(), ()))
            features = index["by_feature"]
            for pid in qdirect:
                candidate_tokens.update(features.get(("D", pid), ()))

        matches = []
        for token in candidate_tokens:
            existing = index["active"].get(token)
            if existing is None:
                continue
            candidate = _H7_meta(existing)
            if selection_objective == "maximize_direct_vocabulary":
                if not candidate["direct"].issubset(qdirect):
                    continue
                if not candidate["lineage"].issubset(query["lineage"]):
                    continue
            elif (
                candidate["direct"] != qdirect
                or candidate["lineage"] != query["lineage"]
            ):
                continue
            if query["depth"] > candidate["depth"] or query["nodes"] > candidate["nodes"]:
                continue
            matches.append(existing)
        return sorted(matches, key=lambda candidate: candidate[5])

    def register(expr: Mapping[str, Any], output_hint: tuple[float, ...] | None = None, metadata_hint: Mapping[str, Any] | None = None):
        nonlocal next_state_token, target_candidate_found
        if metadata_hint is None:
            expr = canonical(expr)
            expression_ident = expr_key(expr)
        else:
            expression_ident = str(metadata_hint["expression_ident"])
        if expression_ident in seen_expression_keys:
            stats["duplicate_states"] += 1
            stats["exact_expression_duplicates_skipped"] += 1
            return None
        seen_expression_keys.add(expression_ident)
        try:
            outputs = tuple(output_hint) if output_hint is not None else eval_vector(expr)
            if output_hint is not None:
                vector_cache[expression_ident] = outputs
        except Exception:
            return None

        if outputs == target and (
            not require_macro
            or (tuple(metadata_hint["state_used"]) if metadata_hint is not None else used(expr))
        ):
            target_candidate_found = True
            stats["target_directed_candidates"] += 1

        if metadata_hint is None:
            state_used = used(expr)
            state_keys = free_keys(expr)
            skey = structural_key(expr)
            direct_lineage = set(executable_lineage_ids(state_used))
        else:
            state_used = tuple(metadata_hint["state_used"])
            state_keys = tuple(metadata_hint["state_keys"])
            skey = str(metadata_hint["skey"])
            direct_lineage = set(metadata_hint["direct_lineage"])
            free_cache[expression_ident] = state_keys
            executable_cost_cache[expression_ident] = (
                int(metadata_hint["expanded_depth"]),
                int(metadata_hint["expanded_nodes"]),
            )
        structural_lineage_by_key.setdefault(skey, set()).update(direct_lineage)
        stats["max_lineage_coverage"] = max(
            int(stats["max_lineage_coverage"]),
            len(direct_lineage),
        )
        stats["max_structural_alias_coverage"] = max(
            int(stats["max_structural_alias_coverage"]),
            len(structural_lineage_by_key[skey]),
        )

        state = (
            expr,
            outputs,
            state_used,
            state_keys,
            skey,
            next_state_token,
        )
        expression_key_by_state_token[next_state_token] = expression_ident

        # Diagnostic-only telemetry: record target-matching executable lineage
        # before dominance/frontier pruning. This is never consumed by search
        # control and does not expose the target to the learner.
        if outputs == target and (not require_macro or state_used):
            stats["target_candidate_count"] += 1
            signature = "|".join(sorted(executable_lineage_ids(state_used)))
            signatures = stats["target_candidate_lineage_signatures"]
            if signature not in signatures and len(signatures) < 512:
                signatures.append(signature)

        next_state_token += 1
        if metadata_hint is None:
            depth = ast_depth(expr)
        else:
            depth = int(metadata_hint["ast_depth"])
        ast_depth_by_state_token[state[5]] = depth
        expanded_depth, expanded_nodes = executable_cost(expr)
        bucket = levels.setdefault(depth, {})
        state_key = (
            outputs, state_used, state_keys, expanded_depth, expanded_nodes
        )
        old = bucket.get(state_key)

        stats["generated_expressions"] += 1
        if old is not None:
            stats["duplicate_states"] += 1
            # The bucket key now means equal observations, direct vocabulary,
            # free-input interface, AST depth, and both expanded-cost measures.
            # Keep the existing representative. Replacing it by rank alone
            # would leave the dominance frontier and online index pointing at a
            # different active identity. Different cost profiles get separate
            # bucket entries and are compared by the Pareto dominance oracle.
            expression_key_by_state_token.pop(state[5], None)
            ast_depth_by_state_token.pop(state[5], None)
            vector_cache.pop(expression_ident, None)
            free_cache.pop(expression_ident, None)
            executable_cost_cache.pop(expression_ident, None)
            _H7_online_metadata.pop(state[5], None)
            return old

        behavior_key = (outputs, state_keys)
        frontier = dominance_frontier.setdefault(behavior_key, [])
        frontier_index = _H7_online_indexes.get(behavior_key)
        if frontier_index is None:
            frontier_index = _H7_new_index()
            _H7_online_indexes[behavior_key] = frontier_index

        existing = _H7_find_dominator(frontier_index, state)
        if existing is not None:
            if not dominates(existing, state):
                raise AssertionError("indexed dominator query returned a false positive")
            stats["dominance_pruned_states"] += 1
            vector_cache.pop(expression_ident, None)
            free_cache.pop(expression_ident, None)
            _H7_online_metadata.pop(state[5], None)
            return existing

        dominated_existing = _H7_find_dominated(frontier_index, state)
        for existing in dominated_existing:
            dominated_state_tokens.add(existing[5])
            stats["dominance_pruned_states"] += 1
            _H7_remove(frontier_index, existing)
            discard_pruned_state(existing)

        if dominated_existing:
            retained_frontier = [
                existing for existing in frontier if not is_dominated(existing)
            ]
            retained_frontier.append(state)
            dominance_frontier[behavior_key] = retained_frontier
        else:
            # Avoid copying/scanning the whole frontier when this state adds a
            # new incomparable element. This is the lazy append path.
            frontier.append(state)
            retained_frontier = frontier
            dominance_frontier[behavior_key] = frontier

        _H7_add(frontier_index, state)
        stats["dominance_frontier_max"] = max(
            int(stats["dominance_frontier_max"]),
            len(retained_frontier),
        )

        bucket[state_key] = state
        stats["unique_states"] += 1
        stats["max_states_in_depth"] = max(
            int(stats["max_states_in_depth"]),
            len(bucket),
        )

        old_any = best_any.get(outputs)
        if old_any is None or is_dominated(old_any) or rank(state) < rank(old_any):
            best_any[outputs] = state
        if state_used:
            old_macro = best_macro.get(outputs)
            if (
                old_macro is None
                or is_dominated(old_macro)
                or rank(state) < rank(old_macro)
            ):
                best_macro[outputs] = state
        return state

    def register_binary(op: str, left: tuple, right: tuple):
        """Preflight exact binary dominance before constructing an AST dict."""
        nonlocal next_state_token, target_candidate_found

        # The operands are already stored canonical expressions. Compose their
        # canonical JSON keys without allocating the parent AST.
        left_key = expression_key_by_state_token.get(left[5])
        if left_key is None:
            left_key = expr_key(left[0])
        right_key = expression_key_by_state_token.get(right[5])
        if right_key is None:
            right_key = expr_key(right[0])
        commutative = op in {"add", "mul", "max", "min"}
        canonical_swap = commutative and left_key > right_key
        if canonical_swap:
            left_key, right_key = right_key, left_key
        expression_ident = (
            '{"left":' + left_key + ',"op":' + json.dumps(op)
            + ',"right":' + right_key + '}'
        )
        if expression_ident in seen_expression_keys:
            stats["duplicate_states"] += 1
            stats["exact_expression_duplicates_skipped"] += 1
            return None

        lvec, rvec = left[1], right[1]
        if op == "add":
            outputs = tuple(a + b for a, b in zip(lvec, rvec))
        elif op == "sub":
            outputs = tuple(a - b for a, b in zip(lvec, rvec))
        elif op == "mul":
            outputs = tuple(a * b for a, b in zip(lvec, rvec))
        elif op == "max":
            outputs = tuple(max(a, b) for a, b in zip(lvec, rvec))
        elif op == "min":
            outputs = tuple(min(a, b) for a, b in zip(lvec, rvec))
        else:
            raise ValueError(op)

        state_used = tuple(sorted(set(left[2]) | set(right[2])))
        state_keys = tuple(sorted(set(left[3]) | set(right[3])))

        left_skey, right_skey = left[4], right[4]
        if commutative and left_skey > right_skey:
            left_skey, right_skey = right_skey, left_skey
        skey = (
            '{"left":' + left_skey + ',"op":' + json.dumps(op)
            + ',"right":' + right_skey + '}'
        )

        left_ast_depth = ast_depth_by_state_token.get(left[5])
        if left_ast_depth is None:
            left_ast_depth = ast_depth(left[0])
        right_ast_depth = ast_depth_by_state_token.get(right[5])
        if right_ast_depth is None:
            right_ast_depth = ast_depth(right[0])
        depth = 1 + max(left_ast_depth, right_ast_depth)
        # Compute expanded execution metadata from child metadata only.
        left_meta = _H7_online_metadata.get(left[5])
        if left_meta is None:
            left_lineage = frozenset(executable_lineage_ids(left[2]))
            left_exec_depth, left_exec_nodes = executable_cost(left[0])
        else:
            left_lineage = left_meta["lineage"]
            left_exec_depth, left_exec_nodes = left_meta["depth"], left_meta["nodes"]
        right_meta = _H7_online_metadata.get(right[5])
        if right_meta is None:
            right_lineage = frozenset(executable_lineage_ids(right[2]))
            right_exec_depth, right_exec_nodes = executable_cost(right[0])
        else:
            right_lineage = right_meta["lineage"]
            right_exec_depth, right_exec_nodes = right_meta["depth"], right_meta["nodes"]
        expanded_depth = 1 + max(left_exec_depth, right_exec_depth)
        expanded_nodes = 1 + left_exec_nodes + right_exec_nodes
        direct_lineage = frozenset(executable_lineage_ids(state_used))

        state_key = (
            outputs, state_used, state_keys, expanded_depth, expanded_nodes
        )
        old_bucket = levels.get(depth, {})
        if old_bucket.get(state_key) is not None:
            # Same semantic/cost bucket: ordinary register() keeps the current
            # canonical representative and records exact duplicate accounting.
            return register(
                {"op": op, "left": left[0], "right": right[0]},
                output_hint=outputs,
            )

        behavior_key = (outputs, state_keys)
        frontier_index = _H7_online_indexes.get(behavior_key)
        existing = None
        if frontier_index is not None and frontier_index["active"]:
            if selection_objective == "maximize_direct_vocabulary":
                required = (
                    [("D", pid) for pid in state_used]
                    + [("L", pid) for pid in direct_lineage]
                )
                features = frontier_index["by_feature"]
                if required:
                    postings = sorted(
                        (features.get(key, set()) for key in required),
                        key=len,
                    )
                    if postings and postings[0]:
                        possible = set(postings[0])
                        for posting in postings[1:]:
                            if not possible:
                                break
                            possible.intersection_update(posting)
                    else:
                        possible = set()
                else:
                    possible = set(frontier_index["active"])
            else:
                possible = set(
                    frontier_index["by_direct_set"].get(frozenset(state_used), ())
                )
            dominators = []
            for token in possible:
                old_state = frontier_index["active"].get(token)
                if old_state is None:
                    continue
                meta = _H7_meta(old_state)
                if meta["depth"] > expanded_depth or meta["nodes"] > expanded_nodes:
                    continue
                if selection_objective == "maximize_direct_vocabulary":
                    if not frozenset(state_used).issubset(meta["direct"]):
                        continue
                    if not direct_lineage.issubset(meta["lineage"]):
                        continue
                elif (
                    meta["direct"] != frozenset(state_used)
                    or meta["lineage"] != direct_lineage
                ):
                    continue
                dominators.append(old_state)
            if dominators:
                existing = min(dominators, key=lambda state: state[5])

        if existing is None:
            # Materialize one canonical parent AST, then hand off preflight
            # metadata to avoid recomputing the same values in register().
            parent_left, parent_right = (
                (right[0], left[0]) if canonical_swap else (left[0], right[0])
            )
            parent_expr = {"op": op, "left": parent_left, "right": parent_right}
            metadata_hint = {
                "expression_ident": expression_ident,
                "state_used": state_used,
                "state_keys": state_keys,
                "skey": skey,
                "direct_lineage": direct_lineage,
                "ast_depth": depth,
                "expanded_depth": expanded_depth,
                "expanded_nodes": expanded_nodes,
            }
            return register(
                parent_expr,
                output_hint=outputs,
                metadata_hint=metadata_hint,
            )

        # Exact preflight hit: reproduce register's semantic bookkeeping while
        # skipping parent-AST allocation and recursive vector evaluation.
        seen_expression_keys.add(expression_ident)
        if outputs == target and (not require_macro or state_used):
            target_candidate_found = True
            stats["target_directed_candidates"] += 1

        structural_lineage_by_key.setdefault(skey, set()).update(direct_lineage)
        stats["max_lineage_coverage"] = max(
            int(stats["max_lineage_coverage"]), len(direct_lineage)
        )
        stats["max_structural_alias_coverage"] = max(
            int(stats["max_structural_alias_coverage"]),
            len(structural_lineage_by_key[skey]),
        )
        if outputs == target and (not require_macro or state_used):
            stats["target_candidate_count"] += 1
            signature = "|".join(sorted(direct_lineage))
            signatures = stats["target_candidate_lineage_signatures"]
            if signature not in signatures and len(signatures) < 512:
                signatures.append(signature)
        next_state_token += 1
        stats["generated_expressions"] += 1
        stats["dominance_pruned_states"] += 1
        # Nothing was inserted in vector_cache/free_cache, so no cache
        # invalidation is required. The returned object is the same retained
        # state that ordinary register() would have returned.
        return existing

    key_names = tuple(
        sorted(
            set().union(*(row.inputs.keys() for row in discovery_rows))
        )
    )
    atoms = tuple(compiler._expr_atoms(key_names, primitive_ids))
    atoms = tuple(
        sorted(
            atoms,
            key=lambda x: (
                0 if x.get("op") == "macro" else 1,
                json.dumps(x, sort_keys=True, separators=(",", ":")),
            ),
        )
    )
    for atom in atoms:
        register(atom)

    def choose_target_in_depth(depth: int) -> tuple | None:
        candidates = []
        for state in levels.get(depth, {}).values():
            if is_dominated(state):
                continue
            if state[1] != target:
                continue
            if require_macro and not state[2]:
                continue
            candidates.append(state)
        if not candidates:
            return None
        stats["target_matches"] += len(candidates)
        chosen = min(candidates, key=rank)
        stats["selected_structural_lineage_ids"] = sorted(
            set(structural_lineage_by_key.get(chosen[4], set()))
            | set(provenance_lineage_ids(chosen[2]))
        )
        stats["selected_executable_lineage_ids"] = list(
            executable_lineage_ids(chosen[2])
        )
        return chosen

    best_target = choose_target_in_depth(1)
    completed = [1]
    if best_target is not None:
        stats["best_target_depth"] = 1

    def lineage_frontier(states: Sequence[tuple]) -> tuple[tuple, ...]:
        """Preserve distinct direct vocabularies and executable lineage variants."""
        if not states:
            return ()
        # Direct executable vocabulary is distinct from its transitive closure.
        # Preserve incomparable direct macro sets even when they have the same
        # executable-lineage signature; the dominance predicate below may prune
        # a state only when the direct-set and executable-lineage supersets hold.
        by_signature: dict[tuple[tuple[str, ...], tuple[str, ...]], tuple] = {}
        for state in states:
            direct_vocabulary = tuple(sorted(set(state[2])))
            executable_signature = tuple(sorted(executable_lineage_ids(state[2])))
            signature = (direct_vocabulary, executable_signature)
            old = by_signature.get(signature)
            if old is None or rank(state) < rank(old):
                by_signature[signature] = state

        candidates = tuple(by_signature.values())
        frontier = []
        for state in candidates:
            if any(
                other is not state and dominates(other, state)
                for other in candidates
            ):
                continue
            frontier.append(state)
        return tuple(sorted(frontier, key=rank))

    def build_extreme_lookup(
        states: Sequence[tuple],
        *,
        want_max: bool,
    ):
        """Index prior states for exact max/min target reconstruction.

        The previous implementation materialized every 2^N equality mask and
        propagated lineage frontiers through all masks. With 14 discovery rows,
        that creates 16,384 buckets per depth for every search. The lookup below
        preserves the same componentwise admissibility relation but indexes only
        actually observed states and answers queries by bitset intersection.
        """
        dimension = len(target)
        valid_states: list[tuple] = []
        equal_index: list[set[int]] = [set() for _ in range(dimension)]

        for state in states:
            vector = state[1]
            eq_mask = 0
            invalid = False
            for idx, (value, wanted) in enumerate(zip(vector, target)):
                if want_max:
                    if value > wanted:
                        invalid = True
                        break
                else:
                    if value < wanted:
                        invalid = True
                        break
                if value == wanted:
                    eq_mask |= 1 << idx
            if invalid:
                continue
            state_index = len(valid_states)
            valid_states.append(state)
            for idx in range(dimension):
                if eq_mask & (1 << idx):
                    equal_index[idx].add(state_index)

        all_indices = set(range(len(valid_states)))

        def lookup(required_equal_mask: int) -> tuple[tuple, ...]:
            candidates = all_indices
            for idx in range(dimension):
                if required_equal_mask & (1 << idx):
                    candidates = candidates & equal_index[idx]
                    if not candidates:
                        return ()
            return lineage_frontier(
                [valid_states[i] for i in sorted(candidates)]
            )

        return lookup

    def mul_target_right(
        left_vector: tuple[float, ...],
        states: Sequence[tuple],
    ) -> tuple[tuple, ...]:
        """Find all right operands satisfying target == left * right."""
        matches = []
        for state in states:
            right_vector = state[1]
            ok = True
            for left_value, right_value, wanted in zip(
                left_vector, right_vector, target
            ):
                if left_value == 0.0:
                    if wanted != 0.0:
                        ok = False
                        break
                elif right_value != wanted / left_value:
                    ok = False
                    break
            if ok:
                matches.append(state)
        return lineage_frontier(matches)

    for depth in range(2, int(max_depth) + 1):
        target_candidate_found = False
        previous = tuple(
            state
            for state in levels.get(depth - 1, {}).values()
            if not is_dominated(state)
        )
        if not previous:
            break

        # Unary closure.
        for state in previous:
            for op in ("abs", "neg"):
                candidate = register({"op": op, "arg": state[0]})
                if candidate is not None and candidate[1] == target and (
                    not require_macro or candidate[2]
                ):
                    stats["inverse_hits"] += 1
                    stats["status"] = "TARGET_CANDIDATE_FOUND"

        prior = [
            state
            for earlier_depth in range(1, depth)
            for state in levels.get(earlier_depth, {}).values()
            if not is_dominated(state)
        ]

        # Target-directed exact reconstruction for every operator in the
        # executable search algebra. This preserves genericity while avoiding
        # enumeration of binary pairs that cannot possibly equal the target.
        # Add/sub use exact vector inversion; mul is zero-safe; max/min use a
        # componentwise relation index.
        prior_states_by_vector: dict[tuple[float, ...], list[tuple]] = {}
        for state in prior:
            prior_states_by_vector.setdefault(state[1], []).append(state)

        prior_by_vector: dict[tuple[float, ...], tuple[tuple, ...]] = {
            vector: lineage_frontier(states)
            for vector, states in prior_states_by_vector.items()
        }
        prior_macro_by_vector: dict[tuple[float, ...], tuple[tuple, ...]] = {
            vector: lineage_frontier([state for state in states if state[2]])
            for vector, states in prior_states_by_vector.items()
        }

        right_candidates = tuple(
            state
            for states in (
                prior_macro_by_vector.values()
                if require_macro
                else prior_by_vector.values()
            )
            for state in states
        )
        extreme_max_lookup = build_extreme_lookup(
            right_candidates,
            want_max=True,
        )
        extreme_min_lookup = build_extreme_lookup(
            right_candidates,
            want_max=False,
        )

        for left in previous:
            lvec = left[1]

            add_right = tuple(t - l for t, l in zip(target, lvec))
            add_states = (
                prior_macro_by_vector.get(add_right, ())
                if require_macro
                else prior_by_vector.get(add_right, ())
            )
            for add_state in add_states:
                candidate = register_binary("add", left, add_state)
                if candidate is not None and candidate[1] == target and (
                    not require_macro or candidate[2]
                ):
                    stats["inverse_hits"] += 1
                    stats["status"] = "TARGET_CANDIDATE_FOUND"

            sub_right = tuple(l - t for l, t in zip(lvec, target))
            sub_states = (
                prior_macro_by_vector.get(sub_right, ())
                if require_macro
                else prior_by_vector.get(sub_right, ())
            )
            for sub_state in sub_states:
                candidate = register_binary("sub", left, sub_state)
                if candidate is not None and candidate[1] == target and (
                    not require_macro or candidate[2]
                ):
                    stats["inverse_hits"] += 1
                    stats["status"] = "TARGET_CANDIDATE_FOUND"

            # Subtraction is non-commutative. Also test the orientation in
            # which the depth-(d-1) operand is the right child.
            reverse_left = tuple(t + l for t, l in zip(target, lvec))
            reverse_left_states = (
                prior_macro_by_vector.get(reverse_left, ())
                if require_macro
                else prior_by_vector.get(reverse_left, ())
            )
            for reverse_left_state in reverse_left_states:
                candidate = register_binary("sub", reverse_left_state, left)
                if candidate is not None and candidate[1] == target and (
                    not require_macro or candidate[2]
                ):
                    stats["inverse_hits"] += 1
                    stats["status"] = "TARGET_CANDIDATE_FOUND"

            if all(value != 0.0 for value in lvec):
                needed = tuple(t / l for t, l in zip(target, lvec))
                mul_states = (
                    prior_macro_by_vector.get(needed, ())
                    if require_macro
                    else prior_by_vector.get(needed, ())
                )
            else:
                mul_states = mul_target_right(lvec, right_candidates)
            for mul_state in mul_states:
                candidate = register_binary("mul", left, mul_state)
                if candidate is not None and candidate[1] == target and (
                    not require_macro or candidate[2]
                ):
                    stats["inverse_hits"] += 1
                    stats["status"] = "TARGET_CANDIDATE_FOUND"

            lt_mask = gt_mask = 0
            for idx, (value, wanted) in enumerate(zip(lvec, target)):
                bit = 1 << idx
                if value < wanted:
                    lt_mask |= bit
                elif value > wanted:
                    gt_mask |= bit

            if not gt_mask:
                extremes = extreme_max_lookup(lt_mask)
                for extreme in extremes:
                    candidate = register_binary("max", left, extreme)
                    if candidate is not None and candidate[1] == target and (
                        not require_macro or candidate[2]
                    ):
                        stats["inverse_hits"] += 1
                        stats["status"] = "TARGET_CANDIDATE_FOUND"

            if not lt_mask:
                extremes = extreme_min_lookup(gt_mask)
                for extreme in extremes:
                    candidate = register_binary("min", left, extreme)
                    if candidate is not None and candidate[1] == target and (
                        not require_macro or candidate[2]
                    ):
                        stats["inverse_hits"] += 1
                        stats["status"] = "TARGET_CANDIDATE_FOUND"

        if not target_candidate_found and not (
            require_macro and first_exact_target_depth is not None
        ):
            # Generic completeness remains available before any exact target is
            # found. Once an exact macro target has been found, later continuation
            # depths stay target-directed only; otherwise a missed exact
            # decomposition would fall back into the old explosive cross-product.
            for left in previous:
                for right in prior:
                    stats["generic_binary_pairs_evaluated"] += 1
                    for op in ("add", "sub", "mul", "max", "min"):
                        if op in {"add", "mul", "max", "min"}:
                            lk = json.dumps(left[0], sort_keys=True, separators=(",", ":"))
                            rk = json.dumps(right[0], sort_keys=True, separators=(",", ":"))
                            if lk > rk:
                                continue
                        candidate = register_binary(op, left, right)
                        if candidate is None:
                            continue
                        if candidate[1] != target:
                            continue
                        if require_macro and not candidate[2]:
                            continue
                        stats["status"] = "TARGET_CANDIDATE_FOUND"
                        target_candidate_found = True
        else:
            stats["generic_binary_path_skipped_after_target"] += 1

        completed.append(depth)
        best_at_depth = choose_target_in_depth(depth)
        if best_at_depth is not None:
            if first_exact_target_depth is None and require_macro:
                first_exact_target_depth = depth
                stats["first_exact_target_depth"] = depth
            if best_target is None or rank(best_at_depth) < rank(best_target):
                best_target = best_at_depth
                stats["best_target_depth"] = depth
            stats["status"] = "TARGET_CANDIDATE_FOUND"

            # Once an exact discovery-qualified target is represented using
            # retained executable vocabulary at depth 2 or shallower, further
            # generic enumeration cannot improve the benchmark's mandatory
            # reuse condition without introducing deeper neutral aliases. The
            # search therefore stops the explosive global cross-product here.
            # Depth-1 targets are deliberately allowed to reach depth 2 so the
            # global-frontier regression can still replace a shallow alias with
            # a stronger multi-primitive composition.
            if require_macro and first_exact_target_depth is not None:
                if depth >= first_exact_target_depth + target_continuation_budget:
                    stats["target_cutoff_depth"] = depth
                    stats["target_cutoff_reason"] = (
                        "bounded_exact_macro_target_continuation"
                    )
                    break

    if best_target is not None:
        stats["status"] = "TARGET_FOUND"
        stats["depths_completed"] = completed
        stats["selected_structural_lineage_ids"] = sorted(
            set(structural_lineage_by_key.get(best_target[4], set()))
            | set(provenance_lineage_ids(best_target[2]))
        )
        stats["selected_executable_lineage_ids"] = list(
            executable_lineage_ids(best_target[2])
        )
        target_states: list[tuple] = []
        for target_depth in completed:
            for state in levels.get(target_depth, {}).values():
                if is_dominated(state):
                    continue
                if state[1] != target:
                    continue
                if require_macro and not state[2]:
                    continue
                target_states.append(state)
        target_frontier = lineage_frontier(target_states)
        target_frontier = target_frontier[: max(1, int(max_alternatives))]
        alternatives = tuple((state[0], state[2]) for state in target_frontier)
        stats["alternative_candidate_count"] = len(alternatives)
        return BehavioralSearchResult(
            best_target[0],
            best_target[2],
            stats,
            alternatives,
        )

    stats["status"] = "TARGET_NOT_FOUND"
    stats["depths_completed"] = completed
    return None