#!/usr/bin/env python3
from __future__ import annotations

import argparse
import itertools
import json
import random
import statistics
from dataclasses import dataclass
from hashlib import sha256
from typing import Callable

from cognitive_core.open_ended_operator_algebra import (
    OpenEndedOperatorAlgebra,
    SequenceOperator,
)


OBS = ("a", "b", "c", "d")
OUT = ("u", "v", "w")
START = "<START>"
DEPTH = 3


@dataclass(frozen=True)
class HiddenRouter:
    table: dict[tuple[str, ...], str]
    depth: int = DEPTH

    def apply(self, sequence: tuple[str, ...]) -> tuple[str, ...]:
        prefix = [START] * (self.depth - 1)
        prefix.extend(sequence[:0])
        out = []
        for i in range(len(sequence)):
            history = prefix + list(sequence[:i])
            out.append(self.table[tuple(history[-self.depth:])])
        return tuple(out)


@dataclass(frozen=True)
class HiddenComposite:
    parent: "HiddenProgram"
    child: HiddenRouter

    def apply(self, sequence: tuple[str, ...]) -> tuple[str, ...]:
        return self.child.apply(self.parent.apply(sequence))


HiddenProgram = HiddenRouter | HiddenComposite


def stable_seed(seed: int, *parts: object) -> int:
    raw = "|".join(map(str, (seed,) + parts)).encode()
    return int.from_bytes(sha256(raw).digest()[:8], "big")


def make_router(
    seed: int,
    generation: int,
    salt: int,
    alphabet: tuple[str, ...],
) -> HiddenRouter:
    rng = random.Random(stable_seed(seed, generation, salt, "router"))
    table = {}
    for pad in range(DEPTH - 1, -1, -1):
        suffix_len = DEPTH - pad
        for suffix in itertools.product(alphabet, repeat=suffix_len):
            key = (START,) * pad + tuple(suffix)
            table[key] = rng.choice(OUT)
    return HiddenRouter(table)


def sample_sequence(seed: int, generation: int, split: str, idx: int, length: int = 96) -> tuple[str, ...]:
    rng = random.Random(stable_seed(seed, generation, split, idx))
    return tuple(rng.choice(OBS) for _ in range(length))


def make_dataset(
    target: HiddenProgram,
    seed: int,
    generation: int,
    split: str,
    count: int,
) -> tuple[tuple[tuple[str, ...], tuple[str, ...]], ...]:
    rows = []
    for i in range(count):
        source = sample_sequence(seed, generation, split, i)
        target_out = target.apply(source)
        rows.append((source, target_out))
    return tuple(rows)


def operator_apply(
    op: SequenceOperator,
    source: tuple[str, ...],
    algebra: OpenEndedOperatorAlgebra,
) -> tuple[str, ...]:
    return op.apply(source, algebra.operators)


def evaluate_accuracy(
    op: SequenceOperator,
    rows: tuple[tuple[tuple[str, ...], tuple[str, ...]], ...],
    algebra: OpenEndedOperatorAlgebra,
) -> float:
    if not rows:
        return 1.0
    good = 0
    total = 0
    for source, target in rows:
        pred = operator_apply(op, source, algebra)
        good += sum(a == b for a, b in zip(pred, target))
        total += len(target)
    return good / max(1, total)


