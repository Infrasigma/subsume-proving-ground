from __future__ import annotations

import unittest

from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import Evaluation, ProcessCandidate


def process(pid: str) -> ProcessCandidate:
    return ProcessCandidate(
        process_id=pid,
        expression={"op": "const", "value": 1.0},
        used_primitives=(),
        generation=0,
        parent_ids=(),
        search_policy={},
        source_families=("opaque",),
        complexity=1,
        provenance={"selection_source": "discovery_and_selection_only"},
    )


class ProcessFrontierValidationSeparationTests(unittest.TestCase):
    def setUp(self) -> None:
        self.compiler = OpenEndedRecursiveCognitiveCompiler(max_depth=2, population=8, seed=19)
        self.first = process("process:first-discovery-ranked")
        self.later = process("process:later-validation-pass")
        self.compiler.last_process_candidate_frontier = (self.first, self.later)
        self.calls: list[str] = []

    def fake_evaluate(self, proc, train, holdout, transfer, ood, conflict):
        self.calls.append(proc.process_id)
        passing = proc.process_id == self.later.process_id
        return Evaluation(
            train_error=0.0,
            holdout_error=0.0,
            transfer_error=0.0 if passing else 1.0,
            ood_error=0.0 if passing else 1.0,
            interference_harm=0.0,
            resource_cost=1.0,
            novelty=0.0,
            accepted=passing,
        )

    def test_later_validation_pass_cannot_replace_first_discovery_candidate(self) -> None:
        self.compiler.evaluate = self.fake_evaluate
        selected, evaluation, summary = self.compiler.validate_process_frontier(
            train=(), holdout=(), transfer=(), ood=()
        )

        self.assertIsNone(selected)
        self.assertIsNone(evaluation)
        self.assertEqual(self.calls, [self.first.process_id])
        self.assertEqual(summary["candidate_count"], 2)
        self.assertEqual(summary["validation_attempted_count"], 1)
        self.assertEqual(summary["validated_candidate_count"], 0)
        self.assertEqual(summary["selected_candidate_id"], self.first.process_id)
        self.assertFalse(summary["validation_fallback_used"])
        self.assertEqual(
            [x["process_id"] for x in summary["validation"]],
            [self.first.process_id],
        )

    def test_first_discovery_candidate_is_admitted_when_it_passes(self) -> None:
        def all_pass(proc, train, holdout, transfer, ood, conflict):
            self.calls.append(proc.process_id)
            return Evaluation(0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, True)

        self.compiler.evaluate = all_pass
        selected, evaluation, summary = self.compiler.validate_process_frontier(
            train=(), holdout=(), transfer=(), ood=()
        )

        self.assertIs(selected, self.first)
        self.assertIsNotNone(evaluation)
        self.assertEqual(self.calls, [self.first.process_id])
        self.assertEqual(summary["validated_candidate_count"], 1)
        self.assertFalse(summary["validation_fallback_used"])


if __name__ == "__main__":
    unittest.main()
