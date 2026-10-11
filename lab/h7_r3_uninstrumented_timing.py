#!/usr/bin/env python3
"""Uninstrumented, single-search H7 R2/R3 timing probe.

Unlike the profiling harness, this script does not install sys.setprofile or
cProfile. It times only find_exact_expression after constructing the fixed
synthetic 2/4/6/8/10-primitive alias fixture. The workflow counterbalances R2/R3
execution order and repeats each cell under two hash seeds. Timing is still a
bounded diagnostic, not a broad performance qualification.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import resource
import subprocess
import sys
import time
from typing import Any, Mapping

from cognitive_core.behavioral_search import find_exact_expression
from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import (
    ArchiveRecord,
    CognitivePrimitive,
    Trace,
    ast_depth,
    ast_nodes,
)


def expanded_cost(
    expression: Mapping[str, Any],
    primitives: Mapping[str, CognitivePrimitive],
    active: tuple[str, ...] = (),
) -> tuple[int, int]:
    op = expression["op"]
    if op == "macro":
        pid = str(expression["id"])
        if pid in active or pid not in primitives:
            raise ValueError(f"invalid/cyclic macro edge: {pid}")
        return expanded_cost(primitives[pid].expression, primitives, active + (pid,))
    if op in {"get", "const"}:
        return (1, 1)
    if op in {"abs", "neg", "threshold"}:
        depth, nodes = expanded_cost(expression["arg"], primitives, active)
        return (depth + 1, nodes + 1)
    left = expanded_cost(expression["left"], primitives, active)
    right = expanded_cost(expression["right"], primitives, active)
    return (1 + max(left[0], right[0]), 1 + left[1] + right[1])


def add_primitive(c: OpenEndedRecursiveCognitiveCompiler, p: CognitivePrimitive) -> None:
    c.primitives[p.primitive_id] = p
    c.archive[p.primitive_id] = ArchiveRecord(p)


def make_compiler(alias_pairs: int) -> OpenEndedRecursiveCognitiveCompiler:
    c = OpenEndedRecursiveCognitiveCompiler(max_depth=2, seed=17)
    p_a = CognitivePrimitive(
        "prim:a",
        {"op": "neg", "arg": {"op": "neg", "arg": {"op": "neg", "arg": {"op": "neg", "arg": {"op": "get", "key": "x"}}}}},
        ("x",), "delta", 0, (), ("opaque",), 0.0, 0.0, 0.0, 5,
        {"origin": "fixed-synthetic-base"},
    )
    p_b = CognitivePrimitive(
        "prim:b",
        {
            "op": "add",
            "left": {"op": "mul", "left": {"op": "get", "key": "y"}, "right": {"op": "const", "value": 1.0}},
            "right": {"op": "mul", "left": {"op": "get", "key": "y"}, "right": {"op": "const", "value": 0.0}},
        },
        ("y",), "delta", 0, (), ("opaque",), 0.0, 0.0, 0.0, 7,
        {"origin": "fixed-synthetic-base"},
    )
    add_primitive(c, p_a)
    add_primitive(c, p_b)
    previous_a, previous_b = p_a.primitive_id, p_b.primitive_id
    for index in range(1, alias_pairs + 1):
        aid = f"prim:alias:a:{index:02d}"
        aexpr = {"op": "neg", "arg": {"op": "neg", "arg": {"op": "macro", "id": previous_a}}}
        add_primitive(c, CognitivePrimitive(
            aid, aexpr, ("x",), "delta", 0, (previous_a,), ("opaque",),
            0.0, 0.0, 0.0, ast_nodes(aexpr),
            {"origin": "bounded-redundant-executable-alias", "alias_index": index},
        ))
        previous_a = aid
        bid = f"prim:alias:b:{index:02d}"
        bexpr = {"op": "neg", "arg": {"op": "neg", "arg": {"op": "macro", "id": previous_b}}}
        add_primitive(c, CognitivePrimitive(
            bid, bexpr, ("y",), "delta", 0, (previous_b,), ("opaque",),
            0.0, 0.0, 0.0, ast_nodes(bexpr),
            {"origin": "bounded-redundant-executable-alias", "alias_index": index},
        ))
        previous_b = bid
    return c


def run_next_notch_probe(args: argparse.Namespace) -> None:
    """Measure one 12-primitive search; report failures as data rather than gating baseline."""
    rows = tuple(
        Trace(
            {"x": x, "y": (-2.0 * x if x < 0.0 else 0.0)},
            (-3.0 * x if x < 0.0 else x),
            "opaque", {}, f"h7-next-notch-{i}",
        )
        for i, x in enumerate((-4.0, -3.0, -2.0, -1.0, 1.0, 2.0, 3.0, 4.0))
    )
    compiler = make_compiler(args.alias_pairs)
    started = time.perf_counter()
    result = None
    error = None
    try:
        result = find_exact_expression(
            compiler, rows, max_depth=3, require_macro=True, max_alternatives=8,
        )
    except Exception as exc:
        error = f"{type(exc).__name__}: {exc}"
    elapsed_ms = (time.perf_counter() - started) * 1000.0
    profiles = []
    if result is not None:
        try:
            profiles = sorted({
                expanded_cost(expr, compiler.primitives)
                for expr, used in result.alternatives
                if set(used) == {"prim:a", "prim:b"} and ast_depth(expr) == 3
            })
        except Exception as exc:
            error = f"{type(exc).__name__}: {exc}"
    peak_rss_mb = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024.0
    preserved = (7, 14) in profiles
    if error:
        status = "CRASHED"
    elif elapsed_ms > 15000:
        status = "TIME_GUARD_EXCEEDED"
    elif peak_rss_mb > 2048:
        status = "MEMORY_GUARD_EXCEEDED"
    elif result is None:
        status = "TARGET_NOT_FOUND"
    elif args.source == "r2" and preserved:
        status = "R2_NEGATIVE_CONTROL_INVALID"
    elif args.source == "r3" and not preserved:
        status = "R3_PROFILE_LOST"
    else:
        status = "PASS"
    stable = {
        "source": args.source,
        "source_commit": (
            "5b6dc50d36a2584ab9c05525ed19047ca1792b3b"
            if args.source == "r2"
            else "d83cb60286f195eec02b4d7aa45a60539316d115"
        ),
        "hash_seed": args.seed,
        "order_pair": args.pair,
        "alias_pairs": args.alias_pairs,
        "primitive_count": len(compiler.primitives),
        "rows": len(rows),
        "max_depth": 3,
        "target_found": result is not None,
        "returned_alternative_count": 0 if result is None else len(result.alternatives),
        "returned_base_pair_profiles": [list(x) for x in profiles],
        "expected_profile_7_14_returned": preserved,
        "selected_expression": None if result is None else result.expression,
        "selected_used_primitives": [] if result is None else list(result.used_primitives),
        "search_stats": {} if result is None else dict(result.stats),
        "error": error,
    }
    payload = {k: v for k, v in stable.items() if k not in {"hash_seed", "order_pair"}}
    signature = hashlib.sha256(
        json.dumps(payload, sort_keys=True, separators=(",", ":")).encode()
    ).hexdigest()
    print(json.dumps({
        "schema": "ACSIE.H7_R3.next_notch_probe.v2",
        "status": status,
        "stable": stable,
        "stable_signature_sha256": signature,
        "performance": {
            "search_elapsed_ms": round(elapsed_ms, 3),
            "process_peak_rss_mb": round(peak_rss_mb, 3),
        },
        "instrumentation": "none within find_exact_expression (no cProfile/sys.setprofile)",
        "interpretation_guard": "12-primitive bounded synthetic diagnostic only.",
    }, sort_keys=True, separators=(",", ":")), flush=True)


def attach_next_notch_probe(args: argparse.Namespace, row: dict[str, Any]) -> None:
    """Probe 12 primitives twice per source and seed, with hard resource guards."""
    if args.alias_pairs != 4 or args.pair not in (1, 4):
        return
    root = pathlib.Path(os.environ.get("GITHUB_WORKSPACE", str(pathlib.Path.cwd())))
    evidence = root / "evidence" / "h7-r3"
    evidence.mkdir(parents=True, exist_ok=True)
    path = evidence / f"next-notch-a5-seed{args.seed}-pair{args.pair}-{args.source}.json"
    command = [
        sys.executable, str(pathlib.Path(__file__).resolve()),
        "--source", args.source, "--seed", str(args.seed),
        "--alias-pairs", "5", "--pair", str(args.pair), "--next-notch-only",
    ]
    item = {
        "schema": "ACSIE.H7_R3.next_notch_wrapper.v2",
        "seed": args.seed, "source": args.source, "pair": args.pair,
        "alias_pairs": 5, "primitive_count": 12,
    }
    try:
        proc = subprocess.run(
            command, capture_output=True, text=True, timeout=17, check=False,
            env=os.environ.copy(),
        )
        item["child_returncode"] = proc.returncode
        if proc.returncode == 0:
            try:
                child = json.loads(proc.stdout.strip().splitlines()[-1])
                item["record"] = child
                item["status"] = child.get("status", "INVALID_CHILD_SCHEMA")
            except Exception as exc:
                item.update(status="INVALID_CHILD_OUTPUT", error=f"{type(exc).__name__}: {exc}")
                item["stdout_tail"] = proc.stdout[-2000:]
        else:
            item.update(status="CHILD_EXIT_NONZERO", stdout_tail=proc.stdout[-2000:])
        item["stderr_tail"] = proc.stderr[-2000:]
    except subprocess.TimeoutExpired as exc:
        def tail(value: Any) -> str:
            if isinstance(value, bytes):
                return value.decode(errors="replace")[-2000:]
            return (value or "")[-2000:]
        item.update(status="WALL_TIMEOUT_17S", stdout_tail=tail(exc.stdout), stderr_tail=tail(exc.stderr))
    except Exception as exc:
        item.update(status="LAUNCH_ERROR", error=f"{type(exc).__name__}: {exc}")
    path.write_text(json.dumps(item, sort_keys=True, indent=2) + "\n")
    row["next_notch_probe"] = {"status": item["status"], "artifact_file": path.name}
    print(
        f"NEXT_NOTCH_PROBE source={args.source} seed={args.seed} pair={args.pair} status={item['status']}",
        file=sys.stderr,
    )

    # The existing loop runs seed 0, then 731; this is its final parent invocation.
    if (args.seed, args.pair, args.source) != (731, 4, "r3"):
        return
    expected_paths = [
        evidence / f"next-notch-a5-seed{seed}-pair{pair}-{source}.json"
        for seed in (0, 731) for pair in (1, 4) for source in ("r2", "r3")
    ]
    records, missing = [], []
    for candidate in expected_paths:
        if not candidate.exists():
            missing.append(candidate.name)
            continue
        try:
            records.append(json.loads(candidate.read_text()))
        except Exception as exc:
            records.append({"status": "INVALID_SIDECAR", "artifact_file": candidate.name,
                            "error": f"{type(exc).__name__}: {exc}"})
    signatures, repeat_checks, cross_seed_checks = {}, [], []
    for seed in (0, 731):
        for source in ("r2", "r3"):
            group = [x for x in records if x.get("seed") == seed and x.get("source") == source]
            sigs = [x.get("record", {}).get("stable_signature_sha256") for x in group]
            okay = len(group) == 2 and sigs[0] is not None and sigs[0] == sigs[1]
            signatures[(seed, source)] = sigs[0] if okay else None
            repeat_checks.append({
                "seed": seed, "source": source, "statuses": [x.get("status") for x in group],
                "signatures": sigs, "repeats_match": okay,
            })
    for source in ("r2", "r3"):
        a, b = signatures.get((0, source)), signatures.get((731, source))
        cross_seed_checks.append({"source": source, "seed0_signature": a,
                                  "seed731_signature": b, "matches": a is not None and a == b})
    all_pass = len(records) == 8 and all(x.get("status") == "PASS" for x in records)
    deterministic = not missing and all(x["repeats_match"] for x in repeat_checks) and all(
        x["matches"] for x in cross_seed_checks
    )
    summary = {
        "schema": "ACSIE.H7_R3.next_notch_summary.v2",
        "r2_source_commit": "5b6dc50d36a2584ab9c05525ed19047ca1792b3b",
        "r3_source_commit": "d83cb60286f195eec02b4d7aa45a60539316d115",
        "fixture": {"primitive_count": 12, "alias_pairs": 5, "rows": 8, "max_depth": 3},
        "hash_seeds": [0, 731], "paired_repetitions_per_source_and_seed": 2,
        "per_search_guard_seconds": 15, "child_wall_timeout_seconds": 17,
        "per_process_rss_guard_mb": 2048, "missing_sidecars": missing,
        "all_eight_searches_passed_target_and_profile_checks": all_pass,
        "repeat_and_cross_seed_signatures_stable": deterministic,
        "repeat_checks": repeat_checks, "cross_seed_checks": cross_seed_checks,
        "records": records,
        "classification": "PASSED_BOUNDED_NEXT_NOTCH_DIAGNOSTIC" if all_pass and deterministic
                          else "INCONCLUSIVE_DIAGNOSTIC_FAILURE_OR_RESOURCE_BOUNDARY",
        "interpretation_guard": "12-primitive bounded synthetic result only; not evidence of open-ended scalability.",
    }
    (evidence / "next-notch-summary.json").write_text(
        json.dumps(summary, sort_keys=True, indent=2) + "\n"
    )
    print(
        f"NEXT_NOTCH_SUMMARY classification={summary['classification']} "
        f"complete={len(records)}/8 missing={len(missing)} deterministic={deterministic}",
        file=sys.stderr,
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", choices=("r2", "r3"), required=True)
    parser.add_argument("--seed", type=int, required=True)
    parser.add_argument("--alias-pairs", choices=(0, 1, 2, 3, 4, 5), type=int, required=True)
    parser.add_argument("--pair", type=int, required=True)
    parser.add_argument("--next-notch-only", action="store_true")
    args = parser.parse_args()
    if args.next_notch_only:
        if args.alias_pairs != 5:
            raise SystemExit("--next-notch-only requires --alias-pairs 5")
        run_next_notch_probe(args)
        return

    rows = tuple(
        Trace(
            {"x": x, "y": (-2.0 * x if x < 0.0 else 0.0)},
            (-3.0 * x if x < 0.0 else x),
            "opaque", {}, f"h7-uninstrumented-{i}",
        )
        for i, x in enumerate((-4.0, -3.0, -2.0, -1.0, 1.0, 2.0, 3.0, 4.0))
    )
    compiler = make_compiler(args.alias_pairs)
    start = time.perf_counter()
    result = find_exact_expression(
        compiler, rows, max_depth=3, require_macro=True, max_alternatives=8,
    )
    elapsed_ms = (time.perf_counter() - start) * 1000.0
    peak_rss_mb = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024.0
    if result is None:
        raise AssertionError("exact target was not found")

    returned_profiles = sorted({
        expanded_cost(expr, compiler.primitives)
        for expr, used in result.alternatives
        if set(used) == {"prim:a", "prim:b"}
        and ast_depth(expr) == 3
    })
    expected_profile_returned = (7, 14) in returned_profiles
    if args.source == "r2" and expected_profile_returned:
        raise AssertionError("R2 negative control unexpectedly retained (7,14)")
    if args.source == "r3" and not expected_profile_returned:
        raise AssertionError("R3 lost preserved (7,14) profile")

    stable = {
        "source": args.source,
        "hash_seed": args.seed,
        "order_pair": args.pair,
        "alias_pairs": args.alias_pairs,
        "primitive_count": len(compiler.primitives),
        "rows": len(rows),
        "max_depth": 3,
        "target_found": True,
        "returned_alternative_count": len(result.alternatives),
        "returned_base_pair_profiles": [list(x) for x in returned_profiles],
        "expected_profile_7_14_returned": expected_profile_returned,
        "selected_expression": result.expression,
        "selected_used_primitives": list(result.used_primitives),
        "search_stats": dict(result.stats),
    }
    signature_payload = {key: value for key, value in stable.items()
                         if key not in {"source", "hash_seed", "order_pair"}}
    signature = hashlib.sha256(
        json.dumps(signature_payload, sort_keys=True, separators=(",", ":")).encode()
    ).hexdigest()
    row = {
        "stable": stable,
        "stable_signature_sha256": signature,
        "performance": {
            "search_elapsed_ms": round(elapsed_ms, 3),
            "process_peak_rss_mb": round(peak_rss_mb, 3),
        },
        "instrumentation": "none within find_exact_expression (no cProfile/sys.setprofile)",
    }
    attach_next_notch_probe(args, row)
    print(json.dumps(row, sort_keys=True, separators=(",", ":")), flush=True)
    if elapsed_ms > 15000:
        raise TimeoutError("15-second per-search guard exceeded")
    if peak_rss_mb > 2048:
        raise MemoryError("2-GiB per-process RSS guard exceeded")


if __name__ == "__main__":
    main()