def run_seed(seed: int, generations: int) -> dict:
    active = OpenEndedOperatorAlgebra(seed)
    baseline = OpenEndedOperatorAlgebra(seed + 10007)

    hidden_retained: list[HiddenProgram] = []
    active_records = []
    baseline_records = []
    parent_indices: list[int] = []

    for generation in range(generations):
        if generation == 0:
            target: HiddenProgram = make_router(seed, generation, 0, OBS)
            parent_index = None
        else:
            parent_index = random.Random(
                stable_seed(seed, generation, "parent-selection")
            ).randrange(len(hidden_retained))
            parent = hidden_retained[parent_index]
            target = HiddenComposite(
                parent=parent,
                child=make_router(seed, generation, generation, OUT),
            )
            parent_indices.append(parent_index)

        train = make_dataset(target, seed, generation, "train", 10)
        holdout = make_dataset(target, seed, generation, "holdout", 6)
        transfer = make_dataset(target, seed, generation, "transfer", 6)

        active.generation = generation
        active_op = active.discover(train, holdout, transfer)
        active_ok = False
        if active_op is not None:
            active_train = evaluate_accuracy(active_op, train, active)
            active_hold = evaluate_accuracy(active_op, holdout, active)
            active_transfer = evaluate_accuracy(active_op, transfer, active)
            active_ok = (
                active_train >= 1.0
                and active_hold >= 1.0
                and active_transfer >= 1.0
            )

        baseline.generation = 0
        base_op = baseline.discover(train, holdout, transfer)
        baseline_ok = False
        if base_op is not None:
            base_train = evaluate_accuracy(base_op, train, baseline)
            base_hold = evaluate_accuracy(base_op, holdout, baseline)
            base_transfer = evaluate_accuracy(base_op, transfer, baseline)
            baseline_ok = (
                base_train >= 1.0
                and base_hold >= 1.0
                and base_transfer >= 1.0
            )

        if active_ok and active_op is not None:
            hidden_retained.append(active_op)

        active_records.append(
            {
                "generation": generation,
                "active_ok": active_ok,
                "baseline_ok": baseline_ok,
                "active_kind": active_op.kind if active_op else None,
                "active_receptive_field": active_op.receptive_field if active_op else 0,
                "active_parent_count": len(active_op.parent_operator_ids) if active_op else 0,
                "retained_count": len(hidden_retained),
                "target_generation_parent_index": parent_index,
            }
        )
        baseline_records.append(
            {
                "generation": generation,
                "baseline_ok": baseline_ok,
                "baseline_receptive_field": base_op.receptive_field if base_op else 0,
            }
        )

    retained_ok = all(r["active_ok"] for r in active_records)
    later_active_ok = all(r["active_ok"] for r in active_records[1:])
    later_baseline_fail = all(not r["baseline_ok"] for r in baseline_records[1:])
    fields = [r["active_receptive_field"] for r in active_records]
    strictly_growing_fields = all(
        b > a for a, b in zip(fields, fields[1:])
    )
    parent_reuse = len(set(parent_indices)) if parent_indices else 0
    branching = parent_reuse >= max(2, generations // 2)

    finite_gate = (
        len(active_records) == generations
        and retained_ok
        and later_active_ok
        and later_baseline_fail
        and strictly_growing_fields
        and active_records[-1]["retained_count"] >= generations
        and active_records[-1]["active_receptive_field"] >= 3 + 2 * (generations - 1)
        and branching
    )

    return {
        "seed": seed,
        "generations": generations,
        "active_records": active_records,
        "baseline_records": baseline_records,
        "final_receptive_field": active_records[-1]["active_receptive_field"],
        "retained_count": active_records[-1]["retained_count"],
        "strict_receptive_field_growth": strictly_growing_fields,
        "unique_parent_reuse_count": parent_reuse,
        "branching_parent_reuse": branching,
        "later_baseline_failure": later_baseline_fail,
        "finite_open_ended_operator_gate": finite_gate,
        "external_model": False,
        "target_identity_available_to_runtime": False,
        "task_family_route": False,
    }


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument(
        "--seeds",
        default="2026100301,2026100302,2026100303,2026100304,2026100305",
    )
    p.add_argument("--generations", type=int, default=7)
    p.add_argument("--strict", action="store_true")
    args = p.parse_args()

    rows = [
        run_seed(int(seed), args.generations)
        for seed in args.seeds.split(",")
        if seed.strip()
    ]
    for row in rows:
        print(json.dumps(row, sort_keys=True))

    gate = (
        len(rows) == 5
        and all(r["finite_open_ended_operator_gate"] for r in rows)
        and all(not r["external_model"] for r in rows)
        and all(not r["target_identity_available_to_runtime"] for r in rows)
        and all(not r["task_family_route"] for r in rows)
    )
    print("OPEN_ENDED_OPERATOR_ALGEBRA_GATE=" + ("PASS" if gate else "FAIL"))
    return 0 if (gate or not args.strict) else 2


if __name__ == "__main__":
    raise SystemExit(main())
