from __future__ import annotations

import unittest
from unittest.mock import patch

from cognitive_core.behavioral_search import BehavioralSearchResult
from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import CognitivePrimitive, Trace


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
        provenance={"lineage_ids": ["provenance-only-must-not-count"]},
    )


class OpenEndedProcessExecutableLineageOrderTests(unittest.TestCase):
    def test_probe_path_preserves_executable_ancestry_when_macro_count_and_cost_tie(self) -> None:
        compiler = OpenEndedRecursiveCognitiveCompiler(max_depth=2, population=8, seed=23)
        compiler.generation = 1
        compiler.primitives = {
            "p-a": primitive("p-a", {"op": "const", "value": 2.0}),
            "p-b": primitive("p-b", {"op": "const", "value": 3.0}),
            "p-c": primitive(
                "p-c",
                {
                    "op": "add",
                    "left": {"op": "macro", "id": "p-a"},
                    "right": {"op": "const", "value": 0.0},
                },
            ),
            "p-d": primitive(
                "p-d",
                {
                    "op": "add",
                    "left": {"op": "macro", "id": "p-b"},
                    "right": {"op": "const", "value": 0.0},
                },
            ),
        }

        direct_expr = {
            "op": "add",
            "left": {"op": "macro", "id": "p-a"},
            "right": {"op": "macro", "id": "p-b"},
        }
        ancestor_rich_expr = {
            "op": "add",
            "left": {"op": "macro", "id": "p-c"},
            "right": {"op": "macro", "id": "p-d"},
        }
        result = BehavioralSearchResult(
            expression=direct_expr,
            used_primitives=("p-a", "p-b"),
            stats={"status": "TEST_FIXTURE"},
            alternatives=(
                (direct_expr, ("p-a", "p-b")),
                (ancestor_rich_expr, ("p-c", "p-d")),
            ),
        )
        row = Trace(
            inputs={"x": 1.0},
            target=5.0,
            family="opaque",
            context={},
            task_id="probe-frontier-lineage-order",
        )

        with patch(
            "cognitive_core.open_ended_growth.find_exact_expression",
            return_value=result,
        ):
            chosen = compiler.synthesize_process((row,), (row,), (row,))

        self.assertIsNotNone(chosen)
        self.assertEqual(chosen.used_primitives, ("p-c", "p-d"))
        self.assertEqual(
            compiler.last_process_candidate_frontier[0].used_primitives,
            ("p-c", "p-d"),
        )
        self.assertEqual(
            compiler.last_process_candidate_frontier[0].process_id,
            chosen.process_id,
        )
        executable = set()
        for pid in chosen.used_primitives:
            executable.update(compiler.executable_lineage(pid))
        self.assertEqual(executable, {"p-a", "p-b", "p-c", "p-d"})
        self.assertNotIn("provenance-only-must-not-count", executable)


if __name__ == "__main__":
    unittest.main()
