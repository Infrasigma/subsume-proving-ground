from __future__ import annotations

import copy
import struct
import unittest

from cognitive_core.recursive_cognitive_compiler import (
    RecursiveCognitiveCompiler,
    Trace,
    eval_expr,
)
from lab.acsie_open_ended_self_extending_proving import synthesize_discovery_only
from lab.h7_cached_baseline_eval import eval_expr_cached, synthesize_discovery_only_cached


def same_float(a: float, b: float) -> bool:
    return struct.pack("!d", float(a)) == struct.pack("!d", float(b))


def expression_nodes(expr):
    yield expr
    op = expr["op"]
    if op in {"abs", "neg", "threshold"}:
        yield from expression_nodes(expr["arg"])
    elif op in {"add", "sub", "mul", "max", "min"}:
        yield from expression_nodes(expr["left"])
        yield from expression_nodes(expr["right"])


class FakeCompiler:
    """Small deterministic stand-in exercising the real baseline selector."""

    def __init__(self):
        self.generation = 5
        self.primitives = {
            "prim:double": type(
                "Primitive",
                (),
                {"expression": {"op": "mul", "left": {"op": "get", "key": "x"},
                                "right": {"op": "const", "value": 2.0}}},
            )()
        }
        self.processes = {}
        self.policy_rows = (
            {"max_depth": 2, "novelty_weight": 0.1},
            {"max_depth": 3, "novelty_weight": 0.2},
        )
        self.expressions = [
            {"op": "get", "key": "x"},
            {"op": "macro", "id": "prim:double"},
            {"op": "add", "left": {"op": "macro", "id": "prim:double"},
             "right": {"op": "const", "value": 0.0}},
            {"op": "sub", "left": {"op": "macro", "id": "prim:double"},
             "right": {"op": "const", "value": 0.0}},
            {"op": "neg", "arg": {"op": "neg", "arg": {"op": "macro", "id": "prim:double"}}},
        ]

    def _pmap(self):
        return {k: v.expression for k, v in self.primitives.items()}

    def propose_search_policies(self):
        return tuple(dict(x) for x in self.policy_rows)

    def _exprs(self, keys, primitive_ids, depth_limit):
        # Return a fresh outer list but stable expression objects, like the
        # retained expression cache used in the runtime compiler.
        return list(self.expressions)

    def _walk(self, expr):
        yield expr
        op = expr["op"]
        if op in {"add", "sub", "mul", "max", "min"}:
            yield from self._walk(expr["left"])
            yield from self._walk(expr["right"])
        elif op in {"abs", "neg", "threshold"}:
            yield from self._walk(expr["arg"])


class CachedEvaluatorTests(unittest.TestCase):
    def setUp(self):
        self.macros = {
            "m0": {"op": "add", "left": {"op": "get", "key": "x"},
                   "right": {"op": "const", "value": 1.25}},
            "m1": {"op": "mul", "left": {"op": "macro", "id": "m0"},
                   "right": {"op": "get", "key": "y"}},
            "m2": {"op": "neg", "arg": {"op": "macro", "id": "m1"}},
        }
        self.rows = (
            {"x": -3.0, "y": 2.0},
            {"x": 0.25, "y": -4.0},
            {"x": 7.0, "y": 0.5},
        )
        self.exprs = [
            {"op": "const", "value": 0.75},
            {"op": "get", "key": "x"},
            {"op": "macro", "id": "m0"},
            {"op": "macro", "id": "m1"},
            {"op": "macro", "id": "m2"},
            {"op": "abs", "arg": {"op": "macro", "id": "m2"}},
            {"op": "neg", "arg": {"op": "macro", "id": "m1"}},
            {"op": "threshold", "arg": {"op": "macro", "id": "m0"}, "lo": 1.0},
            {"op": "add", "left": {"op": "macro", "id": "m0"},
             "right": {"op": "macro", "id": "m1"}},
            {"op": "sub", "left": {"op": "macro", "id": "m1"},
             "right": {"op": "macro", "id": "m0"}},
            {"op": "mul", "left": {"op": "macro", "id": "m1"},
             "right": {"op": "get", "key": "x"}},
            {"op": "max", "left": {"op": "macro", "id": "m2"},
             "right": {"op": "get", "key": "y"}},
            {"op": "min", "left": {"op": "macro", "id": "m0"},
             "right": {"op": "get", "key": "y"}},
        ]

    def test_memoized_scalar_values_are_bit_exact_for_all_operators(self):
        counters = {"calls": 0, "cache_hits": 0, "cache_misses": 0}
        key_cache = {}
        for row in self.rows:
            row_cache = {}
            for expr in self.exprs:
                expected = eval_expr(expr, row, self.macros)
                actual = eval_expr_cached(expr, row, self.macros, row_cache, key_cache, counters)
                self.assertTrue(same_float(expected, actual), (expr, row, expected, actual))
        self.assertGreater(counters["cache_hits"], 0)
        self.assertGreater(counters["cache_misses"], 0)

    def test_row_caches_never_leak_values_between_distinct_inputs(self):
        expr = {"op": "add", "left": {"op": "get", "key": "x"},
                "right": {"op": "const", "value": 2.0}}
        key_cache = {}
        counters = {"calls": 0, "cache_hits": 0, "cache_misses": 0}
        first = eval_expr_cached(expr, {"x": 1.0}, {}, {}, key_cache, counters)
        second = eval_expr_cached(expr, {"x": 9.0}, {}, {}, key_cache, counters)
        self.assertEqual(first, 3.0)
        self.assertEqual(second, 11.0)

    def test_full_selector_picks_same_process_as_legacy_on_control_fixture(self):
        rows = tuple(
            Trace({"x": float(x)}, 2.0 * float(x), "fixture", {}, f"row-{x}")
            for x in (-3, -1, 0, 2, 5, 9)
        )
        transfer = tuple(
            Trace({"x": float(x)}, 2.0 * float(x), "transfer-fixture", {}, f"transfer-{x}")
            for x in (-2, 1, 4)
        )
        baseline = FakeCompiler()
        candidate = FakeCompiler()
        expected = synthesize_discovery_only(baseline, rows, transfer)
        metrics = {}
        actual = synthesize_discovery_only_cached(candidate, rows, transfer, metrics_out=metrics)
        self.assertIsNotNone(expected)
        self.assertIsNotNone(actual)
        self.assertEqual(expected.process_id, actual.process_id)
        self.assertEqual(expected.expression, actual.expression)
        self.assertEqual(expected.used_primitives, actual.used_primitives)
        self.assertEqual(expected.search_policy, actual.search_policy)
        self.assertEqual(expected.source_families, actual.source_families)
        self.assertEqual(expected.complexity, actual.complexity)
        self.assertEqual(metrics["selected_process_id"], expected.process_id)
        self.assertGreater(metrics["cache_hits"], 0)


if __name__ == "__main__":
    unittest.main()
