from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import Evaluation, ProcessCandidate


def test_search_depth_has_no_fixed_ceiling():
    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    policies = c.propose_search_policies()
    assert {p["max_depth"] for p in policies} == {2, 3, 4}


def test_productive_evidence_expands_future_search_depth():
    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    proc = ProcessCandidate(
        "p",
        {"op": "get", "key": "x"},
        (),
        0,
        (),
        {},
        (),
        1,
        {"origin": "unit"},
    )
    ev = Evaluation(0.0, 0.0, 0.0, 0.0, 1.0, 1.0, 1.0, True)
    assert c.retain(proc, ev, novelty=1.0)
    assert c.meta_policy["max_depth"] > 2
    assert c.max_observed_depth == c.meta_policy["max_depth"]


def test_nonaccepted_evidence_does_not_expand_search():
    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    proc = ProcessCandidate(
        "p",
        {"op": "get", "key": "x"},
        (),
        0,
        (),
        {},
        (),
        1,
        {"origin": "unit"},
    )
    ev = Evaluation(0.0, 1.0, 1.0, 1.0, 0.0, 1.0, 0.5, False)
    assert not c.retain(proc, ev, novelty=1.0)
    assert c.meta_policy["max_depth"] == 2



def test_recursive_primitive_invention_accepts_tuple_transfer_rows():
    from cognitive_core.recursive_cognitive_compiler import Trace

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    train = tuple(
        Trace({"x": float(i)}, float(i), "opaque", {}, f"train-{i}")
        for i in range(6)
    )
    holdout = tuple(
        Trace({"x": float(i)}, float(i), "opaque", {}, f"holdout-{i}")
        for i in range(2)
    )
    transfer = tuple(
        Trace({"x": float(i)}, float(i), "opaque", {}, f"transfer-{i}")
        for i in range(4)
    )
    primitive = c.invent_primitive(train, holdout, transfer)
    assert primitive is not None
    assert primitive.transfer_error <= 1e-9



def test_recursive_language_closes_over_unary_composition():
    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    target = {"op": "abs", "arg": {"op": "get", "key": "y"}}
    first = next(
        candidate
        for candidate in c._expression_stream(("x", "y"), (), 2)
        if candidate == target
    )
    assert first == target


def test_complete_lazy_stream_reaches_macro_binary_composition():
    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    target = {
        "op": "mul",
        "left": {"op": "macro", "id": "prim:parent"},
        "right": {"op": "get", "key": "y"},
    }
    first = next(
        candidate
        for candidate in c._expression_stream(("x", "y"), ("prim:parent",), 2)
        if candidate == target
    )
    assert first == target


def test_retained_macro_is_search_prioritized_after_bootstrap():
    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    stream = iter(c._expression_stream(("x", "y"), ("prim:parent",), 2))
    first = next(stream)
    assert first == {"op": "macro", "id": "prim:parent"}



def test_process_synthesis_accepts_tuple_transfer_rows():
    from cognitive_core.recursive_cognitive_compiler import Trace

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=1)
    c.propose_search_policies = lambda: (
        {"max_depth": 1, "novelty_weight": 0.0, "exploration": 0.0, "mutation_rate": 0.0},
    )
    rows = tuple(
        Trace({"x": float(i)}, float(i), "opaque", {}, f"row-{i}")
        for i in range(8)
    )
    transfer = tuple(
        Trace({"x": float(i) + 0.5}, float(i) + 0.5, "opaque", {}, f"transfer-{i}")
        for i in range(4)
    )
    ood = tuple(
        Trace({"x": float(i) - 0.25}, float(i) - 0.25, "opaque", {}, f"ood-{i}")
        for i in range(4)
    )
    process = c.synthesize_process(rows, transfer, ood)
    assert process is not None



def test_self_extending_process_search_reuses_retained_primitive():
    from cognitive_core.recursive_cognitive_compiler import Trace

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    train = tuple(
        Trace({"x": float(i)}, float(i), "opaque", {}, f"bootstrap-{i}")
        for i in range(8)
    )
    holdout = tuple(
        Trace({"x": float(i) + 0.25}, float(i) + 0.25, "opaque", {}, f"bootstrap-h-{i}")
        for i in range(4)
    )
    transfer = tuple(
        Trace({"x": float(i) - 0.5}, float(i) - 0.5, "opaque", {}, f"bootstrap-t-{i}")
        for i in range(4)
    )
    primitive = c.invent_primitive(train, holdout, transfer)
    assert primitive is not None

    c.generation = 1
    rows = tuple(
        Trace(
            {"x": float(i), "y": float(i) + 1.0},
            2.0 * float(i),
            "opaque",
            {},
            f"process-{i}",
        )
        for i in range(8)
    )
    proc_transfer = tuple(
        Trace(
            {"x": float(i), "y": float(i) + 1.0},
            2.0 * float(i),
            "opaque",
            {},
            f"process-t-{i}",
        )
        for i in range(4)
    )
    proc_ood = tuple(
        Trace(
            {"x": float(i), "y": float(i) + 1.0},
            2.0 * float(i),
            "opaque",
            {},
            f"process-o-{i}",
        )
        for i in range(4)
    )
    process = c.synthesize_process(rows, proc_transfer, proc_ood)
    assert process is not None
    assert primitive.primitive_id in process.used_primitives


