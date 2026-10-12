#!/usr/bin/env python3
"""Exact-source diagnostic: cache expanded macro AST subtrees when the macro graph is acyclic.

Both arms share the fixed 128-entry lineage-frontier cache. Candidate alone
memoizes the structurally expanded Mapping for each primitive macro ID. The
whole-search cache is disabled if any primitive dependency cycle exists, so
active-stack cycle semantics remain unchanged.
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
        raise SystemExit(f"{label}: expected {count} marker(s), found {observed}: {old[:120]!r}")
    return source.replace(old, new, count)

GRAPH_HELPER = '''def _h7_expanded_macro_graph_is_acyclic(dependencies):
    """Return True iff the complete finite primitive-macro dependency graph is acyclic."""
    marks = {}

    def visit(node):
        mark = marks.get(node, 0)
        if mark == 1:
            return False
        if mark == 2:
            return True
        marks[node] = 1
        for child in dependencies.get(node, ()):
            if child in dependencies and not visit(child):
                return False
        marks[node] = 2
        return True

    return all(visit(node) for node in dependencies)


'''

CACHE_SETUP = '''    # Memoize expanded *trees*, not only their numeric/string IDs. This is
    # exact only when no macro expansion depends on the current active stack.
    # If any cycle exists anywhere in the primitive graph, bypass the cache
    # for the whole call and preserve the original active-stack semantics.
    _H7_EXPANDED_MACRO_DEPENDENCIES = {}
    for _h7_mid, _h7_body in pmap.items():
        _h7_refs = set()
        for _h7_node in compiler._walk(_h7_body):
            if _h7_node.get("op") == "macro":
                _h7_child = str(_h7_node.get("id"))
                if _h7_child in pmap:
                    _h7_refs.add(_h7_child)
        _H7_EXPANDED_MACRO_DEPENDENCIES[str(_h7_mid)] = _h7_refs
    _H7_EXPANDED_MACRO_CACHE_ENABLED = _h7_expanded_macro_graph_is_acyclic(
        _H7_EXPANDED_MACRO_DEPENDENCIES
    )
    _H7_EXPANDED_MACRO_TREE_CACHE = {}
    _H7_EXPANDED_MACRO_CACHE_STATS = {
        "enabled": _H7_EXPANDED_MACRO_CACHE_ENABLED,
        "dependency_nodes": len(_H7_EXPANDED_MACRO_DEPENDENCIES),
        "macro_tree_cache_hits": 0,
        "macro_tree_cache_misses": 0,
        "cycle_cache_fallbacks": 0,
    }

'''

OLD_MACRO_BRANCH = '''        if op == "macro":
            mid = str(expr["id"])
            if mid in active or mid not in pmap:
                return {"op": "macro", "id": mid}
            return structural_expand(pmap[mid], active + (mid,))'''

NEW_MACRO_BRANCH = '''        if op == "macro":
            mid = str(expr["id"])
            if mid in active or mid not in pmap:
                return {"op": "macro", "id": mid}
            if _H7_EXPANDED_MACRO_CACHE_ENABLED:
                cached = _H7_EXPANDED_MACRO_TREE_CACHE.get(mid)
                if cached is not None:
                    _H7_EXPANDED_MACRO_CACHE_STATS["macro_tree_cache_hits"] += 1
                    return cached
                _H7_EXPANDED_MACRO_CACHE_STATS["macro_tree_cache_misses"] += 1
                expanded = structural_expand(pmap[mid], active + (mid,))
                _H7_EXPANDED_MACRO_TREE_CACHE[mid] = expanded
                return expanded
            _H7_EXPANDED_MACRO_CACHE_STATS["cycle_cache_fallbacks"] += 1
            return structural_expand(pmap[mid], active + (mid,))'''

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("source", type=Path)
    ap.add_argument("--mode", choices=("control","candidate"), required=True)
    args = ap.parse_args()
    path = args.source
    source = path.read_text(encoding="utf-8")
    original_sha = hashlib.sha256(source.encode()).hexdigest()
    if original_sha != EXPECTED_SOURCE_SHA256:
        raise SystemExit(f"pinned source SHA mismatch: expected {EXPECTED_SOURCE_SHA256}, got {original_sha}")
    if "_H7_EXPANDED_MACRO_TREE_CACHE" in source or "_h7_lf_cache" in source:
        raise SystemExit("expanded-macro-tree cache markers already exist")

    # Both arms receive the exact same fixed-capacity lineage-frontier cache.
    from patch_h7_metadata_only_bucket_duplicate import apply_frontier_cache
    patched = apply_frontier_cache(source)

    if args.mode == "candidate":
        patched = replace_exact(
            patched,
            "def find_exact_expression(",
            GRAPH_HELPER + "def find_exact_expression(",
            label="insert acyclic macro graph classifier",
        )
        patched = replace_exact(
            patched,
            "    def structural_expand(\n",
            CACHE_SETUP + "    def structural_expand(\n",
            label="insert macro expansion tree cache",
        )
        patched = replace_exact(
            patched,
            OLD_MACRO_BRANCH,
            NEW_MACRO_BRANCH,
            label="memoize exact expanded macro trees",
        )
        success_return = "        return BehavioralSearchResult(\n"
        patched = replace_exact(
            patched,
            success_return,
            '        find_exact_expression._H7_LAST_EXPANDED_MACRO_CACHE_STATS = dict(_H7_EXPANDED_MACRO_CACHE_STATS)\n'
            + '        find_exact_expression._H7_LAST_EXPANDED_MACRO_CACHE_TREE_ENTRIES = len(_H7_EXPANDED_MACRO_TREE_CACHE)\n'
            + success_return,
            label="export cache counters on found result",
        )
        no_target = '''    stats["status"] = "TARGET_NOT_FOUND"
    stats["depths_completed"] = completed
    return None'''
        no_target_replacement = '''    stats["status"] = "TARGET_NOT_FOUND"
    stats["depths_completed"] = completed
    find_exact_expression._H7_LAST_EXPANDED_MACRO_CACHE_STATS = dict(_H7_EXPANDED_MACRO_CACHE_STATS)
    find_exact_expression._H7_LAST_EXPANDED_MACRO_CACHE_TREE_ENTRIES = len(_H7_EXPANDED_MACRO_TREE_CACHE)
    return None'''
        patched = replace_exact(
            patched,
            no_target,
            no_target_replacement,
            label="export cache counters on no-target result",
        )

        # Fail closed if the graph classifier mishandles cycles or unresolved IDs.
        scope = {}
        start = patched.index("def _h7_expanded_macro_graph_is_acyclic(")
        end = patched.index("\ndef find_exact_expression(", start)
        exec(compile(patched[start:end], "<expanded-macro-graph-classifier>", "exec"), scope)
        acyclic = scope["_h7_expanded_macro_graph_is_acyclic"]
        cases = {
            "empty": ({}, True),
            "chain": ({"a":{"b"},"b":{"c"},"c":set()}, True),
            "diamond": ({"a":{"b","c"},"b":{"d"},"c":{"d"},"d":set()}, True),
            "self_cycle": ({"a":{"a"}}, False),
            "mutual_cycle": ({"a":{"b"},"b":{"a"}}, False),
            "disconnected_cycle_disables_global_cache": ({"a":set(),"b":{"c"},"c":{"b"}}, False),
            "unresolved_reference_is_not_a_cycle": ({"a":{"missing"}}, True),
        }
        observed = {name: acyclic(graph) for name,(graph,_) in cases.items()}
        failed = {name:{"expected":expected,"observed":observed[name]}
                  for name,(_,expected) in cases.items() if observed[name] != expected}
        if failed:
            raise SystemExit(f"expanded-macro graph classifier failed: {failed}")
        print("GRAPH_CLASSIFIER_TESTS=" + ",".join(f"{k}:{observed[k]}" for k in cases))

    compile(patched, str(path), "exec")
    final_sha = hashlib.sha256(patched.encode()).hexdigest()
    path.write_text(patched, encoding="utf-8")
    print(f"MODE={args.mode}")
    print(f"ACSIE_SOURCE_COMMIT={SOURCE_COMMIT}")
    print(f"ACSIE_SOURCE_TREE={SOURCE_TREE}")
    print(f"ORIGINAL_SOURCE_SHA256={original_sha}")
    print(f"DIAGNOSTIC_SOURCE_SHA256={final_sha}")
    print("FIXED_FACTOR=lineage-frontier FIFO cache capacity 128 in both arms")
    print("FACTOR=acyclic-only per-primitive structural_expand Mapping cache" if args.mode == "candidate" else "FACTOR=expanded-macro-tree cache disabled")
    print("CYCLE_POLICY=disable macro-tree cache globally if any primitive dependency cycle exists")
    print("OTHER_FACTORS=structural IDs OFF; macro-ID string cache OFF; grouping OFF; no frontier rule changes")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
