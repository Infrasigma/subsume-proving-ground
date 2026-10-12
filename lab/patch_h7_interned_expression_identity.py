#!/usr/bin/env python3
"""Lab-only experiment: compact expression identity for ACSIE H7.

The control and candidate both use the same compositional structural-key
implementation and the same fixed 128-entry lineage-frontier cache. The
candidate changes only the representation used for exact *syntactic* duplicate
identity: interned structural IDs replace full canonical JSON strings in the
seen-expression set and in preflight duplicate checks.

This is a diagnostic patch applied only to a temporary checkout of an exact,
pinned ACSIE commit. It does not modify ACSIE production files or any acceptance
gate.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import random
from pathlib import Path
from typing import Any, Mapping

from patch_h7_compositional_structural_key import apply_source as _apply_compositional

SOURCE_COMMIT = "d83cb60286f195eec02b4d7aa45a60539316d115"
SOURCE_TREE = "12d2fa8fc1f1e34d3d7c442eef85aa6a71380420"
SOURCE_SHA256 = "2ff981875397b7d0edb2fef1c33c655b0a15b8c404b13c50d13a804f77535aea"
COMMUTATIVE = {"add", "mul", "max", "min"}

INTERCACHE_CLASS = r'''
class _H7ExpressionInterner:
    """Exact structural identity for JSON-like ASTs, with shared child IDs.

    IDs are local to one find_exact_expression invocation. Dict/list/scalar
    freezing follows canonical JSON equality, including bool-vs-int, integer
    vs float, signed zero, NaN spelling, Unicode values, extra fields and
    sorted object keys. The hot binary-composition path receives already
    canonicalized child IDs and does not serialize the parent expression.
    """

    def __init__(self):
        self._ids = {}
        self._next_id = 0

    def _freeze(self, value):
        if isinstance(value, Mapping):
            return (
                "dict",
                tuple((key, self._freeze(value[key])) for key in sorted(value)),
            )
        if isinstance(value, (list, tuple)):
            return ("array", tuple(self._freeze(item) for item in value))
        if value is None:
            return ("null",)
        if isinstance(value, bool):
            return ("bool", value)
        if isinstance(value, int):
            return ("int", str(value))
        if isinstance(value, float):
            return ("float", json.dumps(value, separators=(",", ":")))
        if isinstance(value, str):
            return ("str", value)
        # ACSIE ASTs are JSON data. Retain a strict fallback for a future
        # scalar type instead of silently folding distinct values together.
        return ("json", json.dumps(value, sort_keys=True, separators=(",", ":")))

    def _intern_key(self, key):
        existing = self._ids.get(key)
        if existing is not None:
            return existing
        result = self._next_id
        self._next_id += 1
        self._ids[key] = result
        return result

    @staticmethod
    def _json_key(expr):
        return json.dumps(expr, sort_keys=True, separators=(",", ":"))

    def intern_expr(self, expr):
        """Intern one recursively canonicalized expression without expanding macros."""
        op = str(expr.get("op", ""))
        if op in {"get", "const", "macro"}:
            return self._intern_key(("leaf", self._freeze(expr)))

        if op in {"abs", "neg", "threshold"}:
            child_id = self.intern_expr(expr["arg"])
            known = {"op", "arg"}
            threshold_key = None
            if op == "threshold":
                known.add("lo")
                threshold_key = self._freeze(expr["lo"])
            extras = tuple(
                (key, self._freeze(value))
                for key, value in sorted(expr.items())
                if key not in known
            )
            return self._intern_key(
                ("unary", op, child_id, threshold_key, extras)
            )

        if "left" in expr and "right" in expr:
            # Match ACSIE expr_key() exactly: canonical() sorts only the current
            # node's children. Descendants arrive canonical when synthesized,
            # but a legacy diagnostic can pass a nested noncanonical AST.
            # Do not silently canonicalize below this node, or a test's
            # string-key removal will map to a different identity.
            left_id = self.intern_expr(expr["left"])
            right_id = self.intern_expr(expr["right"])
            known = {"op", "left", "right"}
            extras = tuple(
                (key, self._freeze(value))
                for key, value in sorted(expr.items())
                if key not in known
            )
            return self._intern_key(
                ("binary", op, left_id, right_id, extras)
            )

        return self._intern_key(("other", self._freeze(expr)))

    def intern_binary(self, op, left_id, right_id):
        """Intern a standard binary parent after caller applies exact JSON ordering."""
        return self._intern_key(("binary", str(op), int(left_id), int(right_id), ()))

    @property
    def size(self):
        return len(self._ids)


class _H7SeenExpressionIds(set):
    """Store only integer IDs but accept legacy JSON-key discard in tests."""
    def __init__(self):
        super().__init__()
        self.interner = None

    def discard(self, value):
        if isinstance(value, str):
            if self.interner is None:
                raise RuntimeError("identity interner is not initialized")
            value = self.interner.intern_expr(json.loads(value))
        super().discard(value)
'''

# Define the exact same class in this module so the independent identity self-test
# exercises the implementation that will be injected into the pinned ACSIE source.
exec(INTERCACHE_CLASS, globals())

def replace_exact(source: str, old: str, new: str, *, count: int = 1, label: str) -> str:
    observed = source.count(old)
    if observed != count:
        raise SystemExit(f"{label}: expected {count} marker(s), got {observed}: {old[:180]!r}")
    return source.replace(old, new, count)

REGISTER_START_OLD = '''        if metadata_hint is None:
            expr = canonical(expr)
            expression_ident = expr_key(expr)
        else:
            expression_ident = str(metadata_hint["expression_ident"])
        if expression_ident in seen_expression_keys:
            stats["duplicate_states"] += 1
            stats["exact_expression_duplicates_skipped"] += 1
            return None
        seen_expression_keys.add(expression_ident)
'''

REGISTER_START_NEW = '''        if metadata_hint is None:
            expr = canonical(expr)
            expression_ident = expr_key(expr)
            expression_id = expression_interner.intern_expr(expr)
        else:
            # Full JSON is retained only for a materialized candidate that has
            # passed the cheap indexed preflight. It remains the cache/sort key.
            expression_ident = expr_key(expr)
            expression_id = int(metadata_hint["expression_id"])
        if expression_id in seen_expression_keys:
            stats["duplicate_states"] += 1
            stats["exact_expression_duplicates_skipped"] += 1
            return None
        seen_expression_keys.add(expression_id)
'''

REGISTER_BINARY_START_OLD = '''        left_key = expression_key_by_state_token.get(left[5])
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
'''

REGISTER_BINARY_START_NEW = '''        left_key = expression_key_by_state_token.get(left[5])
        if left_key is None:
            left_key = expr_key(left[0])
        right_key = expression_key_by_state_token.get(right[5])
        if right_key is None:
            right_key = expr_key(right[0])
        left_expression_id = expression_id_by_state_token.get(left[5])
        if left_expression_id is None:
            left_expression_id = expression_interner.intern_expr(left[0])
        right_expression_id = expression_id_by_state_token.get(right[5])
        if right_expression_id is None:
            right_expression_id = expression_interner.intern_expr(right[0])
        commutative = op in {"add", "mul", "max", "min"}
        canonical_swap = commutative and left_key > right_key
        if canonical_swap:
            left_key, right_key = right_key, left_key
            left_expression_id, right_expression_id = right_expression_id, left_expression_id
        expression_id = expression_interner.intern_binary(
            op, left_expression_id, right_expression_id
        )
        if expression_id in seen_expression_keys:
            stats["duplicate_states"] += 1
            stats["exact_expression_duplicates_skipped"] += 1
            return None
'''

def apply_interned_identity(source: str) -> str:
    if "_H7ExpressionInterner" in source or "expression_id_by_state_token" in source:
        raise SystemExit("interned expression-identity markers already present")

    import_marker = "from .recursive_cognitive_compiler import ast_depth, ast_nodes\n"
    source = replace_exact(
        source,
        import_marker,
        import_marker + INTERCACHE_CLASS + "\n",
        label="insert expression interner definition",
    )

    source = replace_exact(
        source,
        "    seen_expression_keys: set[str] = set()\n",
        "    seen_expression_keys: set[int] = _H7SeenExpressionIds()\n",
        label="change seen set to interned identities with test compatibility",
    )
    source = replace_exact(
        source,
        "    expression_key_by_state_token: dict[int, str] = {}\n",
        "    expression_key_by_state_token: dict[int, str] = {}\n"
        "    expression_id_by_state_token: dict[int, int] = {}\n"
        "    expression_interner = _H7ExpressionInterner()\n"
        "    seen_expression_keys.interner = expression_interner\n",
        label="add per-invocation expression IDs and test compatibility",
    )

    source = replace_exact(
        source,
        REGISTER_START_OLD,
        REGISTER_START_NEW,
        label="use IDs in register exact-duplicate check",
    )
    source = replace_exact(
        source,
        "        expression_key_by_state_token[next_state_token] = expression_ident\n",
        "        expression_key_by_state_token[next_state_token] = expression_ident\n"
        "        expression_id_by_state_token[next_state_token] = expression_id\n",
        label="index IDs by state token",
    )
    source = replace_exact(
        source,
        "            expression_key_by_state_token.pop(state[5], None)\n"
        "            ast_depth_by_state_token.pop(state[5], None)\n",
        "            expression_key_by_state_token.pop(state[5], None)\n"
        "            expression_id_by_state_token.pop(state[5], None)\n"
        "            ast_depth_by_state_token.pop(state[5], None)\n",
        label="release identity for bucket duplicate",
    )
    source = replace_exact(
        source,
        REGISTER_BINARY_START_OLD,
        REGISTER_BINARY_START_NEW,
        label="compose interned identity before parent AST allocation",
    )
    source = replace_exact(
        source,
        '                "expression_ident": expression_ident,\n',
        '                "expression_id": expression_id,\n',
        label="pass interned ID to register",
    )
    # The remaining direct add is the indexed-preflight dominator bookkeeping.
    source = replace_exact(
        source,
        "        seen_expression_keys.add(expression_ident)\n",
        "        seen_expression_keys.add(expression_id)\n",
        label="record identity on preflight dominance hit",
    )

    source = replace_exact(
        source,
        "            vector_cache.pop(expression_ident, None)\n"
        "            free_cache.pop(expression_ident, None)\n"
        "            _H7_online_metadata.pop(state[5], None)\n"
        "            return existing\n",
        "            vector_cache.pop(expression_ident, None)\n"
        "            free_cache.pop(expression_ident, None)\n"
        "            expression_key_by_state_token.pop(state[5], None)\n"
        "            expression_id_by_state_token.pop(state[5], None)\n"
        "            _H7_online_metadata.pop(state[5], None)\n"
        "            return existing\n",
        label="release identity on register-time dominance hit",
    )
    source = replace_exact(
        source,
        "        ident = expr_key(expr)\n"
        "        depth = ast_depth(expr)\n",
        "        ident = expr_key(expr)\n"
        "        expression_key_by_state_token.pop(state[5], None)\n"
        "        expression_id_by_state_token.pop(state[5], None)\n"
        "        depth = ast_depth(expr)\n",
        label="release identity when frontier representative is pruned",
    )

    if "expression_ident in seen_expression_keys" in source:
        raise SystemExit("unconverted string-identity lookup remains")
    if "seen_expression_keys.add(expression_ident)" in source:
        raise SystemExit("unconverted string-identity insert remains")
    compile(source, "cognitive_core/behavioral_search.py", "exec")
    return source

def _canonical(expr: Mapping[str, Any]) -> dict:
    """Recursively canonicalize just the raw AST's commutative child order."""
    out = {}
    for key, value in expr.items():
        if key == "left" or key == "right" or key == "arg":
            out[key] = _canonical(value) if isinstance(value, Mapping) else value
        elif isinstance(value, Mapping):
            out[key] = _canonical(value)
        elif isinstance(value, list):
            out[key] = [_canonical(x) if isinstance(x, Mapping) else x for x in value]
        else:
            out[key] = value
    op = str(out.get("op", ""))
    if op in COMMUTATIVE and "left" in out and "right" in out:
        left_key = json.dumps(out["left"], sort_keys=True, separators=(",", ":"))
        right_key = json.dumps(out["right"], sort_keys=True, separators=(",", ":"))
        if left_key > right_key:
            out["left"], out["right"] = out["right"], out["left"]
    return out

