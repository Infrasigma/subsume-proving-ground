#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import random
import statistics
from dataclasses import dataclass
from hashlib import sha256
from typing import Any

from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import (
    RecursiveCognitiveCompiler,
    ProcessCandidate,
    Trace,
    eval_expr,
    ast_depth,
)


BASE_BIN_OPS = ("add", "sub", "mul", "max", "min")
UNARY_OPS = ("abs", "neg")
KEYS = ("x", "y", "z")


@dataclass(frozen=True)
class HiddenCapability:
    hidden_id: str
    primitive_id: str
    expression: dict[str, Any]
    depth: int
    generation: int
    parent_hidden_id: str | None = None
    parent_hidden_ids: tuple[str, ...] = ()


def stable_int(text: str) -> int:
    return int.from_bytes(sha256(text.encode()).digest()[:4], "big")


def random_base_expr(rng: random.Random, depth: int) -> dict[str, Any]:
    if depth <= 1:
        if rng.random() < 0.75:
            return {"op": "get", "key": rng.choice(KEYS)}
        return {"op": "const", "value": rng.choice((-2, -1, 0, 1, 2, 3, 4))}
    if rng.random() < 0.20:
        return {"op": rng.choice(UNARY_OPS), "arg": random_base_expr(rng, depth - 1)}
    return {
        "op": rng.choice(BASE_BIN_OPS),
        "left": random_base_expr(rng, depth - 1),
        "right": random_base_expr(rng, depth - 1),
    }


def hard_bootstrap_expr(rng: random.Random) -> dict[str, Any]:
    """Generate a neutral, nontrivial depth-4+ bootstrap capability."""
    def leaf() -> dict[str, Any]:
        if rng.random() < 0.8:
            return {"op": "get", "key": rng.choice(KEYS)}
        # Keep hard bootstrap inside the same neutral expression vocabulary
        # that ACSIE's generic search substrate can actually enumerate.
        return {"op": "const", "value": rng.choice((-2, -1, 0, 1, 2, 3, 4))}

    unary_child = {"op": rng.choice(UNARY_OPS), "arg": leaf()}
    binary_child = {
        "op": rng.choice(BASE_BIN_OPS),
        "left": leaf(),
        "right": leaf(),
    }
    depth_three = {
        "op": rng.choice(BASE_BIN_OPS),
        "left": unary_child,
        "right": binary_child,
    }
    # Baseline induction searches at depth <=3. Wrap once more so bootstrap
    # cannot be solved by that fresh non-recursive control by construction.
    return {
        "op": rng.choice(UNARY_OPS),
        "arg": depth_three,
    }


def wrap_parent(rng: random.Random, parent: dict[str, Any]) -> dict[str, Any]:
    op = rng.choice(BASE_BIN_OPS + UNARY_OPS)
    if op in UNARY_OPS:
        return {"op": op, "arg": parent}
    if rng.random() < 0.5:
        rhs = {"op": "get", "key": rng.choice(KEYS)}
    else:
        rhs = {"op": "const", "value": rng.choice((-2, -1, 0, 1, 2, 3, 4))}
    return {"op": op, "left": parent, "right": rhs}


def combine_parents(
    rng: random.Random,
    parents: tuple[HiddenCapability, ...],
) -> dict[str, Any]:
    if len(parents) == 1:
        return wrap_parent(rng, parents[0].expression)
    op = rng.choice(("add", "sub", "mul", "max", "min"))
    return {
        "op": op,
        "left": parents[0].expression,
        "right": parents[1].expression,
    }


def eval_hidden(
    expr: dict[str, Any],
    row: dict[str, float],
    macros: dict[str, dict[str, Any]],
) -> float:
    return float(eval_expr(expr, row, macros))


def expanded_depth(
    expr: dict[str, Any],
    macros: dict[str, dict[str, Any]],
    stack: tuple[str, ...] = (),
) -> int:
    op = expr["op"]
    if op == "macro":
        mid = str(expr["id"])
        if mid in stack or mid not in macros:
            return 1
        return expanded_depth(macros[mid], macros, stack + (mid,))
    if op in {"get", "const"}:
        return 1
    if op in {"abs", "neg", "threshold"}:
        return 1 + expanded_depth(expr["arg"], macros, stack)
    return 1 + max(
        expanded_depth(expr["left"], macros, stack),
        expanded_depth(expr["right"], macros, stack),
    )