def test_behavioral_search_prefers_multi_macro_exact_composition():
    from cognitive_core.behavioral_search import find_exact_expression
    from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive, Trace

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    keys = ("x", "y")
    p_x = CognitivePrimitive(
        "prim:x", {"op": "get", "key": "x"}, keys, "delta", 0,
        (), ("opaque",), 0.0, 0.0, 0.0, 1, {},
    )
    p_y = CognitivePrimitive(
        "prim:y", {"op": "get", "key": "y"}, keys, "delta", 0,
        (), ("opaque",), 0.0, 0.0, 0.0, 1, {},
    )
    c.primitives = {p_x.primitive_id: p_x, p_y.primitive_id: p_y}
    c.archive = {
        p_x.primitive_id: ArchiveRecord(p_x),
        p_y.primitive_id: ArchiveRecord(p_y),
    }
    rows = tuple(
        Trace(
            {"x": float(i), "y": float(i + 1)},
            float(2 * i + 1),
            "opaque",
            {},
            f"sum-{i}",
        )
        for i in range(8)
    )
    result = find_exact_expression(c, rows, max_depth=2, require_macro=True, selection_objective="maximize_direct_vocabulary")
    assert result is not None
    assert set(result.used_primitives) == {"prim:x", "prim:y"}


def test_open_ended_primitive_search_is_train_only(monkeypatch):
    import cognitive_core.open_ended_growth as growth
    from cognitive_core.behavioral_search import BehavioralSearchResult
    from cognitive_core.recursive_cognitive_compiler import Trace

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    train = tuple(
        Trace({"x": float(i)}, float(i), "opaque", {}, f"train-{i}")
        for i in range(6)
    )
    holdout = tuple(
        Trace({"x": float(i + 100)}, float(i + 100), "opaque", {}, f"hold-{i}")
        for i in range(2)
    )
    transfer = tuple(
        Trace({"x": float(i + 200)}, float(i + 200), "opaque", {}, f"transfer-{i}")
        for i in range(4)
    )
    seen = {}

    def fake_search(compiler, rows, max_depth, *, require_macro, max_alternatives=8):
        seen["rows"] = tuple(rows)
        return BehavioralSearchResult(
            {"op": "get", "key": "x"},
            (),
            {"schema": "test", "status": "TARGET_FOUND"},
        )

    monkeypatch.setattr(growth, "find_exact_expression", fake_search)
    primitive = c.invent_primitive(train, holdout, transfer)
    assert primitive is not None
    assert len(seen["rows"]) == len(train)
    assert all(row.task_id.startswith("train-") for row in seen["rows"])


def test_primitive_lineage_is_transitive():
    from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    p1 = CognitivePrimitive(
        "prim:a", {"op": "get", "key": "x"}, ("x",), "delta", 0,
        (), ("opaque",), 0.0, 0.0, 0.0, 1, {},
    )
    p2 = CognitivePrimitive(
        "prim:b", {"op": "macro", "id": "prim:a"}, ("x",), "delta", 1,
        ("prim:a",), ("opaque",), 0.0, 0.0, 0.0, 2, {},
    )
    p3 = CognitivePrimitive(
        "prim:c", {"op": "macro", "id": "prim:b"}, ("x",), "delta", 2,
        ("prim:b",), ("opaque",), 0.0, 0.0, 0.0, 3, {},
    )
    c.primitives = {
        p1.primitive_id: p1,
        p2.primitive_id: p2,
        p3.primitive_id: p3,
    }
    c.archive = {pid: ArchiveRecord(p) for pid, p in c.primitives.items()}
    assert c.primitive_lineage("prim:c") == ("prim:a", "prim:b", "prim:c")


def test_behavioral_search_ranks_transitive_lineage_reuse():
    from cognitive_core.behavioral_search import find_exact_expression
    from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive, Trace

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=3)
    p1 = CognitivePrimitive(
        "prim:a", {"op": "get", "key": "x"}, ("x",), "delta", 0,
        (), ("opaque",), 0.0, 0.0, 0.0, 1, {},
    )
    p2 = CognitivePrimitive(
        "prim:b", {"op": "macro", "id": "prim:a"}, ("x",), "delta", 1,
        ("prim:a",), ("opaque",), 0.0, 0.0, 0.0, 2, {},
    )
    c.primitives = {p1.primitive_id: p1, p2.primitive_id: p2}
    c.archive = {
        p1.primitive_id: ArchiveRecord(p1),
        p2.primitive_id: ArchiveRecord(p2),
    }
    rows = tuple(
        Trace(
            {"x": float(i)},
            float(i),
            "opaque",
            {},
            f"lineage-{i}",
        )
        for i in range(8)
    )
    result = find_exact_expression(c, rows, max_depth=3, require_macro=True)
    assert result is not None
    assert "prim:b" in result.used_primitives
    assert result.stats["max_lineage_coverage"] >= 2


