#!/usr/bin/env python3
"""Exact-source H7 compositional structural-key diagnostic patch.

The candidate replaces structural_expand(expr) + JSON serialization of the full
expanded tree with a compositional key builder that emits the byte-for-byte
equivalent sort_keys=True compact JSON string recursively. The old
structural_expand function remains unchanged as an independent test oracle.
Both arms receive the same fixed 128-entry lineage-frontier cache and diagnostic
accessors. No target, objective, dominance, ranking, or candidate-search rule is
changed by this harness.
"""
from __future__ import annotations
import argparse
import hashlib
from pathlib import Path

EXPECTED_SOURCE_SHA256 = "2ff981875397b7d0edb2fef1c33c655b0a15b8c404b13c50d13a804f77535aea"
SOURCE_COMMIT = "d83cb60286f195eec02b4d7aa45a60539316d115"
SOURCE_TREE = "12d2fa8fc1f1e34d3d7c442eef85aa6a71380420"

def replace_exact(source: str, old: str, new: str, *, count: int = 1, label: str) -> str:
    observed = source.count(old)
    if observed != count:
        raise SystemExit(f"{label}: expected {count} marker(s), got {observed}: {old[:160]!r}")
    return source.replace(old, new, count)

OLD_KEY = '''    def structural_key(expr: Mapping[str, Any]) -> str:
        return json.dumps(
            structural_expand(expr),
            sort_keys=True,
            separators=(",", ":"),
        )
'''

HOOKS = '''    find_exact_expression._H7_STRUCTURAL_KEY_FN = structural_key
    find_exact_expression._H7_STRUCTURAL_EXPAND_FN = structural_expand
    find_exact_expression._H7_STRUCTURAL_PMAP = pmap
'''

CONTROL_KEY = OLD_KEY + HOOKS

CANDIDATE_KEY = r'''    def _h7_structural_key(expr: Mapping[str, Any], active: tuple[str, ...] = ()) -> str:
        """Emit the exact canonical structural JSON key without expanded-AST dumps.

        Key order mirrors json.dumps(..., sort_keys=True, separators=(",", ":")):
        binary nodes are left/op/right, unary nodes arg/op, thresholds arg/lo/op.
        Macro recursion/cycle semantics match the original structural_expand.
        """
        op = str(expr["op"])
        if op in {"get", "const"}:
            # Preserve all leaf fields exactly, including any future extensions.
            return json.dumps(dict(expr), sort_keys=True, separators=(",", ":"))

        if op == "macro":
            mid = str(expr["id"])
            if mid in active or mid not in pmap:
                return json.dumps(
                    {"op": "macro", "id": mid},
                    sort_keys=True,
                    separators=(",", ":"),
                )
            return _h7_structural_key(pmap[mid], active + (mid,))

        if op in {"abs", "neg", "threshold"}:
            arg_key = _h7_structural_key(expr["arg"], active)
            op_key = json.dumps(op, separators=(",", ":"))
            if op == "threshold":
                lo_key = json.dumps(expr["lo"], sort_keys=True, separators=(",", ":"))
                return '{"arg":' + arg_key + ',"lo":' + lo_key + ',"op":' + op_key + '}'
            return '{"arg":' + arg_key + ',"op":' + op_key + '}'

        left_key = _h7_structural_key(expr["left"], active)
        right_key = _h7_structural_key(expr["right"], active)
        if op in {"add", "mul", "max", "min"} and left_key > right_key:
            left_key, right_key = right_key, left_key
        op_key = json.dumps(op, separators=(",", ":"))
        return '{"left":' + left_key + ',"op":' + op_key + ',"right":' + right_key + '}'

    def structural_key(expr: Mapping[str, Any]) -> str:
        return _h7_structural_key(expr)
''' + HOOKS

def apply_source(source: str, mode: str) -> str:
    if "_H7_STRUCTURAL_KEY_FN" in source or "_h7_lf_cache" in source:
        raise SystemExit("structural-key diagnostic markers already present")
    from patch_h7_metadata_only_bucket_duplicate import apply_frontier_cache
    source = apply_frontier_cache(source)
    source = replace_exact(
        source,
        OLD_KEY,
        CONTROL_KEY if mode == "control" else CANDIDATE_KEY,
        label=f"install {mode} structural-key implementation and exact-key test hooks",
    )
    compile(source, "cognitive_core/behavioral_search.py", "exec")
    return source

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("source", type=Path)
    ap.add_argument("--mode", choices=("control", "candidate"), required=True)
    args = ap.parse_args()
    path = args.source
    original = path.read_text(encoding="utf-8")
    before = hashlib.sha256(original.encode()).hexdigest()
    if before != EXPECTED_SOURCE_SHA256:
        raise SystemExit(f"pinned source SHA mismatch: expected {EXPECTED_SOURCE_SHA256}, got {before}")
    patched = apply_source(original, args.mode)
    after = hashlib.sha256(patched.encode()).hexdigest()
    path.write_text(patched, encoding="utf-8")
    print(f"MODE={args.mode}")
    print(f"ACSIE_SOURCE_COMMIT={SOURCE_COMMIT}")
    print(f"ACSIE_SOURCE_TREE={SOURCE_TREE}")
    print(f"ORIGINAL_SOURCE_SHA256={before}")
    print(f"DIAGNOSTIC_SOURCE_SHA256={after}")
    print("FIXED_FACTOR=lineage-frontier FIFO cache capacity 128 in both arms")
    print("FACTOR=candidate computes structural JSON sort keys compositionally without materializing expanded AST or repeatedly dumping expanded children")
    print("CONTROL=original structural_expand followed by JSON dump, with identical diagnostic hook")
    print("UNCHANGED=target/objective/selection/dominance/ranking/source-level scientific gates")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
