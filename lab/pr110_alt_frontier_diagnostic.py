from __future__ import annotations

import json

import sys
from pathlib import Path

SNAPSHOT = Path(__file__).resolve().parents[1] / "research" / "pr110_acsie_snapshot" / "B"
sys.path.insert(0, str(SNAPSHOT))

from cognitive_core.behavioral_search import find_exact_expression
from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import ArchiveRecord, CognitivePrimitive, Trace


def build_compiler() -> OpenEndedRecursiveCognitiveCompiler:
    c = OpenEndedRecursiveCognitiveCompiler(max_depth=1, population=24, seed=2026100305)
    for index in range(16):
        pid = f"prim:{'a%02d' % index if index < 15 else 'z15'}"
        prim = CognitivePrimitive(
            primitive_id=pid,
            expression={"op": "get", "key": "x"},
            input_roles=("x",),
            output_semantics="delta",
            generation=1,
            parent_ids=(),
            source_families=("synthetic-alt-frontier",),
            train_error=0.0,
            holdout_error=0.0,
            transfer_error=0.0,
            complexity=1,
            provenance={"diagnostic": True, "lineage_role": "executable"},
        )
        c.primitives[pid] = prim
        c.archive[pid] = ArchiveRecord(prim)
    c.generation = 1
    return c


ROWS = tuple(
    Trace(
        {"x": float(i)},
        float(i),
        "synthetic-alt-frontier",
        {},
        f"alt-{i}",
    )
    for i in range(8)
)


def run_cap(cap: int | None) -> dict:
    c = build_compiler()
    result = find_exact_expression(
        c,
        ROWS,
        max_depth=1,
        require_macro=True,
        max_alternatives=cap,
    )
    assert result is not None
    ids = [
        str(expr["id"])
        for expr, _used in result.alternatives
        if expr.get("op") == "macro"
    ]
    return {
        "cap": cap,
        "alternative_count": len(result.alternatives),
        "candidate_ids": ids,
        "sentinel_z15_present": "prim:z15" in ids,
        "selected_executable_lineage_ids": result.stats["selected_executable_lineage_ids"],
        "target_candidate_count": result.stats["target_candidate_count"],
    }


def main() -> None:
    results = [run_cap(cap) for cap in (8, 16, 32, None)]
    assert results[0]["alternative_count"] == 8
    assert not results[0]["sentinel_z15_present"]
    assert results[1]["sentinel_z15_present"]
    assert results[2]["sentinel_z15_present"]
    assert results[3]["sentinel_z15_present"]
    print(json.dumps({
        "schema": "ACSIE.PR110.alt-frontier-diagnostic.v1",
        "purpose": "Determine whether target lineage candidates beyond rank 8 are actually lost by the truncation.",
        "results": results,
        "scientific_interpretation": (
            "H3_DIRECT_SUPPORT: candidate rank >8 exists and is valid/executable-lineage-preserving."
        ),
    }, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