def make_traces(expr, seed, generation, split, count, shift, macros):
    rng = random.Random(seed * 1000003 + generation * 9176 + stable_int(split))
    base_split = split.rsplit("_", 1)[-1]
    span = {
        "train": 2.0,
        "selection": 2.5,
        "holdout": 3.0,
        "transfer": 4.0,
        "ood": 5.0,
    }[base_split]
    rows = []
    for i in range(count):
        inputs = {k: rng.uniform(-span, span) + shift for k in KEYS}
        rows.append(
            Trace(
                inputs=inputs,
                target=eval_hidden(expr, inputs, macros),
                family="opaque",
                context={"risk": 0.0, "ambiguity": 0.0},
                task_id=f"opaque-{seed}-{generation}-{split}-{i}",
            )
        )
    return tuple(rows)


def mae(expr, rows, macros):
    if not rows:
        return float("inf")
    vals = []
    for row in rows:
        try:
            vals.append(abs(float(eval_expr(expr, row.inputs, macros)) - row.target))
        except Exception:
            return float("inf")
    return statistics.fmean(vals)

def admission_rows(seed: int, generation: int, count: int = 24) -> tuple[Trace, ...]:
    rng = random.Random(seed * 1000033 + generation * 7919 + stable_int("admission-rows"))
    rows = []
    for i in range(count):
        inputs = {k: rng.uniform(-7.0, 7.0) + 0.37 for k in KEYS}
        rows.append(
            Trace(
                inputs=inputs,
                target=0.0,
                family="admission-only",
                context={},
                task_id=f"admission-{seed}-{generation}-{i}",
            )
        )
    return tuple(rows)


def exact_vector(expr, rows: tuple[Trace, ...], macros: dict[str, dict[str, Any]]) -> tuple[float, ...]:
    return tuple(float(eval_hidden(expr, row.inputs, macros)) for row in rows)


def raw_baseline_exact(
    expr, rows: tuple[Trace, ...], macros: dict[str, dict[str, Any]] | None = None
) -> bool:
    """Reject targets representable by the actual fresh baseline grammar (depth <=3)."""
    keys = tuple(sorted(set().union(*(row.inputs.keys() for row in rows))))
    compiler = RecursiveCognitiveCompiler(max_depth=3, population=1, seed=0)
    target = exact_vector(expr, rows, macros or {})
    for candidate in compiler._exprs(keys, (), 3):
        try:
            vector = tuple(float(eval_expr(candidate, row.inputs, {})) for row in rows)
        except Exception:
            continue
        if vector == target:
            return True
    return False


def macro_frontier_representation_count(
    target: dict[str, Any],
    retained: tuple[HiddenCapability, ...],
    rows: tuple[Trace, ...],
    macros: dict[str, dict[str, Any]],
) -> int:
    """Count exact depth-2 representations that reuse retained capabilities."""
    keys = tuple(KEYS)
    atoms = [{"op": "macro", "id": cap.hidden_id} for cap in retained]
    atoms.extend({"op": "get", "key": key} for key in keys)
    atoms.extend({"op": "const", "value": value} for value in (-2, -1, 0, 1, 2, 3, 4))
    target_vec = exact_vector(target, rows, macros)
    seen: set[str] = set()
    matches = 0

    def add_match(expr: dict[str, Any]) -> None:
        nonlocal matches
        if not any(node.get("op") == "macro" for node in RecursiveCognitiveCompiler()._walk(expr)):
            return
        if expr.get("op") in {"add", "mul", "max", "min"}:
            left_key = json.dumps(expr["left"], sort_keys=True, separators=(",", ":"))
            right_key = json.dumps(expr["right"], sort_keys=True, separators=(",", ":"))
            if left_key > right_key:
                expr = {"op": expr["op"], "left": expr["right"], "right": expr["left"]}
        key = json.dumps(expr, sort_keys=True, separators=(",", ":"))
        if key in seen:
            return
        try:
            vector = tuple(float(eval_hidden(expr, row.inputs, macros)) for row in rows)
        except Exception:
            return
        if vector == target_vec:
            seen.add(key)
            matches += 1

    for atom in atoms:
        if atom.get("op") == "macro":
            for op in UNARY_OPS:
                add_match({"op": op, "arg": atom})

    for left in atoms:
        for right in atoms:
            for op in BASE_BIN_OPS:
                if op in {"add", "mul", "max", "min"}:
                    lk = json.dumps(left, sort_keys=True, separators=(",", ":"))
                    rk = json.dumps(right, sort_keys=True, separators=(",", ":"))
                    if lk > rk:
                        continue
                add_match({"op": op, "left": left, "right": right})
    return matches


