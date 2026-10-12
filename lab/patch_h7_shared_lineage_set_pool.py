#!/usr/bin/env python3
"""Exact-source experiment: intern immutable H7 direct/lineage metadata sets.

Both arms use the same compositional structural-key implementation and the
same fixed 128-entry lineage-frontier cache. The candidate only reuses equal
frozenset objects across state-token metadata. It does not drop states, alter
dominance/ranking, change macro IDs, or change acceptance gates.
"""
from __future__ import annotations

from patch_h7_compositional_structural_key import apply_source as _apply_compositional

def replace_exact(source: str, old: str, new: str, *, count: int = 1, label: str) -> str:
    actual = source.count(old)
    if actual != count:
        raise SystemExit(f"{label}: expected {count} occurrence(s), found {actual}: {old[:140]!r}")
    return source.replace(old, new, count)

POOLS = '''    # Invocation-local exact interning of immutable metadata sets. The tuple
    # keys are already present in state[2] or returned by executable_lineage_ids;
    # pooling reuses frozenset payloads but preserves every live representative.
    _H7_direct_frozenset_pool: dict[tuple[str, ...], frozenset[str]] = {}
    _H7_lineage_frozenset_pool: dict[tuple[str, ...], frozenset[str]] = {}

    def _H7_intern_frozenset(
        pool: dict[tuple[str, ...], frozenset[str]],
        members: tuple[str, ...],
    ) -> frozenset[str]:
        cached_set = pool.get(members)
        if cached_set is not None:
            return cached_set
        value = frozenset(members)
        pool[members] = value
        return value

'''

OLD_META = '''        direct = frozenset(state[2])
        lineage = frozenset(executable_lineage_ids(state[2]))
'''
NEW_META = '''        direct_key = state[2]
        lineage_key = executable_lineage_ids(state[2])
        direct = _H7_intern_frozenset(_H7_direct_frozenset_pool, direct_key)
        lineage = _H7_intern_frozenset(_H7_lineage_frozenset_pool, lineage_key)
'''

def apply_set_pool(source: str) -> str:
    if "_H7_direct_frozenset_pool" in source or "_H7_intern_frozenset" in source:
        raise SystemExit("set-pooling patch markers already present")
    marker = "    _H7_online_metadata: dict[int, dict] = {}\n\n"
    source = replace_exact(
        source, marker, marker + POOLS,
        label="insert invocation-local immutable-set pools",
    )
    source = replace_exact(
        source, OLD_META, NEW_META,
        label="intern equivalent metadata sets",
    )
    compile(source, "cognitive_core/behavioral_search.py", "exec")
    return source

def apply_source(source: str, mode: str) -> str:
    # Fix both arms to the same compositional-key/cache source. This ensures
    # that immutable metadata set reuse is the sole difference between arms.
    source = _apply_compositional(source, "candidate")
    if mode == "candidate":
        source = apply_set_pool(source)
    elif mode != "control":
        raise ValueError(f"unsupported mode: {mode}")
    return source
