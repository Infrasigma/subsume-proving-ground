#!/usr/bin/env python3
"""PR110 Phase4 forensic: prove the B determinism repair is scientifically neutral."""

from __future__ import annotations

import hashlib
import json
import os
import random
import shutil
import subprocess
from collections import Counter
from pathlib import Path

SEED = 2026100305
PROVING_SHA = "33259fc5c69d87b99c604f0050508c433a279daf"
OLD_B_RUNTIME = "af0c7ef503c51a5bc7872717b263a01f495c855f"
REPAIRED_B_RUNTIME = "9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be"
OLD_B_BASE = "75d101eabd4be80ce0d7aed2d3cc04e0912925b8"
BOUNDARIES = [0, 1, 3, 5, 6]
UNORDERED_LINEAGE_KEYS = {
    "selected_structural_lineage_ids",
    "selected_executable_lineage_ids",
}


def run_python(code: str, *, root: Path, env: dict[str, str], stdout_path: Path) -> None:
    merged = os.environ.copy()
    merged.update(env)
    merged["PYTHONHASHSEED"] = "0"
    merged["PYTHONPATH"] = f"{root}:{Path.cwd()}"
    proc = subprocess.run(
        ["python3", "-u", "-c", code],
        env=merged,
        text=True,
        stdout=stdout_path.open("w", encoding="utf-8"),
        stderr=subprocess.STDOUT,
        check=False,
    )
    if proc.returncode != 0:
        raise RuntimeError(f"subprocess failed rc={proc.returncode}; see {stdout_path}")


def write_gen0(root: Path, path: Path, runtime_sha: str) -> None:
    code = r'''
import os, random
from pathlib import Path
from cognitive_core.open_ended_growth import OpenEndedRecursiveCognitiveCompiler
from lab.exact_checkpoint import save_checkpoint

seed=int(os.environ["SEED"])
runtime=os.environ["RUNTIME_SHA"]
path=Path(os.environ["CHECKPOINT_PATH"])
learner=OpenEndedRecursiveCognitiveCompiler(max_depth=2, population=24, seed=seed)
save_checkpoint(
    path,
    seed=seed,
    generation_next=0,
    initial_depth=2,
    local_rng=random.Random(seed),
    learner=learner,
    retained=[],
    generations_out=[],
    metadata={"proving_sha":os.environ["PROVING_SHA"],"acsie_ref":runtime},
)
print(path)
'''
    run_python(
        code,
        root=root,
        env={
            "SEED": str(SEED),
            "RUNTIME_SHA": runtime_sha,
            "PROVING_SHA": PROVING_SHA,
            "CHECKPOINT_PATH": str(path),
        },
        stdout_path=path.with_suffix(".stdout.txt"),
    )


def run_to_boundary(root: Path, runtime_sha: str, path: Path, stop_after: int) -> None:
    code = r'''
import json, os
from pathlib import Path
from lab.acsie_open_ended_self_extending_proving import run_seed

seed=int(os.environ["SEED"])
runtime=os.environ["RUNTIME_SHA"]
stop_after=int(os.environ["STOP_AFTER"])
path=Path(os.environ["CHECKPOINT_PATH"])

result=run_seed(
    seed,
    6,
    checkpoint_path=str(path),
    resume_from=None if stop_after == 1 else str(path),
    stop_after=stop_after,
    checkpoint_metadata={"proving_sha":os.environ["PROVING_SHA"],"acsie_ref":runtime},
)
envelope=json.loads(path.read_text())
print(json.dumps({
    "stop_after":stop_after,
    "completed_generations":result.get("completed_generations"),
    "generation_next":envelope.get("generation_next"),
    "state_digest":envelope.get("state_digest"),
}, sort_keys=True))
if envelope.get("generation_next") != stop_after:
    raise SystemExit(f"GENERATION_BOUNDARY_MISMATCH {envelope.get('generation_next')} != {stop_after}")
'''
    run_python(
        code,
        root=root,
        env={
            "SEED": str(SEED),
            "RUNTIME_SHA": runtime_sha,
            "PROVING_SHA": PROVING_SHA,
            "CHECKPOINT_PATH": str(path),
            "STOP_AFTER": str(stop_after),
        },
        stdout_path=path.with_suffix(".stdout.txt"),
    )


def sem_signature(value, key=None):
    if isinstance(value, dict):
        out = {}
        for k, v in sorted(value.items()):
            if k in {"state_digest", "acsie_ref"}:
                continue
            out[k] = sem_signature(v, k)
        return out
    if isinstance(value, list):
        vals = [sem_signature(v) for v in value]
        if key in UNORDERED_LINEAGE_KEYS:
            return sorted(vals, key=lambda x: json.dumps(x, sort_keys=True, separators=(",", ":")))
        return vals
    return value


