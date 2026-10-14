from __future__ import annotations

import unittest
from types import SimpleNamespace

from lab.acsie_open_ended_self_extending_proving import (
    select_discovery_ranked_frontier_head,
    selected_process_lineage_audit,
)


# This regression models selection of an earlier competent process while a later frontier item has different lineage.
class SelectedProcessLineageAuditTests(unittest.TestCase):
    def test_lineage_is_recomputed_for_selected_candidate_not_last_frontier_item(self) -> None:
        selected = SimpleNamespace(
            process_id="selected-first-competent",
            used_primitives=("prim:selected",),
            expression={"op": "macro", "id": "prim:selected"},
        )
        later_frontier_item = SimpleNamespace(
            process_id="later-lineage-matching-candidate",
            used_primitives=("prim:later",),
            expression={"op": "macro", "id": "prim:later"},
        )
        expected = {"prim:parent-a", "prim:parent-b"}

        by_used_primitives = {
            ("prim:selected",): {"prim:selected", "prim:ancestor"},
            ("prim:later",): {"prim:later", "prim:parent-a", "prim:parent-b"},
        }
        resolved: list[tuple[str, ...]] = []

        def resolve(used: tuple[str, ...]) -> set[str]:
            resolved.append(used)
            return by_used_primitives[used]

        # Model the historical loop: a later candidate has the desired lineage,
        # but the selection policy chose the earlier competent candidate.
        later_audit = selected_process_lineage_audit(
            later_frontier_item, expected, resolve, competent=True
        )
        self.assertTrue(later_audit["selected_candidate_reuse_ok"])

        selected_audit = selected_process_lineage_audit(
            selected, expected, resolve, competent=True
        )
        self.assertEqual(selected_audit["selected_process_id"], selected.process_id)
        self.assertEqual(selected_audit["used_primitives"], ["prim:selected"])
        self.assertEqual(selected_audit["expression"], selected.expression)
        self.assertEqual(selected_audit["candidate_lineage"], ["prim:ancestor", "prim:selected"])
        self.assertFalse(selected_audit["selected_candidate_contains_expected_parents"])
        self.assertFalse(selected_audit["selected_candidate_reuse_ok"])
        self.assertEqual(resolved, [("prim:later",), ("prim:selected",)])

    def test_validation_results_cannot_select_a_later_frontier_candidate(self) -> None:
        first = SimpleNamespace(process_id="first-ranked", validation_passes=False)
        later = SimpleNamespace(process_id="later-valid", validation_passes=True)

        # Selection takes only the precomputed discovery-ranked frontier; there
        # is no holdout/transfer/OOD argument that could trigger a fallback.
        selected = select_discovery_ranked_frontier_head((first, later))
        self.assertIs(selected, first)
        self.assertEqual(selected.process_id, "first-ranked")

        self.assertIsNone(select_discovery_ranked_frontier_head(()))


    def test_incompetent_candidate_cannot_count_as_successful_reuse(self) -> None:
        candidate = SimpleNamespace(
            process_id="incompetent",
            used_primitives=("prim:parent-a", "prim:parent-b"),
            expression={"op": "add"},
        )
        audit = selected_process_lineage_audit(
            candidate,
            {"prim:parent-a", "prim:parent-b"},
            lambda used: set(used),
            competent=False,
        )
        self.assertTrue(audit["selected_candidate_contains_expected_parents"])
        self.assertFalse(audit["selected_candidate_reuse_ok"])

    def test_missing_selection_is_explicit_and_does_not_call_resolver(self) -> None:
        def must_not_be_called(used: tuple[str, ...]) -> set[str]:
            raise AssertionError(f"unexpected lineage query for {used!r}")

        audit = selected_process_lineage_audit(
            None,
            {"prim:parent-a"},
            must_not_be_called,
            competent=False,
        )
        self.assertIsNone(audit["selected_process_id"])
        self.assertEqual(audit["used_primitives"], [])
        self.assertIsNone(audit["expression"])
        self.assertEqual(audit["candidate_lineage"], [])
        self.assertFalse(audit["selected_candidate_reuse_ok"])


if __name__ == "__main__":
    unittest.main()
