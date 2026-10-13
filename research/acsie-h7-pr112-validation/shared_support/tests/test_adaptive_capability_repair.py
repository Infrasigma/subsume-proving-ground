from __future__ import annotations

import json

import cognitive_core.adaptive_capability_repair as repair_module
from cognitive_core.adaptive_capability_repair import (
    RepairDrivenCognitiveCore,
    _normalize_program,
    _observed_behavior_signature,
    _program_complexity,
    _prefix_family,
    _select_diverse_frontier,
    apply_extended_program,
    diagnose_failure,
    synthesize_extended_program,
)
from cognitive_core.native_independent_core import NativeCognitiveCore, SolutionCandidate


def test_extended_program_adds_parameterized_rotate():
    examples = [
        ([1, 2, 3, 4], [2, 3, 4, 1]),
        ([5, 6, 7, 8], [6, 7, 8, 5]),
        ([9, 10, 11, 12], [10, 11, 12, 9]),
    ]
    result = synthesize_extended_program(
        examples,
        examples[:1],
        [([13, 14, 15, 16], [14, 15, 16, 13])],
        depth=3,
        beam=128,
        budget=30000,
    )
    assert result.solved is True
    assert apply_extended_program([20, 21, 22], result.program) == [21, 22, 20]


def test_extended_program_discovers_composition():
    examples = [
        ([1, 2, 3], [6, 4, 2]),
        ([2, 4, 6], [12, 8, 4]),
        ([3, 5, 7], [14, 10, 6]),
        ([4, 6, 8], [16, 12, 8]),
    ]
    result = synthesize_extended_program(
        examples,
        [([5, 7, 9], [18, 14, 10])],
        [([6, 8, 10], [20, 16, 12])],
        depth=4,
        beam=160,
        budget=50000,
    )
    assert result.solved is True
    assert apply_extended_program([7, 9, 11], result.program) == [22, 18, 14]


def test_failure_diagnosis_records_structural_capability_gap():
    diagnosis = diagnose_failure(
        [([1, 2, 3], [2, 3, 1])],
        [([4, 5, 6], [5, 6, 4])],
        [([7, 8, 9], [8, 9, 7])],
        train_score=0.0,
        holdout_score=0.0,
        transfer_score=0.0,
    )
    assert diagnosis.category == "train_fit_gap"
    assert "sequence_or_elementwise_transform" in diagnosis.structural_signals
    assert "sequence_reordering" in diagnosis.suggested_capabilities


def test_repair_core_turns_failure_into_capability_and_retries():
    # repeat:n is outside the native synthesizer vocabulary in this test, so
    # diagnosis -> synthesis -> verification must actually be exercised.
    train = [
        ([1, 2, 3], [1, 2, 3, 1, 2, 3]),
        ([4, 5, 6], [4, 5, 6, 4, 5, 6]),
        ([7, 8, 9], [7, 8, 9, 7, 8, 9]),
        ([10, 11, 12], [10, 11, 12, 10, 11, 12]),
    ]
    holdout = [([13, 14, 15], [13, 14, 15, 13, 14, 15])]
    transfer = [([16, 17, 18], [16, 17, 18, 16, 17, 18])]
    adversarial = [([30, 31, 32], [30, 31, 32, 30, 31, 32])]

    baseline = NativeCognitiveCore(seed=7001)
    native = baseline.solve_examples_anytime(train, holdout, transfer, budget=256)
    assert native.verified is False

    core = RepairDrivenCognitiveCore(seed=7001)
    first = core.solve_with_repair(
        train, holdout, transfer, adversarial,
        max_repairs=2, depth=3, beam=128
    )
    assert first.solved is True
    assert first.diagnosis is not None
    assert core.failure_archive
    assert core.capability_gaps
    assert any(g["origin"] == "native_failure_directed_program_repair" for g in (
        p.provenance for p in core.procedures.values()
    ))

    second = core.solve_with_repair(
        train, holdout, transfer, adversarial,
        max_repairs=1, depth=2, beam=64
    )
    assert second.solved is True
    assert second.program == first.program


def test_repair_core_persists_failure_memory_and_cache():
    examples = [
        ([1, 2, 3], [3, 2, 1]),
        ([4, 5, 6], [6, 5, 4]),
        ([7, 8, 9], [9, 8, 7]),
    ]
    core = RepairDrivenCognitiveCore(seed=7002)
    result = core.solve_with_repair(
        examples,
        [([10, 11, 12], [12, 11, 10])],
        [([13, 14, 15], [15, 14, 13])],
        [([20, 21], [21, 20])],
        max_repairs=1,
        depth=2,
        beam=96,
    )
    assert result.solved is True
    restored = RepairDrivenCognitiveCore.from_state(
        json.loads(json.dumps(core.export_state()))
    )
    assert restored.export_state() == core.export_state()