def test_behavioral_search_does_not_stop_at_shallow_exact_solution():
    from cognitive_core.behavioral_search import find_exact_expression
    from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive, Trace

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    p_a = CognitivePrimitive(
        "prim:a",
        {"op": "get", "key": "x"},
        ("x",),
        "delta",
        0,
        (),
        ("opaque",),
        0.0,
        0.0,
        0.0,
        1,
        {},
    )
    p_b = CognitivePrimitive(
        "prim:b",
        {"op": "get", "key": "x"},
        ("x",),
        "delta",
        0,
        (),
        ("opaque",),
        0.0,
        0.0,
        0.0,
        1,
        {},
    )
    p_c = CognitivePrimitive(
        "prim:c",
        {
            "op": "add",
            "left": {"op": "get", "key": "x"},
            "right": {"op": "get", "key": "x"},
        },
        ("x",),
        "delta",
        0,
        (),
        ("opaque",),
        0.0,
        0.0,
        0.0,
        3,
        {},
    )
    c.primitives = {
        p_a.primitive_id: p_a,
        p_b.primitive_id: p_b,
        p_c.primitive_id: p_c,
    }
    c.archive = {
        pid: ArchiveRecord(p)
        for pid, p in c.primitives.items()
    }
    rows = tuple(
        Trace(
            {"x": float(i)},
            float(2 * i),
            "opaque",
            {},
            f"global-{i}",
        )
        for i in range(8)
    )
    result = find_exact_expression(c, rows, max_depth=3, require_macro=True, selection_objective="maximize_direct_vocabulary")
    assert result is not None
    assert {"prim:a", "prim:b"} <= set(result.used_primitives)
    assert result.stats["best_target_depth"] >= 2
    assert 3 in result.stats["depths_completed"]
    assert result.stats["first_exact_target_depth"] == 2


def test_structural_alias_inherits_reusable_lineage():
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
        {"op": "add", "left": {"op": "get", "key": "x"}, "right": {"op": "get", "key": "x"}},
        ("x",),
        "delta",
        1,
        (),
        ("opaque",),
        0.0, 0.0, 0.0, 3,
        {"lineage_ids": ["prim:c"]},
    )
    c.primitives = {p_a.primitive_id: p_a, p_b.primitive_id: p_b, p_c.primitive_id: p_c}
    c.archive = {pid: ArchiveRecord(p) for pid, p in c.primitives.items()}
    rows = tuple(
        Trace({"x": float(i)}, float(2 * i), "opaque", {}, f"alias-{i}")
        for i in range(8)
    )
    result = find_exact_expression(c, rows, max_depth=3, require_macro=True)
    assert result is not None
    assert {"prim:a", "prim:b"} <= set(
        result.stats["selected_structural_lineage_ids"]
    )


def test_primitive_lineage_reads_structural_alias_provenance():
    from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    p = CognitivePrimitive(
        "prim:c",
        {"op": "get", "key": "x"},
        ("x",),
        "delta",
        1,
        (),
        ("opaque",),
        0.0, 0.0, 0.0, 1,
        {"lineage_ids": ["prim:a", "prim:b"]},
    )
    c.primitives = {p.primitive_id: p}
    c.archive = {p.primitive_id: ArchiveRecord(p)}
    assert c.primitive_lineage("prim:c") == ("prim:a", "prim:b", "prim:c")


def test_behavioral_search_target_directed_max_min_avoids_binary_cross_product():
    from cognitive_core.behavioral_search import find_exact_expression
    from cognitive_core.recursive_cognitive_compiler import (
        ArchiveRecord,
        CognitivePrimitive,
        Trace,
    )

    for op in ("max", "min"):
        c = OpenEndedRecursiveCognitiveCompiler(max_depth=4)
        p_x = CognitivePrimitive(
            "prim:x",
            {"op": "get", "key": "x"},
            ("x", "y"),
            "delta",
            0,
            (),
            ("opaque",),
            0.0,
            0.0,
            0.0,
            1,
            {},
        )
        p_y = CognitivePrimitive(
            "prim:y",
            {"op": "get", "key": "y"},
            ("x", "y"),
            "delta",
            0,
            (),
            ("opaque",),
            0.0,
            0.0,
            0.0,
            1,
            {},
        )
        c.primitives = {
            p_x.primitive_id: p_x,
            p_y.primitive_id: p_y,
        }
        c.archive = {
            p_x.primitive_id: ArchiveRecord(p_x),
            p_y.primitive_id: ArchiveRecord(p_y),
        }
        rows = tuple(
            Trace(
                {"x": float(i), "y": float(i + 1)},
                (
                    max(float(i), float(i + 1))
                    if op == "max"
                    else min(float(i), float(i + 1))
                ),
                "opaque",
                {},
                f"{op}-{i}",
            )
            for i in range(8)
        )
        result = find_exact_expression(c, rows, max_depth=4, require_macro=True)
        assert result is not None
        # Optional/macro-required search now selects by expanded execution
        # cost first; it must not pay extra cost merely to maximize macro count.
        # The Pareto frontier should still return the richer direct-vocabulary
        # composition as an alternative, preserving the experiment's option.
        expected_single = "prim:y" if op == "max" else "prim:x"
        assert set(result.used_primitives) == {expected_single}
        assert any(
            set(ids) == {"prim:x", "prim:y"}
            for _, ids in result.alternatives
        )
        assert result.stats["first_exact_target_depth"] == 2
        assert 3 in result.stats["depths_completed"]
        assert result.stats["target_cutoff_depth"] in (None, 4)
        assert result.stats["generic_binary_path_skipped_after_target"] >= 1


