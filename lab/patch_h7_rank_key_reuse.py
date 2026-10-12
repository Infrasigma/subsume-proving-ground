#!/usr/bin/env python3
"""Exact-source diagnostic: reuse the already-registered JSON rank tie-break key.

This is intentionally a single-variable patch. It uses the canonical lexical
key already stored for each registered state instead of serializing that AST
again each time rank() is evaluated. Missing keys use the exact original
json.dumps fallback, which preserves hand-constructed/test-only states.
"""
from __future__ import annotations

import argparse
import hashlib
from pathlib import Path

EXPECTED_SOURCE_SHA256 = "2ff981875397b7d0edb2fef1c33c655b0a15b8c404b13c50d13a804f77535aea"
SOURCE_COMMIT = "d83cb60286f195eec02b4d7aa45a60539316d115"


def replace_exact(source: str, old: str, new: str, *, count: int = 1, label: str) -> str:
    observed = source.count(old)
    if observed != count:
        raise SystemExit(f"{label}: expected {count} source marker(s), found {observed}: {old[:100]!r}")
    return source.replace(old, new, count)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("source", type=Path)
    args = parser.parse_args()
    path = args.source
    source = path.read_text(encoding="utf-8")
    original_sha = hashlib.sha256(source.encode()).hexdigest()
    if original_sha != EXPECTED_SOURCE_SHA256:
        raise SystemExit(
            f"pinned source SHA mismatch: expected {EXPECTED_SOURCE_SHA256}, got {original_sha}"
        )
    if "_H7_RANK_KEY_REUSE_STATS" in source:
        raise SystemExit("rank-key reuse marker already present")

    # Add diagnostic-only counters immediately before the one local rank() routine.
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
        label="insert rank-key counters",
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
            # Preserve exact behavior for synthetic states not registered by this search.
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
    source = replace_exact(
        source,
        old,
        new,
        label="reuse existing per-state canonical lexical key in rank()",
    )

    return_marker = "        return BehavioralSearchResult(\n"
    source = replace_exact(
        source,
        return_marker,
        '''        find_exact_expression._H7_LAST_RANK_KEY_REUSE_STATS = dict(_H7_RANK_KEY_REUSE_STATS)
        return BehavioralSearchResult(
''',
        label="export rank-key reuse counters",
    )

    compile(source, str(path), "exec")
    patched_sha = hashlib.sha256(source.encode()).hexdigest()
    path.write_text(source, encoding="utf-8")
    print(f"ACSIE_SOURCE_COMMIT={SOURCE_COMMIT}")
    print(f"ORIGINAL_SOURCE_SHA256={original_sha}")
    print(f"RANK_KEY_REUSE_DIAGNOSTIC_SHA256={patched_sha}")
    print("FACTOR=rank() reuses exact registered JSON key, otherwise original JSON fallback")
    print("OTHER_FACTORS_UNCHANGED=no AST-key interning; no macro-ID memoization; no frontier cache; no structural-expansion cache; no grouping")
    print("SEMANTICS=rank tuple fields and exact JSON lexical tie-break string unchanged")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
