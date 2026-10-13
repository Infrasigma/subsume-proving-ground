#!/usr/bin/env python3
"""Exact-semantics memoized scalar evaluator for the proving-ground baseline.

This is an isolated research helper, not a production ACSIE change. It preserves
the scalar interpreter's operation order and candidate selection sequence while
memoizing pure expression values per discovery row for one baseline synthesis call.
"""
from __future__ import annotations

import hashlib
import json
from typing import Any, Mapping, MutableMapping, Sequence

from cognitive_core.recursive_cognitive_compiler import ProcessCandidate

Expression = Mapping[str, Any]


def _expr_key(
    expr: Expression,
    key_cache: MutableMapping[int, tuple[Expression, str]],
) -> str:
    ident = id(expr)
    entry = key_cache.get(ident)
    if entry is not None and entry[0] is expr:
        return entry[1]
    key = json.dumps(expr, sort_keys=True, separators=(",", ":"))
    # Retain the object alongside its key so CPython cannot reuse the id for a
    # different expression while this invocation-local cache remains alive.
    key_cache[ident] = (expr, key)
    return key


def eval_expr_cached(
    expr: Expression,
    inputs: Mapping[str, float],
    macros: Mapping[str, Expression],
    row_cache: MutableMapping[str, float],
    key_cache: MutableMapping[int, tuple[Expression, str]],
    counters: MutableMapping[str, int],
) -> float:
    """Evaluate the exact scalar semantics with a cache scoped to one input row."""
    key = _expr_key(expr, key_cache)
    counters["calls"] = counters.get("calls", 0) + 1
    if key in row_cache:
        counters["cache_hits"] = counters.get("cache_hits", 0) + 1
        return row_cache[key]

    counters["cache_misses"] = counters.get("cache_misses", 0) + 1
    op = expr["op"]
    if op == "const":
        value = float(expr["value"])
    elif op == "get":
        value = float(inputs[expr["key"]])
    elif op == "macro":
        value = eval_expr_cached(
            macros[expr["id"]], inputs, macros, row_cache, key_cache, counters
        )
    elif op == "abs":
        value = abs(eval_expr_cached(expr["arg"], inputs, macros, row_cache, key_cache, counters))
    elif op == "neg":
        value = -eval_expr_cached(expr["arg"], inputs, macros, row_cache, key_cache, counters)
    elif op == "threshold":
        child = eval_expr_cached(expr["arg"], inputs, macros, row_cache, key_cache, counters)
        value = 1.0 if child >= float(expr["lo"]) else 0.0
    else:
        left = eval_expr_cached(expr["left"], inputs, macros, row_cache, key_cache, counters)
        right = eval_expr_cached(expr["right"], inputs, macros, row_cache, key_cache, counters)
        if op == "add":
            value = left + right
        elif op == "sub":
            value = left - right
        elif op == "mul":
            value = left * right
        elif op == "max":
            value = max(left, right)
        elif op == "min":
            value = min(left, right)
        else:
            raise ValueError(op)

    row_cache[key] = value
    return value


def synthesize_discovery_only_cached(
    compiler,
    rows: tuple,
    transfer: tuple,
    *,
    metrics_out: MutableMapping[str, Any] | None = None,
):
    """Same baseline selector, with only deterministic row-expression memoization.

    Candidate order, policy order, train-error formula, acceptance threshold,
    tie-breaking, selected expression, and ProcessCandidate metadata mirror
    lab.acsie_open_ended_self_extending_proving.synthesize_discovery_only.
    Transfer is used only for retained process metadata, exactly as in the
    original; it does not enter candidate evaluation or ranking.
    """
    keys = sorted(set().union(*(trace.inputs.keys() for trace in rows)))
    pids = tuple(compiler.primitives)
    candidates = []
    pmap = compiler._pmap()
    row_caches: list[dict[str, float]] = [dict() for _ in rows]
    key_cache: dict[int, tuple[Expression, str]] = {}
    counters: dict[str, int] = {
        "candidate_expression_count_seen": 0,
        "row_evaluation_requests": 0,
        "calls": 0,
        "cache_hits": 0,
        "cache_misses": 0,
    }

    for policy in compiler.propose_search_policies():
        expressions = compiler._exprs(keys, pids, min(3, int(policy["max_depth"])))
        for expr in expressions:
            counters["candidate_expression_count_seen"] += 1
            try:
                per_row_errors = []
                for idx, trace in enumerate(rows):
                    prediction = eval_expr_cached(
                        expr, trace.inputs, pmap, row_caches[idx], key_cache, counters
                    )
                    counters["row_evaluation_requests"] += 1
                    per_row_errors.append(abs(prediction - trace.target))
                train_error = sum(per_row_errors) / len(rows)
                if train_error > 1e-9:
                    continue

                used = tuple(sorted({
                    str(node["id"])
                    for node in compiler._walk(expr)
                    if node.get("op") == "macro"
                }))
                novelty = 1.0 / (1 + len(json.dumps(expr, sort_keys=True)))
                candidates.append((
                    len(used),
                    novelty,
                    -len(json.dumps(expr, sort_keys=True)),
                    json.dumps(expr, sort_keys=True),
                    expr,
                    used,
                    policy,
                ))
            except Exception:
                continue

    if not candidates:
        result = None
        if metrics_out is not None:
            metrics_out.update(counters)
            metrics_out["qualified_candidate_count"] = 0
            metrics_out["selected_process_id"] = None
        return result

    _, _, _, _, expr, used, policy = max(
        candidates,
        key=lambda candidate: (candidate[0], candidate[1], candidate[2], candidate[3]),
    )
    process_id = "baseline-proc:" + hashlib.sha256(
        json.dumps((expr, policy, compiler.generation), sort_keys=True).encode()
    ).hexdigest()[:20]
    proc = ProcessCandidate(
        process_id,
        expr,
        used,
        compiler.generation,
        tuple(used),
        dict(policy),
        tuple(sorted({trace.family for trace in list(rows) + list(transfer)})),
        len(json.dumps(expr, sort_keys=True)),
        {"origin": "discovery_only_baseline", "external_model": False, "ood_used_for_selection": False},
    )
    compiler.processes[proc.process_id] = proc
    if metrics_out is not None:
        metrics_out.update(counters)
        metrics_out.update({
            "qualified_candidate_count": len(candidates),
            "selected_process_id": proc.process_id,
            "selected_expression_sha256": hashlib.sha256(
                json.dumps(expr, sort_keys=True, separators=(",", ":")).encode()
            ).hexdigest(),
            "selected_policy": dict(policy),
            "selected_used_primitives": list(used),
            "selected_complexity": proc.complexity,
            "total_cached_values_across_rows": sum(len(cache) for cache in row_caches),
            "per_row_cache_sizes": [len(cache) for cache in row_caches],
        })
    return proc