def test_behavioral_search_target_directed_subtraction_covers_reverse_depth():
    from cognitive_core.behavioral_search import find_exact_expression
    from cognitive_core.recursive_cognitive_compiler import (
        ArchiveRecord,
        CognitivePrimitive,
        Trace,
    )

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=4)
    p_x = CognitivePrimitive(
        "prim:x",
        {"op": "get", "key": "x"},
        ("x", "y"),
        "delta",
        0,
        (),
        ("opaque",),
        0.0,
        0.0,
        0.0,
        1,
        {},
    )
    p_y = CognitivePrimitive(
        "prim:y",
        {"op": "get", "key": "y"},
        ("x", "y"),
        "delta",
        0,
        (),
        ("opaque",),
        0.0,
        0.0,
        0.0,
        1,
        {},
    )
    c.primitives = {
        p_x.primitive_id: p_x,
        p_y.primitive_id: p_y,
    }
    c.archive = {
        p_x.primitive_id: ArchiveRecord(p_x),
        p_y.primitive_id: ArchiveRecord(p_y),
    }
    rows = tuple(
        Trace(
            {"x": float(i), "y": float(i + 1)},
            float(i - (i + 1)),
            "opaque",
            {},
            f"sub-{i}",
        )
        for i in range(8)
    )
    result = find_exact_expression(c, rows, max_depth=4, require_macro=True, selection_objective="maximize_direct_vocabulary")
    assert result is not None
    assert set(result.used_primitives) == {"prim:x", "prim:y"}
    assert result.stats["first_exact_target_depth"] == 2
    assert 3 in result.stats["depths_completed"]
    assert result.stats["target_cutoff_depth"] in (None, 4)


def test_behavioral_search_preserves_distinct_direct_macro_aliases():
    from cognitive_core.behavioral_search import find_exact_expression
    from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive, Trace

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=1)
    parent = CognitivePrimitive(
        "prim:parent",
        {"op": "get", "key": "x"},
        ("x",),
        "delta",
        0,
        (),
        ("opaque",),
        0.0,
        0.0,
        0.0,
        1,
        {},
    )
    child = CognitivePrimitive(
        "prim:child",
        {"op": "macro", "id": "prim:parent"},
        ("x",),
        "delta",
        1,
        ("prim:parent",),
        ("opaque",),
        0.0,
        0.0,
        0.0,
        1,
        {},
    )
    c.primitives = {
        parent.primitive_id: parent,
        child.primitive_id: child,
    }
    c.archive = {
        parent.primitive_id: ArchiveRecord(parent),
        child.primitive_id: ArchiveRecord(child),
    }
    rows = tuple(
        Trace({"x": float(i)}, float(i), "opaque", {}, f"dom-{i}")
        for i in range(8)
    )
    result = find_exact_expression(c, rows, max_depth=1, require_macro=True)
    assert result is not None
    assert "prim:child" in result.used_primitives
    assert result.stats["dominance_pruned_states"] == 0
    alternative_vocabularies = {frozenset(vocab) for _, vocab in result.alternatives}
    assert frozenset({"prim:parent"}) in alternative_vocabularies
    assert frozenset({"prim:child"}) in alternative_vocabularies


def test_behavioral_search_continues_beyond_depth_two_macro_target():
    from cognitive_core.behavioral_search import find_exact_expression
    from cognitive_core.recursive_cognitive_compiler import (
        ArchiveRecord,
        CognitivePrimitive,
        Trace,
    )

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=6)
    parent = CognitivePrimitive(
        "prim:x",
        {"op": "get", "key": "x"},
        ("x",),
        "delta",
        0,
        (),
        ("opaque",),
        0.0, 0.0, 0.0, 1,
        {},
    )
    c.primitives = {parent.primitive_id: parent}
    c.archive = {parent.primitive_id: ArchiveRecord(parent)}
    rows = tuple(
        Trace({"x": float(i)}, float(2 * i), "opaque", {}, f"cut-{i}")
        for i in range(8)
    )

    result = find_exact_expression(c, rows, max_depth=6, require_macro=True)

    assert result is not None
    assert result.stats["best_target_depth"] == 2
    assert result.stats["first_exact_target_depth"] == 2
    assert 3 in result.stats["depths_completed"]
    assert result.stats["depths_completed"][-1] > 2
    if result.stats["target_cutoff_depth"] is not None:
        assert result.stats["target_cutoff_depth"] >= 3
        assert result.stats["target_cutoff_reason"] == (
            "bounded_exact_macro_target_continuation"
        )



