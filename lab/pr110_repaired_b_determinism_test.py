#!/usr/bin/env python3
"""PR110 repaired-B determinism regression.

The child process runs the exact repaired B snapshot under two different
PYTHONHASHSEED values. Identical initial state, seed, target, evaluator
identity, and checkpoint metadata must produce byte-identical checkpoint
artifacts and identical target/state digests.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path


CHILD = r'''
import hashlib
import json
import random
import tempfile
from pathlib import Path

from cognitive_core.behavioral_search import find_exact_expression
from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from cognitive_core.recursive_cognitive_compiler import (
    ArchiveRecord,
    CognitivePrimitive,
    Trace,
)
from lab.exact_checkpoint import save_checkpoint


SEED = 2026100305
PROVING_SHA = "33259fc5c69d87b99c604f0050508c433a279daf"
ACSIE_REF = "9b0c42eb439a4ee457f8b7adecc45a3f4ea7c4be"

learner = OpenEndedRecursiveCognitiveCompiler(max_depth=2, population=24, seed=SEED)

primitives = (
    CognitivePrimitive(
        "prim:a",
        {"op": "get", "key": "x"},
        ("x",),
        "delta",
        0,
        (),
        ("synthetic-determinism",),
        0.0, 0.0, 0.0, 1,
        {"lineage_ids": (), "external_model": False},
    ),
    CognitivePrimitive(
        "prim:b",
        {"op": "get", "key": "y"},
        ("y",),
        "delta",
        0,
        (),
        ("synthetic-determinism",),
        0.0, 0.0, 0.0, 1,
        {"lineage_ids": (), "external_model": False},
    ),
)
for primitive in primitives:
    learner.primitives[primitive.primitive_id] = primitive
    learner.archive[primitive.primitive_id] = ArchiveRecord(primitive)

rows = (
    Trace({"x": 1.0, "y": 2.0}, 3.0, "synthetic-determinism", {}, "r1"),
    Trace({"x": 2.0, "y": 3.0}, 5.0, "synthetic-determinism", {}, "r2"),
    Trace({"x": 4.0, "y": 5.0}, 9.0, "synthetic-determinism", {}, "r3"),
)

result = find_exact_expression(
    learner,
    rows,
    max_depth=2,
    require_macro=True,
    max_alternatives=8,
)
assert result is not None
learner.last_search_stats = dict(result.stats)

members = tuple(learner.last_search_stats["selected_structural_lineage_ids"])
assert set(members) == {"prim:a", "prim:b"}, members

target = tuple(float(row.target) for row in rows)
target_digest = hashlib.sha256(
    json.dumps(target, sort_keys=True, separators=(",", ":")).encode("utf-8")
).hexdigest()

with tempfile.TemporaryDirectory() as td:
    checkpoint = Path(td) / "checkpoint.json"
    state_digest = save_checkpoint(
        checkpoint,
        seed=SEED,
        generation_next=1,
        initial_depth=2,
        local_rng=random.Random(SEED),
        learner=learner,
        retained=[],
        generations_out=[],
        metadata={"proving_sha": PROVING_SHA, "acsie_ref": ACSIE_REF},
    )
    checkpoint_bytes = checkpoint.read_bytes()

print(json.dumps({
    "lineage_members": list(members),
    "semantic_state_digest": state_digest,
    "target_digest": target_digest,
    "checkpoint_sha256": hashlib.sha256(checkpoint_bytes).hexdigest(),
    "checkpoint_bytes_hex": checkpoint_bytes.hex(),
}, sort_keys=True))

def run_child(hash_seed: str) -> dict:
    env = dict(os.environ)
    env["PYTHONHASHSEED"] = hash_seed
    env["PYTHONPATH"] = (
        str(Path(__file__).resolve().parents[1] / "research" / "pr110_acsie_snapshot" / "B")
        + os.pathsep
        + str(Path(__file__).resolve().parents[1])
    )
    raw = subprocess.check_output(
        [sys.executable, "-c", CHILD],
        env=env,
        text=True,
    )
    return json.loads(raw)


def main() -> None:
    first = run_child("1")
    second = run_child("2")

    for key in (
        "lineage_members",
        "semantic_state_digest",
        "target_digest",
        "checkpoint_sha256",
        "checkpoint_bytes_hex",
    ):
        assert first[key] == second[key], (key, first[key], second[key])

    assert first["lineage_members"] == sorted(first["lineage_members"])
    assert set(first["lineage_members"]) == {"prim:a", "prim:b"}

    print("PASS: repaired PR110 B is hash-seed deterministic")
    print(json.dumps({
        "lineage_members": first["lineage_members"],
        "semantic_state_digest": first["semantic_state_digest"],
        "target_digest": first["target_digest"],
        "checkpoint_sha256": first["checkpoint_sha256"],
    }, sort_keys=True))


if __name__ == "__main__":
    main()
