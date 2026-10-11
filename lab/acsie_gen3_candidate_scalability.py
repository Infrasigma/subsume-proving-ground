#!/usr/bin/env python3
"""Gen3 compositional candidate-generation scaling only.

This diagnostic deliberately does not claim a verified Gen2 predecessor or
execute Gen3 selection/retention gates. It measures native candidate generator
work on fixed synthetic input volumes when the separate verified predecessor
export is blocked.
"""
from __future__ import annotations

import hashlib
import json
import os
import resource
import statistics
import time
from dataclasses import asdict
from typing import Any


def _json(value: Any) -> str:
    return json.dumps(value, sort_keys=True, separators=(",", ":"), default=str)


def main() -> int:
    import argparse
    parser = argparse.ArgumentParser()
    parser.add_argument("--seed", type=int, default=771221)
    parser.add_argument("--streams", type=int, required=True, choices=(3, 6, 12, 24))
    parser.add_argument("--max-programs", type=int, default=128)
    args = parser.parse_args()

    from cognitive_core.native_independent_core import NativeCognitiveCore
    from research.native_self_learning_kernel_gen3_compositional_retention_v1 import _make_streams

    stream_seed = args.seed + 8000
    streams = _make_streams(stream_seed, args.streams, rule="parity_relation")
    row_count = sum(len(s) for s in streams)
    core = NativeCognitiveCore(seed=args.seed)
    start = time.perf_counter()
    error = None
    variants = ()
    try:
        variants = NativeCognitiveCore._compositional_kernel_variants(
            core.learning_kernel,
            streams,
            max_programs=args.max_programs,
        )
    except BaseException as exc:
        error = f"{type(exc).__name__}: {exc}"
    elapsed_ms = (time.perf_counter() - start) * 1000.0

    serial = [asdict(v) for v in variants]
    signature = hashlib.sha256(_json(serial).encode()).hexdigest()
    program_signatures = {
        _json(v.get("context_program"))
        for v in serial
        if v.get("context_program") is not None
    }
    record = {
        "schema": "ACSIE.gen3_candidate_generation_scale_only.v1",
        "scientific_scope": "candidate enumeration and resource scaling only; no Gen3 selection or retention claim",
        "seed": args.seed,
        "stream_seed": stream_seed,
        "hash_seed": os.environ.get("PYTHONHASHSEED", "UNSET"),
        "stream_count": args.streams,
        "rows_per_stream": [len(s) for s in streams],
        "input_row_count": row_count,
        "candidate_program_cap": args.max_programs,
        "candidate_count": len(variants),
        "unique_candidate_signatures": len({ _json(v) for v in serial }),
        "unique_compositional_program_signatures": len(program_signatures),
        "candidate_generation_elapsed_ms": round(elapsed_ms, 3),
        "process_peak_rss_mb": round(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024.0, 3),
        "candidate_output_sha256": signature,
        "error": error,
        "integrity": {
            "external_model": False,
            "network_dependency": False,
            "candidate_limit_unchanged": args.max_programs == 128,
            "input_streams_are_gen3_inner_only": True,
            "outer_or_holdout_streams_used_for_selection": False,
        },
        "interpretation_guard": (
            "The Gen2 predecessor export failed its unchanged Gen2 scientific gate in the paired workflow. "
            "This fallback isolates only the cost and determinism of Gen3 native candidate generation. "
            "It does not test retention, holdout gains, transfer gains, or recursive Gen3 success."
        ),
    }
    print(_json(record), flush=True)
    return 0 if error is None else 2


if __name__ == "__main__":
    raise SystemExit(main())