def test_open_ended_process_history_is_bounded_without_touching_active():
    from cognitive_core.recursive_cognitive_compiler import ProcessCandidate

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    c.active = "proc:active"
    active = ProcessCandidate(
        "proc:active", {"op": "get", "key": "x"}, (), 0, (), {}, (), 1, {}
    )
    c.processes[active.process_id] = active
    for i in range(c.PROCESS_HISTORY_LIMIT + 32):
        c._remember_process(
            ProcessCandidate(
                f"proc:{i}", {"op": "const", "value": i}, (), 0, (), {}, (), 1, {}
            )
        )
    assert len(c.processes) <= c.PROCESS_HISTORY_LIMIT
    assert "proc:active" in c.processes


def test_open_ended_primitive_selection_ignores_validation_rows(monkeypatch):
    import cognitive_core.open_ended_growth as growth
    from cognitive_core.behavioral_search import BehavioralSearchResult
    from cognitive_core.recursive_cognitive_compiler import Trace

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    train = tuple(
        Trace({"x": float(i)}, float(i), "opaque", {}, f"train-{i}")
        for i in range(6)
    )
    holdout = tuple(
        Trace({"x": float(i + 100)}, -999.0, "opaque", {}, f"hold-{i}")
        for i in range(2)
    )
    transfer = tuple(
        Trace({"x": float(i + 200)}, -999.0, "opaque", {}, f"transfer-{i}")
        for i in range(4)
    )

    def fake_search(compiler, rows, max_depth, *, require_macro, max_alternatives=8):
        return BehavioralSearchResult(
            {"op": "get", "key": "x"},
            (),
            {"schema": "test", "status": "TARGET_FOUND"},
        )

    monkeypatch.setattr(growth, "find_exact_expression", fake_search)
    primitive = c.invent_primitive(train, holdout, transfer)
    assert primitive is not None
    assert primitive.holdout_error > 1.0
    assert primitive.transfer_error > 1.0


def test_process_frontier_ignores_transfer_and_ood_during_selection():
    from cognitive_core.recursive_cognitive_compiler import (
        ArchiveRecord,
        CognitivePrimitive,
        Trace,
    )

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=1)
    p_good = CognitivePrimitive(
        "prim:good",
        {"op": "get", "key": "x"},
        ("x",),
        "delta",
        0,
        (),
        ("opaque",),
        0.0,
        0.0,
        0.0,
        1,
        {},
    )
    p_bad = CognitivePrimitive(
        "prim:bad",
        {"op": "const", "value": 0.0},
        ("x",),
        "delta",
        0,
        (),
        ("opaque",),
        0.0,
        0.0,
        0.0,
        1,
        {},
    )
    c.primitives = {p_good.primitive_id: p_good, p_bad.primitive_id: p_bad}
    c.archive = {
        p_good.primitive_id: ArchiveRecord(p_good),
        p_bad.primitive_id: ArchiveRecord(p_bad),
    }

    rows = tuple(
        Trace({"x": 0.0}, 0.0, "opaque", {}, f"row-{i}")
        for i in range(6)
    )
    transfer = tuple(
        Trace({"x": 1.0}, 1.0, "opaque", {}, f"transfer-{i}")
        for i in range(4)
    )
    ood = tuple(
        Trace({"x": 1.0}, 1.0, "opaque", {}, f"ood-{i}")
        for i in range(4)
    )

    frontier = c.synthesize_process_frontier(rows, transfer, ood, max_candidates=32)
    used = {tuple(p.used_primitives) for p in frontier}
    assert ("prim:good",) in used
    assert ("prim:bad",) in used
    assert c.last_process_candidate_selection_summary["post_selection_validation_ignored"] is True


def test_primitive_version_space_survives_post_selection_validation(monkeypatch):
    import cognitive_core.open_ended_growth as growth
    from cognitive_core.behavioral_search import BehavioralSearchResult
    from cognitive_core.recursive_cognitive_compiler import Trace

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    train = tuple(Trace({"x": float(i)}, float(i), "opaque", {}, f"t-{i}") for i in range(6))
    hold = tuple(Trace({"x": float(i)}, float(i), "opaque", {}, f"h-{i}") for i in range(2))
    transfer = tuple(Trace({"x": float(i)}, float(i), "opaque", {}, f"u-{i}") for i in range(4))
    ood = tuple(Trace({"x": float(i)}, float(i), "opaque", {}, f"o-{i}") for i in range(4))
    good = {"op": "get", "key": "x"}
    alias = {"op": "add", "left": {"op": "get", "key": "x"}, "right": {"op": "const", "value": 0}}

    def fake_search(compiler, rows, max_depth, *, require_macro, max_alternatives=8):
        return BehavioralSearchResult(good, (), {"status": "TARGET_FOUND"}, ((good, ()), (alias, ())))

    monkeypatch.setattr(growth, "find_exact_expression", fake_search)
    primary = c.invent_primitive(train, hold, transfer)
    assert primary is not None
    assert len(c.last_primitive_candidate_frontier) == 2
    result = c.accept_primitive_frontier(transfer_rows=transfer, ood_rows=ood, novelty=1.0, resource_cost=1.0)
    assert result["accepted"] is True
    assert result["validated_candidate_count"] == 2


