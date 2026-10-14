from __future__ import annotations

import unittest

from cognitive_core.recursive_cognitive_compiler import (
    CognitivePrimitive,
    RecursiveCognitiveCompiler,
    Trace,
)


class FixedProcessCandidateCompiler(RecursiveCognitiveCompiler):
    """Tiny deterministic fixture for process-frontier ranking."""

    def __init__(self) -> None:
        super().__init__(max_depth=2, population=8, seed=7)
        self.shallow = {
            "op": "add",
            "left": {"op": "macro", "id": "p-a"},
            "right": {"op": "macro", "id": "p-b"},
        }
        self.deep = {
            "op": "add",
            "left": {"op": "macro", "id": "p-c"},
            "right": {"op": "macro", "id": "p-d"},
        }
        self.single_lineage_heavy = {"op": "macro", "id": "p-e"}
        self.candidate_set = [self.shallow, self.deep]

    def propose_search_policies(self):
        return ({"max_depth": 2},)

    def _exprs(self, keys, primitive_ids, depth_limit):
        # Both expressions are equally competent, have two direct macros, and
        # have the same local AST cost. The shallow option sorts first lexically.
        return list(self.candidate_set)


def primitive(pid: str, expression: dict, *, fake_provenance: tuple[str, ...] = ()):
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
        provenance={"lineage_ids": list(fake_provenance)},
    )


class ProcessFrontierExecutableLineageTieBreakTests(unittest.TestCase):
    def test_executable_ancestry_breaks_equal_direct_macro_and_node_cost_tie(self) -> None:
        compiler = FixedProcessCandidateCompiler()
        compiler.primitives = {
            "p-a": primitive("p-a", {"op": "const", "value": 2}, fake_provenance=("fake-a",)),
            "p-b": primitive("p-b", {"op": "const", "value": 3}, fake_provenance=("fake-b",)),
            "p-c": primitive(
                "p-c",
                {"op": "add", "left": {"op": "macro", "id": "p-a"}, "right": {"op": "const", "value": 0}},
                fake_provenance=("fake-c",),
            ),
            "p-d": primitive(
                "p-d",
                {"op": "add", "left": {"op": "macro", "id": "p-b"}, "right": {"op": "const", "value": 0}},
                fake_provenance=("fake-d",),
            ),
        }
        row = Trace(inputs={}, target=5.0, family="opaque", context={}, task_id="lineage-tie")
        frontier = compiler.synthesize_process_frontier((row,), (row,), (row,), max_candidates=32)

        self.assertEqual(len(frontier), 2)
        self.assertEqual(frontier[0].used_primitives, ("p-c", "p-d"))
        chosen_lineage = set()
        for pid in frontier[0].used_primitives:
            chosen_lineage.update(compiler.executable_lineage(pid))
        self.assertEqual(chosen_lineage, {"p-a", "p-b", "p-c", "p-d"})
        self.assertNotIn("fake-a", chosen_lineage)
        self.assertNotIn("fake-b", chosen_lineage)
        self.assertNotIn("fake-c", chosen_lineage)
        self.assertNotIn("fake-d", chosen_lineage)
        self.assertEqual(
            compiler.last_process_candidate_selection_summary["lineage_source"],
            "actual_executable_macro_references",
        )
        self.assertEqual(
            compiler.last_process_candidate_selection_summary["discovery_qualified"],
            2,
        )

    def test_direct_macro_count_remains_primary_over_lineage_cardinality(self) -> None:
        compiler = FixedProcessCandidateCompiler()
        compiler.primitives = {
            "p-a": primitive("p-a", {"op": "const", "value": 2}),
            "p-b": primitive("p-b", {"op": "const", "value": 3}),
            "p-c": primitive(
                "p-c",
                {"op": "add", "left": {"op": "macro", "id": "p-a"}, "right": {"op": "const", "value": 0}},
            ),
            "p-d": primitive(
                "p-d",
                {"op": "add", "left": {"op": "macro", "id": "p-b"}, "right": {"op": "const", "value": 0}},
            ),
            # One direct macro with five ancestors is still ranked below
            # candidates that preserve two direct macro references.
            "p-e": primitive(
                "p-e",
                {"op": "add", "left": {"op": "macro", "id": "p-c"}, "right": {"op": "macro", "id": "p-d"}},
            ),
        }
        compiler.candidate_set = [compiler.shallow, compiler.deep, compiler.single_lineage_heavy]
        frontier = compiler.synthesize_process_frontier(
            (Trace(inputs={}, target=5.0, family="opaque", context={}, task_id="lineage-tie"),),
            (),
            (),
        )
        self.assertEqual(frontier[0].used_primitives, ("p-c", "p-d"))
        self.assertEqual(frontier[1].used_primitives, ("p-a", "p-b"))
        self.assertEqual(frontier[-1].used_primitives, ("p-e",))



if __name__ == "__main__":
    unittest.main()
