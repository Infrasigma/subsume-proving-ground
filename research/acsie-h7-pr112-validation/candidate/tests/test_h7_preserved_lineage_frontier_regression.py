from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler


def test_lineage_frontier_preserves_incomparable_direct_macro_vocabularies_with_shared_transitive_lineage():
    from cognitive_core.behavioral_search import find_exact_expression
    from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive, Trace

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    p_a = CognitivePrimitive(
        "prim:a", {"op": "get", "key": "x"}, ("x",), "delta", 0,
        (), ("opaque",), 0.0, 0.0, 0.0, 1, {},
    )
    p_b = CognitivePrimitive(
        "prim:b", {"op": "get", "key": "x"}, ("x",), "delta", 0,
        (), ("opaque",), 0.0, 0.0, 0.0, 1, {},
    )
    p_c = CognitivePrimitive(
        "prim:c",
        {
            "op": "add",
            "left": {"op": "macro", "id": "prim:a"},
            "right": {"op": "macro", "id": "prim:b"},
        },
        ("x",), "delta", 1,
        ("prim:a", "prim:b"), ("opaque",), 0.0, 0.0, 0.0, 3, {},
    )
    c.primitives = {p.primitive_id: p for p in (p_a, p_b, p_c)}
    c.archive = {pid: ArchiveRecord(p) for pid, p in c.primitives.items()}
    rows = tuple(
        Trace({"x": float(i)}, float(3 * i), "opaque", {}, f"shared-lineage-{i}")
        for i in range(1, 9)
    )

    result = find_exact_expression(c, rows, max_depth=2, require_macro=True)
    assert result is not None
    direct_vocabularies = {frozenset(used) for _, used in result.alternatives}
    assert frozenset({"prim:a", "prim:c"}) in direct_vocabularies
    assert frozenset({"prim:b", "prim:c"}) in direct_vocabularies
