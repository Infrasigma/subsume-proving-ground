"""Minimal task-independent executable meta-language for ASI-CORE-004.

This module is deliberately semantic-light.  It provides generic computational
facilities only: values, variables, data construction, inspection, comparison,
arithmetic, conditional control, mapping, folding, composition and bounded
repetition.  No target-task operator is named here.

The interpreter is part of the substrate.  Operator search and registration live
on ``GeneralLearner``; this file is only the executable representation of a
constructed function.
"""
from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Iterable, Mapping
import copy

Json = Any


class MetaLanguageError(ValueError):
    pass


MAX_RECURSION_STEPS = 256


def _canon_expr(expr: Any) -> Any:
    if isinstance(expr, tuple):
        return [_canon_expr(x) for x in expr]
    if isinstance(expr, list):
        return [_canon_expr(x) for x in expr]
    if isinstance(expr, dict):
        return {str(k): _canon_expr(v) for k, v in sorted(expr.items(), key=lambda kv: str(kv[0]))}
    return expr


def expr_size(expr: Any) -> int:
    if isinstance(expr, dict):
        return 1 + sum(expr_size(v) for v in expr.values())
    if isinstance(expr, (list, tuple)):
        return 1 + sum(expr_size(v) for v in expr)
    return 1


def program_size(program: Mapping[str, Any]) -> int:
    return expr_size(program)


def _lookup(env: Mapping[str, Json], name: str) -> Json:
    if name not in env:
        raise MetaLanguageError(f"unbound variable: {name}")
    return env[name]


def eval_expr(expr: Any, env: Mapping[str, Json], *, depth: int = 0) -> Json:
    if depth > MAX_RECURSION_STEPS:
        raise MetaLanguageError("meta-language recursion bound exceeded")
    if not isinstance(expr, Mapping):
        return copy.deepcopy(expr)
    op = expr.get("op")
    args = expr.get("args", ())
    if op == "const":
        return copy.deepcopy(expr.get("value"))
    if op == "var":
        return copy.deepcopy(_lookup(env, str(expr["name"])))
    if op == "get":
        base = eval_expr(expr["base"], env, depth=depth + 1)
        key = eval_expr(expr["key"], env, depth=depth + 1)
        try:
            if isinstance(base, Mapping):
                return copy.deepcopy(base[key])
            return copy.deepcopy(base[key])
        except (KeyError, IndexError, TypeError):
            raise MetaLanguageError("inspection failed")
    if op == "len":
        value = eval_expr(expr["value"], env, depth=depth + 1)
        try:
            return len(value)
        except TypeError:
            raise MetaLanguageError("length requires a sized value")
    if op == "tuple":
        return [eval_expr(a, env, depth=depth + 1) for a in args]
    if op == "list":
        return [eval_expr(a, env, depth=depth + 1) for a in args]
    if op in {"add", "sub", "mul", "mod", "eq", "lt", "gt", "and", "or"}:
        left = eval_expr(expr["left"], env, depth=depth + 1)
        right = eval_expr(expr["right"], env, depth=depth + 1)
        try:
            if op == "add": return left + right
            if op == "sub": return left - right
            if op == "mul": return left * right
            if op == "mod": return left % right
            if op == "eq": return left == right
            if op == "lt": return left < right
            if op == "gt": return left > right
            if op == "and": return bool(left and right)
            if op == "or": return bool(left or right)
        except Exception as exc:
            raise MetaLanguageError(f"binary operation failed: {op}") from exc
    if op == "not":
        return not bool(eval_expr(expr["value"], env, depth=depth + 1))
    if op == "if":
        cond = bool(eval_expr(expr["cond"], env, depth=depth + 1))
        return eval_expr(expr["then"] if cond else expr["else"], env, depth=depth + 1)
    if op == "call":
        fn = eval_expr(expr["fn"], env, depth=depth + 1)
        values = [eval_expr(a, env, depth=depth + 1) for a in args]
        if not callable(fn):
            raise MetaLanguageError("call target is not callable")
        return fn(*values)
    raise MetaLanguageError(f"unknown meta-language op: {op}")


def _execute_fold(program: Mapping[str, Any], value: Json) -> Json:
    seq = eval_expr(program["input"], {"input": value})
    if not isinstance(seq, Iterable) or isinstance(seq, (str, bytes, Mapping)):
        raise MetaLanguageError("fold input must be an iterable sequence")
    state = {k: copy.deepcopy(v) for k, v in program.get("init", {}).items()}
    updates = program.get("updates", {})
    for index, item in enumerate(seq):
        previous = copy.deepcopy(state)
        env = {"input": value, "item": copy.deepcopy(item), "index": index, "state": previous}
        env.update({f"state:{k}": copy.deepcopy(v) for k, v in previous.items()})
        next_state = dict(previous)
        for key, expression in updates.items():
            next_state[key] = eval_expr(expression, env)
        state = next_state
    env = {"input": value, "state": state}
    env.update({f"state:{k}": copy.deepcopy(v) for k, v in state.items()})
    return eval_expr(program["output"], env)


def _execute_map(program: Mapping[str, Any], value: Json) -> Json:
    seq = eval_expr(program["input"], {"input": value})
    if not isinstance(seq, Iterable) or isinstance(seq, (str, bytes, Mapping)):
        raise MetaLanguageError("map input must be an iterable sequence")
    body = program["body"]
    return [eval_expr(body, {"input": value, "item": copy.deepcopy(item), "index": i}) for i, item in enumerate(seq)]


def _execute_repeat(program: Mapping[str, Any], value: Json) -> Json:
    count = int(eval_expr(program["count"], {"input": value}))
    if count < 0 or count > MAX_RECURSION_STEPS:
        raise MetaLanguageError("repeat count outside bound")
    out = copy.deepcopy(value)
    body = program["body"]
    for i in range(count):
        out = eval_expr(body, {"input": out, "index": i})
    return out


def execute_program(program: Mapping[str, Any], value: Json, *, max_steps: int = MAX_RECURSION_STEPS) -> Json | None:
    """Execute one generic constructed function.  Returns None on a type/domain failure."""
    try:
        kind = program.get("kind")
        if kind == "identity":
            return copy.deepcopy(value)
        if kind == "map":
            return _execute_map(program, value)
        if kind == "fold":
            return _execute_fold(program, value)
        if kind == "repeat":
            return _execute_repeat(program, value)
        if kind == "compose":
            out = copy.deepcopy(value)
            for child in program.get("programs", ()):
                out = execute_program(child, out, max_steps=max_steps)
                if out is None:
                    return None
            return out
        if kind == "apply":
            fn = program.get("function")
            args = program.get("args", ())
            if not isinstance(fn, Mapping):
                return None
            values = [execute_program(a, value, max_steps=max_steps) for a in args]
            if any(v is None for v in values):
                return None
            local = {"input": value, "args": values}
            return eval_expr(program["result"], local)
        raise MetaLanguageError(f"unknown program kind: {kind}")
    except (MetaLanguageError, TypeError, ValueError, IndexError, KeyError, ZeroDivisionError):
        return None


@dataclass(frozen=True)
class GenericOperator:
    operator_id: str
    signature: str
    input_type: str
    output_type: str
    body: Mapping[str, Any]

    def apply(self, value: Json) -> Json | None:
        return execute_program(self.body, value)

    @property
    def complexity(self) -> int:
        return program_size(self.body)

    def canonical_body(self) -> Any:
        return _canon_expr(self.body)
