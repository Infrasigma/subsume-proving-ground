"""Representation search for ASI-CORE-002.

Representations are executable transformations, not metadata.  Candidates are built by
composing generic transformations, scored by downstream predictive utility, mutated,
combined, rejected, and retained.  The evaluator never tells the learner which
representation is semantically correct.
"""
from __future__ import annotations

from dataclasses import dataclass
from collections import Counter, deque
from functools import lru_cache
from typing import Any, Callable, Iterable, Sequence
import copy
import json
import math

Json = Any


def _leaf_values(x: Json) -> list[Any]:
    out: list[Any] = []
    if isinstance(x, dict):
        for k in sorted(x): out.extend(_leaf_values(x[k]))
    elif isinstance(x, list):
        for v in x: out.extend(_leaf_values(v))
    else:
        out.append(x)
    return out


def _numeric_leaves(x: Json) -> list[float]:
    return [float(v) for v in _leaf_values(x) if isinstance(v, (int, float)) and not isinstance(v, bool)]


def _stable_json(x: Json) -> str:
    return json.dumps(x, sort_keys=True, separators=(",", ":"), ensure_ascii=True)


def _canonicalize(x: Json) -> Json:
    if isinstance(x, dict):
        return {k: _canonicalize(x[k]) for k in sorted(x)}
    if isinstance(x, list):
        return [_canonicalize(v) for v in x]
    return x


def _flatten_numeric(x: Json) -> list[float]:
    return sorted(_numeric_leaves(x))


def _numeric_surface_order(x: Json) -> list[float]:
    return _numeric_leaves(x)


def _record_numeric_multiset(x: Json) -> list[float] | None:
    if not isinstance(x, list) or not all(isinstance(v, dict) for v in x):
        return None
    vals: list[float] = []
    for row in x:
        nums = _numeric_leaves(row)
        if len(nums) != 1:
            return None
        vals.append(nums[0])
    return sorted(vals)


def _sequence_tokens(x: Json) -> list[Any] | None:
    if isinstance(x, list) and all(not isinstance(v, (dict, list)) for v in x):
        return list(x)
    if isinstance(x, dict):
        for key in ("items", "tokens", "sequence", "values"):
            if isinstance(x.get(key), list): return list(x[key])
    return None


def _relational_role_values(x: Json) -> list[float] | None:
    """Canonicalize a graph around a designated seed using only structural relations."""
    if not isinstance(x, dict) or not isinstance(x.get("nodes"), list) or not isinstance(x.get("relations"), list):
        return None
    nodes = [n for n in x["nodes"] if isinstance(n, dict) and "id" in n]
    seed = x.get("seed")
    if seed is None or seed not in {n["id"] for n in nodes}:
        return None
    ids = [n["id"] for n in nodes]
    val = {n["id"]: _numeric_leaves(n.get("value", n.get("v"))) for n in nodes}
    value: dict[Any, float] = {}
    for k, vs in val.items():
        if len(vs) != 1: return None
        value[k] = vs[0]
    out_edges: dict[Any, list[Any]] = {i: [] for i in ids}
    in_edges: dict[Any, list[Any]] = {i: [] for i in ids}
    for e in x["relations"]:
        if isinstance(e, (list, tuple)) and len(e) == 2 and e[0] in out_edges and e[1] in out_edges:
            out_edges[e[0]].append(e[1]); in_edges[e[1]].append(e[0])
    dist = {seed: 0}; q = deque([seed])
    while q:
        u = q.popleft()
        for v in out_edges[u]:
            if v not in dist:
                dist[v] = dist[u] + 1; q.append(v)
    # Iterative structural refinement, invariant to IDs and edge serialization.
    colors = {i: (len(in_edges[i]), len(out_edges[i]), dist.get(i, 10**6)) for i in ids}
    for _ in range(3):
        nxt: dict[Any, tuple[Any, ...]] = {}
        for i in ids:
            nxt[i] = (
                colors[i],
                tuple(sorted(colors[j] for j in in_edges[i])),
                tuple(sorted(colors[j] for j in out_edges[i])),
            )
        ranks: dict[str, int] = {}
        ordered = sorted(((_stable_json(v), i) for i, v in nxt.items()))
        for rank, (sig, i) in enumerate(ordered): ranks[sig] = rank
        colors = {i: (ranks[_stable_json(nxt[i])],) for i in ids}
    # Distance from seed is primary so relational depth survives surface shuffles;
    # structural refinement resolves ties without using object identity.
    reachable_ids = [i for i in ids if i in dist]
    ordered_ids = sorted(reachable_ids, key=lambda i: (dist[i], colors[i]))
    return [value[i] for i in ordered_ids]


