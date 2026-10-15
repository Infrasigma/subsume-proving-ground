from __future__ import annotations
import json
import unittest
from lab.pr110_gen6_parent_function_class_coverage import summarize


class ParentFunctionClassCoverageTests(unittest.TestCase):
    def test_alias_macro_covers_target_function_without_exact_id(self):
        checkpoint = {"seed": 2026100305, "generation_next": 7,
            "learner_state": {"primitives": {
                "prim:expected": {"expression": {"op": "get", "key": "x"}},
                "prim:alias": {"expression": {"op": "get", "key": "x"}},
                "prim:y": {"expression": {"op": "get", "key": "y"}}}},
            "retained": [
                {"hidden_id": "hidden:x", "primitive_id": "prim:expected", "expression": {"op": "get", "key": "x"}, "generation": 0},
                {"hidden_id": "hidden:y", "primitive_id": "prim:y", "expression": {"op": "get", "key": "y"}, "generation": 1}]}
        event = {"event": "CLOSURE_LINEAGE_MISMATCH", "generation": 6,
            "expected_parent_ids": ["prim:expected", "prim:y"],
            "frontier_candidate_audit": [{
                "process_id": "alias-process", "is_preselected_candidate": True,
                "closure_competent": True, "contains_expected_parents": False,
                "candidate_lineage": ["prim:alias", "prim:y"],
                "expression": {"op": "add", "left": {"op": "macro", "id": "prim:alias"},
                               "right": {"op": "macro", "id": "prim:y"}}}]}
        result = summarize((json.dumps(event) + "\n").encode(), json.dumps(checkpoint).encode(),
                           require_pinned_hashes=False)
        self.assertTrue(result["all_unique_parent_function_classes_covered"])
        self.assertEqual(result["function_class_coverage_count"], 1)
        self.assertEqual(result["exact_expected_parent_id_lineage_count"], 0)
        self.assertFalse(result["rows"][0]["selected_contains_exact_expected_parent_ids"])
        self.assertFalse(result["runtime_selection_changed"])
        self.assertFalse(result["qualification_gate_changed"])

    def test_missing_parent_function_class_does_not_pass(self):
        checkpoint = {"seed": 2026100305, "generation_next": 7,
            "learner_state": {"primitives": {"prim:alias": {"expression": {"op": "get", "key": "y"}}}},
            "retained": [
                {"hidden_id": "hidden:x", "primitive_id": "prim:x", "expression": {"op": "get", "key": "x"}, "generation": 0},
                {"hidden_id": "hidden:y", "primitive_id": "prim:y", "expression": {"op": "get", "key": "y"}, "generation": 1}]}
        event = {"event": "CLOSURE_LINEAGE_MISMATCH", "generation": 6,
            "expected_parent_ids": ["prim:x", "prim:y"],
            "frontier_candidate_audit": [{
                "process_id": "wrong", "is_preselected_candidate": True,
                "closure_competent": True, "contains_expected_parents": False,
                "candidate_lineage": ["prim:alias"], "expression": {"op": "macro", "id": "prim:alias"}}]}
        result = summarize((json.dumps(event) + "\n").encode(), json.dumps(checkpoint).encode(),
                           require_pinned_hashes=False)
        self.assertFalse(result["all_unique_parent_function_classes_covered"])
        self.assertEqual(result["function_class_coverage_count"], 0)


if __name__ == "__main__":
    unittest.main()