def test_process_version_space_validates_after_discovery_selection():
    from cognitive_core.recursive_cognitive_compiler import Trace

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=1)
    rows = tuple(Trace({"x": float(i)}, float(i), "opaque", {}, f"r-{i}") for i in range(8))
    proc = c.synthesize_process(rows, rows, rows)
    assert proc is not None
    chosen, evaluation, summary = c.validate_process_frontier(
        train=rows, holdout=rows, transfer=rows, ood=rows
    )
    assert chosen is not None
    assert evaluation is not None
    assert summary["selection_source"] == "discovery_and_selection_only"
    assert summary["validation_role"] == "admission_only"


def test_behavioral_search_preserves_explicit_multi_parent_reuse_over_composite_alias():
    from cognitive_core.behavioral_search import find_exact_expression
    from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive, Trace

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    p_a = CognitivePrimitive(
        "prim:a", {"op": "get", "key": "x"}, ("x",), "delta", 0,
        (), ("opaque",), 0.0, 0.0, 0.0, 1, {},
    )
    p_b = CognitivePrimitive(
        "prim:b", {"op": "get", "key": "y"}, ("y",), "delta", 0,
        (), ("opaque",), 0.0, 0.0, 0.0, 1, {},
    )
    p_c = CognitivePrimitive(
        "prim:c",
        {
            "op": "add",
            "left": {"op": "macro", "id": "prim:a"},
            "right": {"op": "macro", "id": "prim:b"},
        },
        ("x", "y"), "delta", 1,
        ("prim:a", "prim:b"), ("opaque",), 0.0, 0.0, 0.0, 3, {},
    )
    c.primitives = {
        p.primitive_id: p for p in (p_a, p_b, p_c)
    }
    c.archive = {
        pid: ArchiveRecord(p)
        for pid, p in c.primitives.items()
    }
    rows = tuple(
        Trace(
            {"x": float(i), "y": float(i + 1)},
            float(2 * i + 1),
            "opaque",
            {},
            f"reuse-{i}",
        )
        for i in range(8)
    )

    result = find_exact_expression(
        c, rows, max_depth=2, require_macro=True, selection_objective="maximize_direct_vocabulary"
    )
    assert result is not None
    assert set(result.used_primitives) == {"prim:a", "prim:b"}
    assert result.stats["dominance_frontier_max"] >= 2



def test_executable_lineage_ignores_provenance_only_aliases():
    from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    parent = CognitivePrimitive(
        "prim:parent", {"op": "get", "key": "x"}, ("x",), "delta", 0,
        (), ("opaque",), 0.0, 0.0, 0.0, 1, {},
    )
    alias = CognitivePrimitive(
        "prim:alias", {"op": "get", "key": "x"}, ("x",), "delta", 1,
        (), ("opaque",), 0.0, 0.0, 0.0, 1,
        {"lineage_ids": ["prim:parent"]},
    )
    c.primitives = {parent.primitive_id: parent, alias.primitive_id: alias}
    c.archive = {pid: ArchiveRecord(p) for pid, p in c.primitives.items()}

    assert c.primitive_lineage("prim:alias") == ("prim:alias", "prim:parent")
    assert c.executable_lineage("prim:alias") == ("prim:alias",)


def test_behavioral_search_uses_executable_lineage_for_direct_reuse_dominance():
    from cognitive_core.behavioral_search import find_exact_expression
    from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive, Trace

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
    p_a = CognitivePrimitive(
        "prim:a", {"op": "get", "key": "x"}, ("x",), "delta", 0,
        (), ("opaque",), 0.0, 0.0, 0.0, 1, {},
    )
    p_b = CognitivePrimitive(
        "prim:b", {"op": "get", "key": "y"}, ("y",), "delta", 0,
        (), ("opaque",), 0.0, 0.0, 0.0, 1, {},
    )
    p_c = CognitivePrimitive(
        "prim:c",
        {
            "op": "add",
            "left": {"op": "macro", "id": "prim:a"},
            "right": {"op": "macro", "id": "prim:b"},
        },
        ("x", "y"), "delta", 1,
        ("prim:a", "prim:b"), ("opaque",), 0.0, 0.0, 0.0, 3, {},
    )
    provenance_alias = CognitivePrimitive(
        "prim:alias",
        {"op": "get", "key": "x"},
        ("x", "y"), "delta", 2,
        (),
        ("opaque",), 0.0, 0.0, 0.0, 1,
        {"lineage_ids": ["prim:a", "prim:b"]},
    )
    c.primitives = {
        p_a.primitive_id: p_a,
        p_b.primitive_id: p_b,
        p_c.primitive_id: p_c,
        provenance_alias.primitive_id: provenance_alias,
    }
    c.archive = {pid: ArchiveRecord(p) for pid, p in c.primitives.items()}

    rows = tuple(
        Trace(
            {"x": float(i), "y": float(i + 1)},
            float(2 * i + 1),
            "opaque",
            {},
            f"reuse-{i}",
        )
        for i in range(8)
    )
    result = find_exact_expression(c, rows, max_depth=2, require_macro=True, selection_objective="maximize_direct_vocabulary")
    assert result is not None
    assert set(result.used_primitives) == {"prim:a", "prim:b"}
    assert set(result.stats["selected_executable_lineage_ids"]) == {"prim:a", "prim:b"}
    assert result.stats["dominance_frontier_max"] >= 2