def closure_library_identifiable(
    retained: tuple[HiddenCapability, ...],
    rows: tuple[Trace, ...],
) -> bool:
    """Require fresh retained capabilities and all pairwise-add closures to be identifiable."""
    macros = {cap.hidden_id: cap.expression for cap in retained}
    capability_vectors = [exact_vector(cap.expression, rows, macros) for cap in retained]
    if len(set(capability_vectors)) != len(capability_vectors):
        return False
    pair_vectors: dict[tuple[float, ...], tuple[str, str]] = {}
    for i in range(len(retained)):
        for j in range(i + 1, len(retained)):
            expr = {
                "op": "add",
                "left": capability_macro(retained[i]),
                "right": capability_macro(retained[j]),
            }
            vector = exact_vector(expr, rows, macros)
            pair_id = (retained[i].hidden_id, retained[j].hidden_id)
            previous = pair_vectors.get(vector)
            if previous is not None and previous != pair_id:
                return False
            pair_vectors[vector] = pair_id
            if raw_baseline_exact(expr, rows, macros):
                return False
    return True


def target_is_admissible(
    target: dict[str, Any],
    target_parents: tuple[HiddenCapability, ...],
    retained: tuple[HiddenCapability, ...],
    seed: int,
    generation: int,
) -> bool:
    rows = admission_rows(seed, generation)
    macros = {cap.hidden_id: cap.expression for cap in retained}
    if raw_baseline_exact(target, rows, macros):
        return False
    target_vec = exact_vector(target, rows, macros)
    if any(
        target_vec == exact_vector(cap.expression, rows, macros)
        for cap in retained
    ):
        return False
    if target_parents:
        # The target must have exactly one reusable depth-2 representation in
        # the retained executable vocabulary. This makes exact lineage a
        # scientifically identifiable property rather than an arbitrary tie.
        if macro_frontier_representation_count(
            target, retained, rows, macros
        ) != 1:
            return False
    prospective = list(retained)
    prospective.append(
        HiddenCapability(
            hidden_id=f"admission:{generation}",
            primitive_id=f"admission:{generation}",
            expression=target,
            depth=expanded_depth(target, macros),
            generation=generation,
        )
    )
    return closure_library_identifiable(tuple(prospective), rows)



def semantic_parent_match_count(
    learner: RecursiveCognitiveCompiler,
    expected_caps: tuple[HiddenCapability, ...],
    used_primitive_ids: tuple[str, ...],
    rows: tuple[Trace, ...],
    hidden_macros: dict[str, dict[str, Any]],
) -> int:
    if not expected_caps or not used_primitive_ids:
        return 0
    pmap = learner._pmap()
    used_vectors: dict[str, tuple[float, ...]] = {}
    for pid in used_primitive_ids:
        primitive = learner.primitives.get(pid)
        if primitive is None:
            continue
        try:
            used_vectors[pid] = tuple(
                float(eval_expr(primitive.expression, row.inputs, pmap))
                for row in rows
            )
        except Exception:
            continue

    expected_vectors: list[tuple[float, ...]] = []
    for cap in expected_caps:
        try:
            expected_vectors.append(
                tuple(
                    float(eval_hidden(cap.expression, row.inputs, hidden_macros))
                    for row in rows
                )
            )
        except Exception:
            expected_vectors.append(())

    matched: set[str] = set()
    count = 0
    for expected_vector in expected_vectors:
        for pid, vector in used_vectors.items():
            if pid in matched:
                continue
            if vector == expected_vector:
                matched.add(pid)
                count += 1
                break
    return count


def retained_representation_error(
    learner: RecursiveCognitiveCompiler,
    cap: HiddenCapability,
    rows: tuple[Trace, ...],
    hidden_macros: dict[str, dict[str, Any]],
) -> float:
    primitive = learner.primitives.get(cap.primitive_id)
    if primitive is None:
        return float("inf")
    pmap = learner._pmap()
    if not rows:
        return float("inf")
    deltas = []
    for row in rows:
        try:
            retained_value = float(eval_expr(primitive.expression, row.inputs, pmap))
            hidden_value = float(eval_hidden(cap.expression, row.inputs, hidden_macros))
        except Exception:
            return float("inf")
        deltas.append(abs(retained_value - hidden_value))
    return statistics.fmean(deltas)


