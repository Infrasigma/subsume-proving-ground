#!/usr/bin/env python3
"""Run the frozen H7 exact-key experiment with an interned-expression-ID candidate.

The underlying runner retains its exact-source checkout, signature parity,
regression suite, per-arm wall/RSS guards, stack/live snapshots and artifact
contract. This adapter replaces only the patch hook so both arms use the same
compositional structural key and the candidate additionally uses interned
expression identity.
"""
from __future__ import annotations

import sys
from pathlib import Path

LAB = Path(__file__).resolve().parent
sys.path.insert(0, str(LAB))

import patch_h7_compositional_structural_key as base_patch
from patch_h7_interned_expression_identity import apply_source as interned_apply_source

# The interned patch module captured the original compositional patch helper
# before this assignment. The runner imports apply_source from this module.
base_patch.apply_source = interned_apply_source

import run_h7_compositional_structural_key as runner

if __name__ == "__main__":
    raise SystemExit(runner.main())
