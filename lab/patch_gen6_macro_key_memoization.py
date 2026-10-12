#!/usr/bin/env python3
"""Enable cycle-safe primitive-macro structural-ID memoization.

This patch applies only to the isolated exact-source structural-ID diagnostic
snapshot in proving-ground worktrees. It never modifies ACSIE production refs.
"""
from __future__ import annotations

import argparse
import hashlib
from pathlib import Path

EXPECTED_ID_PATCH_SHA256 = "9b53f025955bd8e72249679300efae4343abec586421db0ff39a6e2655aad061"
SOURCE_COMMIT = "d83cb60286f195eec02b4d7aa45a60539316d115"


def replace_exact(source: str, old: str, new: str, *, count: int = 1, label: str) -> str:
    observed = source.count(old)
    if observed != count:
        raise SystemExit(f"{label}: expected {count} marker(s), found {observed}: {old[:100]!r}")
    return source.replace(old, new, count)


GRAPH_HELPER = '''def _h7_macro_dependency_graph_is_acyclic(dependencies):
    """Return True exactly when the finite directed dependency graph is acyclic."""
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


CACHE_SETUP = '''    # Cache a primitive body's expanded structural ID only when the complete
    # active primitive-macro dependency graph is acyclic. If any cycle exists,
    # bypass this cache for the entire search, preserving active-stack semantics.
    _H7_MACRO_DEPENDENCY_GRAPH = {}
    for _h7_mid, _h7_body in pmap.items():
        _h7_refs = set()
        for _h7_node in compiler._walk(_h7_body):
            if _h7_node.get("op") == "macro":
                _h7_child = str(_h7_node.get("id"))
                if _h7_child in pmap:
                    _h7_refs.add(_h7_child)
        _H7_MACRO_DEPENDENCY_GRAPH[str(_h7_mid)] = _h7_refs
    _H7_MACRO_ID_CACHE_ENABLED = _h7_macro_dependency_graph_is_acyclic(
        _H7_MACRO_DEPENDENCY_GRAPH
    )
    _H7_MACRO_STRUCTURAL_ID_CACHE = {}
    _H7_STRUCTURAL_ID_STATS.update({
        "macro_id_cache_enabled": _H7_MACRO_ID_CACHE_ENABLED,
        "macro_dependency_node_count": len(_H7_MACRO_DEPENDENCY_GRAPH),
        "macro_id_cache_hits": 0,
        "macro_id_cache_misses": 0,
        "macro_id_cache_saved_expansions": 0,
        "macro_id_cache_cycle_fallbacks": 0,
    })

'''


OLD_MACRO_BRANCH = '''            _H7_STRUCTURAL_ID_STATS["macro_expansion_edges"] += 1
            return structural_key(pmap[mid], active + (mid,))'''

NEW_MACRO_BRANCH = '''            if _H7_MACRO_ID_CACHE_ENABLED:
                cached_macro_id = _H7_MACRO_STRUCTURAL_ID_CACHE.get(mid)
                if cached_macro_id is not None:
                    _H7_STRUCTURAL_ID_STATS["macro_id_cache_hits"] += 1
                    _H7_STRUCTURAL_ID_STATS["macro_id_cache_saved_expansions"] += 1
                    return cached_macro_id
                _H7_STRUCTURAL_ID_STATS["macro_id_cache_misses"] += 1
            else:
                _H7_STRUCTURAL_ID_STATS["macro_id_cache_cycle_fallbacks"] += 1
            _H7_STRUCTURAL_ID_STATS["macro_expansion_edges"] += 1
            result = structural_key(pmap[mid], active + (mid,))
            if _H7_MACRO_ID_CACHE_ENABLED:
                _H7_MACRO_STRUCTURAL_ID_CACHE[mid] = result
            return result'''


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("source", type=Path)
    args = ap.parse_args()
    path = args.source
    source = path.read_text(encoding="utf-8")
    sha = hashlib.sha256(source.encode()).hexdigest()
    if sha != EXPECTED_ID_PATCH_SHA256:
        raise SystemExit(
            "unexpected base structural-ID diagnostic source: "
            f"expected {EXPECTED_ID_PATCH_SHA256}, got {sha}"
        )
    if "_H7_MACRO_STRUCTURAL_ID_CACHE" in source:
        raise SystemExit("macro-ID cache patch marker already present")

    marker = "def find_exact_expression("
    if source.count(marker) != 1:
        raise SystemExit("find_exact_expression module boundary marker mismatch")
    source = source.replace(marker, GRAPH_HELPER + marker, 1)

    marker = "    def structural_key(expr: Mapping[str, Any], active: tuple[str, ...] = ()) -> int:\n"
    source = replace_exact(
        source,
        marker,
        CACHE_SETUP + marker,
        label="inject macro DAG cache setup",
    )
    source = replace_exact(
        source,
        OLD_MACRO_BRANCH,
        NEW_MACRO_BRANCH,
        label="memoize safe macro-body IDs",
    )

    # Validate the exact graph classifier against cyclic, acyclic, disconnected,
    # and unresolved-reference examples before writing the ephemeral patch.
    scope = {}
    graph_code_start = source.index("def _h7_macro_dependency_graph_is_acyclic(")
    graph_code_end = source.index("\ndef find_exact_expression(", graph_code_start)
    exec(compile(source[graph_code_start:graph_code_end], "<macro-graph-classifier>", "exec"), scope)
    acyclic = scope["_h7_macro_dependency_graph_is_acyclic"]
    cases = {
        "empty": ({}, True),
        "chain": ({"a": {"b"}, "b": {"c"}, "c": set()}, True),
        "diamond": ({"a": {"b", "c"}, "b": {"d"}, "c": {"d"}, "d": set()}, True),
        "self_cycle": ({"a": {"a"}}, False),
        "mutual_cycle": ({"a": {"b"}, "b": {"a"}}, False),
        "disconnected_cycle_disables_global_cache": ({"a": set(), "b": {"c"}, "c": {"b"}}, False),
        "unresolved_reference_is_not_a_cycle": ({"a": {"missing"}}, True),
    }
    outcomes = {name: acyclic(graph) for name, (graph, _) in cases.items()}
    failed = {name: {"expected": expected, "observed": outcomes[name]}
              for name, (_, expected) in cases.items() if outcomes[name] != expected}
    if failed:
        raise SystemExit(f"macro dependency classifier failed: {failed}")

    final_sha = hashlib.sha256(source.encode()).hexdigest()
    path.write_text(source, encoding="utf-8")
    print(f"ACSIE_SOURCE_COMMIT={SOURCE_COMMIT}")
    print(f"BASE_STRUCTURAL_ID_SHA256={sha}")
    print(f"MACRO_ID_CACHE_DIAGNOSTIC_SHA256={final_sha}")
    print("CACHE_POLICY=cache expanded structural ID per primitive macro ID only if the full primitive dependency graph is acyclic")
    print("CYCLE_POLICY=if any dependency cycle exists, disable macro-ID caching for the whole search")
    print("GRAPH_CLASSIFIER_TESTS=" + ",".join(f"{k}:{outcomes[k]}" for k in cases))
    print("OTHER_FACTORS_UNCHANGED=structural-ID representation; frontier cache 128; no expansion cache; no direct AST-key cache; no grouping")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
