#!/usr/bin/env python3
"""Exact-source, temporary structural-key representation experiment for ACSIE Gen6.

--mode control: adds the fixed FIFO lineage-frontier cache only.
--mode candidate: adds the same cache and replaces expanded JSON strings with
exact, collision-safe interned structural-node IDs.
No ACSIE production file is committed or mutated.
"""
from __future__ import annotations

import argparse
import hashlib
from pathlib import Path

EXPECTED_SOURCE_SHA256 = "2ff981875397b7d0edb2fef1c33c655b0a15b8c404b13c50d13a804f77535aea"
SOURCE_ID = "d83cb60286f195eec02b4d7aa45a60539316d115"
TREE_ID = "12d2fa8fc1f1e34d3d7c442eef85aa6a71380420"


def replace_exact(source: str, old: str, new: str, count: int, label: str) -> str:
    observed = source.count(old)
    if observed != count:
        raise SystemExit(f"{label}: expected {count} marker(s), found {observed}: {old[:100]!r}")
    return source.replace(old, new, count)


def apply_frontier_cache(source: str) -> str:
    start = "    def lineage_frontier(states: Sequence[tuple]) -> tuple[tuple, ...]:"
    end = "    def build_extreme_lookup("
    if source.count(start) != 1 or source.count(end) != 1 or "_h7_lf_cache" in source:
        raise SystemExit("frontier-cache marker mismatch")
    source = source.replace(
        start,
        '    _h7_lf_cache = {}\n'
        '    _h7_lf_cache_stats = {"hits": 0, "misses": 0, "evictions": 0}\n\n'
        + start,
        1,
    )
    a = source.index(start)
    b = source.index(end, a + len(start))
    block = source[a:b]
    block = replace_exact(
        block,
        "        if not states:\n            return ()",
        "        if not states:\n            return ()\n"
        '        cache_enabled = __import__("os").environ.get("ACSIE_DISABLE_LF_CACHE") != "1"\n'
        '        cache_key = tuple(sorted((int(st[5]), tuple(sorted(structural_lineage_by_key.get(st[4], set())))) for st in states)) if cache_enabled else None\n'
        "        cached = _h7_lf_cache.get(cache_key) if cache_enabled else None\n"
        "        if cached is not None:\n"
        '            _h7_lf_cache_stats["hits"] += 1\n'
        "            return cached\n"
        '        if cache_enabled: _h7_lf_cache_stats["misses"] += 1',
        1,
        "frontier-cache entry",
    )
    block = replace_exact(
        block,
        "        return tuple(sorted(frontier, key=rank))",
        "        result = tuple(sorted(frontier, key=rank))\n"
        "        if cache_enabled:\n"
        "            if len(_h7_lf_cache) >= 128:\n"
        "                del _h7_lf_cache[next(iter(_h7_lf_cache))]\n"
        '                _h7_lf_cache_stats["evictions"] += 1\n'
        "            _h7_lf_cache[cache_key] = result\n"
        "        return result",
        1,
        "frontier-cache return",
    )
    source = source[:a] + block + source[b:]
    at = source.rfind("        return BehavioralSearchResult(")
    if at < 0:
        raise SystemExit("final result return marker missing for frontier cache stats")
    source = source[:at] + (
        "        find_exact_expression._H7_LAST_LF_CACHE_STATS = dict(_h7_lf_cache_stats)\n"
    ) + source[at:]
    return source


STRUCTURAL_ID_BLOCK = '''    _H7_STRUCTURAL_ID_INTERN: dict[tuple[Any, ...], int] = {}
    _H7_STRUCTURAL_ID_NEXT = 1
    _H7_STRUCTURAL_ID_STATS = {
        "representation": "exact_hash_consed_structural_ids",
        "structural_key_calls": 0,
        "structural_id_intern_calls": 0,
        "structural_id_intern_hits": 0,
        "structural_id_unique_nodes": 0,
        "macro_expansion_edges": 0,
        "expanded_json_serializations": 0,
        "binary_structural_keys_composed_without_expansion": 0,
    }

    def _H7_intern_structural_id(signature: tuple[Any, ...]) -> int:
        nonlocal _H7_STRUCTURAL_ID_NEXT
        _H7_STRUCTURAL_ID_STATS["structural_id_intern_calls"] += 1
        existing = _H7_STRUCTURAL_ID_INTERN.get(signature)
        if existing is not None:
            _H7_STRUCTURAL_ID_STATS["structural_id_intern_hits"] += 1
            return existing
        ident = _H7_STRUCTURAL_ID_NEXT
        _H7_STRUCTURAL_ID_NEXT += 1
        _H7_STRUCTURAL_ID_INTERN[signature] = ident
        _H7_STRUCTURAL_ID_STATS["structural_id_unique_nodes"] += 1
        return ident

    def _H7_intern_structural_binary_id(op: str, left_id: int, right_id: int) -> int:
        if op in {"add", "mul", "max", "min"} and left_id > right_id:
            left_id, right_id = right_id, left_id
        _H7_STRUCTURAL_ID_STATS["binary_structural_keys_composed_without_expansion"] += 1
        return _H7_intern_structural_id(("binary", op, left_id, right_id))

    def structural_key(expr: Mapping[str, Any], active: tuple[str, ...] = ()) -> int:
        """Intern exact expanded structural identity without constructing expanded AST/JSON."""
        _H7_STRUCTURAL_ID_STATS["structural_key_calls"] += 1
        op = str(expr["op"])
        if op in {"get", "const"}:
            # Preserve the exact source's leaf semantics including any extra leaf keys.
            _H7_STRUCTURAL_ID_STATS["expanded_json_serializations"] += 1
            leaf_payload = json.dumps(dict(expr), sort_keys=True, separators=(",", ":"))
            return _H7_intern_structural_id(("leaf", op, leaf_payload))
        if op == "macro":
            mid = str(expr["id"])
            if mid in active or mid not in pmap:
                return _H7_intern_structural_id(("macro", mid))
            _H7_STRUCTURAL_ID_STATS["macro_expansion_edges"] += 1
            return structural_key(pmap[mid], active + (mid,))
        if op in {"abs", "neg", "threshold"}:
            arg_id = structural_key(expr["arg"], active)
            if op == "threshold":
                _H7_STRUCTURAL_ID_STATS["expanded_json_serializations"] += 1
                lo_payload = json.dumps(expr["lo"], sort_keys=True, separators=(",", ":"))
                return _H7_intern_structural_id(("unary", op, arg_id, lo_payload))
            return _H7_intern_structural_id(("unary", op, arg_id, None))
        left_id = structural_key(expr["left"], active)
        right_id = structural_key(expr["right"], active)
        return _H7_intern_structural_binary_id(op, left_id, right_id)

    def structural_lineage(expr: Mapping[str, Any]) -> tuple[str, ...]:
        key = structural_key(expr)
        lineage = set(structural_lineage_by_key.get(key, set()))
        lineage.update(provenance_lineage_ids(used(expr)))
        return tuple(sorted(lineage))

'''


