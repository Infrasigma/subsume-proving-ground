#!/usr/bin/env python3
"""Exact-source H7 metadata-only duplicate-bucket preflight diagnostic.

Both modes receive the same fixed 128-entry lineage-frontier cache. Candidate
changes only register_binary's same-bucket duplicate path: it reproduces the
existing register() bookkeeping from metadata already computed for both child
states, avoiding construction and traversal of a parent AST that will not be
retained. Every output/capability/statistics side effect is preserved.
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
        raise SystemExit(f"{label}: expected {count} marker(s), got {observed}: {old[:100]!r}")
    return source.replace(old, new, count)


def apply_frontier_cache(source: str) -> str:
    start = "    def lineage_frontier(states: Sequence[tuple]) -> tuple[tuple, ...]:"
    end = "    def build_extreme_lookup("
    if source.count(start) != 1 or source.count(end) != 1 or "_h7_lf_cache" in source:
        raise SystemExit("frontier cache markers mismatch")
    source = source.replace(
        start,
        '    _h7_lf_cache = {}\n'
        '    _h7_lf_cache_stats = {"hits": 0, "misses": 0, "evictions": 0}\n\n' + start,
        1,
    )
    a=source.index(start);b=source.index(end,a+len(start));block=source[a:b]
    block=replace_exact(
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
        label="frontier cache entry"
    )
    block=replace_exact(
        block,
        "        return tuple(sorted(frontier, key=rank))",
        "        result = tuple(sorted(frontier, key=rank))\n"
        "        if cache_enabled:\n"
        "            if len(_h7_lf_cache) >= 128:\n"
        "                del _h7_lf_cache[next(iter(_h7_lf_cache))]\n"
        '                _h7_lf_cache_stats["evictions"] += 1\n'
        "            _h7_lf_cache[cache_key] = result\n"
        "        return result",
        label="frontier cache return"
    )
    source=source[:a]+block+source[b:]
    at=source.rfind("        return BehavioralSearchResult(")
    if at<0: raise SystemExit("frontier cache stats return marker missing")
    source=source[:at]+'        find_exact_expression._H7_LAST_LF_CACHE_STATS = dict(_h7_lf_cache_stats)\n'+source[at:]
    # Shared low-overhead identity markers make per-search telemetry distinguishable
    # across nested searches in the same Gen6 generation. Applied identically to both arms.
    doc = '''    """Exact discovery-set synthesis with behavior/provenance quotienting.

    OOD is intentionally not part of this function. The caller must verify OOD
    only after a discovery-qualified candidate has been selected.
    """
'''
    identity = '''    """Exact discovery-set synthesis with behavior/provenance quotienting.

    OOD is intentionally not part of this function. The caller must verify OOD
    only after a discovery-qualified candidate has been selected.
    """
    _H7_BUCKET_SEARCH_CALL_ID = int(getattr(find_exact_expression, "_H7_BUCKET_SEARCH_NEXT_CALL_ID", 0)) + 1
    find_exact_expression._H7_BUCKET_SEARCH_NEXT_CALL_ID = _H7_BUCKET_SEARCH_CALL_ID
    _H7_BUCKET_SEARCH_CALL_STARTED = __import__("time").perf_counter()
    _H7_BUCKET_SEARCH_FRAME_ID = id(__import__("sys")._getframe())
'''
    source = replace_exact(
        source,
        doc,
        identity,
        label="insert shared per-search telemetry identity",
    )
    return source


BUCKET_STATS = '''    _H7_BUCKET_DUPLICATE_STATS = {
        "metadata_only_bucket_hits": 0,
        "exact_seen_key_bails": 0,
        "parent_ast_allocations_avoided": 0,
        "lineage_keys_updated": 0,
    }

'''


METADATA_ONLY_BUCKET_BRANCH = '''        old_bucket = levels.get(depth, {})
        existing_bucket_state = old_bucket.get(state_key)
        if existing_bucket_state is not None:
            # Mirror register()'s same-bucket duplicate bookkeeping using the
            # exact child metadata already computed above. Do not construct a
            # parent AST that would be immediately discarded by register().
            if expression_ident in seen_expression_keys:
                stats["duplicate_states"] += 1
                stats["exact_expression_duplicates_skipped"] += 1
                _H7_BUCKET_DUPLICATE_STATS["exact_seen_key_bails"] += 1
                return None

            seen_expression_keys.add(expression_ident)
            if outputs == target and (not require_macro or state_used):
                target_candidate_found = True
                stats["target_directed_candidates"] += 1

            lineage_for_key = structural_lineage_by_key.setdefault(skey, set())
            before_lineage_count = len(lineage_for_key)
            lineage_for_key.update(direct_lineage)
            _H7_BUCKET_DUPLICATE_STATS["lineage_keys_updated"] += 1
            stats["max_lineage_coverage"] = max(
                int(stats["max_lineage_coverage"]), len(direct_lineage)
            )
            stats["max_structural_alias_coverage"] = max(
                int(stats["max_structural_alias_coverage"]), len(lineage_for_key)
            )

            if outputs == target and (not require_macro or state_used):
                stats["target_candidate_count"] += 1
                signature = "|".join(sorted(executable_lineage_ids(state_used)))
                signatures = stats["target_candidate_lineage_signatures"]
                if signature not in signatures and len(signatures) < 512:
                    signatures.append(signature)

            next_state_token += 1
            stats["generated_expressions"] += 1
            stats["duplicate_states"] += 1
            _H7_BUCKET_DUPLICATE_STATS["metadata_only_bucket_hits"] += 1
            _H7_BUCKET_DUPLICATE_STATS["parent_ast_allocations_avoided"] += 1
            return existing_bucket_state
'''


def apply_candidate(source: str) -> str:
    source=replace_exact(
        source,
        "    def register_binary(op: str, left: tuple, right: tuple):\n",
        BUCKET_STATS+"    def register_binary(op: str, left: tuple, right: tuple):\n",
        label="insert bucket fast-path counters",
    )
    old='''        old_bucket = levels.get(depth, {})
        if old_bucket.get(state_key) is not None:
            # Same semantic/cost bucket: ordinary register() keeps the current
            # canonical representative and records exact duplicate accounting.
            return register(
                {"op": op, "left": left[0], "right": right[0]},
                output_hint=outputs,
            )
'''
    source=replace_exact(
        source,
        old,
        METADATA_ONLY_BUCKET_BRANCH,
        label="replace AST-materializing same-bucket duplicate path",
    )
    marker="        find_exact_expression._H7_LAST_LF_CACHE_STATS = dict(_h7_lf_cache_stats)\n"
    source=replace_exact(
        source,
        marker,
        marker+"        find_exact_expression._H7_LAST_BUCKET_DUPLICATE_STATS = dict(_H7_BUCKET_DUPLICATE_STATS)\n",
        label="export metadata-only bucket counters",
    )
    return source


def main() -> int:
    ap=argparse.ArgumentParser()
    ap.add_argument("source",type=Path)
    ap.add_argument("--mode",choices=("control","candidate"),required=True)
    args=ap.parse_args()
    path=args.source
    source=path.read_text(encoding="utf-8")
    original_sha=hashlib.sha256(source.encode()).hexdigest()
    if original_sha!=EXPECTED_SOURCE_SHA256:
        raise SystemExit(f"pinned source SHA mismatch: expected {EXPECTED_SOURCE_SHA256}, got {original_sha}")
    if "_H7_BUCKET_DUPLICATE_STATS" in source or "_h7_lf_cache" in source:
        raise SystemExit("diagnostic patch markers already present")
    patched=apply_frontier_cache(source)
    if args.mode=="candidate":
        patched=apply_candidate(patched)
    compile(patched,str(path),"exec")
    patched_sha=hashlib.sha256(patched.encode()).hexdigest()
    path.write_text(patched,encoding="utf-8")
    print(f"MODE={args.mode}")
    print(f"ACSIE_SOURCE_COMMIT={SOURCE_COMMIT}")
    print(f"ACSIE_SOURCE_TREE={SOURCE_TREE}")
    print(f"ORIGINAL_SOURCE_SHA256={original_sha}")
    print(f"DIAGNOSTIC_SOURCE_SHA256={patched_sha}")
    print("FIXED_FACTOR=lineage-frontier FIFO cache capacity 128")
    if args.mode=="candidate":
        print("FACTOR=metadata-only same-bucket duplicate path; avoid materializing/serializing/discarding parent AST")
        print("PRESERVED=exact seen-key check; target flags/counters/signatures; structural-lineage map; next token and generation counters; same retained representative")
    else:
        print("FACTOR=original register() fallback on same-bucket duplicate")
    print("OTHER_FACTORS_UNCHANGED=no structural-ID interning; no macro-ID memoization; no rank-key reuse; no expansion cache; no grouping")
    return 0


if __name__=="__main__":
    raise SystemExit(main())
