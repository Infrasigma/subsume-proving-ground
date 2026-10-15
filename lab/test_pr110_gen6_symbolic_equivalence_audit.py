from __future__ import annotations
import json
import unittest
import sympy as sp
from lab.pr110_gen6_symbolic_equivalence_audit import AuditError, as_sympy, summarize


class SymbolicEquivalenceAuditTests(unittest.TestCase):
    def test_inline_and_macro_forms_simplify_to_same_function(self):
        macros = {
            "parent:a": {"op": "min", "left": {"op": "get", "key": "x"}, "right": {"op": "get", "key": "y"}},
            "parent:b": {"op": "neg", "arg": {"op": "get", "key": "z"}},
        }
        target = {"op": "add", "left": {"op": "macro", "id": "parent:a"}, "right": {"op": "macro", "id": "parent:b"}}
        candidate = {"op": "add", "left": {"op": "min", "left": {"op": "get", "key": "y"}, "right": {"op": "get", "key": "x"}}, "right": {"op": "neg", "arg": {"op": "get", "key": "z"}}}
        self.assertEqual(sp.simplify(as_sympy(candidate, {}) - as_sympy(target, macros)), sp.Integer(0))

    def test_wrong_lineage_but_identical_function_is_reported_read_only(self):
        checkpoint = {"seed": 2026100305, "generation_next": 7,
            "learner_state": {"primitives": {
                "prim:a": {"expression": {"op": "get", "key": "x"}},
                "prim:b": {"expression": {"op": "get", "key": "y"}}}},
            "retained": [
                {"hidden_id": "hidden:a", "primitive_id": "prim:a", "expression": {"op": "get", "key": "x"}, "generation": 0},
                {"hidden_id": "hidden:b", "primitive_id": "prim:b", "expression": {"op": "get", "key": "y"}, "generation": 1}]}
        event = {"event": "CLOSURE_LINEAGE_MISMATCH", "generation": 6,
            "expected_parent_ids": ["prim:a", "prim:b"], "frontier_candidate_audit": [{
                "process_id": "inlined-equivalent", "is_preselected_candidate": True,
                "closure_competent": True, "contains_expected_parents": False,
                "expression": {"op": "add", "left": {"op": "get", "key": "x"}, "right": {"op": "get", "key": "y"}}}]}
        report = summarize((json.dumps(event) + "\n").encode(), json.dumps(checkpoint).encode(), require_pinned_hashes=False)
        self.assertTrue(report["all_records_zero"])
        self.assertTrue(report["selected_candidate_symbolic_identity_matches_target"])
        self.assertTrue(report["all_selected_candidates_marked_wrong_lineage"])
        self.assertFalse(report["runtime_selection_changed"])
        self.assertFalse(report["qualification_gate_changed"])

    def test_unresolved_macro_fails_closed(self):
        with self.assertRaises(AuditError): as_sympy({"op": "macro", "id": "missing"}, {})

    def test_macro_cycle_fails_closed(self):
        with self.assertRaises(AuditError): as_sympy({"op": "macro", "id": "a"}, {"a": {"op": "macro", "id": "a"}})


if __name__ == "__main__":
    unittest.main()
