from __future__ import annotations

import unittest
from unittest.mock import patch

from cognitive_core.behavioral_search import BehavioralSearchResult
from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import (
    ArchiveRecord,
    CognitivePrimitive,
    Trace,
)


def primitive(pid: str, expression: dict) -> CognitivePrimitive:
    return CognitivePrimitive(
        primitive_id=pid,
        expression=expression,
        input_roles=(),
        output_semantics="delta",
        generation=0,
        parent_ids=(),
        source_families=("opaque",),
        train_error=0.0,
        holdout_error=0.0,
        transfer_error=0.0,
        complexity=1,
        provenance={"lineage_ids": ["not-executable-provenance"]},
    )


class OpenEndedCandidateRankingTests(unittest.TestCase):
    def setUp(self) -> None:
        self.compiler = OpenEndedRecursiveCognitiveCompiler(max_depth=2, population=8, seed=31)
        self.compiler.generation = 1
        self.compiler.primitives = {
            "p-a": primitive("p-a", {"op": "const", "value": 2.0}),
            "p-b": primitive("p-b", {"op": "const", "value": 3.0}),
            "p-e": primitive(
                "p-e",
                {
                    "op": "add",
                    "left": {
                        "op": "add",
                        "left": {"op": "macro", "id": "p-a"},
                        "right": {"op": "const", "value": 0.0},
                    },
                    "right": {"op": "macro", "id": "p-b"},
                },
            ),
        }
        self.compiler.archive = {
            pid: ArchiveRecord(p)
            for pid, p in self.compiler.primitives.items()
        }
        self.cheap = {
            "op": "add",
            "left": {"op": "macro", "id": "p-a"},
            "right": {"op": "macro", "id": "p-b"},
        }
        self.expensive_alias = {"op": "macro", "id": "p-e"}
        self.result = BehavioralSearchResult(
            expression=self.expensive_alias,
            used_primitives=("p-e",),
            stats={"status": "TEST_FIXTURE", "target_candidate_count": 2},
            # Deliberately put the more expensive one first: the receiving
            # runtime must enforce its own declared selection objective.
            alternatives=(
                (self.expensive_alias, ("p-e",)),
                (self.cheap, ("p-a", "p-b")),
            ),
        )
        self.rows = tuple(
            Trace(
                inputs={"x": float(i)},
                target=5.0,
                family="opaque",
                context={},
                task_id=f"cost-first-{i}",
            )
            for i in range(6)
        )

    def test_primitive_invention_reranks_alternatives_by_expanded_cost(self) -> None:
        with patch(
            "cognitive_core.open_ended_growth.find_exact_expression",
            return_value=self.result,
        ):
            primary = self.compiler.invent_primitive(self.rows, self.rows, self.rows)

        self.assertIsNotNone(primary)
        self.assertEqual(primary.expression, self.cheap)
        self.assertEqual(primary.provenance["selection_objective"], "minimize_expanded_cost")
        self.assertEqual(primary.provenance["expanded_execution_depth"], 2)
        self.assertEqual(primary.provenance["expanded_execution_nodes"], 3)
        self.assertEqual(self.compiler.last_primitive_candidate_frontier[0].primitive_id, primary.primitive_id)
        self.assertGreater(
            self.compiler.last_primitive_candidate_frontier[-1].provenance["expanded_execution_nodes"],
            primary.provenance["expanded_execution_nodes"],
        )
        self.assertEqual(self.compiler.last_primitive_candidate_selection_summary["selected_candidate_id"], primary.primitive_id)
        self.assertFalse(self.compiler.last_primitive_candidate_selection_summary["validation_split_used_for_selection"])

    def test_process_synthesis_reranks_alternatives_by_expanded_cost(self) -> None:
        with patch(
            "cognitive_core.open_ended_growth.find_exact_expression",
            return_value=self.result,
        ):
            primary = self.compiler.synthesize_process(self.rows, self.rows, self.rows)

        self.assertIsNotNone(primary)
        self.assertEqual(primary.expression, self.cheap)
        self.assertEqual(primary.provenance["selection_objective"], "minimize_expanded_cost")
        self.assertEqual(primary.provenance["expanded_execution_depth"], 2)
        self.assertEqual(primary.provenance["expanded_execution_nodes"], 3)
        self.assertEqual(self.compiler.last_process_candidate_frontier[0].process_id, primary.process_id)
        self.assertGreater(
            self.compiler.last_process_candidate_frontier[-1].provenance["expanded_execution_nodes"],
            primary.provenance["expanded_execution_nodes"],
        )
        self.assertEqual(self.compiler.last_process_candidate_selection_summary["selected_process_id"], primary.process_id)
        self.assertFalse(self.compiler.last_process_candidate_selection_summary["validation_split_used_for_selection"])


if __name__ == "__main__":
    unittest.main()
