from cognitive_core.recursive_cognitive_compiler import (
    CognitivePrimitive,
    RecursiveCognitiveCompiler,
)


def primitive(pid, expression):
    return CognitivePrimitive(
        primitive_id=pid,
        expression=expression,
        input_roles=("x",),
        output_semantics="delta",
        generation=0,
        parent_ids=(),
        source_families=("test",),
        train_error=0.0,
        holdout_error=0.0,
        transfer_error=0.0,
        complexity=1,
        provenance={"external_model": False},
    )


def test_executable_lineage_cache_preserves_transitive_semantics():
    compiler = RecursiveCognitiveCompiler(seed=7)
    compiler.primitives["p0"] = primitive(
        "p0",
        {"op": "get", "key": "x"},
    )
    compiler.primitives["p1"] = primitive(
        "p1",
        {
            "op": "add",
            "left": {"op": "macro", "id": "p0"},
            "right": {"op": "const", "value": 1},
        },
    )

    assert compiler.executable_lineage("p1") == ("p0", "p1")
    assert compiler.executable_lineage("p1") == ("p0", "p1")
    assert compiler._executable_lineage_cache["p1"] == ("p0", "p1")

    compiler.primitives["p2"] = primitive(
        "p2",
        {
            "op": "mul",
            "left": {"op": "macro", "id": "p1"},
            "right": {"op": "const", "value": 2},
        },
    )

    assert compiler.executable_lineage("p2") == ("p0", "p1", "p2")
    assert compiler.executable_lineage("p1") == ("p0", "p1")
