from __future__ import annotations

import json
import unittest

from lab.pr110_gen6_blind_selection_replay import summarize


def event():
    return {
        "event": "CLOSURE_LINEAGE_MISMATCH",
        "seed": 7,
        "generation": 6,
        "expected_parent_ids": ["secret-a", "secret-b"],
        "frontier_candidate_audit": [
            {
                "process_id": "first",
                "expression": {"op": "macro", "id": "p1"},
                "used_primitives": ["p1"],
                "candidate_lineage": ["p1"],
                "contains_expected_parents": False,
                "closure_competent": True,
            },
            {
                "process_id": "lineage-rich",
                "expression": {"op": "add", "left": {"op": "macro", "id": "p1"}, "right": {"op": "macro", "id": "p2"}},
                "used_primitives": ["p1", "p2"],
                "candidate_lineage": ["p1", "p2", "ancestor"],
                "contains_expected_parents": True,
                "closure_competent": True,
            },
        ],
    }


class BlindSelectionReplayTests(unittest.TestCase):
    def test_structural_features_change_choice_without_hidden_ids(self):
        report = summarize(json.dumps(event()), source_sha256="abc")
        self.assertEqual(report["policies"]["discovery_first"]["expected_lineage_match_count_posthoc"], 0)
        self.assertEqual(report["policies"]["max_ancestor_lineage_count"]["expected_lineage_match_count_posthoc"], 1)
        self.assertFalse(report["selection_contract"]["target_parent_ids_used_for_scoring"])
        self.assertFalse(report["selection_contract"]["candidate_correctness_labels_used_for_scoring"])

    def test_hidden_correctness_labels_do_not_change_choices(self):
        e1 = event()
        e2 = event()
        for candidate in e2["frontier_candidate_audit"]:
            candidate["contains_expected_parents"] = not candidate["contains_expected_parents"]
            candidate["closure_competent"] = not candidate["closure_competent"]
        a = summarize(json.dumps(e1), source_sha256="a")
        b = summarize(json.dumps(e2), source_sha256="b")
        for name in a["policies"]:
            self.assertEqual(
                [row.get("selected_process_id") for row in a["policies"][name]["rows"]],
                [row.get("selected_process_id") for row in b["policies"][name]["rows"]],
            )

    def test_empty_frontier_is_inconclusive(self):
        e = event()
        e["frontier_candidate_audit"] = []
        report = summarize(json.dumps(e), source_sha256="abc")
        self.assertEqual(report["policies"]["discovery_first"]["selected_candidate_count"], 0)
        self.assertEqual(report["policies"]["discovery_first"]["rows"][0]["status"], "INCONCLUSIVE_NO_CANDIDATES")


if __name__ == "__main__":
    unittest.main()
