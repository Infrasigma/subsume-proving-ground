from __future__ import annotations

from lab.acsie_open_ended_self_extending_proving import HiddenCapability, admission_rows, target_is_admissible


def test_recursive_target_is_baseline_resistant_and_identifiable():
    parent = HiddenCapability(
        hidden_id="h0",
        primitive_id="p0",
        expression={
            "op": "abs",
            "arg": {
                "op": "add",
                "left": {"op": "mul", "left": {"op": "get", "key": "x"}, "right": {"op": "get", "key": "y"}},
                "right": {"op": "neg", "arg": {"op": "get", "key": "z"}},
            },
        },
        depth=4,
        generation=0,
    )
    target = {
        "op": "add",
        "left": {"op": "macro", "id": "h0"},
        "right": {"op": "get", "key": "x"},
    }
    assert target_is_admissible(
        target,
        (parent,),
        (parent,),
        seed=2026100301,
        generation=1,
    )


def test_admission_rows_are_independent_of_target_identity():
    rows_a = admission_rows(2026100301, 3)
    rows_b = admission_rows(2026100301, 3)
    assert rows_a == rows_b
    assert all(row.family == "admission-only" for row in rows_a)
    assert all(row.target == 0.0 for row in rows_a)
