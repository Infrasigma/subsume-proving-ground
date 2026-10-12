#!/usr/bin/env python3
"""Parity-gated wrapper for exact invocation-local lineage-set pooling."""
from __future__ import annotations

import sys
from pathlib import Path

LAB = Path(__file__).resolve().parent
sys.path.insert(0, str(LAB))

import patch_h7_compositional_structural_key as base_patch
from patch_h7_shared_lineage_set_pool import apply_source as pooled_apply_source

# Use the same compositional structural key and FIFO-128 frontier cache in both
# arms. The only behavioral-search difference is bounded lineage-set interning.
base_patch.apply_source = pooled_apply_source

import run_h7_compositional_structural_key as runner

_original_make_sampler = runner.make_sampler

def make_sampler_with_pool_telemetry(diag_dir: Path) -> None:
    _original_make_sampler(diag_dir)
    sample_file = diag_dir / "sitecustomize.py"
    script = sample_file.read_text(encoding="utf-8")
    marker = '        row["online_metadata_count"]=len(loc.get("_H7_online_metadata",{}))\n'
    addition = marker + '        row["lineage_frozenset_pool_entries"]=len(loc.get("_H7_lineage_frozenset_pool",{}))\n'
    if marker not in script:
        raise SystemExit("sampler contract changed: online metadata marker missing")
    sample_file.write_text(script.replace(marker, addition, 1), encoding="utf-8")

runner.make_sampler = make_sampler_with_pool_telemetry

if __name__ == "__main__":
    raise SystemExit(runner.main())
