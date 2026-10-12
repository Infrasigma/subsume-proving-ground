#!/usr/bin/env python3
"""Prototype exact executable-AST key interning on the pinned macro-cache snapshot.

This only patches an isolated proving-ground worktree. It keeps exact macro IDs
and exact AST identity separate from expanded structural IDs. Commutative-key
canonicalization, selected-rank JSON tie-breaks, and the existing source files
remain unchanged outside this diagnostic patch.
"""
from __future__ import annotations

import argparse
import hashlib
from pathlib import Path

EXPECTED_BASE_SHA256 = "7d9f1d9767996705bc4edbeedbfb46a78b3b1c3b205ee1ecf1a2f6e1de79aa12"
SOURCE_COMMIT = "d83cb60286f195eec02b4d7aa45a60539316d115"


def replace_exact(source: str, old: str, new: str, *, count: int = 1, label: str) -> str:
    observed = source.count(old)
    if observed != count:
        raise SystemExit(f"{label}: expected {count} marker(s), found {observed}: {old[:100]!r}")
    return source.replace(old, new, count)


EXACT_EXPR_KEY_BLOCK = '''    # Exact executable AST IDs are a separate namespace from expanded structural IDs.
    # The bounded identity cache accelerates repeated visits to immutable/shared AST
    # objects; equality itself is decided by collision-free structural signatures.
    from collections import OrderedDict as _H7_OrderedDict
    _H7_EXACT_EXPR_ID_INTERN = {}
    _H7_EXACT_EXPR_ID_NEXT = 1
    _H7_EXACT_EXPR_ID_OBJECT_CACHE = _H7_OrderedDict()
    _H7_EXACT_EXPR_ID_OBJECT_CACHE_CAPACITY = 8192
    _H7_EXACT_EXPR_ID_STATS = {
        "representation": "exact_executable_ast_interned_integer_ids",
        "exact_expr_id_calls": 0,
        "object_cache_hits": 0,
        "object_cache_misses": 0,
        "intern_calls": 0,
        "intern_hits": 0,
        "unique_expr_ids": 0,
        "payload_json_serializations": 0,
        "binary_keys_composed_from_child_ids": 0,
        "object_cache_capacity": _H7_EXACT_EXPR_ID_OBJECT_CACHE_CAPACITY,
    }
    _H7_EXACT_BINARY_OPS = {"add", "mul", "max", "min"}
    _H7_EXACT_BINARY_PAYLOADS = {
        op: json.dumps({"op": op}, sort_keys=True, separators=(",", ":"))
        for op in ("add", "sub", "mul", "max", "min")
    }

    def _H7_intern_exact_expr_signature(signature: tuple[Any, ...]) -> int:
        nonlocal _H7_EXACT_EXPR_ID_NEXT
        _H7_EXACT_EXPR_ID_STATS["intern_calls"] += 1
        old_id = _H7_EXACT_EXPR_ID_INTERN.get(signature)
        if old_id is not None:
            _H7_EXACT_EXPR_ID_STATS["intern_hits"] += 1
            return old_id
        new_id = _H7_EXACT_EXPR_ID_NEXT
        _H7_EXACT_EXPR_ID_NEXT += 1
        _H7_EXACT_EXPR_ID_INTERN[signature] = new_id
        _H7_EXACT_EXPR_ID_STATS["unique_expr_ids"] += 1
        return new_id

    def _H7_exact_expr_binary_id(op: str, left_id: int, right_id: int) -> int:
        if op in _H7_EXACT_BINARY_OPS and left_id > right_id:
            left_id, right_id = right_id, left_id
        _H7_EXACT_EXPR_ID_STATS["binary_keys_composed_from_child_ids"] += 1
        payload = _H7_EXACT_BINARY_PAYLOADS.get(op)
        if payload is None:
            payload = json.dumps({"op": op}, sort_keys=True, separators=(",", ":"))
            _H7_EXACT_EXPR_ID_STATS["payload_json_serializations"] += 1
        return _H7_intern_exact_expr_signature(("binary", payload, left_id, right_id))

    def _H7_exact_expr_id(expr: Mapping[str, Any]) -> int:
        _H7_EXACT_EXPR_ID_STATS["exact_expr_id_calls"] += 1
        object_key = id(expr)
        cached_object = _H7_EXACT_EXPR_ID_OBJECT_CACHE.get(object_key)
        if cached_object is not None and cached_object[0] is expr:
            _H7_EXACT_EXPR_ID_STATS["object_cache_hits"] += 1
            _H7_EXACT_EXPR_ID_OBJECT_CACHE.move_to_end(object_key)
            return cached_object[1]
        _H7_EXACT_EXPR_ID_STATS["object_cache_misses"] += 1

        op = str(expr["op"])
        if "left" in expr and "right" in expr:
            left_id = _H7_exact_expr_id(expr["left"])
            right_id = _H7_exact_expr_id(expr["right"])
            if set(expr.keys()) == {"op", "left", "right"}:
                result = _H7_exact_expr_binary_id(op, left_id, right_id)
            else:
                payload_fields = {
                    key: value for key, value in expr.items()
                    if key not in {"left", "right"}
                }
                payload = json.dumps(payload_fields, sort_keys=True, separators=(",", ":"))
                _H7_EXACT_EXPR_ID_STATS["payload_json_serializations"] += 1
                if op in _H7_EXACT_BINARY_OPS and left_id > right_id:
                    left_id, right_id = right_id, left_id
                result = _H7_intern_exact_expr_signature(
                    ("binary", payload, left_id, right_id)
                )
        elif "arg" in expr:
            child_id = _H7_exact_expr_id(expr["arg"])
            payload_fields = {key: value for key, value in expr.items() if key != "arg"}
            payload = json.dumps(payload_fields, sort_keys=True, separators=(",", ":"))
            _H7_EXACT_EXPR_ID_STATS["payload_json_serializations"] += 1
            result = _H7_intern_exact_expr_signature(("unary", payload, child_id))
        else:
            # Leaves, including {"op":"macro","id":...}, retain exact node fields.
            payload = json.dumps(dict(expr), sort_keys=True, separators=(",", ":"))
            _H7_EXACT_EXPR_ID_STATS["payload_json_serializations"] += 1
            result = _H7_intern_exact_expr_signature(("leaf", payload))

        _H7_EXACT_EXPR_ID_OBJECT_CACHE[object_key] = (expr, result)
        _H7_EXACT_EXPR_ID_OBJECT_CACHE.move_to_end(object_key)
        if len(_H7_EXACT_EXPR_ID_OBJECT_CACHE) > _H7_EXACT_EXPR_ID_OBJECT_CACHE_CAPACITY:
            _H7_EXACT_EXPR_ID_OBJECT_CACHE.popitem(last=False)
        return result

'''

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("source", type=Path)
    args = ap.parse_args()
    path = args.source
    source = path.read_text(encoding="utf-8")
    source_sha = hashlib.sha256(source.encode()).hexdigest()
    if source_sha != EXPECTED_BASE_SHA256:
        raise SystemExit(
            f"pinned macro-cache diagnostic SHA mismatch: expected {EXPECTED_BASE_SHA256}, got {source_sha}"
        )
    if "_H7_EXACT_EXPR_ID_INTERN" in source:
        raise SystemExit("exact-expression-ID patch is already present")

    source = replace_exact(
        source,
        "    active_macros: set[str] = set()\n",
        "    active_macros: set[str] = set()\n\n" + EXACT_EXPR_KEY_BLOCK,
        label="install exact executable-AST ID interner",
    )

    # Cache keys in these three routines are exact executable-AST identities,
    # not expanded structural identities.
    source = replace_exact(
        source,
        "vector_cache: dict[str, tuple[float, ...]] = {}",
        "vector_cache: dict[int, tuple[float, ...]] = {}",
        label="vector cache key type",
    )
    source = replace_exact(
        source,
        "free_cache: dict[str, tuple[str, ...]] = {}",
        "free_cache: dict[int, tuple[str, ...]] = {}",
        label="free-key cache key type",
    )
    source = replace_exact(
        source,
        "executable_cost_cache: dict[str, tuple[int, int]] = {}",
        "executable_cost_cache: dict[int, tuple[int, int]] = {}",
        label="execution-cost cache key type",
    )
    source = replace_exact(
        source,
        '        ident = json.dumps(expr, sort_keys=True, separators=(",", ":"))\n',
        "        ident = _H7_exact_expr_id(expr)\n",
        count=3,
        label="switch free/vector/execution-cost cache lookups to exact AST IDs",
    )

    seen_type_block = '''    class _H7_ExactSeenExpressionKeys:
        """Integer-only exact-key membership with lazy JSON-key test compatibility."""
        def __init__(self):
            self._ids = set()

        def _id_for_json_key(self, value):
            if not isinstance(value, str):
                return None
            try:
                return _H7_exact_expr_id(json.loads(value))
            except Exception:
                return None

        def __contains__(self, value):
            if isinstance(value, int):
                return value in self._ids
            ident = self._id_for_json_key(value)
            return ident in self._ids if ident is not None else False

        def __len__(self):
            return len(self._ids)

        def __iter__(self):
            # This local search structure is queried for membership only.
            # Expose the compact identities instead of re-retaining JSON strings.
            return iter(self._ids)

        def add(self, value):
            if isinstance(value, int):
                self._ids.add(value)
            else:
                ident = self._id_for_json_key(value)
                if ident is not None:
                    self._ids.add(ident)

        def add_with_order_key(self, ident, order_key):
            # order_key is already owned by expression_key_by_state_token for
            # active/registered states. Do not duplicate it in this membership set.
            self._ids.add(int(ident))

        def discard(self, value):
            if isinstance(value, int):
                self._ids.discard(value)
                return
            ident = self._id_for_json_key(value)
            if ident is not None:
                self._ids.discard(ident)

    seen_expression_keys = _H7_ExactSeenExpressionKeys()
'''

    source = replace_exact(
        source,
        "    seen_expression_keys: set[str] = set()\n",
        seen_type_block,
        label="install set-compatible exact-ID membership adapter",
    )
    source = replace_exact(
        source,
        '''        if metadata_hint is None:
            expr = canonical(expr)
            expression_ident = expr_key(expr)
        else:
            expression_ident = str(metadata_hint["expression_ident"])''',
        '''        if metadata_hint is None:
            expr = canonical(expr)
            expression_ident = _H7_exact_expr_id(expr)
            expression_order_key = expr_key(expr)
        else:
            expression_ident = int(metadata_hint["expression_ident"])
            expression_order_key = str(metadata_hint["expression_order_key"])''',
        label="register exact expression IDs and preserve JSON order keys",
    )
    source = replace_exact(
        source,
        "        seen_expression_keys.add(expression_ident)\n        try:\n            outputs = tuple(output_hint) if output_hint is not None else eval_vector(expr)\n",
        "        seen_expression_keys.add_with_order_key(expression_ident, expression_order_key)\n        try:\n            outputs = tuple(output_hint) if output_hint is not None else eval_vector(expr)\n",
        label="retain JSON-key test compatibility while using integer membership",
    )

    source = replace_exact(
        source,
        '''        state = (
            expr,
            outputs,
            state_used,
            state_keys,
            skey,
            next_state_token,
        )
        expression_key_by_state_token[next_state_token] = expression_ident''',
        '''        state = (
            expr,
            outputs,
            state_used,
            state_keys,
            skey,
            next_state_token,
            expression_ident,
        )
        # Preserve the original JSON lexical order for canonical operand ordering;
        # exact duplicate detection and cache lookups use the integer ID above.
        expression_key_by_state_token[next_state_token] = expression_order_key''',
        label="append exact ID to state and retain canonical order string",
    )

    source = replace_exact(
        source,
        "        expr, _, state_used, state_keys, skey, _ = state\n",
        "        expr, _, state_used, state_keys, skey, _ = state[:6]\n",
        label="rank accepts legacy six-slot probe states and extended internal states",
    )
    source = replace_exact(
        source,
        "        expr, _, _, _, _, _ = state\n",
        "        expr = state[0]\n",
        label="pruned-state helper accepts legacy and extended state tuples",
    )
    source = replace_exact(
        source,
        "        ident = expr_key(expr)\n        depth = ast_depth(expr)\n",
        "        ident = _H7_exact_expr_id(expr)\n        depth = ast_depth(expr)\n",
        label="pruned-state cache invalidation uses exact key ID",
    )

    old_parent_ident = '''        expression_ident = (
            '{"left":' + left_key + ',"op":' + json.dumps(op)
            + ',"right":' + right_key + '}'
        )'''
    source = replace_exact(
        source,
        old_parent_ident,
        "        expression_ident = _H7_exact_expr_binary_id(op, int(left[6]), int(right[6]))",
        label="compose parent exact key from child IDs",
    )

    # Only construct the parent JSON ordering string after it survives dominance
    # preflight. This preserves lexical rank/canonicalization while avoiding a
    # large joined JSON string for candidates rejected before AST materialization.
    source = replace_exact(
        source,
        '''            metadata_hint = {
                "expression_ident": expression_ident,
                "state_used": state_used,''',
        '''            expression_order_key = (
                '{"left":' + left_key + ',"op":' + json.dumps(op)
                + ',"right":' + right_key + '}'
            )
            metadata_hint = {
                "expression_ident": expression_ident,
                "expression_order_key": expression_order_key,
                "state_used": state_used,''',
        label="defer parent order-key string until candidate survives preflight",
    )

    # Export representation telemetry without altering scientific result.stats.
    marker = "        find_exact_expression._H7_LAST_STRUCTURAL_ID_STATS = dict(_H7_STRUCTURAL_ID_STATS)\n"
    if source.count(marker) != 1:
        raise SystemExit("structural-ID stats export marker not found")
    source = source.replace(
        marker,
        marker + "        find_exact_expression._H7_LAST_EXACT_EXPR_ID_STATS = dict(_H7_EXACT_EXPR_ID_STATS)\n",
        1,
    )

    compile(source, str(path), "exec")
    patched_sha = hashlib.sha256(source.encode()).hexdigest()
    path.write_text(source, encoding="utf-8")
    print(f"ACSIE_SOURCE_COMMIT={SOURCE_COMMIT}")
    print(f"BASE_MACRO_KEY_CACHE_SHA256={source_sha}")
    print(f"EXACT_EXPRESSION_ID_DIAGNOSTIC_SHA256={patched_sha}")
    print("EXACT_KEY=interned collision-free tuple signatures; macro IDs remain exact, not expanded")
    print("ID_OBJECT_CACHE=LRU capacity 8192")
    print("SEEN_KEY_STORAGE=integer IDs only; JSON-key test hooks decode lazily; no duplicated JSON strings in seen set")
    print("JSON_ORDERING=preserved for canonicalization and final rank tie-break")
    print("PARENT_KEY_BUILD=integer child-ID composition before allocation; JSON order key deferred until candidate survives dominance preflight")
    print("OTHER_FACTORS_UNCHANGED=expanded structural IDs; cycle-safe macro-ID cache; frontier cache 128; expansion cache OFF; grouping OFF")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