def test_behavioral_search_does_not_reward_redundant_macro_vocabulary_in_either_mode():
    from cognitive_core.behavioral_search import find_exact_expression
    from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive, Trace

    rows = tuple(
        Trace({"x": float(i + 1)}, float(i + 1), "opaque", {}, f"redundant-{i}")
        for i in range(8)
    )

    for require_macro in (False, True):
        c = OpenEndedRecursiveCognitiveCompiler(max_depth=2)
        p_x = CognitivePrimitive(
            "prim:x", {"op": "get", "key": "x"}, ("x",), "delta", 0,
            (), ("opaque",), 0.0, 0.0, 0.0, 1, {},
        )
        p_zero = CognitivePrimitive(
            "prim:zero", {"op": "const", "value": 0.0}, (), "delta", 0,
            (), ("opaque",), 0.0, 0.0, 0.0, 1, {},
        )
        c.primitives = {p.primitive_id: p for p in (p_x, p_zero)}
        c.archive = {pid: ArchiveRecord(p) for pid, p in c.primitives.items()}

        result = find_exact_expression(
            c, rows, max_depth=2, require_macro=require_macro, max_alternatives=32
        )
        assert result is not None
        if require_macro:
            assert result.expression == {"op": "macro", "id": "prim:x"}
            assert set(result.used_primitives) == {"prim:x"}
        else:
            assert result.expression == {"op": "get", "key": "x"}
            assert result.used_primitives == ()
        assert "prim:zero" not in result.used_primitives
        # Selection favors the least costly expression, but does not erase a
        # nondominated direct-vocabulary variant merely because its macro count
        # is larger. Its cost/generalization utility remains separately testable.
        assert any(
            set(vocabulary) == {"prim:x", "prim:zero"}
            for _, vocabulary in result.alternatives
        )
        experimental = find_exact_expression(
            c, rows, max_depth=2, require_macro=require_macro,
            selection_objective="maximize_direct_vocabulary", max_alternatives=32,
        )
        assert experimental is not None
        assert set(experimental.used_primitives) == {"prim:x", "prim:zero"}
        assert len(experimental.used_primitives) > len(result.used_primitives)
        assert result.stats["status"] == "TARGET_FOUND"


def test_behavioral_search_does_not_publish_global_target_diagnostics():
    import cognitive_core.behavioral_search as search_module
    from cognitive_core.behavioral_search import find_exact_expression
    from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive, Trace

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=1)
    primitive = CognitivePrimitive(
        "prim:x", {"op": "get", "key": "x"}, ("x",), "delta", 0,
        (), ("opaque",), 0.0, 0.0, 0.0, 1, {},
    )
    c.primitives = {primitive.primitive_id: primitive}
    c.archive = {primitive.primitive_id: ArchiveRecord(primitive)}
    rows = tuple(
        Trace({"x": float(i)}, float(i), "opaque", {}, f"no-global-{i}")
        for i in range(4)
    )

    assert find_exact_expression(c, rows, max_depth=1, require_macro=True) is not None
    assert not hasattr(search_module, "_H7_LAST_SEARCH_STATS")



