import unittest

from lab.pr110_h7_offline_counterfactual import KS, POLICIES, greedy_select, summarize_capture


def rep(token, used, lineage, depth, nodes, key):
    return {
        "state_token": token,
        "used_primitives": used,
        "executable_lineage_ids": lineage,
        "expanded_execution_depth": depth,
        "expanded_execution_nodes": nodes,
        "expression_key": key,
    }


class CounterfactualCoverageTests(unittest.TestCase):
    def setUp(self):
        # A directly executes A, whose macro body also references B. B is a
        # direct macro option too. Keeping A preserves transitive B lineage,
        # but does NOT preserve direct executable use of B.
        self.reps = [
            rep(0, ["A"], ["A", "B"], 1, 3, "expr-A"),
            rep(1, ["B"], ["B"], 2, 4, "expr-B"),
        ]
        self.data = {
            "schema": "test",
            "runtime_sha": "runtime",
            "proving_sha": "proving",
            "seed": 1,
            "generation": 5,
            "captures": [{
                "generation": 5,
                "depth": 2,
                "class_count": 1,
                "classes": [{
                    "behavior_key": {"outputs": [1.0, 0.0], "free_keys": []},
                    "representatives": self.reps,
                }],
            }],
        }

    def test_lineage_only_can_hide_direct_vocabulary_loss(self):
        selected = greedy_select(self.reps, 1, "lineage_only")
        self.assertEqual([r["state_token"] for r in selected], [0])
        report = summarize_capture(self.data, 1, "lineage_only")
        self.assertEqual(report["transitive_lineage_coverage_retained"], 1.0)
        self.assertEqual(report["direct_vocabulary_coverage_retained"], 0.5)
        self.assertEqual(report["classes_with_direct_vocabulary_loss"], 1)
        self.assertEqual(report["classes_with_transitive_lineage_loss"], 0)
        self.assertFalse(report["direct_coverage_lossless_by_class"])

    def test_unbounded_control_retains_all_identities(self):
        report = summarize_capture(self.data, None, "lineage_only")
        self.assertEqual(report["fraction_removed"], 0.0)
        self.assertEqual(report["direct_vocabulary_coverage_retained"], 1.0)
        self.assertEqual(report["transitive_lineage_coverage_retained"], 1.0)
        self.assertTrue(report["direct_coverage_lossless_by_class"])
        self.assertTrue(report["lineage_coverage_lossless_by_class"])

    def test_all_policies_are_deterministic(self):
        for policy in POLICIES:
            first = greedy_select(self.reps, 1, policy)
            second = greedy_select(self.reps, 1, policy)
            self.assertEqual([x["state_token"] for x in first], [x["state_token"] for x in second])

    def test_all_k_values_are_positive_or_unbounded(self):
        self.assertEqual(KS, (1, 2, 4, 8, None))
        self.assertTrue(all(k is None or k > 0 for k in KS))

    def test_scope_does_not_claim_subsumption_proof(self):
        report = summarize_capture(self.data, 1, "dual_coverage")
        self.assertFalse(report["representative_subsumption_proved"])
        self.assertEqual(
            report["by_depth"]["2"]["direct_coverage_lossless_by_class"],
            False,
        )


if __name__ == "__main__":
    unittest.main()
