#!/usr/bin/env python3
"""Source-pinned low-overhead phase counters for actual ACSIE Gen6.

The control and candidate use the same fixed 128-entry lineage-frontier cache.
Only the candidate adds integer counters and byte-length accounting. It does not
change search decisions, ranking, dominance, targets, or retained states.
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

def apply_profile_counters(source: str) -> str:
    if "_H7_PHASE_PROFILE" in source:
        raise SystemExit("phase profile markers already present")

    stats_end = '''        "target_candidate_lineage_signatures": [],
    }
'''
    profile = '''        "target_candidate_lineage_signatures": [],
    }

    # Diagnostic-only low-overhead work counters. Do not feed these into search
    # decisions or alter the scientific stats/result signature.
    _H7_PHASE_PROFILE = {
        "register_calls": 0,
        "exact_seen_duplicate_returns": 0,
        "register_eval_exceptions": 0,
        "register_same_bucket_duplicates": 0,
        "register_dominator_rejections": 0,
        "register_new_states": 0,
        "register_pruned_existing_states": 0,
        "register_binary_calls": 0,
        "register_binary_exact_seen_bails": 0,
        "register_binary_bucket_hits": 0,
        "register_binary_preflight_dominator_hits": 0,
        "register_binary_parent_materializations": 0,
        "register_binary_parent_key_chars": 0,
        "expr_key_calls": 0,
        "expr_key_output_chars": 0,
        "free_keys_calls": 0,
        "free_keys_input_json_chars": 0,
        "eval_vector_calls": 0,
        "eval_vector_input_json_chars": 0,
        "executable_cost_calls": 0,
        "executable_cost_input_json_chars": 0,
        "executable_cost_root_cache_hits": 0,
        "executable_cost_root_cache_misses": 0,
        "structural_expand_calls": 0,
        "structural_expand_macro_edges": 0,
        "structural_expand_max_active_macro_depth": 0,
        "structural_expand_commutative_sort_dumps": 0,
        "structural_expand_commutative_sort_chars": 0,
        "structural_key_calls": 0,
        "structural_key_output_chars": 0,
        "canonical_calls": 0,
        "canonical_sort_dumps": 0,
        "canonical_sort_chars": 0,
        "used_calls": 0,
        "used_walk_nodes": 0,
        "lineage_key_calls": 0,
        "executable_lineage_calls": 0,
        "executable_lineage_cache_hits": 0,
        "executable_lineage_cache_misses": 0,
        "executable_lineage_ids_returned": 0,
        "provenance_lineage_calls": 0,
        "provenance_lineage_cache_hits": 0,
        "provenance_lineage_cache_misses": 0,
        "dominance_predicate_calls": 0,
        "online_meta_calls": 0,
        "online_meta_cache_hits": 0,
        "frontier_calls": 0,
        "frontier_input_state_total": 0,
        "frontier_output_state_total": 0,
    }
'''
    source = replace_exact(source, stats_end, profile, label="declare phase profile")

    source = replace_exact(
        source,
        '''    def free_keys(expr: Mapping[str, Any]) -> tuple[str, ...]:
        ident = json.dumps(expr, sort_keys=True, separators=(",", ":"))''',
        '''    def free_keys(expr: Mapping[str, Any]) -> tuple[str, ...]:
        _H7_PHASE_PROFILE["free_keys_calls"] += 1
        ident = json.dumps(expr, sort_keys=True, separators=(",", ":"))
        _H7_PHASE_PROFILE["free_keys_input_json_chars"] += len(ident)''',
        label="instrument free-key serialization",
    )

    source = replace_exact(
        source,
        '''    def eval_vector(expr: Mapping[str, Any]) -> tuple[float, ...]:
        ident = json.dumps(expr, sort_keys=True, separators=(",", ":"))''',
        '''    def eval_vector(expr: Mapping[str, Any]) -> tuple[float, ...]:
        _H7_PHASE_PROFILE["eval_vector_calls"] += 1
        ident = json.dumps(expr, sort_keys=True, separators=(",", ":"))
        _H7_PHASE_PROFILE["eval_vector_input_json_chars"] += len(ident)''',
        label="instrument vector evaluation serialization",
    )

    source = replace_exact(
        source,
        '''    def _lineage_key(used_primitives: tuple[str, ...]) -> tuple[str, ...]:
        return tuple(sorted(set(used_primitives)))''',
        '''    def _lineage_key(used_primitives: tuple[str, ...]) -> tuple[str, ...]:
        _H7_PHASE_PROFILE["lineage_key_calls"] += 1
        return tuple(sorted(set(used_primitives)))''',
        label="instrument lineage-key calls",
    )

    source = replace_exact(
        source,
        '''    def provenance_lineage_ids(used_primitives: tuple[str, ...]) -> tuple[str, ...]:
        key = _lineage_key(used_primitives)
        cached = provenance_lineage_cache.get(key)
        if cached is not None:
            return cached''',
        '''    def provenance_lineage_ids(used_primitives: tuple[str, ...]) -> tuple[str, ...]:
        _H7_PHASE_PROFILE["provenance_lineage_calls"] += 1
        key = _lineage_key(used_primitives)
        cached = provenance_lineage_cache.get(key)
        if cached is not None:
            _H7_PHASE_PROFILE["provenance_lineage_cache_hits"] += 1
            return cached
        _H7_PHASE_PROFILE["provenance_lineage_cache_misses"] += 1''',
        label="instrument provenance-lineage cache",
    )

    source = replace_exact(
        source,
        '''    def executable_lineage_ids(used_primitives: tuple[str, ...]) -> tuple[str, ...]:
        key = _lineage_key(used_primitives)
        cached = executable_lineage_cache.get(key)
        if cached is not None:
            return cached''',
        '''    def executable_lineage_ids(used_primitives: tuple[str, ...]) -> tuple[str, ...]:
        _H7_PHASE_PROFILE["executable_lineage_calls"] += 1
        key = _lineage_key(used_primitives)
        cached = executable_lineage_cache.get(key)
        if cached is not None:
            _H7_PHASE_PROFILE["executable_lineage_cache_hits"] += 1
            _H7_PHASE_PROFILE["executable_lineage_ids_returned"] += len(cached)
            return cached
        _H7_PHASE_PROFILE["executable_lineage_cache_misses"] += 1''',
        label="instrument executable-lineage cache",
    )
    source = replace_exact(
        source,
        '''        result = tuple(sorted(lineage))
        executable_lineage_cache[key] = result
        return result''',
        '''        result = tuple(sorted(lineage))
        _H7_PHASE_PROFILE["executable_lineage_ids_returned"] += len(result)
        executable_lineage_cache[key] = result
        return result''',
        label="count executable-lineage output ids",
    )

    source = replace_exact(
        source,
        '''    def executable_cost(expr: Mapping[str, Any], active: tuple[str, ...] = ()) -> tuple[int, int]:
        ident = json.dumps(expr, sort_keys=True, separators=(",", ":"))
        if not active and ident in executable_cost_cache:
            return executable_cost_cache[ident]''',
        '''    def executable_cost(expr: Mapping[str, Any], active: tuple[str, ...] = ()) -> tuple[int, int]:
        _H7_PHASE_PROFILE["executable_cost_calls"] += 1
        ident = json.dumps(expr, sort_keys=True, separators=(",", ":"))
        _H7_PHASE_PROFILE["executable_cost_input_json_chars"] += len(ident)
        if not active and ident in executable_cost_cache:
            _H7_PHASE_PROFILE["executable_cost_root_cache_hits"] += 1
            return executable_cost_cache[ident]
        if not active:
            _H7_PHASE_PROFILE["executable_cost_root_cache_misses"] += 1''',
        label="instrument executable-cost traversal",
    )

    source = replace_exact(
        source,
        '''    def structural_expand(
        expr: Mapping[str, Any],
        active: tuple[str, ...] = (),
    ) -> Mapping[str, Any]:
        op = str(expr["op"])''',
        '''    def structural_expand(
        expr: Mapping[str, Any],
        active: tuple[str, ...] = (),
    ) -> Mapping[str, Any]:
        _H7_PHASE_PROFILE["structural_expand_calls"] += 1
        _H7_PHASE_PROFILE["structural_expand_max_active_macro_depth"] = max(
            _H7_PHASE_PROFILE["structural_expand_max_active_macro_depth"], len(active)
        )
        op = str(expr["op"])
        if op == "macro":
            _H7_PHASE_PROFILE["structural_expand_macro_edges"] += 1''',
        label="instrument structural expansion nodes",
    )
    source = replace_exact(
        source,
        '''            left_key = json.dumps(left, sort_keys=True, separators=(",", ":"))
            right_key = json.dumps(right, sort_keys=True, separators=(",", ":"))
            if left_key > right_key:''',
        '''            left_key = json.dumps(left, sort_keys=True, separators=(",", ":"))
            right_key = json.dumps(right, sort_keys=True, separators=(",", ":"))
            _H7_PHASE_PROFILE["structural_expand_commutative_sort_dumps"] += 2
            _H7_PHASE_PROFILE["structural_expand_commutative_sort_chars"] += len(left_key) + len(right_key)
            if left_key > right_key:''',
        label="count structural-expand child sorting dumps",
    )
    source = replace_exact(
        source,
        '''    def structural_key(expr: Mapping[str, Any]) -> str:
        return json.dumps(
            structural_expand(expr),
            sort_keys=True,
            separators=(",", ":"),
        )''',
        '''    def structural_key(expr: Mapping[str, Any]) -> str:
        _H7_PHASE_PROFILE["structural_key_calls"] += 1
        result = json.dumps(
            structural_expand(expr),
            sort_keys=True,
            separators=(",", ":"),
        )
        _H7_PHASE_PROFILE["structural_key_output_chars"] += len(result)
        return result''',
        label="instrument expanded structural-key serialization",
    )

    source = replace_exact(
        source,
        '''    def used(expr: Mapping[str, Any]) -> tuple[str, ...]:
        return tuple(
            sorted(
                {
                    str(node["id"])
                    for node in compiler._walk(expr)
                    if node.get("op") == "macro"
                }
            )
        )''',
        '''    def used(expr: Mapping[str, Any]) -> tuple[str, ...]:
        _H7_PHASE_PROFILE["used_calls"] += 1
        found: set[str] = set()
        for node in compiler._walk(expr):
            _H7_PHASE_PROFILE["used_walk_nodes"] += 1
            if node.get("op") == "macro":
                found.add(str(node["id"]))
        return tuple(sorted(found))''',
        label="instrument expression macro walk",
    )

    source = replace_exact(
        source,
        '''    def canonical(expr: Mapping[str, Any]) -> Mapping[str, Any]:
        op = str(expr["op"])
        if op in {"add", "mul", "max", "min"}:''',
        '''    def canonical(expr: Mapping[str, Any]) -> Mapping[str, Any]:
        _H7_PHASE_PROFILE["canonical_calls"] += 1
        op = str(expr["op"])
        if op in {"add", "mul", "max", "min"}:''',
        label="instrument canonicalization calls",
    )
    source = replace_exact(
        source,
        '''            right_key = json.dumps(
                expr["right"], sort_keys=True, separators=(",", ":")
            )
            if left_key > right_key:''',
        '''            right_key = json.dumps(
                expr["right"], sort_keys=True, separators=(",", ":")
            )
            _H7_PHASE_PROFILE["canonical_sort_dumps"] += 2
            _H7_PHASE_PROFILE["canonical_sort_chars"] += len(left_key) + len(right_key)
            if left_key > right_key:''',
        label="count canonicalization sort dumps",
    )
    source = replace_exact(
        source,
        '''    def expr_key(expr: Mapping[str, Any]) -> str:
        return json.dumps(expr, sort_keys=True, separators=(",", ":"))''',
        '''    def expr_key(expr: Mapping[str, Any]) -> str:
        result = json.dumps(expr, sort_keys=True, separators=(",", ":"))
        _H7_PHASE_PROFILE["expr_key_calls"] += 1
        _H7_PHASE_PROFILE["expr_key_output_chars"] += len(result)
        return result''',
        label="instrument expression-key JSON",
    )

    source = replace_exact(
        source,
        '''    def dominates(left: tuple, right: tuple) -> bool:
        # A composite alias may have a larger transitive lineage while''',
        '''    def dominates(left: tuple, right: tuple) -> bool:
        _H7_PHASE_PROFILE["dominance_predicate_calls"] += 1
        # A composite alias may have a larger transitive lineage while''',
        label="instrument dominance predicate calls",
    )

    source = replace_exact(
        source,
        '''    def _H7_meta(state: tuple) -> dict:
        token = state[5]
        cached = _H7_online_metadata.get(token)
        if cached is not None:
            return cached''',
        '''    def _H7_meta(state: tuple) -> dict:
        _H7_PHASE_PROFILE["online_meta_calls"] += 1
        token = state[5]
        cached = _H7_online_metadata.get(token)
        if cached is not None:
            _H7_PHASE_PROFILE["online_meta_cache_hits"] += 1
            return cached''',
        label="instrument online-metadata cache",
    )

    source = replace_exact(
        source,
        '''    def register(expr: Mapping[str, Any], output_hint: tuple[float, ...] | None = None, metadata_hint: Mapping[str, Any] | None = None):
        nonlocal next_state_token, target_candidate_found
        if metadata_hint is None:''',
        '''    def register(expr: Mapping[str, Any], output_hint: tuple[float, ...] | None = None, metadata_hint: Mapping[str, Any] | None = None):
        nonlocal next_state_token, target_candidate_found
        _H7_PHASE_PROFILE["register_calls"] += 1
        if metadata_hint is None:''',
        label="instrument register calls",
    )
    duplicate_bail = '''        if expression_ident in seen_expression_keys:
            stats["duplicate_states"] += 1
            stats["exact_expression_duplicates_skipped"] += 1
            return None'''
    duplicate_bail_profiled = '''        if expression_ident in seen_expression_keys:
            _H7_PHASE_PROFILE["exact_seen_duplicate_returns"] += 1
            stats["duplicate_states"] += 1
            stats["exact_expression_duplicates_skipped"] += 1
            return None'''
    # The same short bail appears in register() and register_binary(); count
    # each returned duplicate here, while also recording the binary subset in
    # register_binary's dedicated counter below.
    source = replace_exact(
        source, duplicate_bail, duplicate_bail_profiled, count=2,
        label="count exact-key duplicate returns in both registration paths",
    )
    source = replace_exact(
        source,
        '''        except Exception:
            return None

        if outputs == target and (''',
        '''        except Exception:
            _H7_PHASE_PROFILE["register_eval_exceptions"] += 1
            return None

        if outputs == target and (''',
        label="count register evaluation failures",
    )
    source = replace_exact(
        source,
        '''        if old is not None:
            stats["duplicate_states"] += 1
            # The bucket key now means equal observations,''',
        '''        if old is not None:
            _H7_PHASE_PROFILE["register_same_bucket_duplicates"] += 1
            stats["duplicate_states"] += 1
            # The bucket key now means equal observations,''',
        label="count register bucket duplicates",
    )
    source = replace_exact(
        source,
        '''        if existing is not None:
            if not dominates(existing, state):''',
        '''        if existing is not None:
            _H7_PHASE_PROFILE["register_dominator_rejections"] += 1
            if not dominates(existing, state):''',
        label="count register dominance rejects",
    )
    source = replace_exact(
        source,
        '''        for existing in dominated_existing:
            dominated_state_tokens.add(existing[5])''',
        '''        _H7_PHASE_PROFILE["register_pruned_existing_states"] += len(dominated_existing)
        for existing in dominated_existing:
            dominated_state_tokens.add(existing[5])''',
        label="count existing states pruned by registration",
    )
    source = replace_exact(
        source,
        '''        stats["unique_states"] += 1
        stats["max_states_in_depth"] = max(''',
        '''        _H7_PHASE_PROFILE["register_new_states"] += 1
        stats["unique_states"] += 1
        stats["max_states_in_depth"] = max(''',
        label="count new registered states",
    )

    source = replace_exact(
        source,
        '''    def register_binary(op: str, left: tuple, right: tuple):
        """Preflight exact binary dominance before constructing an AST dict."""
        nonlocal next_state_token, target_candidate_found''',
        '''    def register_binary(op: str, left: tuple, right: tuple):
        """Preflight exact binary dominance before constructing an AST dict."""
        nonlocal next_state_token, target_candidate_found
        _H7_PHASE_PROFILE["register_binary_calls"] += 1''',
        label="instrument register_binary calls",
    )
    source = replace_exact(
        source,
        '''        expression_ident = (
            '{"left":' + left_key + ',"op":' + json.dumps(op)
            + ',"right":' + right_key + '}'
        )
        if expression_ident in seen_expression_keys:''',
        '''        expression_ident = (
            '{"left":' + left_key + ',"op":' + json.dumps(op)
            + ',"right":' + right_key + '}'
        )
        _H7_PHASE_PROFILE["register_binary_parent_key_chars"] += len(expression_ident)
        if expression_ident in seen_expression_keys:''',
        label="count binary expression-key serialization length",
    )
    source = replace_exact(
        source,
        '''        if expression_ident in seen_expression_keys:
            _H7_PHASE_PROFILE["exact_seen_duplicate_returns"] += 1
            stats["duplicate_states"] += 1
            stats["exact_expression_duplicates_skipped"] += 1
            return None

        lvec, rvec = left[1], right[1]''',
        '''        if expression_ident in seen_expression_keys:
            _H7_PHASE_PROFILE["exact_seen_duplicate_returns"] += 1
            _H7_PHASE_PROFILE["register_binary_exact_seen_bails"] += 1
            stats["duplicate_states"] += 1
            stats["exact_expression_duplicates_skipped"] += 1
            return None

        lvec, rvec = left[1], right[1]''',
        label="count binary exact-key bails",
    )
    source = replace_exact(
        source,
        '''        if old_bucket.get(state_key) is not None:
            # Same semantic/cost bucket: ordinary register() keeps the current''',
        '''        if old_bucket.get(state_key) is not None:
            _H7_PHASE_PROFILE["register_binary_bucket_hits"] += 1
            # Same semantic/cost bucket: ordinary register() keeps the current''',
        label="count binary bucket hits",
    )
    source = replace_exact(
        source,
        '''        if existing is None:
            # Materialize one canonical parent AST, then hand off preflight''',
        '''        if existing is None:
            _H7_PHASE_PROFILE["register_binary_parent_materializations"] += 1
            # Materialize one canonical parent AST, then hand off preflight''',
        label="count parent AST materializations",
    )
    source = replace_exact(
        source,
        '''        # Exact preflight hit: reproduce register's semantic bookkeeping while
        # skipping parent-AST allocation and recursive vector evaluation.
        seen_expression_keys.add(expression_ident)''',
        '''        # Exact preflight hit: reproduce register's semantic bookkeeping while
        # skipping parent-AST allocation and recursive vector evaluation.
        _H7_PHASE_PROFILE["register_binary_preflight_dominator_hits"] += 1
        seen_expression_keys.add(expression_ident)''',
        label="count binary preflight dominator hits",
    )

    source = replace_exact(
        source,
        '''    def lineage_frontier(states: Sequence[tuple]) -> tuple[tuple, ...]:
        """Return context-local Pareto frontiers without collapsing search-budget trade-offs."""
        if not states:''',
        '''    def lineage_frontier(states: Sequence[tuple]) -> tuple[tuple, ...]:
        """Return context-local Pareto frontiers without collapsing search-budget trade-offs."""
        _H7_PHASE_PROFILE["frontier_calls"] += 1
        _H7_PHASE_PROFILE["frontier_input_state_total"] += len(states)
        if not states:''',
        label="instrument final frontier reduction",
    )
    source = replace_exact(
        source,
        '''        return tuple(sorted(frontier, key=rank))''',
        '''        result = tuple(sorted(frontier, key=rank))
        _H7_PHASE_PROFILE["frontier_output_state_total"] += len(result)
        return result''',
        label="count final frontier reduction outputs",
    )
    source = replace_exact(
        source,
        '''        find_exact_expression._H7_LAST_LF_CACHE_STATS = dict(_h7_lf_cache_stats)
        return BehavioralSearchResult(''',
        '''        find_exact_expression._H7_LAST_LF_CACHE_STATS = dict(_h7_lf_cache_stats)
        find_exact_expression._H7_LAST_PHASE_PROFILE = dict(_H7_PHASE_PROFILE)
        return BehavioralSearchResult(''',
        label="export profile counters on successful search",
    )
    # The final no-target return also publishes counters for fixtures whose
    # scientific result is None. Do not change the return value.
    source = replace_exact(
        source,
        '''    stats["status"] = "TARGET_NOT_FOUND"
    stats["depths_completed"] = completed
    return None''',
        '''    stats["status"] = "TARGET_NOT_FOUND"
    stats["depths_completed"] = completed
    find_exact_expression._H7_LAST_LF_CACHE_STATS = dict(_h7_lf_cache_stats)
    find_exact_expression._H7_LAST_PHASE_PROFILE = dict(_H7_PHASE_PROFILE)
    return None''',
        label="export profile counters on no-target result",
    )
    compile(source, "cognitive_core/behavioral_search.py", "exec")
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
    if "_H7_PHASE_PROFILE" in source or "_h7_lf_cache" in source:
        raise SystemExit("phase profile patch markers already present")
    # Fixed algorithm factor for both arms: the existing H7 lineage-frontier
    # FIFO cache (capacity 128). No bucket-fast-path or key-lifetime treatment.
    from patch_h7_metadata_only_bucket_duplicate import apply_frontier_cache
    patched=apply_frontier_cache(source)
    if args.mode=="candidate":
        patched=apply_profile_counters(patched)
    compile(patched,str(path),"exec")
    path.write_text(patched,encoding="utf-8")
    print(f"MODE={args.mode}")
    print(f"ACSIE_SOURCE_COMMIT={SOURCE_COMMIT}")
    print(f"ACSIE_SOURCE_TREE={SOURCE_TREE}")
    print(f"ORIGINAL_SOURCE_SHA256={original_sha}")
    print(f"DIAGNOSTIC_SOURCE_SHA256={hashlib.sha256(patched.encode()).hexdigest()}")
    print("FIXED_FACTOR=lineage-frontier FIFO cache capacity 128 in both arms")
    print("FACTOR=lightweight integer work counters and serialization-size accounting" if args.mode=="candidate" else "FACTOR=telemetry disabled; same fixed frontier cache")
    print("UNCHANGED=dominance, state identity, ranking, objectives, selection, targets and source-level scientific gates")
    return 0

if __name__=="__main__":
    raise SystemExit(main())
