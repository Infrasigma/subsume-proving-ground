#!/usr/bin/env python3
"""Auditable Gen6 open-ended growth and Gen3 native self-improvement scaling probes.

Only research instrumentation is added here. The pinned ACSIE source and its
scientific gates are not patched. All generated benchmark targets remain inside
the evaluator; the runtime sees only rows passed to the original APIs.
"""
from __future__ import annotations

import argparse
import hashlib
import importlib
import json
import os
import resource
import sys
import time
from collections import defaultdict
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "lab"))


def _rss_mb() -> float:
    return resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024.0


def _sha256(value: Any) -> str:
    raw = json.dumps(value, sort_keys=True, separators=(",", ":"), default=str).encode()
    return hashlib.sha256(raw).hexdigest()


def run_gen6(seed: int, generations: int) -> dict[str, Any]:
    """Instrument the actual open-ended self-extending compiler through G6."""
    import acsie_open_ended_self_extending_proving as growth_lab
    import cognitive_core.open_ended_growth as open_growth
    import cognitive_core.recursive_cognitive_compiler as rcc

    base_type = rcc.RecursiveCognitiveCompiler
    learner_type = open_growth.OpenEndedRecursiveCognitiveCompiler
    original_run_seed = growth_lab.run_seed
    original_make_traces = growth_lab.make_traces
    original_find = open_growth.find_exact_expression

    method_specs = (
        (base_type, "invent_primitive"),
        (learner_type, "invent_primitive"),
        (base_type, "_exprs"),
        (base_type, "synthesize_process_frontier"),
        (base_type, "synthesize_process"),
        (base_type, "evaluate"),
        (learner_type, "accept_primitive"),
    )
    original_methods = {(cls, name): getattr(cls, name) for cls, name in method_specs}
    original_init = base_type.__init__
    instance_roles: dict[int, str] = {}
    state: dict[str, Any] = {"generation": None}

    def new_role_metrics():
        return {
            "method_calls": defaultdict(int),
            "method_elapsed_ms": defaultdict(float),
            "search_calls": [],
            "primitive_changes": [],
            "process_frontier_calls": [],
            "accepted_primitive_count": 0,
        }

    meter: dict[int, dict[str, Any]] = defaultdict(
        lambda: {"learner": new_role_metrics(), "baseline": new_role_metrics()}
    )

    def role_for(compiler: Any) -> str:
        return instance_roles.get(
            id(compiler),
            "learner" if isinstance(compiler, learner_type) else "baseline",
        )

    def slot_for(compiler: Any) -> dict[str, Any]:
        generation = state["generation"]
        if generation is None:
            generation = int(getattr(compiler, "generation", 0))
        return meter[max(0, int(generation))][role_for(compiler)]

    def init_wrapper(self, *args, **kwargs):
        original_init(self, *args, **kwargs)
        instance_roles[id(self)] = (
            "learner" if isinstance(self, learner_type) else "baseline"
        )

    def traces_wrapper(expr, trace_seed, generation, split, count, shift, macros):
        state["generation"] = int(generation)
        return original_make_traces(
            expr, trace_seed, generation, split, count, shift, macros
        )

    def traced_find(compiler, rows, max_depth, **kwargs):
        started = time.perf_counter()
        result = None
        error = None
        try:
            result = original_find(compiler, rows, max_depth, **kwargs)
            return result
        except BaseException as exc:
            error = f"{type(exc).__name__}: {exc}"
            raise
        finally:
            stats = dict(getattr(result, "stats", {}) or {})
            slot = slot_for(compiler)
            keys = (
                "status", "generated_expressions", "unique_states",
                "duplicate_states", "dominance_pruned_states",
                "dominance_frontier_max", "target_candidate_count",
                "generic_binary_pairs_evaluated", "generic_binary_path_skipped_after_target",
                "max_lineage_coverage", "first_exact_target_depth", "best_target_depth",
                "depths_completed", "frontier_profile_count",
            )
            slot["search_calls"].append({
                "engine": "behavioral_search.find_exact_expression",
                "elapsed_ms": round((time.perf_counter() - started) * 1000.0, 3),
                "input_rows": len(rows),
                "max_depth_argument": int(max_depth),
                "primitive_count": len(getattr(compiler, "primitives", {})),
                "require_macro": bool(kwargs.get("require_macro", False)),
                "target_found": result is not None,
                "stats": {k: stats.get(k) for k in keys if k in stats},
                "error": error,
            })

    def wrap_method(cls, name, original):
        def wrapped(self, *args, **kwargs):
            if name == "__init__":
                return original(self, *args, **kwargs)
            slot = slot_for(self)
            started = time.perf_counter()
            before_count = len(getattr(self, "primitives", {}))
            result = None
            error = None
            try:
                result = original(self, *args, **kwargs)
                return result
            except BaseException as exc:
                error = f"{type(exc).__name__}: {exc}"
                raise
            finally:
                elapsed = (time.perf_counter() - started) * 1000.0
                slot["method_calls"][name] += 1
                slot["method_elapsed_ms"][name] += elapsed

                if name == "_exprs":
                    values = result or []
                    slot["search_calls"].append({
                        "engine": "recursive_compiler._exprs",
                        "elapsed_ms": round(elapsed, 3),
                        "input_rows": None,
                        "max_depth_argument": int(
                            args[2] if len(args) > 2 else kwargs.get("depth_limit", 0)
                        ),
                        "primitive_count": len(getattr(self, "primitives", {})),
                        "target_found": None,
                        "stats": {
                            "generated_expressions": len(values),
                            "unique_states": len(values),
                        },
                        "error": error,
                    })

                if name == "invent_primitive":
                    slot["primitive_changes"].append({
                        "primitive_count_before": before_count,
                        "primitive_count_after": len(getattr(self, "primitives", {})),
                        "returned_primitive": result is not None,
                        "primitive_id": getattr(result, "primitive_id", None),
                        "parent_ids": list(getattr(result, "parent_ids", ()) or ()),
                        "train_rows": len(args[0]) if len(args) > 0 else None,
                        "holdout_rows": len(args[1]) if len(args) > 1 else None,
                        "transfer_rows": len(args[2]) if len(args) > 2 else None,
                        "elapsed_ms": round(elapsed, 3),
                        "error": error,
                    })

                if name == "synthesize_process_frontier":
                    summary = dict(
                        getattr(self, "last_process_candidate_selection_summary", {}) or {}
                    )
                    try:
                        returned_count = len(result or ())
                    except TypeError:
                        returned_count = None
                    slot["process_frontier_calls"].append({
                        "returned_candidate_count": returned_count,
                        "discovery_qualified_count": summary.get("discovery_qualified"),
                        "primitive_count": len(getattr(self, "primitives", {})),
                        "generation": int(getattr(self, "generation", 0)),
                        "elapsed_ms": round(elapsed, 3),
                        "error": error,
                    })

                if name == "accept_primitive" and bool(result):
                    slot["accepted_primitive_count"] += 1

        return wrapped

    growth_lab.make_traces = traces_wrapper
    open_growth.find_exact_expression = traced_find
    base_type.__init__ = init_wrapper
    for (cls, name), original in original_methods.items():
        setattr(cls, name, wrap_method(cls, name, original))

    started = time.perf_counter()
    try:
        raw = original_run_seed(seed, generations)
    finally:
        growth_lab.make_traces = original_make_traces
        open_growth.find_exact_expression = original_find
        base_type.__init__ = original_init
        for (cls, name), original in original_methods.items():
            setattr(cls, name, original)

    raw_generations = list(raw.get("generations_detail", []))
    per_generation = []
    for gen in range(generations):
        outcome = raw_generations[gen] if gen < len(raw_generations) else {}
        role_metrics = {}
        for role in ("learner", "baseline"):
            r = meter[gen][role]
            calls = list(r["search_calls"])
            exact_calls = [x for x in calls if x.get("engine") == "behavioral_search.find_exact_expression"]
            enum_calls = [x for x in calls if x.get("engine") == "recursive_compiler._exprs"]
            all_stats = [x.get("stats", {}) for x in calls]
            def max_stat(key: str) -> int:
                return max((int(x[key]) for x in all_stats if isinstance(x.get(key), (int, float))), default=0)

            role_metrics[role] = {
                "method_calls": dict(sorted(r["method_calls"].items())),
                "method_elapsed_ms": {
                    k: round(v, 3) for k, v in sorted(r["method_elapsed_ms"].items())
                },
                "primitive_changes": r["primitive_changes"],
                "accepted_primitive_count": r["accepted_primitive_count"],
                "process_frontier_calls": r["process_frontier_calls"],
                "exact_search_calls": len(exact_calls),
                "exact_search_elapsed_ms_total": round(sum(x["elapsed_ms"] for x in exact_calls), 3),
                "exact_search_generated_expressions_total": sum(
                    int(x.get("stats", {}).get("generated_expressions", 0) or 0)
                    for x in exact_calls
                ),
                "exact_search_unique_states_total": sum(
                    int(x.get("stats", {}).get("unique_states", 0) or 0)
                    for x in exact_calls
                ),
                "exact_search_max_dominance_frontier": max(
                    (int(x.get("stats", {}).get("dominance_frontier_max", 0) or 0)
                     for x in exact_calls), default=0
                ),
                "expression_enum_calls": len(enum_calls),
                "expression_enum_elapsed_ms_total": round(sum(x["elapsed_ms"] for x in enum_calls), 3),
                "expression_enum_candidate_total": sum(
                    int(x.get("stats", {}).get("generated_expressions", 0) or 0)
                    for x in enum_calls
                ),
                "search_calls": calls,
            }

        per_generation.append({
            "generation": gen,
            "target_depth": outcome.get("target_depth"),
            "accepted": outcome.get("accepted"),
            "parent_used": outcome.get("parent_used"),
            "multi_parent_used": outcome.get("multi_parent_used"),
            "retained_count": outcome.get("retained_count"),
            "runtime_max_depth": outcome.get("runtime_max_depth"),
            "closure_pair_count": outcome.get("closure_pair_count"),
            "closure_success_count": outcome.get("closure_success_count"),
            "probe_success_rate": outcome.get("probe_success_rate"),
            "baseline_probe_success_rate": outcome.get("baseline_probe_success_rate"),
            "metrics": role_metrics,
        })

    result = {
        "schema": "ACSIE.gen6_scalability_diagnostic.v2",
        "mode": "gen6_open_ended_self_extending_compiler",
        "seed": seed,
        "generation_indices": list(range(generations)),
        "gen6_checkpoint_index": 6,
        "generation_6_reached": generations >= 7 and len(raw_generations) >= 7,
        "original_growth_lab_result": raw,
        "per_generation_telemetry": per_generation,
        "run_elapsed_ms_including_lightweight_telemetry": round(
            (time.perf_counter() - started) * 1000.0, 3
        ),
        "process_peak_rss_mb": round(_rss_mb(), 3),
        "telemetry": (
            "Pass-through timing/count wrappers on the actual OpenEndedRecursiveCognitiveCompiler, "
            "its inherited process frontier, and its imported behavioral search; no cProfile/sys.setprofile."
        ),
        "source_integrity": {
            "external_model": raw.get("external_model") is False,
            "target_identity_available_to_runtime": raw.get("target_identity_available_to_runtime") is False,
            "task_family_route": raw.get("task_family_route") is False,
            "source_script_finite_gate": raw.get("finite_open_ended_growth_gate") is True,
        },
        "raw_result_sha256": _sha256(raw),
        "interpretation_guard": (
            "Exact ACSIE open-ended self-extending synthetic growth workload; G6 is generation index 6. "
            "This is not the 5x12 frozen qualification, real-world generality, or an AGI/ASI claim."
        ),
    }
    return result

