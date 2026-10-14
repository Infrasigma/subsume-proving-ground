from __future__ import annotations

import unittest

from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive, Trace


def make_primitive(pid: str, value: float, transfer_error: float) -> CognitivePrimitive:
    return CognitivePrimitive(
        primitive_id=pid,
        expression={"op": "const", "value": value},
        input_roles=("x",),
        output_semantics="delta",
        generation=0,
        parent_ids=(),
        source_families=("opaque",),
        train_error=0.0,
        holdout_error=0.0,
        transfer_error=transfer_error,
        complexity=1,
        provenance={"external_model": False},
    )


class PrimitiveFrontierValidationSeparationTests(unittest.TestCase):
    def setUp(self) -> None:
        self.compiler = OpenEndedRecursiveCognitiveCompiler(max_depth=2, population=8, seed=17)
        self.rows = (
            Trace(
                inputs={"x": 0.0},
                target=2.0,
                family="opaque",
                context={},
                task_id="primitive-frontier-validation",
            ),
        )

    def install(self, candidates: tuple[CognitivePrimitive, ...]) -> None:
        self.compiler.primitives = {p.primitive_id: p for p in candidates}
        self.compiler.archive = {p.primitive_id: ArchiveRecord(p) for p in candidates}
        self.compiler.last_primitive_candidate_frontier = candidates

    def test_later_passing_alternative_cannot_replace_selected_failing_candidate(self) -> None:
        selected = make_primitive("prim:selected", 1.0, transfer_error=1.0)
        later_passing = make_primitive("prim:later-passing", 2.0, transfer_error=0.0)
        self.install((selected, later_passing))

        result = self.compiler.accept_primitive_frontier(
            transfer_rows=self.rows,
            ood_rows=self.rows,
            novelty=0.1,
            resource_cost=1.0,
        )

        self.assertFalse(result["accepted"])
        self.assertIsNone(result["primary_primitive"])
        self.assertEqual(result["selected_candidate_id"], "prim:selected")
        self.assertEqual(result["candidate_count"], 2)
        self.assertEqual(result["validated_candidate_count"], 1)
        self.assertFalse(result["validation_fallback_used"])
        self.assertEqual([x["primitive_id"] for x in result["validation"]], ["prim:selected", "prim:later-passing"])
        self.assertFalse(result["validation"][0]["passed"])
        self.assertTrue(result["validation"][1]["passed"])
        self.assertEqual(self.compiler.primitives, {})
        self.assertEqual(self.compiler.archive, {})
        self.assertEqual(self.compiler.last_primitive_candidate_frontier, ())

    def test_first_discovery_candidate_is_retained_when_it_passes(self) -> None:
        selected = make_primitive("prim:selected", 2.0, transfer_error=0.0)
        later = make_primitive("prim:later", 2.0, transfer_error=0.0)
        self.install((selected, later))

        result = self.compiler.accept_primitive_frontier(
            transfer_rows=self.rows,
            ood_rows=self.rows,
            novelty=0.1,
            resource_cost=1.0,
        )

        self.assertTrue(result["accepted"])
        self.assertEqual(result["primary_primitive"].primitive_id, "prim:selected")
        self.assertEqual(result["accepted_candidate_ids"], ["prim:selected", "prim:later"])
        self.assertEqual(result["candidate_count"], 2)
        self.assertEqual(result["selected_candidate_id"], "prim:selected")
        self.assertEqual(result["validated_candidate_count"], 2)
        self.assertFalse(result["validation_fallback_used"])
        self.assertEqual([x["primitive_id"] for x in result["validation"]], ["prim:selected", "prim:later"])
        self.assertEqual(set(self.compiler.primitives), {"prim:selected", "prim:later"})
        self.assertEqual(set(self.compiler.archive), {"prim:selected", "prim:later"})
        self.assertEqual(self.compiler.last_primitive_candidate_frontier, (selected, later))


if __name__ == "__main__":
    unittest.main()