def test_h7_frontier_keeps_cost_tradeoffs_and_bucket_index_identity_consistent():
    import sys
    import cognitive_core.behavioral_search as search_module
    from cognitive_core.behavioral_search import find_exact_expression
    from cognitive_core.recursive_cognitive_compiler import (
        ArchiveRecord, CognitivePrimitive, Trace, ast_depth,
    )

    c = OpenEndedRecursiveCognitiveCompiler(max_depth=3)
    p_a = CognitivePrimitive(
        "prim:a",
        {"op": "neg", "arg": {"op": "neg", "arg": {"op": "neg", "arg": {"op": "neg", "arg": {"op": "get", "key": "x"}}}}},
        ("x",), "delta", 0, (), ("opaque",), 0.0, 0.0, 0.0, 5, {},
    )
    p_b = CognitivePrimitive(
        "prim:b",
        {
            "op": "add",
            "left": {
                "op": "mul",
                "left": {"op": "get", "key": "y"},
                "right": {"op": "const", "value": 1.0},
            },
            "right": {
                "op": "mul",
                "left": {"op": "get", "key": "y"},
                "right": {"op": "const", "value": 0.0},
            },
        },
        ("y",), "delta", 0, (), ("opaque",), 0.0, 0.0, 0.0, 7, {},
    )
    c.primitives = {p.primitive_id: p for p in (p_a, p_b)}
    c.archive = {pid: ArchiveRecord(p) for pid, p in c.primitives.items()}
    xs = (-4.0, -3.0, -2.0, -1.0, 1.0, 2.0, 3.0, 4.0)
    rows = tuple(
        Trace(
            {"x": x, "y": (-2.0 * x if x < 0.0 else 0.0)},
            (-3.0 * x if x < 0.0 else x),
            "opaque", {}, f"cost-tradeoff-{i}",
        )
        for i, x in enumerate(xs)
    )

    captured = {}
    function_code = find_exact_expression.__code__

    def local_trace(frame, event, arg):
        if event == "return":
            captured.update(frame.f_locals)
        return local_trace

    def global_trace(frame, event, arg):
        if event == "call" and frame.f_code is function_code:
            return local_trace
        return None

    old_trace = sys.gettrace()
    sys.settrace(global_trace)
    try:
        result = find_exact_expression(
            c, rows, max_depth=3, require_macro=True
        )
    finally:
        sys.settrace(old_trace)

    assert result is not None
    levels = captured["levels"]
    dominance_frontier = captured["dominance_frontier"]
    indexes = captured["_H7_online_indexes"]
    dominated_tokens = captured["dominated_state_tokens"]
    executable_cost = captured["executable_cost"]

    # Isolate this behavior/interface frontier, then register two expressions
    # with the same outputs, direct vocabulary, free-input interface, and AST
    # depth but Pareto-incomparable expanded costs. This deliberately reaches
    # register() with the pair, rather than depending on search enumeration.
    target = tuple(row.target for row in rows)
    behavior_key = (target, ("x", "y"))
    frontier_index = indexes[behavior_key]
    for old_state in tuple(dominance_frontier.get(behavior_key, ())):
        old_token = old_state[5]
        old_ident = captured["expression_key_by_state_token"].pop(old_token, None)
        if old_ident is not None:
            captured["seen_expression_keys"].discard(old_ident)
        captured["ast_depth_by_state_token"].pop(old_token, None)
        captured["_H7_remove"](frontier_index, old_state)
        captured["discard_pruned_state"](old_state)
    dominance_frontier[behavior_key] = []

    low_depth_more_nodes = {
        "op": "add",
        "left": {"op": "macro", "id": "prim:a"},
        "right": {
            "op": "add",
            "left": {"op": "macro", "id": "prim:b"},
            "right": {"op": "macro", "id": "prim:b"},
        },
    }
    high_depth_fewer_nodes = {
        "op": "abs",
        "arg": {
            "op": "sub",
            "left": {"op": "macro", "id": "prim:a"},
            "right": {"op": "macro", "id": "prim:b"},
        },
    }
    for expr in (low_depth_more_nodes, high_depth_fewer_nodes):
        captured["seen_expression_keys"].discard(captured["expr_key"](expr))
    low_state = captured["register"](low_depth_more_nodes)
    high_state = captured["register"](high_depth_fewer_nodes)
    assert low_state is not None and high_state is not None
    assert executable_cost(low_state[0]) == (6, 21)
    assert executable_cost(high_state[0]) == (7, 14)

    level_by_token = {}
    for bucket in levels.values():
        for state in bucket.values():
            assert state[5] not in dominated_tokens
            assert state[5] not in level_by_token
            level_by_token[state[5]] = state

    frontier_by_token = {}
    for behavior_key, states in dominance_frontier.items():
        assert behavior_key in indexes
        for state in states:
            assert state[5] not in dominated_tokens
            frontier_by_token[state[5]] = state

    index_by_token = {}
    for index in indexes.values():
        for token, state in index["active"].items():
            assert token not in index_by_token
            index_by_token[token] = state

    assert set(level_by_token) == set(frontier_by_token) == set(index_by_token)
    for token in level_by_token:
        assert level_by_token[token] is frontier_by_token[token]
        assert level_by_token[token] is index_by_token[token]

    profiles = {
        executable_cost(state[0])
        for state in level_by_token.values()
        if state[1] == target
        and set(state[2]) == {"prim:a", "prim:b"}
        and state[3] == ("x", "y")
        and ast_depth(state[0]) == 3
    }
    # These expressions are behaviorally equivalent on all discovery rows,
    # with the same direct vocabulary/interface/AST depth. One has lower
    # expanded depth but more nodes, so neither dominates the other.
    assert (6, 21) in profiles
    assert (7, 14) in profiles

    dominates = captured["dominates"]
    find_dominator = captured["_H7_find_dominator"]
    find_dominated = captured["_H7_find_dominated"]
    token = 10**9
    for behavior_key, index in indexes.items():
        active_states = list(index["active"].values())
        for candidate in active_states:
            # Compare indexed superset lookup to a straightforward linear oracle
            # for the candidate's actual direct set and its subsets.
            direct_ids = tuple(sorted(candidate[2]))
            query_sets = [direct_ids]
            query_sets.extend((item,) for item in direct_ids)
            query_sets.append(())
            for direct_set in query_sets:
                query = (
                    candidate[0], candidate[1], tuple(sorted(set(direct_set))),
                    candidate[3], candidate[4], token,
                )
                token += 1
                expected = [
                    state for state in active_states
                    if dominates(state, query)
                ]
                expected_state = min(expected, key=lambda state: state[5]) if expected else None
                actual_state = find_dominator(index, query)
                assert (
                    None if actual_state is None else actual_state[5]
                ) == (
                    None if expected_state is None else expected_state[5]
                )

                expected_dominated = {
                    state[5] for state in active_states
                    if dominates(query, state)
                }
                actual_dominated = {
                    state[5] for state in find_dominated(index, query)
                }
                assert actual_dominated == expected_dominated