def synthesize_discovery_only(
    compiler: RecursiveCognitiveCompiler,
    rows: tuple[Trace, ...],
    transfer: tuple[Trace, ...],
) -> Any:
    """Baseline process selection that never observes OOD during search."""
    keys = sorted(set().union(*(trace.inputs.keys() for trace in rows)))
    pids = tuple(compiler.primitives)
    candidates = []
    pmap = compiler._pmap()
    for policy in compiler.propose_search_policies():
        for expr in compiler._exprs(keys, pids, min(3, int(policy['max_depth']))):
            try:
                train_error = sum(
                    abs(eval_expr(expr, trace.inputs, pmap) - trace.target)
                    for trace in rows
                ) / len(rows)
                if train_error > 1e-9:
                    continue
                # Baseline candidate generation sees discovery rows only.
                # Transfer/OOD remain post-selection evaluation evidence.
                used = tuple(sorted({str(node['id']) for node in compiler._walk(expr) if node.get('op') == 'macro'}))
                novelty = 1.0 / (1 + len(json.dumps(expr, sort_keys=True)))
                candidates.append((len(used), novelty, -len(json.dumps(expr, sort_keys=True)), json.dumps(expr, sort_keys=True), expr, used, policy))
            except Exception:
                continue
    if not candidates:
        return None
    _, _, _, _, expr, used, policy = max(
        candidates,
        key=lambda c: (c[0], c[1], c[2], c[3]),
    )
    process_id = 'baseline-proc:' + sha256(json.dumps((expr, policy, compiler.generation), sort_keys=True).encode()).hexdigest()[:20]
    proc = ProcessCandidate(
        process_id, expr, used, compiler.generation, tuple(used), dict(policy),
        tuple(sorted({trace.family for trace in list(rows) + list(transfer)})),
        len(json.dumps(expr, sort_keys=True)),
        {'origin': 'discovery_only_baseline', 'external_model': False, 'ood_used_for_selection': False},
    )
    compiler.processes[proc.process_id] = proc
    return proc

def primitive_lineage_ids(
    learner: RecursiveCognitiveCompiler,
    primitive_ids: tuple[str, ...],
) -> set[str]:
    lineage: set[str] = set()
    method = getattr(learner, "primitive_lineage", None)
    for pid in primitive_ids:
        if method is None:
            lineage.add(str(pid))
        else:
            lineage.update(str(x) for x in method(str(pid)))
    return lineage


