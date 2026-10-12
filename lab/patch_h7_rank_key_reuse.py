#!/usr/bin/env python3
"""Exact-source H7 rank-key reuse diagnostic patcher.

Both modes install the identical 128-entry lineage-frontier cache. Candidate
adds only reuse of each registered state's exact canonical JSON key for the
last rank() tie-break, avoiding repeated json.dumps on the same AST. Missing
keys retain the exact original JSON fallback.
"""
from __future__ import annotations

import argparse
import hashlib
from pathlib import Path

EXPECTED_SOURCE_SHA256 = "2ff981875397b7d0edb2fef1c33c655b0a15b8c404b13c50d13a804f77535aea"
SOURCE_COMMIT = "d83cb60286f195eec02b4d7aa45a60539316d115"
SOURCE_TREE = "12d2fa8fc1f1e34d3d7c442eef85aa6a71380420"


def replace_exact(source: str, old: str, new: str, *, count: int = 1, label: str) -> str:
    observed = source.count(old)
    if observed != count:
        raise SystemExit(f"{label}: expected {count} marker(s), found {observed}: {old[:100]!r}")
    return source.replace(old, new, count)


def apply_frontier_cache(source: str) -> str:
    start = "    def lineage_frontier(states: Sequence[tuple]) -> tuple[tuple, ...]:"
    end = "    def build_extreme_lookup("
    if source.count(start) != 1 or source.count(end) != 1 or "_h7_lf_cache" in source:
        raise SystemExit("frontier-cache markers mismatch")
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
        '        cache_key = tuple(sorted((int(st[5]), tuple(sorted(structural_lineage_by_key.get(str(st[4]), set())))) for st in states)) if cache_enabled else None\n'
        "        cached = _h7_lf_cache.get(cache_key) if cache_enabled else None\n"
        "        if cached is not None:\n"
        '            _h7_lf_cache_stats["hits"] += 1\n'
        "            return cached\n"
        '        if cache_enabled: _h7_lf_cache_stats["misses"] += 1',
        label="frontier-cache entry",
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
        label="frontier-cache return",
    )
    source = source[:a] + block + source[b:]
    at = source.rfind("        return BehavioralSearchResult(")
    if at < 0:
        raise SystemExit("final result return marker not found")
    source = source[:at] + (
        "        find_exact_expression._H7_LAST_LF_CACHE_STATS = dict(_h7_lf_cache_stats)\n"
    ) + source[at:]
    return source


def apply_rank_key_reuse(source: str) -> str:
    source = replace_exact(
        source,
        '''    """Exact discovery-set synthesis with behavior/provenance quotienting.

    OOD is intentionally not part of this function. The caller must verify OOD
    only after a discovery-qualified candidate has been selected.
    """
''',
        '''    """Exact discovery-set synthesis with behavior/provenance quotienting.

    OOD is intentionally not part of this function. The caller must verify OOD
    only after a discovery-qualified candidate has been selected.
    """
    _H7_RANK_KEY_REUSE_CALL_ID = int(getattr(find_exact_expression, "_H7_RANK_KEY_REUSE_NEXT_CALL_ID", 0)) + 1
    find_exact_expression._H7_RANK_KEY_REUSE_NEXT_CALL_ID = _H7_RANK_KEY_REUSE_CALL_ID
    _H7_RANK_KEY_REUSE_CALL_STARTED = __import__("time").perf_counter()
    _H7_RANK_KEY_REUSE_FRAME_ID = id(__import__("sys")._getframe())
''',
        label="insert search-call identity/time",
    )
    source = replace_exact(
        source,
        "    def rank(state: tuple) -> tuple[Any, ...]:\n",
        '''    _H7_RANK_KEY_REUSE_STATS = {
        "cached_registered_key_hits": 0,
        "fallback_json_serializations": 0,
        "rank_calls": 0,
    }

    def rank(state: tuple) -> tuple[Any, ...]:
''',
        label="insert rank-key reuse counters",
    )
    old = '''        return (
            *objective_key,
            -len(executable_lineage),
            -len(lineage),
            state_keys,
            ast_nodes(expr),
            json.dumps(expr, sort_keys=True, separators=(",", ":")),
        )'''
    new = '''        _H7_RANK_KEY_REUSE_STATS["rank_calls"] += 1
        rank_tiebreak_key = expression_key_by_state_token.get(state[5])
        if rank_tiebreak_key is None:
            # Preserve the exact original JSON tie-break for synthetic states.
            _H7_RANK_KEY_REUSE_STATS["fallback_json_serializations"] += 1
            rank_tiebreak_key = json.dumps(expr, sort_keys=True, separators=(",", ":"))
        else:
            _H7_RANK_KEY_REUSE_STATS["cached_registered_key_hits"] += 1
        return (
            *objective_key,
            -len(executable_lineage),
            -len(lineage),
            state_keys,
            ast_nodes(expr),
            rank_tiebreak_key,
        )'''
    source = replace_exact(source, old, new, label="reuse registered canonical JSON key in rank()")
    source = replace_exact(
        source,
        "        return BehavioralSearchResult(\n",
        '''        find_exact_expression._H7_LAST_RANK_KEY_REUSE_STATS = dict(_H7_RANK_KEY_REUSE_STATS)
        return BehavioralSearchResult(
''',
        label="export rank-key reuse statistics",
    )
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
        raise SystemExit(f"pinned source SHA mismatch: expected {EXPECTED_SOURCE_SHA256}, got {original_sha}")
    if "_h7_lf_cache" in source or "_H7_RANK_KEY_REUSE_STATS" in source:
        raise SystemExit("rank/frontier diagnostic patch already present")
    source = apply_frontier_cache(source)
    if args.mode == "candidate":
        source = apply_rank_key_reuse(source)
    compile(source, str(path), "exec")
    patched_sha = hashlib.sha256(source.encode()).hexdigest()
    path.write_text(source, encoding="utf-8")
    print(f"MODE={args.mode}")
    print(f"ACSIE_SOURCE_COMMIT={SOURCE_COMMIT}")
    print(f"ACSIE_SOURCE_TREE={SOURCE_TREE}")
    print(f"ORIGINAL_SOURCE_SHA256={original_sha}")
    print(f"DIAGNOSTIC_SOURCE_SHA256={patched_sha}")
    print("FIXED_FRONTIER_CACHE=FIFO capacity 128 in both arms")
    if args.mode == "candidate":
        print("FACTOR=rank() reuses registered expression_key_by_state_token key; exact JSON fallback for missing token")
        print("EXPECTED_KEY=exactly same canonical JSON text as json.dumps(expr,sort_keys=True,separators=(',',':')) for registered states")
    else:
        print("FACTOR=original rank() JSON serialization")
    print("OTHER_FACTORS_UNCHANGED=no AST-key interning; no macro-ID memoization; no structural-expansion cache; no grouping")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
