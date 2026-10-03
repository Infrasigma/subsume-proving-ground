from __future__ import annotations

import random

from lab.acsie_open_ended_self_extending_proving import hard_bootstrap_expr
from cognitive_core.recursive_cognitive_compiler import ast_depth

ALLOWED_OPS = {"get", "const", "abs", "neg", "add", "sub", "mul", "max", "min"}
ALLOWED_CONSTS = {-2, -1, 0, 1, 2, 3, 4}


def walk(expr):
    yield expr
    op = expr["op"]
    if op in {"abs", "neg"}:
        yield from walk(expr["arg"])
    elif op in {"add", "sub", "mul", "max", "min"}:
        yield from walk(expr["left"])
        yield from walk(expr["right"])


def test_hard_bootstrap_is_inside_generic_enumerable_language():
    for seed in range(32):
        expr = hard_bootstrap_expr(random.Random(seed))
        assert ast_depth(expr) == 3
        for node in walk(expr):
            assert node["op"] in ALLOWED_OPS
            if node["op"] == "const":
                assert node["value"] in ALLOWED_CONSTS