def load_checkpoint(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def process_identity_multiset(d: dict) -> Counter:
    return Counter(
        json.dumps(sem_signature(v), sort_keys=True, separators=(",", ":"))
        for v in d["learner_state"]["processes"].values()
    )


def process_content_multiset(d: dict) -> Counter:
    out = []
    for v in d["learner_state"]["processes"].values():
        s = sem_signature(v)
        out.append({k: x for k, x in s.items() if k != "process_id"})
    return Counter(json.dumps(x, sort_keys=True, separators=(",", ":")) for x in out)


def frontier_content_multiset(d: dict) -> Counter:
    ps = d["learner_state"]["processes"]
    return Counter(
        json.dumps(sem_signature(ps[k]), sort_keys=True, separators=(",", ":"))
        for k in d["learner_state"]["process_frontier"]
        if k in ps
    )


def compare(old: dict, new: dict, old_path: Path, new_path: Path, generation: int) -> dict:
    return {
        "generation": generation,
        "old_checkpoint_sha256": hashlib.sha256(old_path.read_bytes()).hexdigest(),
        "repaired_checkpoint_sha256": hashlib.sha256(new_path.read_bytes()).hexdigest(),
        "old_state_digest": old.get("state_digest"),
        "repaired_state_digest": new.get("state_digest"),
        "generation_next_equal": old.get("generation_next") == new.get("generation_next"),
        "seed_equal": old.get("seed") == new.get("seed"),
        "schema_equal": old.get("schema") == new.get("schema"),
        "candidate_identity_multiset_equal": process_identity_multiset(old) == process_identity_multiset(new),
        "candidate_content_multiset_equal": process_content_multiset(old) == process_content_multiset(new),
        "frontier_ids_equal": old["learner_state"]["process_frontier"] == new["learner_state"]["process_frontier"],
        "frontier_content_multiset_equal": frontier_content_multiset(old) == frontier_content_multiset(new),
        "retained_equal": sem_signature(old["retained"]) == sem_signature(new["retained"]),
        "primitives_equal": sem_signature(old["learner_state"]["primitives"]) == sem_signature(new["learner_state"]["primitives"]),
        "generation_results_equal": sem_signature(old["generations_out"]) == sem_signature(new["generations_out"]),
        "events_equal": sem_signature(old["learner_state"]["events"]) == sem_signature(new["learner_state"]["events"]),
        "local_rng_state_equal": old["local_rng_state"] == new["local_rng_state"],
        "first_generation_target_digests_equal": [
            x.get("target_digest") for x in old.get("generations_out", [])
        ] == [
            x.get("target_digest") for x in new.get("generations_out", [])
        ],
    }


def main() -> int:
    evidence = Path("evidence/phase4-neutrality")
    shutil.rmtree(evidence, ignore_errors=True)
    (evidence / "old").mkdir(parents=True)
    (evidence / "repaired").mkdir(parents=True)

    subprocess.run(
        ["git", "archive", OLD_B_BASE, "research/pr110_acsie_snapshot/B"],
        check=True,
        stdout=open(evidence / "old_snapshot.tar", "wb"),
    )
    shutil.unpack_archive(str(evidence / "old_snapshot.tar"), str(evidence / "old"))
    # git archive preserves the path prefix; locate the snapshot root.
    candidates = list((evidence / "old").rglob("behavioral_search.py"))
    if len(candidates) != 1:
        raise RuntimeError(f"old snapshot discovery failed: {candidates}")
    old_root = candidates[0].parents[1]
    repaired_root = Path("research/pr110_acsie_snapshot/B").resolve()
    (evidence / "provenance.json").write_text(json.dumps({
        "seed": SEED,
        "proving_sha": PROVING_SHA,
        "old_runtime": OLD_B_RUNTIME,
        "repaired_runtime": REPAIRED_B_RUNTIME,
        "old_b_parent": OLD_B_BASE,
        "pythonhashseed": "0",
        "boundaries": BOUNDARIES,
    }, indent=2, sort_keys=True) + "
", encoding="utf-8")

    for label, root, runtime_sha in [
        ("old", old_root, OLD_B_RUNTIME),
        ("repaired", repaired_root, REPAIRED_B_RUNTIME),
    ]:
        previous = evidence / label / f"gen0.json"
        write_gen0(root, previous, runtime_sha)
        for generation in [1, 3, 5, 6]:
            current = evidence / label / f"gen{generation}.json"
            if generation != 1:
                shutil.copy2(previous, current)
            run_to_boundary(root, runtime_sha, current, generation)
            previous = current

    rows = []
    for generation in BOUNDARIES:
        old_path = evidence / "old" / f"gen{generation}.json"
        new_path = evidence / "repaired" / f"gen{generation}.json"
        rows.append(compare(load_checkpoint(old_path), load_checkpoint(new_path), old_path, new_path, generation))

    required = [
        "generation_next_equal",
        "seed_equal",
        "schema_equal",
        "candidate_identity_multiset_equal",
        "candidate_content_multiset_equal",
        "frontier_ids_equal",
        "frontier_content_multiset_equal",
        "retained_equal",
        "primitives_equal",
        "generation_results_equal",
        "events_equal",
        "local_rng_state_equal",
        "first_generation_target_digests_equal",
    ]
    for row in rows:
        row["scientifically_neutral_at_boundary"] = all(row[k] for k in required)

    report = {
        "protocol": "ACSIE.PR110.phase4.scientific-neutrality.v2",
        "seed": SEED,
        "proving_sha": PROVING_SHA,
        "old_runtime": OLD_B_RUNTIME,
        "repaired_runtime": REPAIRED_B_RUNTIME,
        "old_b_parent": OLD_B_BASE,
        "hash_seed": "0",
        "boundaries": rows,
        "scientific_neutrality": (
            "PASSED" if all(r["scientifically_neutral_at_boundary"] for r in rows)
            else "NOT_SCIENTIFICALLY_NEUTRAL"
        ),
    }
    report_path = evidence / "phase4-neutrality-report.json"
    report_path.write_text(json.dumps(report, indent=2, sort_keys=True) + "
", encoding="utf-8")
    print(json.dumps(report, indent=2, sort_keys=True))
    return 0 if report["scientific_neutrality"] == "PASSED" else 20


if __name__ == "__main__":
    raise SystemExit(main())
