from __future__ import annotations

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
    max_alternatives: int = 8,
) -> BehavioralSearchResult | None:
    """Exact discovery-set synthesis with behavior/provenance quotienting.

    OOD is intentionally not part of this function. The caller must verify OOD
    only after a discovery-qualified candidate has been selected.
    """
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

    def lineage_ids(used_primitives: tuple[str, ...]) -> tuple[str, ...]:
        if not used_primitives:
            return ()
        lineage: set[str] = set()
        primitive_lineage = getattr(compiler, "primitive_lineage", None)
        if primitive_lineage is None:
            lineage.update(used_primitives)
        else:
            for pid in used_primitives:
                lineage.update(str(x) for x in primitive_lineage(pid))
        return tuple(sorted(lineage))


    structural_lineage_by_key: dict[str, set[str]] = {}

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
        lineage.update(lineage_ids(used(expr)))
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
        lineage.update(lineage_ids(state_used))
        return (
            -len(lineage),
            -len(lineage_ids(state_used)),
            -len(state_used),
            state_keys,
            ast_nodes(expr),
            json.dumps(expr, sort_keys=True, separators=(",", ":")),
        )

    levels: dict[
        int,
        dict[
            tuple[tuple[float, ...], tuple[str, ...], tuple[str, ...]],
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
    target_candidate_found = False
    first_exact_target_depth: int | None = None
    target_continuation_budget = 2

    def expr_key(expr: Mapping[str, Any]) -> str:
        return json.dumps(expr, sort_keys=True, separators=(",", ":"))

    def is_dominated(state: tuple) -> bool:
        return state[5] in dominated_state_tokens

    def dominates(left: tuple, right: tuple) -> bool:
        # Dominance is about the concrete executable state, not the
        # cross-representation alias family. Alias lineage is retained for
        # provenance/final selection but must never make one concrete state
        # erase an incomparable executable representative.
        left_lineage = set(lineage_ids(left[2]))
        right_lineage = set(lineage_ids(right[2]))
        return (
            right_lineage.issubset(left_lineage)
            and ast_depth(left[0]) <= ast_depth(right[0])
            and ast_nodes(left[0]) <= ast_nodes(right[0])
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
            state_key = (state[1], state[2], state[3])
            if bucket.get(state_key) is state:
                bucket.pop(state_key, None)
        vector_cache.pop(ident, None)
        free_cache.pop(ident, None)

    def register(expr: Mapping[str, Any]):
        nonlocal next_state_token, target_candidate_found
        expr = canonical(expr)
        expression_ident = expr_key(expr)
        if expression_ident in seen_expression_keys:
            stats["duplicate_states"] += 1
            stats["exact_expression_duplicates_skipped"] += 1
            return None
        seen_expression_keys.add(expression_ident)
        try:
            outputs = eval_vector(expr)
        except Exception:
            return None

        if outputs == target and (not require_macro or used(expr)):
            target_candidate_found = True
            stats["target_directed_candidates"] += 1

        state_used = used(expr)
        state_keys = free_keys(expr)
        skey = structural_key(expr)
        direct_lineage = set(lineage_ids(state_used))
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

        # Diagnostic-only telemetry: record target-matching executable lineage
        # before dominance/frontier pruning. This is never consumed by search
        # control and does not expose the target to the learner.
        if outputs == target and (not require_macro or state_used):
            stats["target_candidate_count"] += 1
            signature = "|".join(sorted(lineage_ids(state_used)))
            signatures = stats["target_candidate_lineage_signatures"]
            if signature not in signatures and len(signatures) < 512:
                signatures.append(signature)

        next_state_token += 1
        depth = ast_depth(expr)
        bucket = levels.setdefault(depth, {})
        state_key = (outputs, state_used, state_keys)
        old = bucket.get(state_key)

        stats["generated_expressions"] += 1
        if old is not None:
            stats["duplicate_states"] += 1
            if rank(state) < rank(old):
                bucket[state_key] = state
                if best_any.get(outputs) == old:
                    best_any[outputs] = state
                if state_used and best_macro.get(outputs) == old:
                    best_macro[outputs] = state
            return bucket[state_key]

        behavior_key = (outputs, state_keys)
        frontier = dominance_frontier.setdefault(behavior_key, [])

        # Exact observational equivalence on discovery rows plus identical
        # free-input interface makes two states interchangeable for all future
        # discovery-only composition. Preserve every incomparable lineage set;
        # prune only states strictly dominated by stronger lineage at no greater
        # structural cost.
        for existing in tuple(frontier):
            if is_dominated(existing):
                continue
            if dominates(existing, state):
                stats["dominance_pruned_states"] += 1
                vector_cache.pop(expression_ident, None)
                free_cache.pop(expression_ident, None)
                return existing

        retained_frontier = []
        for existing in frontier:
            if is_dominated(existing):
                continue
            if dominates(state, existing):
                dominated_state_tokens.add(existing[5])
                stats["dominance_pruned_states"] += 1
                discard_pruned_state(existing)
                continue
            retained_frontier.append(existing)

        retained_frontier.append(state)
        dominance_frontier[behavior_key] = retained_frontier
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
        stats["selected_structural_lineage_ids"] = list(
            set(structural_lineage_by_key.get(chosen[4], set()))
            | set(lineage_ids(chosen[2]))
        )
        return chosen

    best_target = choose_target_in_depth(1)
    completed = [1]
    if best_target is not None:
        stats["best_target_depth"] = 1

    def lineage_frontier(states: Sequence[tuple]) -> tuple[tuple, ...]:
        """Preserve incomparable executable lineage variants for a vector."""
        if not states:
            return ()
        if not require_macro:
            return (min(states, key=rank),)

        by_signature: dict[tuple[str, ...], tuple] = {}
        for state in states:
            signature = tuple(sorted(lineage_ids(state[2])))
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
                candidate = register(
                    {"op": "add", "left": left[0], "right": add_state[0]}
                )
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
                candidate = register(
                    {"op": "sub", "left": left[0], "right": sub_state[0]}
                )
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
                candidate = register(
                    {"op": "sub", "left": reverse_left_state[0], "right": left[0]}
                )
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
                candidate = register(
                    {"op": "mul", "left": left[0], "right": mul_state[0]}
                )
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
                    candidate = register(
                        {"op": "max", "left": left[0], "right": extreme[0]}
                    )
                    if candidate is not None and candidate[1] == target and (
                        not require_macro or candidate[2]
                    ):
                        stats["inverse_hits"] += 1
                        stats["status"] = "TARGET_CANDIDATE_FOUND"

            if not lt_mask:
                extremes = extreme_min_lookup(gt_mask)
                for extreme in extremes:
                    candidate = register(
                        {"op": "min", "left": left[0], "right": extreme[0]}
                    )
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
                        candidate = register(
                            {
                                "op": op,
                                "left": left[0],
                                "right": right[0],
                            }
                        )
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
        stats["selected_structural_lineage_ids"] = list(
            set(structural_lineage_by_key.get(best_target[4], set()))
            | set(lineage_ids(best_target[2]))
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