def test_original_program_synthesizer_can_use_parameterized_operator():
    from cognitive_core.program_synthesis import synthesize_program

    examples = [
        ([1, 2, 3, 4], [2, 3, 4, 1]),
        ([5, 6, 7, 8], [6, 7, 8, 5]),
        ([9, 10, 11, 12], [10, 11, 12, 9]),
    ]
    candidate, _ = synthesize_program(examples, max_depth=3, beam=128)
    assert candidate is not None
    assert candidate.apply([20, 21, 22, 23]) == [21, 22, 23, 20]


def test_repair_search_never_receives_hidden_evaluation_rows(monkeypatch):
    train = [
        ([1, 2, 3], [3, 2, 1]),
        ([4, 5, 6], [6, 5, 4]),
        ([7, 8, 9], [9, 8, 7]),
    ]
    holdout = [([10, 11, 12], [12, 11, 10])]
    transfer = [([20, 21, 22], [22, 21, 20])]
    captured = []

    original = repair_module.synthesize_extended_program

    def wrapped(examples, hidden_holdout, hidden_transfer, **kwargs):
        captured.append((tuple(hidden_holdout), tuple(hidden_transfer)))
        return original(examples, hidden_holdout, hidden_transfer, **kwargs)

    monkeypatch.setattr(
        repair_module, "synthesize_extended_program", wrapped
    )

    core = RepairDrivenCognitiveCore(seed=7003)

    def force_native_failure(*args, **kwargs):
        return SolutionCandidate(
            "forced_test_failure", None, 0.0, False, 0, {"test": "force_failure"}
        )

    monkeypatch.setattr(
        RepairDrivenCognitiveCore, "solve_examples_anytime", force_native_failure
    )

    core.solve_with_repair(
        train, holdout, transfer,
        max_repairs=1, depth=2, beam=64,
    )

    assert captured
    assert all(hidden_holdout == () and hidden_transfer == () for hidden_holdout, hidden_transfer in captured)


def test_verified_capability_library_grows_and_survives_restart():
    examples = [
        ([1, 2, 3, 4], [4, 3, 2, 1]),
        ([5, 6, 7, 8], [8, 7, 6, 5]),
        ([9, 10, 11, 12], [12, 11, 10, 9]),
    ]
    holdout = [([13, 14, 15, 16], [16, 15, 14, 13])]
    transfer = [([20, 21, 22, 23], [23, 22, 21, 20])]

    core = RepairDrivenCognitiveCore(seed=7010)
    proc = core.learn_generic_program(examples, holdout, transfer, family="library-growth")
    assert proc is not None
    assert core.program_library
    assert any(
        row.get("type") == "capability_library_growth"
        and row.get("executable_representation_change") is True
        for row in core.change_history
    )

    restored = RepairDrivenCognitiveCore.from_state(
        json.loads(json.dumps(core.export_state()))
    )
    assert restored.program_library == core.program_library
    macro = restored.program_library[-1]
    from cognitive_core.program_synthesis import apply_program
    assert apply_program([30, 31, 32, 33], (macro,)) == [33, 32, 31, 30]


def test_repair_reuses_persistent_macro_capability():
    from cognitive_core.program_synthesis import encode_macro

    macro = encode_macro(("repeat:2",))
    examples = [
        ([1, 2, 3], [1, 2, 3, 1, 2, 3]),
        ([4, 5], [4, 5, 4, 5]),
        ([6], [6, 6]),
    ]
    result = synthesize_extended_program(
        examples,
        (),
        (),
        depth=1,
        beam=16,
        budget=64,
        library=(macro,),
    )
    assert result.solved is True
    assert result.program == (macro,)


def test_extended_transpose_supports_ragged_rows():
    x = [[1, 2, 3], [4, 5], [6, 7, 8, 9]]
    expected = [[1, 4, 6], [2, 5, 7]]
    assert apply_extended_program(x, ("transpose",)) == expected


def test_extended_repair_primitives_cover_sequence_composition():
    assert apply_extended_program([[1, 2], [3, 4]], ("concat",)) == [1, 2, 3, 4]
    assert apply_extended_program([[1, 2], [3, 4]], ("interleave",)) == [1, 3, 2, 4]
    assert apply_extended_program([[1, 2], [3, 4]], ("reverse_each",)) == [[2, 1], [4, 3]]
    assert apply_extended_program([1, 2, 3], ("prefix_sum",)) == [1, 3, 6]
    assert apply_extended_program([1, 4, 9], ("differences",)) == [3, 5]