def _make_expression(rng: random.Random, depth: int) -> dict:
    if depth <= 0 or rng.random() < 0.27:
        kind = rng.randrange(5)
        if kind == 0:
            return {"op": "get", "key": rng.choice(["x", "y", "z", "unicode-ß"])}
        if kind == 1:
            return {"op": "const", "value": rng.choice([0, 1, -1, 1.0, -0.0, 2.25, float("nan")])}
        if kind == 2:
            return {"op": "macro", "id": rng.choice(["prim:a", "prim:b", "missing:ø"])}
        if kind == 3:
            return {"op": "get", "key": "x", "extra": {"z": 1, "a": "é"}}
        return {"op": "const", "value": 2, "extra": ["alpha", {"b": True, "a": None}]}
    kind = rng.randrange(3)
    if kind == 0:
        return {"op": rng.choice(["abs", "neg"]), "arg": _make_expression(rng, depth - 1)}
    if kind == 1:
        return {"op": "threshold", "arg": _make_expression(rng, depth - 1), "lo": rng.choice([-1.0, 0.0, 1.5])}
    return {
        "op": rng.choice(["add", "mul", "max", "min", "sub"]),
        "left": _make_expression(rng, depth - 1),
        "right": _make_expression(rng, depth - 1),
    }

def run_self_test() -> dict:
    rng = random.Random(20261013)
    corpus = [
        {"op": "get", "key": "x"},
        {"op": "add", "left": {"op": "get", "key": "x"}, "right": {"op": "get", "key": "y"}},
        {"op": "add", "left": {"op": "get", "key": "y"}, "right": {"op": "get", "key": "x"}},
        {"op": "sub", "left": {"op": "get", "key": "x"}, "right": {"op": "get", "key": "y"}},
        {"op": "sub", "left": {"op": "get", "key": "y"}, "right": {"op": "get", "key": "x"}},
        {"op": "const", "value": float("nan"), "extra": {"z": 1, "a": "é"}},
        {"op": "const", "value": -0.0},
        {"op": "macro", "id": "missing:ø"},
    ]
    corpus.extend(_make_expression(rng, 5) for _ in range(4099))
    canonical_corpus = [_canonical(expr) for expr in corpus]
    interner = _H7ExpressionInterner()
    raw_to_id = {}
    id_to_raw = {}
    for expr in canonical_corpus:
        raw = json.dumps(expr, sort_keys=True, separators=(",", ":"))
        ident = interner.intern_expr(expr)
        if raw in raw_to_id and raw_to_id[raw] != ident:
            raise AssertionError("equal canonical JSON keys received different interned IDs")
        if ident in id_to_raw and id_to_raw[ident] != raw:
            raise AssertionError("different canonical JSON keys collided in the interner")
        raw_to_id[raw] = ident
        id_to_raw[ident] = raw

    # Independently verify that incremental binary composition produces the
    # same identity as interning the completed canonical AST.
    compositions = 0
    ops = ["add", "sub", "mul", "max", "min"]
    for i in range(0, min(1000, len(canonical_corpus) - 1), 2):
        left = canonical_corpus[i]
        right = canonical_corpus[i + 1]
        left_key = json.dumps(left, sort_keys=True, separators=(",", ":"))
        right_key = json.dumps(right, sort_keys=True, separators=(",", ":"))
        op = ops[(i // 2) % len(ops)]
        left_id = interner.intern_expr(left)
        right_id = interner.intern_expr(right)
        if op in COMMUTATIVE and left_key > right_key:
            left, right = right, left
            left_key, right_key = right_key, left_key
            left_id, right_id = right_id, left_id
        parent = {"op": op, "left": left, "right": right}
        expected_id = interner.intern_expr(parent)
        composed_id = interner.intern_binary(op, left_id, right_id)
        if expected_id != composed_id:
            raise AssertionError(f"compositional identity mismatch at pair {i}, op={op}")
        compositions += 1

    return {
        "status": "PASSED",
        "corpus_expression_count": len(canonical_corpus),
        "distinct_canonical_json_keys": len(raw_to_id),
        "interner_size_after_composition_checks": interner.size,
        "incremental_composition_cases": compositions,
        "exact_equivalence_partition": "PASSED",
        "json_reference": "json.dumps(sort_keys=True,separators=(',',':'))",
        "production_source_modified": False,
    }

def apply_source(source: str, mode: str) -> str:
    # Install the same compositional structural key and fixed frontier cache in
    # both arms. The only between-arm factor is representation of seen syntax.
    source = _apply_compositional(source, "candidate")
    if mode == "candidate":
        source = apply_interned_identity(source)
    elif mode != "control":
        raise ValueError(f"unsupported mode: {mode}")
    return source

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--check-source", type=Path)
    ap.add_argument("--mode", choices=("control", "candidate"), default="candidate")
    args = ap.parse_args()
    if args.self_test:
        print(json.dumps(run_self_test(), indent=2, sort_keys=True, allow_nan=True))
    if args.check_source is not None:
        original = args.check_source.read_text(encoding="utf-8")
        digest = hashlib.sha256(original.encode("utf-8")).hexdigest()
        if digest != SOURCE_SHA256:
            raise SystemExit(f"source SHA mismatch: expected {SOURCE_SHA256}, got {digest}")
        patched = apply_source(original, args.mode)
        print(json.dumps({
            "status": "PATCH_COMPILED",
            "mode": args.mode,
            "source_commit": SOURCE_COMMIT,
            "source_tree": SOURCE_TREE,
            "source_sha256": digest,
            "patched_sha256": hashlib.sha256(patched.encode("utf-8")).hexdigest(),
            "production_source_modified": False,
        }, indent=2, sort_keys=True))
    if not args.self_test and args.check_source is None:
        ap.error("specify --self-test and/or --check-source")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