def run_gen3(seed: int, n_gen3_inner_streams: int, frozen_state_path: str) -> dict[str, Any]:
    """Scale the documented, verified-frozen-Gen2 Gen3 continuation (not a fresh Gen2 rerun)."""
    gen3 = importlib.import_module(
        "research.gen3_from_frozen_verified_gen2_v1"
    )
    core_type = importlib.import_module(
        "cognitive_core.native_independent_core"
    ).NativeCognitiveCore
    original_make_streams = gen3._make_streams
    original_regression = gen3._regression
    original_variants = core_type._compositional_kernel_variants
    original_kernel_eval = core_type._evaluate_kernel

    telemetry: dict[str, Any] = {
        "stream_construction": [],
        "variant_generation": [],
        "kernel_evaluation": {"calls": 0, "elapsed_ms": 0.0, "streams": 0, "rows": 0},
        "historical_regression": {"calls": 0, "elapsed_ms": 0.0, "sets": 0, "streams": 0, "rows": 0},
    }
    target_stream_seed = seed + 8000

    def scaled_make_streams(stream_seed, count, *, rule):
        scale_this_gen3_input = (
            int(stream_seed) == target_stream_seed
            and int(count) == 6
            and str(rule) == "parity_relation"
        )
        actual_count = n_gen3_inner_streams if scale_this_gen3_input else int(count)
        streams = original_make_streams(stream_seed, actual_count, rule=rule)
        telemetry["stream_construction"].append({
            "stream_seed": int(stream_seed),
            "seed_offset": int(stream_seed) - seed,
            "rule": str(rule),
            "requested_count": int(count),
            "actual_count": actual_count,
            "row_count": sum(len(row) for row in streams),
            "scaled_gen3_inner_only": scale_this_gen3_input,
        })
        return streams

    def variants_wrapper(cls, base, streams, *, max_programs=128):
        started = time.perf_counter()
        result = original_variants(base, streams, max_programs=max_programs)
        telemetry["variant_generation"].append({
            "input_streams": len(streams),
            "input_rows": sum(len(stream) for stream in streams),
            "max_programs_argument": int(max_programs),
            "returned_variants": len(result),
            "elapsed_ms": round((time.perf_counter() - started) * 1000.0, 3),
        })
        return result

    def kernel_eval_wrapper(kernel, streams, *, seed=0, include_retention=True, protected_hypotheses=None):
        started = time.perf_counter()
        result = original_kernel_eval(
            kernel, streams, seed=seed, include_retention=include_retention,
            protected_hypotheses=protected_hypotheses,
        )
        telemetry["kernel_evaluation"]["calls"] += 1
        telemetry["kernel_evaluation"]["elapsed_ms"] += (
            time.perf_counter() - started
        ) * 1000.0
        telemetry["kernel_evaluation"]["streams"] += len(streams)
        telemetry["kernel_evaluation"]["rows"] += sum(len(s) for s in streams)
        return result

    def regression_wrapper(pre_state, candidate, regression_sets):
        started = time.perf_counter()
        result = original_regression(pre_state, candidate, regression_sets)
        telemetry["historical_regression"]["calls"] += 1
        telemetry["historical_regression"]["elapsed_ms"] += (
            time.perf_counter() - started
        ) * 1000.0
        telemetry["historical_regression"]["sets"] += len(regression_sets)
        telemetry["historical_regression"]["streams"] += sum(len(s) for s in regression_sets)
        telemetry["historical_regression"]["rows"] += sum(
            len(stream) for group in regression_sets for stream in group
        )
        return result

    gen3._make_streams = scaled_make_streams
    gen3._regression = regression_wrapper
    core_type._compositional_kernel_variants = classmethod(variants_wrapper)
    core_type._evaluate_kernel = staticmethod(kernel_eval_wrapper)

    started = time.perf_counter()
    try:
        result = gen3.run(frozen_state_path, seed=seed)
    finally:
        gen3._make_streams = original_make_streams
        gen3._regression = original_regression
        core_type._compositional_kernel_variants = classmethod(original_variants.__func__)
        core_type._evaluate_kernel = staticmethod(original_kernel_eval)

    for entry in telemetry["stream_construction"]:
        if entry["scaled_gen3_inner_only"]:
            assert entry["actual_count"] == n_gen3_inner_streams
        else:
            assert entry["actual_count"] == entry["requested_count"]

    for key in ("kernel_evaluation", "historical_regression"):
        telemetry[key]["elapsed_ms"] = round(telemetry[key]["elapsed_ms"], 3)

    scientific_signature = _sha256(result)
    integrity = result.get("integrity", {})
    actual_gen3_stream_records = [
        item for item in telemetry["stream_construction"]
        if item["scaled_gen3_inner_only"]
    ]
    all_outer_fixed = all(
        item["actual_count"] == 6
        for item in telemetry["stream_construction"]
        if item["rule"] == "parity_relation" and not item["scaled_gen3_inner_only"]
    )
    return {
        "schema": "ACSIE.gen3_frozen_scalability_diagnostic.v1",
        "mode": "gen3_frozen_verified_gen2_compositional_continuation",
        "seed": seed,
        "gen3_inner_stream_count": n_gen3_inner_streams,
        "frozen_state_path": str(frozen_state_path),
        "frozen_predecessor_commit": result.get("provenance", {}).get("frozen_predecessor_commit"),
        "executed_gen3_source_commit": result.get("provenance", {}).get("executed_source_commit"),
        "gen3_scientific_status": result.get("scientific_status"),
        "gen3_reason": result.get("reason"),
        "candidate_count": result.get("candidate_count"),
        "eligible_count": result.get("eligible_count"),
        "gen3_holdout_gain": result.get("gen3", {}).get("holdout_gain"),
        "gen3_transfer_gain": result.get("gen3", {}).get("transfer_gain"),
        "retention": result.get("retention"),
        "selection": result.get("selection"),
        "fixed_candidate_program_cap": 128,
        "scaled_gen3_inner_stream_builds": actual_gen3_stream_records,
        "all_outer_stream_counts_fixed": all_outer_fixed,
        "search_scaling_telemetry": telemetry,
        "run_elapsed_ms_including_lightweight_telemetry": round(
            (time.perf_counter() - started) * 1000.0, 3
        ),
        "process_peak_rss_mb": round(_rss_mb(), 3),
        "original_unmodified_gen3_result": result,
        "scientific_signature_sha256": scientific_signature,
        "integrity": {
            "holdout_contamination": integrity.get("holdout_contamination") is False,
            "task_routing": integrity.get("task_routing") is False,
            "external_model": integrity.get("external_model") is False,
            "network_dependency": integrity.get("network_dependency") is False,
            "outer_streams_unchanged": all_outer_fixed,
            "exactly_one_target_stream_group_scaled": len(actual_gen3_stream_records) == 1,
        },
        "interpretation_guard": (
            "Scales only Gen3 inner discovery streams while loading the verified historical Gen2 state. "
            "Gen1/Gen2 predecessor and all outer regression/Gen3 outer streams remain fixed. "
            "This is a single-seed diagnostic with repeats, not a multi-seed qualification or AGI/ASI claim."
        ),
    }

