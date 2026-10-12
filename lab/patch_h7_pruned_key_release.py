#!/usr/bin/env python3
"""Exact-source H7 pruned expression-key lifetime experiment.

Both arms receive the same FIFO lineage-frontier cache (capacity 128), the same
instrumentation, and the same pinned ACSIE source. Candidate alone drops
expression_key_by_state_token entries when a new candidate is dominance-rejected
or an existing state is actually pruned. Duplicate-state cleanup already exists
and is measured identically in both arms.
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
        raise SystemExit(f"{label}: expected {count} marker(s), got {observed}: {old[:140]!r}")
    return source.replace(old, new, count)

def apply_frontier_cache(source: str) -> str:
    start = "    def lineage_frontier(states: Sequence[tuple]) -> tuple[tuple, ...]:"
    end = "    def build_extreme_lookup("
    if source.count(start) != 1 or source.count(end) != 1 or "_h7_lf_cache" in source:
        raise SystemExit("frontier cache source markers mismatch")
    source = source.replace(
        start,
        '    _h7_lf_cache = {}\n'
        '    _h7_lf_cache_stats = {"hits": 0, "misses": 0, "evictions": 0}\n\n' + start,
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
        label="frontier cache entry",
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
        label="frontier cache return",
    )
    source = source[:a] + block + source[b:]
    # Stable search-call identity used only by the snapshotter.
    doc = '''    """Exact discovery-set synthesis with behavior/provenance quotienting.

    OOD is intentionally not part of this function. The caller must verify OOD
    only after a discovery-qualified candidate has been selected.
    """
'''
    identity = '''    """Exact discovery-set synthesis with behavior/provenance quotienting.

    OOD is intentionally not part of this function. The caller must verify OOD
    only after a discovery-qualified candidate has been selected.
    """
    _H7_KEY_SEARCH_CALL_ID = int(getattr(find_exact_expression, "_H7_KEY_SEARCH_NEXT_CALL_ID", 0)) + 1
    find_exact_expression._H7_KEY_SEARCH_NEXT_CALL_ID = _H7_KEY_SEARCH_CALL_ID
    _H7_KEY_SEARCH_CALL_STARTED = __import__("time").perf_counter()
    _H7_KEY_SEARCH_FRAME_ID = id(__import__("sys")._getframe())
'''
    source = replace_exact(source, doc, identity, label="insert shared search identity")
    at = source.rfind("        return BehavioralSearchResult(")
    if at < 0:
        raise SystemExit("frontier-cache stats return marker missing")
    source = source[:at] + '        find_exact_expression._H7_LAST_LF_CACHE_STATS = dict(_h7_lf_cache_stats)\n' + source[at:]
    marker = "        find_exact_expression._H7_LAST_LF_CACHE_STATS = dict(_h7_lf_cache_stats)\n"
    source = replace_exact(
        source, marker,
        marker + "        find_exact_expression._H7_LAST_KEY_LIFETIME_STATS = dict(_H7_KEY_LIFETIME_STATS)\n",
        label="export key lifetime counters",
    )
    return source

def apply_key_lifetime_instrumentation(source: str, *, release: bool) -> str:
    map_decl = "    expression_key_by_state_token: dict[int, str] = {}\n"
    map_decl_new = map_decl + (
        '    _H7_KEY_LIFETIME_STATS = {"keys_added": 0, "keys_removed_duplicate": 0, '
        '"keys_removed_rejected": 0, "keys_removed_dominated": 0, "peak_map_entries": 0}\n'
    )
    source = replace_exact(source, map_decl, map_decl_new, label="declare key lifetime counters")
    add_key = "        expression_key_by_state_token[next_state_token] = expression_ident\n"
    add_key_new = add_key + (
        '        _H7_KEY_LIFETIME_STATS["keys_added"] += 1\n'
        '        _H7_KEY_LIFETIME_STATS["peak_map_entries"] = max(\n'
        '            _H7_KEY_LIFETIME_STATS["peak_map_entries"], len(expression_key_by_state_token)\n'
        "        )\n"
    )
    source = replace_exact(source, add_key, add_key_new, label="count every expression-key insertion")
    duplicate_pop = "            expression_key_by_state_token.pop(state[5], None)\n"
    duplicate_pop_new = (
        "            if expression_key_by_state_token.pop(state[5], None) is not None:\n"
        '                _H7_KEY_LIFETIME_STATS["keys_removed_duplicate"] += 1\n'
    )
    source = replace_exact(source, duplicate_pop, duplicate_pop_new, label="count existing duplicate-key cleanup")
    if release:
        rejected_anchor = '''            stats["dominance_pruned_states"] += 1
            vector_cache.pop(expression_ident, None)
            free_cache.pop(expression_ident, None)
            _H7_online_metadata.pop(state[5], None)
            return existing'''
        rejected_candidate = '''            stats["dominance_pruned_states"] += 1
            if expression_key_by_state_token.pop(state[5], None) is not None:
                _H7_KEY_LIFETIME_STATS["keys_removed_rejected"] += 1
            vector_cache.pop(expression_ident, None)
            free_cache.pop(expression_ident, None)
            _H7_online_metadata.pop(state[5], None)
            return existing'''
        source = replace_exact(source, rejected_anchor, rejected_candidate, label="release dominance-rejected candidate key")
        discard_anchor = '''        vector_cache.pop(ident, None)
        free_cache.pop(ident, None)

        # Invocation-local exact index'''
        discard_candidate = '''        vector_cache.pop(ident, None)
        free_cache.pop(ident, None)
        if expression_key_by_state_token.pop(state[5], None) is not None:
            _H7_KEY_LIFETIME_STATS["keys_removed_dominated"] += 1

        # Invocation-local exact index'''
        source = replace_exact(source, discard_anchor, discard_candidate, label="release dominated existing-state key")
    compile(source, "cognitive_core/behavioral_search.py", "exec")
    return source

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("source", type=Path)
    ap.add_argument("--mode", choices=("control","candidate"), required=True)
    args = ap.parse_args()
    path = args.source
    original = path.read_text(encoding="utf-8")
    before = hashlib.sha256(original.encode()).hexdigest()
    if before != EXPECTED_SOURCE_SHA256:
        raise SystemExit(f"pinned source SHA mismatch: expected {EXPECTED_SOURCE_SHA256}, got {before}")
    if "_H7_KEY_LIFETIME_STATS" in original or "_h7_lf_cache" in original:
        raise SystemExit("diagnostic patch markers already exist")
    patched = apply_frontier_cache(original)
    patched = apply_key_lifetime_instrumentation(patched, release=args.mode == "candidate")
    after = hashlib.sha256(patched.encode()).hexdigest()
    path.write_text(patched, encoding="utf-8")
    print(f"MODE={args.mode}")
    print(f"ACSIE_SOURCE_COMMIT={SOURCE_COMMIT}")
    print(f"ACSIE_SOURCE_TREE={SOURCE_TREE}")
    print(f"ORIGINAL_SOURCE_SHA256={before}")
    print(f"DIAGNOSTIC_SOURCE_SHA256={after}")
    print("FIXED_FACTOR=lineage-frontier FIFO cache capacity 128 in both arms")
    print("FACTOR=remove expression_key_by_state_token only for dominance-rejected candidates and actually pruned existing states" if args.mode == "candidate" else "FACTOR=preserve original key mappings after dominance rejection/pruning")
    print("UNCHANGED=duplicate-bucket key release; all other caches OFF; grouping OFF")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