def _length_multiset(x: Json) -> list[int] | None:
    if not isinstance(x, list) or not all(isinstance(v, list) for v in x): return None
    return sorted(len(v) for v in x)


def _pair_normalize(x: Json) -> list[list[Any]] | None:
    if not isinstance(x, list): return None
    out=[]
    for v in x:
        if not isinstance(v, (list, tuple)) or len(v)!=2: return None
        out.append(sorted(v, key=lambda z: _stable_json(z)))
    out.sort(key=_stable_json)
    return out


def _compose_rep(base: Json, op: str) -> Json | None:
    if op == "identity": return copy.deepcopy(base)
    if op == "canonical": return _canonicalize(base)
    if op == "numeric_flat": return _flatten_numeric(base)
    if op == "numeric_surface_order": return _numeric_surface_order(base)
    if op == "record_numeric_multiset": return _record_numeric_multiset(base)
    if op == "sequence_tokens": return _sequence_tokens(base)
    if op == "relational_roles": return _relational_role_values(base)
    if op == "length_multiset": return _length_multiset(base)
    if op == "pair_normalize": return _pair_normalize(base)
    if op == "sort":
        try: return sorted(base)
        except Exception: return None
    return None


REP_OPS = (
    "identity",
    "canonical",
    "numeric_flat",
    "numeric_surface_order",
    "record_numeric_multiset",
    "sequence_tokens",
    "relational_roles",
    "length_multiset",
    "pair_normalize",
    "sort",
)


@dataclass(frozen=True)
class RepresentationCandidate:
    representation_id: str
    steps: tuple[str, ...]
    score: float
    transfer_score: float
    complexity: int
    accepted: bool = True

    @property
    def name(self) -> str:
        # Preserve the CS-001/001 compatibility surface for atomic candidate names.
        if len(self.steps) == 2 and self.steps[0] == "identity":
            return self.steps[1]
        if self.steps == ("identity",):
            return "identity"
        return self.representation_id

    def apply(self, x: Json) -> Json | None:
        out = copy.deepcopy(x)
        for step in self.steps:
            out = _compose_rep(out, step)
            if out is None: return None
        return out


def representation_search(
    examples: Sequence[tuple[Json, Json]],
    program_finder: Callable[[Sequence[tuple[Json, Json]]], tuple[tuple[str, ...] | None, int]],
    program_runner: Callable[[tuple[str, ...], Json], Json | None],
    *,
    max_depth: int = 2,
    beam: int = 16,
) -> tuple[RepresentationCandidate, ...]:
    if not examples:
        return (RepresentationCandidate("r:identity", ("identity",), 0.0, 0.0, 1),)

    def score_steps(steps: tuple[str, ...]) -> tuple[float, int]:
        transformed=[]
        for x,y in examples:
            tx=_compose_chain(x, steps)
            if tx is None: return -1.0, 0
            transformed.append((tx,y))
        cut=max(1,len(transformed)//2)
        fit,tested=program_finder(transformed[:cut])
        if fit is None:
            return 0.0, tested
        train_fit=sum(int(program_runner(fit,x)==y) for x,y in transformed[:cut])/max(1,len(transformed[:cut]))
        holdout=sum(int(program_runner(fit,x)==y) for x,y in transformed[cut:])/max(1,len(transformed[cut:]))
        return 0.65*holdout + 0.35*train_fit - 0.005*len(steps), tested

    frontier=[("identity",)]
    seen=set(frontier)
    scored: dict[tuple[str,...],tuple[float,int]]={}
    for depth in range(1,max_depth+1):
        scored_depth=[]
        for steps in frontier:
            sc=score_steps(steps); scored[steps]=sc; scored_depth.append((sc[0],steps))
        if depth==max_depth:
            break
        expanded=[]
        for _,steps in sorted(scored_depth,key=lambda z:(-z[0],z[1]))[:beam]:
            for op in REP_OPS:
                cand=steps+(op,)
                if cand not in seen:
                    seen.add(cand); expanded.append(cand)
            if len(steps)>1:
                for op in REP_OPS:
                    cand=steps[:-1]+(op,)
                    if cand not in seen:
                        seen.add(cand); expanded.append(cand)
        expanded.sort(key=lambda st:(-score_steps(st)[0],st))
        frontier=expanded[:beam]
        if not frontier: break

    ranked=sorted(scored.items(),key=lambda kv:(-kv[1][0],len(kv[0]),kv[0]))[:beam]
    out=[]
    for steps,(score,tested) in ranked:
        out.append(RepresentationCandidate("r:"+"+".join(steps),steps,score,score,len(steps),score>0.0))
    return tuple(out)


def _compose_chain(x: Json, steps: Sequence[str]) -> Json | None:
    out=copy.deepcopy(x)
    for op in steps:
        out=_compose_rep(out,op)
        if out is None: return None
    return out