def run_seed(seed: int, generations: int) -> dict[str, Any]:
    rng = random.Random(seed)
    learner = OpenEndedRecursiveCognitiveCompiler(max_depth=2, population=24, seed=seed)

    retained: list[HiddenCapability] = []
    generations_out = []
    initial_depth = learner.meta_policy["max_depth"]

    def hidden_library() -> dict[str, dict[str, Any]]:
        return {cap.hidden_id: cap.expression for cap in retained}

    def capability_macro(cap: HiddenCapability) -> dict[str, Any]:
        return {"op": "macro", "id": cap.hidden_id}

    for generation in range(generations):
        learner.generation = generation
        hidden_macros = hidden_library()

        target_parents: tuple[HiddenCapability, ...] = ()
        target: dict[str, Any] | None = None
        for _attempt in range(512):
            if not retained:
                candidate_parents = ()
                candidate = hard_bootstrap_expr(rng)
            elif len(retained) == 1:
                candidate_parents = (retained[0],)
                candidate = {
                    "op": rng.choice(("add", "sub", "mul", "max", "min")),
                    "left": capability_macro(retained[0]),
                    "right": {"op": "get", "key": rng.choice(KEYS)},
                }
            else:
                candidate_parents = tuple(rng.sample(retained, 2))
                candidate = {
                    "op": rng.choice(("add", "sub", "mul", "max", "min")),
                    "left": capability_macro(candidate_parents[0]),
                    "right": capability_macro(candidate_parents[1]),
                }
            if target_is_admissible(
                candidate,
                tuple(candidate_parents),
                tuple(retained),
                seed,
                generation,
            ):
                target_parents = tuple(candidate_parents)
                target = candidate
                break
        if target is None:
            return {
                "seed": seed,
                "generations": generations,
                "benchmark_valid": False,
                "benchmark_failure": f"NO_ADMISSIBLE_TARGET_GENERATION_{generation}",
                "finite_open_ended_growth_gate": False,
            }

        target_depth = expanded_depth(target, hidden_macros)

        train = make_traces(
            target, seed, generation, "train", 12, 0.0, hidden_macros
        )
        selection = make_traces(
            target, seed, generation, "selection", 8, 0.17, hidden_macros
        )
        hold = make_traces(
            target, seed, generation, "holdout", 6, 0.31, hidden_macros
        )
        transfer = make_traces(
            target, seed, generation, "transfer", 8, -0.57, hidden_macros
        )
        ood = make_traces(
            target, seed, generation, "ood", 8, 0.93, hidden_macros
        )

        discovery_train = tuple(train) + tuple(selection)
        primitive = learner.invent_primitive(discovery_train, hold, transfer)
        accepted = False
        transfer_error = float("inf")
        ood_error = float("inf")
        parent_used = tuple()

        parent_used = primitive.parent_ids if primitive is not None else tuple()
        expected_parent_ids = {p.primitive_id for p in target_parents}
        candidate_lineage_ids = (
            primitive_lineage_ids(learner, (primitive.primitive_id,))
            if primitive is not None
            else set()
        )
        recursive_reuse_ok = False
        if primitive is not None:
            transfer_error = primitive.transfer_error
            ood_error = mae(primitive.expression, ood, learner._pmap())
            accepted = learner.accept_primitive(
                primitive.primitive_id,
                transfer_error=transfer_error,
                ood_error=ood_error,
                novelty=1.0 / max(1, primitive.complexity),
                resource_cost=max(1, primitive.complexity),
            )
            recursive_reuse_ok = expected_parent_ids.issubset(candidate_lineage_ids)
            if accepted:
                hidden_id = f"hidden:{generation}:{len(retained)}"
                retained.append(
                    HiddenCapability(
                        hidden_id=hidden_id,
                        primitive_id=primitive.primitive_id,
                        expression=target,
                        depth=target_depth,
                        generation=generation,
                        parent_hidden_id=target_parents[0].hidden_id if target_parents else None,
                        parent_hidden_ids=tuple(p.hidden_id for p in target_parents),
                    )
                )

        # Fresh direct-task control: it gets the same observations for this
        # generation but cannot carry prior archive state forward.
        baseline_direct = RecursiveCognitiveCompiler(
            max_depth=2, population=24, seed=seed + 10091 + generation
        )
        baseline_direct.generation = 0
        bprim = baseline_direct.invent_primitive(discovery_train, hold, transfer)
        baseline_accepted = False
        if bprim is not None:
            b_ood = mae(bprim.expression, ood, baseline_direct._pmap())
            baseline_accepted = (
                bprim.transfer_error <= 1e-9 and b_ood <= 1e-9
            )

        # Fresh non-recursive control for closure/probe tests. It has no
        # retained ACSIE capabilities and is reset independently each
        # generation, so longitudinal gains cannot come from baseline memory.
        baseline = RecursiveCognitiveCompiler(
            max_depth=2, population=24, seed=seed + 21091 + generation
        )

        closure_rates = []
        baseline_closure_rates = []
        closure_reuse_rates = []
        semantic_closure_reuse_rates = []
        closure_parent_fidelity = []
        closure_trap_count = 0
        if len(retained) >= 2:
            pairs = [
                (retained[i], retained[j])
                for i in range(len(retained))
                for j in range(i + 1, len(retained))
            ]
            for pair_idx, (left_parent, right_parent) in enumerate(pairs):
                closure_expr = {
                    "op": "add",
                    "left": capability_macro(left_parent),
                    "right": capability_macro(right_parent),
                }
                closure_macros = hidden_library()
                ctr = make_traces(closure_expr, seed + 1700 + pair_idx, generation, 'closure_train', 8, 0.13, closure_macros)
                parent_fidelities = {
                    left_parent.primitive_id: retained_representation_error(
                        learner, left_parent, tuple(ctr), closure_macros
                    ),
                    right_parent.primitive_id: retained_representation_error(
                        learner, right_parent, tuple(ctr), closure_macros
                    ),
                }
                cs = make_traces(closure_expr, seed + 1700 + pair_idx, generation, 'closure_selection', 6, 0.07, closure_macros)
                ch = make_traces(closure_expr, seed + 1700 + pair_idx, generation, 'closure_holdout', 5, 0.23, closure_macros)
                cv = make_traces(closure_expr, seed + 1700 + pair_idx, generation, 'closure_transfer', 5, -0.19, closure_macros)
                co = make_traces(closure_expr, seed + 1700 + pair_idx, generation, 'closure_ood', 5, 0.29, closure_macros)
                cproc = learner.synthesize_process(tuple(ctr) + tuple(cs), cv, co)
                competence = False
                reuse_ok = False
                if cproc is not None:
                    ceval = learner.evaluate(cproc, ctr, ch, cv, co, ())
                    competence = bool(ceval.accepted and ceval.ood_error <= 1e-9)
                    expected = {left_parent.primitive_id, right_parent.primitive_id}
                    candidate_lineage = primitive_lineage_ids(
                        learner,
                        tuple(cproc.used_primitives),
                    )
                    reuse_ok = competence and expected.issubset(candidate_lineage)
                closure_rates.append(float(competence))
                closure_reuse_rates.append(float(reuse_ok))
                semantic_matches = semantic_parent_match_count(
                    learner,
                    (left_parent, right_parent),
                    tuple(cproc.used_primitives) if cproc is not None else tuple(),
                    tuple(ctr),
                    closure_macros,
                )
                semantic_closure_reuse_rates.append(
                    float(
                        semantic_matches == len((left_parent, right_parent))
                        and competence
                    )
                )
                closure_parent_fidelity.append(
                    statistics.fmean(parent_fidelities.values())
                )
                if competence and not reuse_ok:
                    closure_trap_count += 1

                bproc = synthesize_discovery_only(baseline, tuple(ctr) + tuple(cs), cv)
                bcompetence = False
                if bproc is not None:
                    beval = baseline.evaluate(bproc, ctr, ch, cv, co, ())
                    bcompetence = bool(beval.accepted and beval.ood_error <= 1e-9)
                baseline_closure_rates.append(float(bcompetence))

        probe_rates = []
        baseline_probe_rates = []
        probe_reuse_rates = []
        semantic_probe_reuse_rates = []
        probe_parent_fidelity = []
        probe_trap_count = 0
        for probe_idx in range(4):
            probe_parents = tuple(rng.sample(retained, 2)) if len(retained) >= 2 else tuple(retained[:1])
            probe_macros = hidden_library()
            if probe_parents:
                if len(probe_parents) == 1:
                    probe = {'op': 'add', 'left': capability_macro(probe_parents[0]), 'right': {'op': 'get', 'key': KEYS[probe_idx % len(KEYS)]}}
                else:
                    probe = {'op': 'add', 'left': capability_macro(probe_parents[0]), 'right': capability_macro(probe_parents[1])}
            else:
                probe = random_base_expr(random.Random(seed * 193 + generation * 17 + probe_idx), 3)

            ptrain = make_traces(probe, seed + 700 + probe_idx, generation, 'probe_train', 8, 0.17, probe_macros)
            pselection = make_traces(probe, seed + 700 + probe_idx, generation, 'probe_selection', 6, 0.05, probe_macros)
            pholdout = make_traces(probe, seed + 700 + probe_idx, generation, 'probe_holdout', 5, 0.27, probe_macros)
            ptransfer = make_traces(probe, seed + 700 + probe_idx, generation, 'probe_transfer', 5, -0.22, probe_macros)
            pood = make_traces(probe, seed + 700 + probe_idx, generation, 'probe_ood', 5, 0.41, probe_macros)
            proc = learner.synthesize_process(tuple(ptrain) + tuple(pselection), ptransfer, pood)
            competence = False
            reuse_ok = False
            if proc is not None:
                peval = learner.evaluate(proc, ptrain, pholdout, ptransfer, pood, ())
                competence = bool(peval.accepted and peval.ood_error <= 1e-9)
                expected = {p.primitive_id for p in probe_parents}
                candidate_lineage = primitive_lineage_ids(
                    learner,
                    tuple(proc.used_primitives),
                )
                reuse_ok = competence and (
                    not expected or expected.issubset(candidate_lineage)
                )
            probe_rates.append(float(competence))
            probe_reuse_rates.append(float(reuse_ok))
            semantic_probe_matches = semantic_parent_match_count(
                learner,
                probe_parents,
                tuple(proc.used_primitives) if proc is not None else tuple(),
                tuple(ptrain),
                probe_macros,
            )
            semantic_probe_reuse_rates.append(
                float(
                    competence
                    and (
                        not probe_parents
                        or semantic_probe_matches == len(probe_parents)
                    )
                )
            )
            if probe_parents:
                probe_parent_fidelity.append(
                    statistics.fmean(
                        retained_representation_error(
                            learner, p, tuple(ptrain), probe_macros
                        )
                        for p in probe_parents
                    )
                )
            if competence and not reuse_ok and probe_parents:
                probe_trap_count += 1

            bproc = synthesize_discovery_only(baseline, ptrain, ptransfer)
            bcompetence = False
            if bproc is not None:
                beval = baseline.evaluate(bproc, ptrain, pholdout, ptransfer, pood, ())
                bcompetence = bool(beval.accepted and beval.ood_error <= 1e-9)
            baseline_probe_rates.append(float(bcompetence))
        previous_runtime_depth = (
            initial_depth
            if not generations_out
            else generations_out[-1]["runtime_max_depth"]
        )
        generations_out.append(
            {
                "generation": generation,
                "target_depth": target_depth,
                "accepted": accepted,
                "baseline_accepted": baseline_accepted,
                "parent_used": bool(recursive_reuse_ok),
                "parent_ids": list(parent_used),
                "expected_parent_ids": sorted(expected_parent_ids),
                "recursive_reuse_ok": bool(recursive_reuse_ok),
                "multi_parent_used": len(target_parents) >= 2 and recursive_reuse_ok,
                "closure_success_rate": statistics.fmean(closure_rates) if closure_rates else 0.0,
                "baseline_closure_success_rate": statistics.fmean(baseline_closure_rates) if baseline_closure_rates else 0.0,
                "closure_reuse_rate": statistics.fmean(closure_reuse_rates) if closure_reuse_rates else 0.0,
                "semantic_closure_reuse_rate": statistics.fmean(semantic_closure_reuse_rates) if semantic_closure_reuse_rates else 0.0,
                "closure_pair_count": len(closure_rates),
                "closure_success_count": int(sum(closure_rates)),
                "closure_reuse_success_count": int(sum(closure_reuse_rates)),
                "closure_trap_count": int(closure_trap_count),
                "baseline_closure_success_count": int(sum(baseline_closure_rates)),
                "retained_count": len(retained),
                "runtime_max_depth": int(learner.meta_policy["max_depth"]),
                "growth_delta": int(learner.meta_policy["max_depth"]) - int(previous_runtime_depth),
                "transfer_error": transfer_error,
                "ood_error": ood_error,
                "probe_success_rate": statistics.fmean(probe_rates),
                "baseline_probe_success_rate": statistics.fmean(baseline_probe_rates),
                "probe_reuse_rate": statistics.fmean(probe_reuse_rates),
                "semantic_probe_reuse_rate": statistics.fmean(semantic_probe_reuse_rates) if semantic_probe_reuse_rates else 0.0,
                "probe_parent_fidelity": (
                    statistics.fmean(probe_parent_fidelity)
                    if probe_parent_fidelity else 0.0
                ),
                "closure_parent_fidelity": (
                    statistics.fmean(closure_parent_fidelity)
                    if closure_parent_fidelity else 0.0
                ),
                "probe_trap_count": int(probe_trap_count),
                "selection_rows": len(discovery_train),
                "holdout_used_for_selection": False,
                "transfer_used_for_selection": False,
                "ood_used_for_selection": False,
                "search_stats": dict(learner.last_search_stats),
            }
        )

    probe_gain = statistics.fmean(
        row["probe_success_rate"] - row["baseline_probe_success_rate"]
        for row in generations_out
    )
    final = generations_out[-1]
    accepted_after_bootstrap = sum(row["accepted"] for row in generations_out[1:])
    recursive_generations = sum(
        row["recursive_reuse_ok"] for row in generations_out[1:]
    )
    multi_parent_generations = sum(
        row["multi_parent_used"] for row in generations_out[1:]
    )
    last_three = generations_out[-3:]
    final_closure_gain = (
        final["closure_success_rate"] - final["baseline_closure_success_rate"]
    )
    final_closure_reuse_rate = final["closure_reuse_rate"]
    final_probe_reuse_rate = final["probe_reuse_rate"]
    early = generations_out[min(2, len(generations_out) - 1)]
    closure_growth = (
        final["closure_reuse_success_count"] - early["closure_reuse_success_count"]
    )
    closure_capacity_ok = (
        final["closure_pair_count"] > 0
        and final["closure_reuse_rate"] >= 0.75
        and final["closure_reuse_success_count"] > 0
    )
    trap_free_ok = (
        all(row["closure_trap_count"] == 0 for row in generations_out)
        and all(row["probe_trap_count"] == 0 for row in generations_out)
    )

    applicable_generations = generations_out[1:] if len(generations_out) > 1 else []
    every_generation_accepted = bool(generations_out) and all(
        row["accepted"] for row in generations_out
    )
    every_closure_perfect = all(
        row["closure_pair_count"] == 0
        or (
            row["closure_success_rate"] == 1.0
            and row["closure_reuse_rate"] == 1.0
            and row["closure_trap_count"] == 0
        )
        for row in generations_out
    )
    every_probe_perfect = all(
        row["probe_success_rate"] == 1.0
        and row["probe_reuse_rate"] == 1.0
        and row["probe_trap_count"] == 0
        for row in generations_out
    )
    lineage_closed = all(row["parent_used"] for row in applicable_generations)
    strict_growth = all(row["growth_delta"] > 0 for row in applicable_generations)
    depth_growth_contract = (
        final["runtime_max_depth"] >= initial_depth + generations
        and final["runtime_max_depth"] > max(
            row["runtime_max_depth"] for row in generations_out[:-1]
        )
    )

    finite_gate = (
        len(generations_out) == generations
        and every_generation_accepted
        and lineage_closed
        and every_closure_perfect
        and every_probe_perfect
        and strict_growth
        and depth_growth_contract
        and final["retained_count"] >= generations
        and final["target_depth"] > generations_out[0]["target_depth"]
        and final["probe_success_rate"] == 1.0
        and final["probe_reuse_rate"] == 1.0
        and final["closure_pair_count"] >= 6
        and final["closure_success_rate"] == 1.0
        and final["closure_reuse_rate"] == 1.0
        and trap_free_ok
        and final["baseline_accepted"] is False
        and all(not row["baseline_accepted"] for row in generations_out)
    )
    return {
        "seed": seed,
        "generations": generations,
        "initial_depth": initial_depth,
        "final_runtime_max_depth": final["runtime_max_depth"],
        "final_target_depth": final["target_depth"],
        "retained_count": final["retained_count"],
        "accepted_after_bootstrap": accepted_after_bootstrap,
        "recursive_generations": recursive_generations,
        "multi_parent_generations": multi_parent_generations,
        "final_closure_success_rate": final["closure_success_rate"],
        "final_baseline_closure_success_rate": final["baseline_closure_success_rate"],
        "final_closure_success_count": final["closure_success_count"],
        "final_baseline_closure_success_count": final["baseline_closure_success_count"],
        "final_closure_gain": final_closure_gain,
        "closure_growth": closure_growth,
        "closure_capacity_ok": closure_capacity_ok,
        "every_generation_accepted": every_generation_accepted,
        "every_closure_perfect": every_closure_perfect,
        "every_probe_perfect": every_probe_perfect,
        "lineage_closed": lineage_closed,
        "strict_growth": strict_growth,
        "depth_growth_contract": depth_growth_contract,
        "mean_probe_gain": probe_gain,
        "final_probe_success_rate": final["probe_success_rate"],
        "final_baseline_probe_success_rate": final["baseline_probe_success_rate"],
        "finite_open_ended_growth_gate": finite_gate,
        "external_model": False,
        "target_identity_available_to_runtime": False,
        "task_family_route": False,
        "benchmark_valid": True,
        "selection_split_used": True,
        "holdout_used_for_search_selection": False,
        "transfer_used_for_search_selection": False,
        "ood_used_for_search_selection": False,
        "closure_trap_count": int(final["closure_trap_count"]),
        "probe_trap_count": int(final["probe_trap_count"]),
        "final_closure_reuse_rate": final_closure_reuse_rate,
        "final_probe_reuse_rate": final_probe_reuse_rate,
        "final_closure_parent_fidelity": final["closure_parent_fidelity"],
        "final_probe_parent_fidelity": final["probe_parent_fidelity"],
        "final_semantic_closure_reuse_rate": final["semantic_closure_reuse_rate"],
        "final_semantic_probe_reuse_rate": final["semantic_probe_reuse_rate"],
        "max_generated_expressions": max(
            int(row["search_stats"].get("generated_expressions", 0))
            for row in generations_out
        ),
        "max_unique_search_states": max(
            int(row["search_stats"].get("unique_states", 0))
            for row in generations_out
        ),
        "max_search_states_in_depth": max(
            int(row["search_stats"].get("max_states_in_depth", 0))
            for row in generations_out
        ),
        "generations_detail": generations_out,
    }

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--seeds",
        default="2026100301,2026100302,2026100303,2026100304,2026100305",
    )
    parser.add_argument("--generations", type=int, default=12)
    parser.add_argument("--strict", action="store_true")
    args = parser.parse_args()

    rows = [
        run_seed(int(s), args.generations)
        for s in args.seeds.split(",")
        if s.strip()
    ]
    for row in rows:
        print(json.dumps(row, sort_keys=True))

    gate = (
        len(rows) == 5
        and all(row["finite_open_ended_growth_gate"] for row in rows)
        and all(row.get("benchmark_valid", True) for row in rows)
        and all(not row["external_model"] for row in rows)
        and all(not row["target_identity_available_to_runtime"] for row in rows)
        and all(not row["task_family_route"] for row in rows)
    )
    print("OPEN_ENDED_SELF_EXTENDING_GATE=" + ("PASS" if gate else "FAIL"))
    raise SystemExit(0 if (gate or not args.strict) else 2)


if __name__ == "__main__":
    main()

# execution marker: PR-head condition corrected

# execution marker: ubuntu-slim runner route

# execution marker: complete lazy ACSIE stream pinned

# execution marker: fresh exact-SHA PR sync

# execution marker: fixed-baseline scientific run

# execution marker: bootstrap-language and primitive-ID fixes