def test_repair_selection_prefers_invariant_program_over_training_alias():
    train = [
        ([1, 2, 3, 4, 5, 6, 7], [4, 5, 6, 7, 1, 2, 3]),
        ([7, 6, 5, 4, 3, 2, 1], [4, 3, 2, 1, 7, 6, 5]),
        ([2, 4, 6, 8, 10, 12, 14], [8, 10, 12, 14, 2, 4, 6]),
        ([3, 1, 4, 1, 5, 9, 2], [1, 5, 9, 2, 3, 1, 4]),
    ]
    holdout = [
        ([10, 11, 12, 13, 14], [13, 14, 10, 11, 12]),
    ]
    transfer = [
        ([20, 21, 22, 23, 24, 25], [23, 24, 25, 20, 21, 22]),
    ]
    result = synthesize_extended_program(
        train,
        holdout,
        transfer,
        depth=2,
        beam=192,
        budget=30000,
    )
    assert result.solved is True
    assert result.holdout_score == 1.0
    assert result.transfer_score == 1.0
    assert result.program == ("rotate:3",)


def test_program_normalization_reduces_composed_rotation_cost():
    assert _normalize_program(("rotate:-1", "rotate:-3")) == ("rotate:-4",)
    assert _program_complexity(("macro:invalid",)) == 1


def test_observed_behavior_signature_quotients_equivalent_programs():
    rows = [
        ([1, 2, 3, 4], [2, 3, 4, 1]),
        ([5, 6, 7, 8], [6, 7, 8, 5]),
    ]
    assert _observed_behavior_signature(("rotate:1",), rows) == (
        "[2,3,4,1]",
        "[6,7,8,5]",
    )
    assert _observed_behavior_signature(
        ("rotate:5",),
        rows,
    ) == _observed_behavior_signature(("rotate:1",), rows)



def test_numeric_generalization_search_uses_behavioral_quotient():
    rows = [
        ([1, 2, 3, 4], [2, 3, 4, 1]),
        ([5, 6, 7, 8], [6, 7, 8, 5]),
        ([9, 10, 11, 12], [10, 11, 12, 9]),
    ]
    sig_a = _observed_behavior_signature(("rotate:1",), rows)
    sig_b = _observed_behavior_signature(("rotate:5",), rows)
    assert sig_a == sig_b



def test_bounded_rotation_learns_length_clamped_parameter():
    train = [
        ([1, 2, 3], [3, 1, 2]),
        ([4, 5, 6, 7], [7, 4, 5, 6]),
        ([8, 9, 10, 11, 12], [11, 12, 8, 9, 10]),
        ([13, 14, 15, 16, 17, 18], [16, 17, 18, 13, 14, 15]),
    ]
    holdout = [([20, 21, 22], [22, 20, 21])]
    transfer = [([30, 31, 32, 33, 34], [33, 34, 30, 31, 32])]
    result = synthesize_extended_program(
        train,
        holdout,
        transfer,
        depth=2,
        beam=192,
        budget=30000,
    )
    assert result.solved is True
    assert result.program == ("rotate_clamped:3",)
    assert apply_extended_program([40, 41, 42], result.program) == [42, 40, 41]


def test_bounded_rotation_persists_through_macro_execution():
    from cognitive_core.program_synthesis import encode_macro, apply_program

    macro = encode_macro(("rotate_clamped:3",))
    assert apply_program([1, 2, 3], (macro,)) == [3, 1, 2]
    assert apply_program([1, 2, 3, 4, 5], (macro,)) == [4, 5, 1, 2, 3]


def test_frontier_preserves_distinct_composition_prefixes():
    candidates = [
        (0.01, ("reverse", "map_add:1"), 0.0),
        (0.02, ("reverse", "map_add:2"), 0.0),
        (0.03, ("reverse", "map_add:3"), 0.0),
        (0.50, ("flatten", "reverse"), 0.0),
        (0.60, ("sort", "map_add:5"), 0.0),
    ]
    selected = _select_diverse_frontier(candidates, beam=3)
    assert {
        _prefix_family(program) for program in selected
    } == {
        ("reverse", "map_add"),
        ("flatten", "reverse"),
        ("sort", "map_add"),
    }



def test_frontier_supports_depth_adaptive_prefix_skeletons():
    candidates = [
        (0.01, ("reverse", "map_add:1", "sort"), 0.0),
        (0.02, ("reverse", "map_add:1", "chunk:2"), 0.0),
        (0.03, ("reverse", "map_add:2", "sort"), 0.0),
        (0.50, ("flatten", "reverse", "chunk:2"), 0.0),
        (0.60, ("sort", "map_add:5", "window:2"), 0.0),
    ]
    selected = _select_diverse_frontier(candidates, beam=3, prefix_width=3)
    assert {
        _prefix_family(program, 3) for program in selected
    } == {
        ("reverse", "map_add", "sort"),
        ("reverse", "map_add", "chunk"),
        ("flatten", "reverse", "chunk"),
    }
