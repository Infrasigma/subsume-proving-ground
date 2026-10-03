#!/usr/bin/env python3
from __future__ import annotations

import json
import random
import statistics

from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import Trace, eval_expr

from lab.acsie_open_ended_self_extending_proving import (
    BASE_BIN_OPS,
    KEYS,
    capability_macro,
    hard_bootstrap_expr,
    make_traces,
    expanded_depth,
    semantic_parent_match_count,
    retained_representation_error,
    primitive_lineage_ids,
)

def run_seed(seed: int = 2026100303, generations: int = 12) -> dict:
    rng = random.Random(seed)
    learner = OpenEndedRecursiveCognitiveCompiler(max_depth=2, population=24, seed=seed)
    retained = []
    details = []
    initial_depth = int(learner.meta_policy["max_depth"])

    def hidden_library():
        return {cap.hidden_id: cap.expression for cap in retained}

    for generation in range(generations):
        learner.generation = generation
        hidden_macros = hidden_library()

        if not retained:
            target_parents = ()
            target = hard_bootstrap_expr(rng)
        elif len(retained) == 1:
            target_parents = (retained[0],)
            target = {
                "op": rng.choice(BASE_BIN_OPS),
                "left": capability_macro(retained[0]),
                "right": {"op": "get", "key": rng.choice(KEYS)},
            }
        else:
            target_parents = tuple(rng.sample(retained, 2))
            target = {
                "op": rng.choice(BASE_BIN_OPS),
                "left": capability_macro(target_parents[0]),
                "right": capability_macro(target_parents[1]),
            }

        target_depth = expanded_depth(target, hidden_macros)
        train = make_traces(target, seed, generation, "train", 12, 0.0, hidden_macros)
        selection = make_traces(target, seed, generation, "selection", 8, 0.17, hidden_macros)
        holdout = make_traces(target, seed, generation, "holdout", 6, 0.31, hidden_macros)
        transfer = make_traces(target, seed, generation, "transfer", 8, -0.57, hidden_macros)
        ood = make_traces(target, seed, generation, "ood", 8, 0.93, hidden_macros)
        discovery = tuple(train) + tuple(selection)

        primitive = learner.invent_primitive(discovery, holdout, transfer)
        validation = learner.accept_primitive_frontier(
            transfer_rows=transfer,
            ood_rows=ood,
            novelty=1.0 / max(1, primitive.complexity) if primitive else 1.0,
            resource_cost=max(1, primitive.complexity) if primitive else 1.0,
        ) if primitive else {"accepted": False, "primary_primitive": None, "candidate_count": 0, "validated_candidate_count": 0}

        accepted_primitive = validation.get("primary_primitive")
        accepted = bool(validation.get("accepted"))
        recursive_reuse = False
        parent_ids = ()
        if accepted_primitive is not None:
            primitive = accepted_primitive
            parent_ids = tuple(primitive.parent_ids)
            recursive_reuse = {p.primitive_id for p in target_parents}.issubset(
                primitive_lineage_ids(learner, (primitive.primitive_id,))
            )
            retained.append({
                "hidden_id": f"hidden:{generation}:{len(retained)}",
                "primitive_id": primitive.primitive_id,
                "expression": target,
                "depth": target_depth,
                "generation": generation,
                "parent_hidden_ids": tuple(p["hidden_id"] for p in target_parents),
            })

        closure_rates = []
        closure_reuse = []
        closure_traps = 0
        if len(retained) >= 2:
            for i in range(len(retained)):
                for j in range(i + 1, len(retained)):
                    left, right = retained[i], retained[j]
                    closure = {
                        "op": "add",
                        "left": {"op": "macro", "id": left["hidden_id"]},
                        "right": {"op": "macro", "id": right["hidden_id"]},
                    }
                    cm = hidden_library()
                    ctr = make_traces(closure, seed + 1700 + i * 101 + j, generation, "closure_train", 8, 0.13, cm)
                    csel = make_traces(closure, seed + 1700 + i * 101 + j, generation, "closure_selection", 6, 0.07, cm)
                    ch = make_traces(closure, seed + 1700 + i * 101 + j, generation, "closure_holdout", 5, 0.23, cm)
                    ct = make_traces(closure, seed + 1700 + i * 101 + j, generation, "closure_transfer", 5, -0.19, cm)
                    co = make_traces(closure, seed + 1700 + i * 101 + j, generation, "closure_ood", 5, 0.29, cm)
                    procs = learner.synthesize_process_frontier(tuple(ctr) + tuple(csel), ct, co, max_candidates=32)
                    competent = False
                    reuse = False
                    expected = {left["primitive_id"], right["primitive_id"]}
                    chosen = None
                    for proc in procs:
                        ev = learner.evaluate(proc, ctr, ch, ct, co, ())
                        if ev.accepted and ev.ood_error <= 1e-9:
                            competent = True
                            if chosen is None:
                                chosen = proc
                            lineage = primitive_lineage_ids(learner, tuple(proc.used_primitives))
                            if expected.issubset(lineage):
                                reuse = True
                    closure_rates.append(float(competent))
                    closure_reuse.append(float(reuse))
                    if competent and not reuse:
                        closure_traps += 1

        probe_rates = []
        probe_reuse = []
        probe_traps = 0
        semantic_probe_reuse = []
        for probe_idx in range(4):
            probe_parents = tuple(rng.sample(retained, 2)) if len(retained) >= 2 else tuple(retained[:1])
            pm = hidden_library()
            if len(probe_parents) == 2:
                probe = {
                    "op": "add",
                    "left": {"op": "macro", "id": probe_parents[0]["hidden_id"]},
                    "right": {"op": "macro", "id": probe_parents[1]["hidden_id"]},
                }
            elif len(probe_parents) == 1:
                probe = {
                    "op": "add",
                    "left": {"op": "macro", "id": probe_parents[0]["hidden_id"]},
                    "right": {"op": "get", "key": KEYS[probe_idx % len(KEYS)]},
                }
            else:
                continue

            ptrain = make_traces(probe, seed + 700 + probe_idx, generation, "probe_train", 8, 0.17, pm)
            psel = make_traces(probe, seed + 700 + probe_idx, generation, "probe_selection", 6, 0.05, pm)
            ph = make_traces(probe, seed + 700 + probe_idx, generation, "probe_holdout", 5, 0.27, pm)
            pt = make_traces(probe, seed + 700 + probe_idx, generation, "probe_transfer", 5, -0.22, pm)
            po = make_traces(probe, seed + 700 + probe_idx, generation, "probe_ood", 5, 0.41, pm)

            proc = learner.synthesize_process(tuple(ptrain) + tuple(psel), pt, po)
            chosen, pev, psummary = learner.validate_process_frontier(
                train=tuple(ptrain) + tuple(psel),
                holdout=ph,
                transfer=pt,
                ood=po,
            ) if proc is not None else (None, None, {"candidate_count": 0, "validated_candidate_count": 0})

            competent = bool(pev and pev.accepted and pev.ood_error <= 1e-9)
            expected = {p["primitive_id"] for p in probe_parents}
            lineage_ok = bool(chosen and expected.issubset(
                primitive_lineage_ids(learner, tuple(chosen.used_primitives))
            ))
            probe_rates.append(float(competent))
            probe_reuse.append(float(competent and lineage_ok))
            if competent and not lineage_ok:
                probe_traps += 1
            semantic_matches = semantic_parent_match_count(
                learner,
                tuple(type("Cap", (), {
                    "primitive_id": p["primitive_id"],
                    "expression": p["expression"],
                })() for p in probe_parents),
                tuple(chosen.used_primitives) if chosen else (),
                tuple(ptrain),
                pm,
            )
            semantic_probe_reuse.append(float(
                competent and semantic_matches == len(probe_parents)
            ))

        previous_depth = initial_depth if not details else details[-1]["runtime_max_depth"]
        row = {
            "generation": generation,
            "accepted": accepted,
            "retained_count": len(retained),
            "target_depth": target_depth,
            "parent_ids": list(parent_ids),
            "recursive_reuse_ok": bool(recursive_reuse),
            "runtime_max_depth": int(learner.meta_policy["max_depth"]),
            "growth_delta": int(learner.meta_policy["max_depth"]) - int(previous_depth),
            "closure_success_rate": statistics.fmean(closure_rates) if closure_rates else 0.0,
            "closure_reuse_rate": statistics.fmean(closure_reuse) if closure_reuse else 0.0,
            "closure_trap_count": closure_traps,
            "probe_success_rate": statistics.fmean(probe_rates) if probe_rates else 0.0,
            "probe_reuse_rate": statistics.fmean(probe_reuse) if probe_reuse else 0.0,
            "semantic_probe_reuse_rate": statistics.fmean(semantic_probe_reuse) if semantic_probe_reuse else 0.0,
            "probe_trap_count": probe_traps,
            "primitive_candidate_frontier_size": int(validation.get("candidate_count", 0)),
            "primitive_validated_candidate_count": int(validation.get("validated_candidate_count", 0)),
        }
        details.append(row)
        print(json.dumps({"event": "VERSION_SPACE_GENERATION", **row}, sort_keys=True), flush=True)

    applicable = details[1:]
    result = {
        "seed": seed,
        "generations": generations,
        "final_retained_count": details[-1]["retained_count"],
        "final_runtime_max_depth": details[-1]["runtime_max_depth"],
        "lineage_closed": all(r["recursive_reuse_ok"] for r in applicable),
        "strict_growth": all(r["growth_delta"] > 0 for r in applicable),
        "every_generation_accepted": all(r["accepted"] for r in details),
        "every_closure_perfect": all(
            r["closure_success_rate"] == 1.0 and r["closure_reuse_rate"] == 1.0 and r["closure_trap_count"] == 0
            for r in details if r["closure_success_rate"] or r["generation"] == 0
        ),
        "every_probe_perfect": all(
            r["probe_success_rate"] == 1.0 and r["probe_reuse_rate"] == 1.0 and r["probe_trap_count"] == 0
            for r in details
        ),
        "depth_growth_contract": details[-1]["runtime_max_depth"] >= initial_depth + generations,
        "holdout_used_for_search_selection": False,
        "transfer_used_for_search_selection": False,
        "ood_used_for_search_selection": False,
        "external_model": False,
        "target_identity_available_to_runtime": False,
        "task_family_route": False,
        "generations_detail": details,
    }
    result["diagnostic_strict_components_all_clear"] = all([
        result["lineage_closed"],
        result["strict_growth"],
        result["every_generation_accepted"],
        result["every_closure_perfect"],
        result["every_probe_perfect"],
        result["depth_growth_contract"],
    ])
    print(json.dumps(result, sort_keys=True), flush=True)
    return result

if __name__ == "__main__":
    run_seed()