def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", required=True, choices=("gen6", "gen3"))
    parser.add_argument("--seed", type=int, required=True)
    parser.add_argument("--generations", type=int, default=7)
    parser.add_argument("--gen3-inner-streams", type=int, choices=(3, 6, 12, 24), default=6)
    parser.add_argument("--frozen-state", default=None)
    args = parser.parse_args()

    try:
        if args.mode == "gen6":
            result = run_gen6(args.seed, args.generations)
        else:
            if not args.frozen_state:
                raise ValueError("--frozen-state is required for the verified-Gen2 Gen3 protocol")
            result = run_gen3(args.seed, args.gen3_inner_streams, args.frozen_state)
        print(json.dumps(result, sort_keys=True, separators=(",", ":"), default=str), flush=True)
        return 0
    except BaseException as exc:
        failure = {
            "schema": "ACSIE.gen6_gen3_scalability.child_failure.v1",
            "mode": args.mode,
            "seed": args.seed,
            "generations": args.generations,
            "gen3_inner_streams": args.gen3_inner_streams,
            "status": "CHILD_EXCEPTION",
            "error": f"{type(exc).__name__}: {exc}",
        }
        print(json.dumps(failure, sort_keys=True, separators=(",", ":")), flush=True)
        import traceback
        traceback.print_exc()
        return 0


if __name__ == "__main__":
    raise SystemExit(main())