def apply_candidate_ids(source: str) -> str:
    source = replace_exact(
        source,
        "    structural_lineage_by_key: dict[str, set[str]] = {}\n",
        "    structural_lineage_by_key: dict[int, set[str]] = {}\n",
        1,
        "structural lineage key type",
    )
    start = "    def structural_expand("
    end = "    def structural_lineage("
    if source.count(start) != 1 or source.count(end) != 1:
        raise SystemExit("structural expansion block markers mismatch")
    a = source.index(start)
    b = source.index(end, a)
    # Keep the original structural_lineage definition replacement tied to exact boundaries.
    end_next = source.index("\n    def used(", b)
    source = source[:a] + STRUCTURAL_ID_BLOCK + source[end_next + 1:]

    old = '''        left_skey, right_skey = left[4], right[4]
        if commutative and left_skey > right_skey:
            left_skey, right_skey = right_skey, left_skey
        skey = (
            '{"left":' + left_skey + ',"op":' + json.dumps(op)
            + ',"right":' + right_skey + '}'
        )'''
    new = '''        left_skey, right_skey = left[4], right[4]
        skey = _H7_intern_structural_binary_id(op, left_skey, right_skey)'''
    source = replace_exact(source, old, new, 1, "compose structural identity without expanded JSON")

    old = 'structural_lineage_by_key.get(str(st[4]), set())'
    if old in source:
        source = source.replace(old, 'structural_lineage_by_key.get(st[4], set())')

    at = source.rfind("        return BehavioralSearchResult(")
    if at < 0:
        raise SystemExit("final result return marker missing for structural-id stats")
    source = source[:at] + (
        "        find_exact_expression._H7_LAST_STRUCTURAL_ID_STATS = dict(_H7_STRUCTURAL_ID_STATS)\n"
    ) + source[at:]
    return source


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("source", type=Path)
    parser.add_argument("--mode", choices=("control", "candidate"), required=True)
    args = parser.parse_args()
    path = args.source
    source = path.read_text(encoding="utf-8")
    original_sha = hashlib.sha256(source.encode()).hexdigest()
    if original_sha != EXPECTED_SOURCE_SHA256:
        raise SystemExit(f"pinned source SHA mismatch: expected {EXPECTED_SOURCE_SHA256}; got {original_sha}")
    if "_H7_STRUCTURAL_ID_INTERN" in source or "_h7_lf_cache" in source:
        raise SystemExit("diagnostic patch markers already present")

    patched = apply_frontier_cache(source)
    if args.mode == "candidate":
        patched = apply_candidate_ids(patched)
    compile(patched, str(path), "exec")
    patched_sha = hashlib.sha256(patched.encode()).hexdigest()
    path.write_text(patched, encoding="utf-8")
    print(f"MODE={args.mode}")
    print(f"ACSIE_SOURCE_COMMIT={SOURCE_ID}")
    print(f"ACSIE_SOURCE_TREE={TREE_ID}")
    print(f"ORIGINAL_SOURCE_SHA256={original_sha}")
    print(f"DIAGNOSTIC_SOURCE_SHA256={patched_sha}")
    print("FRONTIER_CACHE_CAPACITY=128 (identical fixed control factor)")
    if args.mode == "candidate":
        print("STRUCTURAL_KEY_REPRESENTATION=exact collision-safe interned integer IDs")
        print("NO_EXPANDED_AST_JSON_FOR_BINARY_PRECHECKS=true")
    else:
        print("STRUCTURAL_KEY_REPRESENTATION=original canonical expanded JSON strings")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
