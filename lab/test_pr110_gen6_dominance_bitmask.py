from __future__ import annotations

import json
import os
import unittest

from cognitive_core.behavioral_search import find_exact_expression
from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import CognitivePrimitive, Trace, eval_expr


class DominanceBitmaskParityTests(unittest.TestCase):
    def test_set_and_bitset_modes_have_identical_search_result_and_stats(self) -> None:
        compiler = OpenEndedRecursiveCognitiveCompiler(
            max_depth=3, population=24, seed=2026100305
        )
        # Distinct macro IDs and parent graphs deliberately create multiple
        # incomparable lineage sets over identical observed behavior.
        primitive_specs = [
            ("prim:bitset-a", {"op": "get", "key": "x"}, ()),
            ("prim:bitset-b", {"op": "get", "key": "x"}, ("prim:bitset-a",)),
            ("prim:bitset-c", {"op": "get", "key": "x"}, ("prim:bitset-a", "prim:bitset-b")),
            ("prim:bitset-d", {"op": "get", "key": "y"}, ()),
            ("prim:bitset-e", {"op": "get", "key": "y"}, ("prim:bitset-d",)),
            ("prim:bitset-f", {"op": "neg", "arg": {"op": "get", "key": "x"}}, ()),
            ("prim:bitset-g", {"op": "neg", "arg": {"op": "get", "key": "x"}}, ("prim:bitset-f",)),
        ]
        for pid, expression, parents in primitive_specs:
            compiler.primitives[pid] = CognitivePrimitive(
                primitive_id=pid,
                expression=expression,
                input_roles=("x", "y"),
                output_semantics="synthetic",
                generation=0,
                parent_ids=parents,
                source_families=("synthetic-dominance-bitset-parity",),
                train_error=0.0,
                holdout_error=0.0,
                transfer_error=0.0,
                complexity=1,
                provenance={"origin": "synthetic_test", "external_model": False},
            )

        rows = tuple(
            Trace(
                inputs={"x": float(x), "y": float(2 * x)},
                target=2.0 * float(x),
                family="synthetic-dominance-bitset-parity",
                context={},
                task_id=f"bitset-parity:{i}",
            )
            for i, x in enumerate((-9, -7, -5, -3, -2, -1, 1, 2, 3, 5, 7, 9, 11, 13))
        )

        old_mode = os.environ.get("ACSIE_DOMINANCE_BITSETS")
        old_profile = os.environ.pop("ACSIE_SEARCH_PROFILE", None)
        old_trace = os.environ.pop("ACSIE_SEARCH_TRACE", None)
        try:
            os.environ["ACSIE_DOMINANCE_BITSETS"] = "0"
            reference = find_exact_expression(
                compiler, rows, 3, require_macro=True, max_alternatives=8
            )
            os.environ["ACSIE_DOMINANCE_BITSETS"] = "1"
            candidate = find_exact_expression(
                compiler, rows, 3, require_macro=True, max_alternatives=8
            )
        finally:
            if old_mode is None:
                os.environ.pop("ACSIE_DOMINANCE_BITSETS", None)
            else:
                os.environ["ACSIE_DOMINANCE_BITSETS"] = old_mode
            if old_profile is not None:
                os.environ["ACSIE_SEARCH_PROFILE"] = old_profile
            if old_trace is not None:
                os.environ["ACSIE_SEARCH_TRACE"] = old_trace

        self.assertIsNotNone(reference, "set reference failed to find exact candidate")
        self.assertIsNotNone(candidate, "bitset candidate failed to find exact candidate")
        assert reference is not None and candidate is not None
        self.assertEqual(reference.expression, candidate.expression)
        self.assertEqual(reference.used_primitives, candidate.used_primitives)
        self.assertEqual(reference.alternatives, candidate.alternatives)
        self.assertEqual(dict(reference.stats), dict(candidate.stats))

        pmap = compiler._pmap()
        for result in (reference, candidate):
            self.assertTrue(
                all(abs(eval_expr(result.expression, row.inputs, pmap) - row.target) <= 1e-9 for row in rows)
            )

    def test_bitset_subset_relation_matches_set_subset_for_many_masks(self) -> None:
        # Independent exhaustive check of the exact encoding used by RC1.
        from itertools import combinations

        universe = tuple(f"id:{i}" for i in range(7))
        masks: list[int] = []
        sets: list[frozenset[str]] = []
        for size in range(len(universe) + 1):
            for members in combinations(universe, size):
                values = frozenset(members)
                mask = sum(1 << universe.index(item) for item in values)
                sets.append(values)
                masks.append(mask)
        for left_set, left_mask in zip(sets, masks):
            for right_set, right_mask in zip(sets, masks):
                self.assertEqual(
                    right_set.issubset(left_set),
                    (right_mask & ~left_mask) == 0,
                )


if __name__ == "__main__":
    unittest.main()
