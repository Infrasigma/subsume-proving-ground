"""Frozen evaluator for the H4 concept-language induction kill-test."""

from __future__ import annotations

import os
import random
import sys
from dataclasses import replace
from typing import Iterable, Sequence

from h4_concept_engine import (
    ATOMS,
    PATTERNS,
    Concept,
    ConceptLibrary,
    Episode,
    Obj,
    accuracy,
    fresh_composed_search,
    fresh_learn,
    matches,
)


SHAPES = ("a", "b", "c")
COLORS = ("r", "g", "b")
GRID = tuple((x, y) for x in range(5) for y in range(5))


def scene(rng: random.Random, n: int = 8) -> tuple[Obj, ...]:
    cells = rng.sample(GRID, n)
    out = []
    for i, (x, y) in enumerate(cells):
        out.append(Obj(
            shape=rng.choice(SHAPES),
            color=rng.choice(COLORS),
            x=x,
            y=y,
            token=f"t{i}-{rng.randrange(100000)}",
        ))
    return tuple(out)


def dataset_for_pattern(
    pattern: Sequence[str],
    seed: int,
    n: int = 80,
) -> tuple[Episode, ...]:
    rng = random.Random(seed)
    positives: list[Episode] = []
    negatives: list[Episode] = []
    attempts = 0
    while (len(positives) < n // 2 or len(negatives) < n // 2) and attempts < n * 800:
        attempts += 1
        objs = scene(rng)
        label = int(matches(objs, pattern))
        ep = Episode(objs, label)
        (positives if label else negatives).append(ep)
    if len(positives) < n // 2 or len(negatives) < n // 2:
        raise RuntimeError(f"unable to balance pattern {pattern}: p={len(positives)} n={len(negatives)}")
    return tuple(positives[: n // 2] + negatives[: n // 2])


def random_prevalence(pattern: Sequence[str], seed: int, n: int = 500) -> float:
    rng = random.Random(seed)
    return sum(matches(scene(rng), pattern) for _ in range(n)) / n


def choose_patterns(seed: int, count: int = 4) -> list[tuple[str, ...]]:
    rng = random.Random(seed)
    shuffled = list(PATTERNS)
    rng.shuffle(shuffled)
    selected: list[tuple[str, ...]] = []
    shared = [scene(rng) for _ in range(350)]
    for pattern in shuffled:
        if len(pattern) != 4:
            continue
        prevalence = sum(matches(s, pattern) for s in shared) / len(shared)
        if not 0.08 <= prevalence <= 0.55:
            continue
        probe = []
        for s in shared[:140]:
            label = int(matches(s, pattern))
            probe.append(Episode(s, label))
        held = shared[140:280]
        held_ds = [Episode(s, int(matches(s, pattern))) for s in held]
        short_equivalent = False
        import itertools
        for k in (1, 2):
            for sub in itertools.combinations(pattern, k):
                if accuracy(probe, sub) >= 0.90 and accuracy(held_ds, sub) >= 0.90:
                    short_equivalent = True
                    break
            if short_equivalent:
                break
        if short_equivalent:
            continue
        selected.append(pattern)
        if len(selected) == count:
            return selected
    raise RuntimeError(f"could not select {count} admissible hidden concepts")


def rename_surface(
    audit: Sequence[Episode],
    seed: int,
    pattern: Sequence[str],
) -> tuple[Episode, ...]:
    rng = random.Random(seed)
    shape_perm = list(SHAPES)
    color_perm = list(COLORS)
    rng.shuffle(shape_perm)
    rng.shuffle(color_perm)
    smap = dict(zip(SHAPES, shape_perm))
    cmap = dict(zip(COLORS, color_perm))
    out: list[Episode] = []
    for ep in audit:
        objs = [
            Obj(
                shape=smap[o.shape],
                color=cmap[o.color],
                x=o.x,
                y=o.y,
                token=f"renamed-{rng.randrange(10**9)}",
            )
            for o in ep.objects
        ]
        rng.shuffle(objs)
        transformed = tuple(objs)
        # Keep only semantics-preserving distractor injections.
        if matches(transformed, pattern) != bool(ep.label):
            out.append(Episode(transformed, int(matches(transformed, pattern))))
            continue
        remaining = [c for c in GRID if c not in {(o.x, o.y) for o in transformed}]
        rng.shuffle(remaining)
        for x, y in remaining[:2]:
            transformed = transformed + (
                Obj(
                    shape=rng.choice(SHAPES),
                    color=rng.choice(COLORS),
                    x=x,
                    y=y,
                    token=f"d-{rng.randrange(10**9)}",
                ),
            )
            if matches(transformed, pattern) == bool(ep.label):
                break
            transformed = transformed[:-1]
        out.append(Episode(tuple(transformed), ep.label))
    return tuple(out)


def compose_dataset(
    a: Sequence[str],
    b: Sequence[str],
    seed: int,
    n: int = 100,
    op: str = "or",
) -> tuple[Episode, ...]:
    rng = random.Random(seed)
    pos: list[Episode] = []
    neg: list[Episode] = []
    attempts = 0
    while (len(pos) < n // 2 or len(neg) < n // 2) and attempts < n * 1000:
        attempts += 1
        objs = scene(rng)
        pa, pb = matches(objs, a), matches(objs, b)
        if op == "and":
            label = int(pa and pb)
        elif op == "xor":
            label = int(bool(pa) ^ bool(pb))
        else:
            label = int(pa or pb)
        ep = Episode(objs, label)
        (pos if label else neg).append(ep)
    if len(pos) < n // 2 or len(neg) < n // 2:
        raise RuntimeError(f"unable to balance composed task {op}")
    return tuple(pos[: n // 2] + neg[: n // 2])


def cost_ratio(k1: int, k0: int) -> float:
    return k1 / max(k0, 1)


def run_block(seed: int) -> dict:
    patterns = choose_patterns(seed)
    lib = ConceptLibrary()
    learned: list[Concept] = []
    library_snapshots: list[ConceptLibrary] = []
    invention_ok = True
    validity_ok = True
    acquisition_records = []

    for i, pattern in enumerate(patterns[:3]):
        train = dataset_for_pattern(pattern, seed + 1000 + i * 17, 80)
        heldout = dataset_for_pattern(pattern, seed + 2000 + i * 19, 80)
        concept, evals = lib.discover(train, heldout)
        if concept is None:
            invention_ok = False
            validity_ok = False
            acquisition_records.append({"index": i, "found": False, "evals": evals})
            continue
        short_equal = False
        import itertools
        for k in (1, 2):
            for sub in itertools.combinations(concept.atoms, k):
                if accuracy(heldout, sub) >= 0.90:
                    short_equal = True
        if concept.complexity < 3 or short_equal:
            invention_ok = False
        if concept.heldout_accuracy < 0.90:
            validity_ok = False
        learned.append(concept)
        library_snapshots.append(ConceptLibrary.loads(lib.dumps()))
        acquisition_records.append({
            "index": i,
            "found": True,
            "digest": concept.digest,
            "atoms": list(concept.atoms),
            "heldout_accuracy": concept.heldout_accuracy,
            "complexity": concept.complexity,
            "search_evals": evals,
        })

    if len(learned) < 3:
        return {
            "patterns": [list(p) for p in patterns],
            "invention_ok": invention_ok,
            "validity_ok": validity_ok,
            "acquisition": acquisition_records,
            "verdict": "KILLED",
            "reason": "fewer than three admissible concepts discovered",
        }

    ratios = []
    transfer_records = []
    for i, concept in enumerate(learned):
        support = dataset_for_pattern(concept.atoms, seed + 3000 + i * 23, 60)
        audit = dataset_for_pattern(concept.atoms, seed + 4000 + i * 29, 120)
        transformed = rename_surface(audit, seed + 5000 + i * 31, concept.atoms)

        fresh, k0 = fresh_learn(support, audit)
        k1_concept, k1_acc, k1 = lib.query(support, audit)
        _, k1_surface, _ = lib.query(support, transformed)
        ratio = cost_ratio(k1 + 20 * len(lib.concepts), k0)
        ratios.append(ratio)
        transfer_records.append({
            "digest": concept.digest,
            "fresh_accuracy": None if fresh is None else fresh.heldout_accuracy,
            "k1_accuracy": k1_acc,
            "surface_accuracy": k1_surface,
            "ratio": ratio,
            "k0_cost": k0,
            "k1_cost": k1 + 20 * len(lib.concepts),
        })

    transfer_ok = all(
        r["k1_accuracy"] >= 0.90 and
        r["surface_accuracy"] >= 0.80 and
        r["surface_accuracy"] / max(r["k1_accuracy"], 1e-9) >= 0.80 and
        r["ratio"] <= 0.50
        for r in transfer_records
    )

    # Three-step compounding: after each retained concept, evaluate a fresh
    # surface of that same concept with a growing library.
    compounding = []
    for i, concept in enumerate(learned):
        support = dataset_for_pattern(concept.atoms, seed + 6000 + i * 41, 60)
        audit = dataset_for_pattern(concept.atoms, seed + 7000 + i * 43, 120)
        _, k0 = fresh_learn(support, audit)
        prefix_lib = library_snapshots[i]
        _, acc, k1 = prefix_lib.query(support, audit)
        ratio = cost_ratio(k1 + 20 * len(prefix_lib.concepts), k0)
        compounding.append({"step": i + 1, "ratio": ratio, "accuracy": acc})

    recursive_ok = all(x["ratio"] < 0.75 and x["accuracy"] >= 0.90 for x in compounding[:3])

    # Composition uses two already learned concepts. The evaluator gives only
    # the final binary outcome, not the component identities.
    a, b = learned[0].atoms, learned[1].atoms
    ctrain = compose_dataset(a, b, seed + 8000, 80, op="or")
    caudit = compose_dataset(a, b, seed + 9000, 140, op="or")
    k0_formula, k0_acc, k0_comp = fresh_composed_search(ctrain, caudit)
    k1_formula, k1_acc, k1_comp = lib.query_composition(ctrain, caudit)
    comp_ratio = cost_ratio(k1_comp + 20 * len(lib.concepts), k0_comp)
    composition_ok = k1_acc >= 0.90 and comp_ratio <= 0.75

    # Deletion must materially erase the learned advantage; rehydration restores it.
    d_support = dataset_for_pattern(learned[2].atoms, seed + 10000, 60)
    d_audit = dataset_for_pattern(learned[2].atoms, seed + 11000, 120)
    _, k0_del = fresh_learn(d_support, d_audit)
    _, acc_before, k_before = lib.query(d_support, d_audit)
    advantage_before = max(0.0, 1.0 - cost_ratio(k_before + 20 * len(lib.concepts), k0_del))
    serialized = lib.dumps()
    lib.remove_all()
    _, acc_deleted, k_deleted = lib.query(d_support, d_audit)
    deleted_advantage = 0.0
    if k_deleted == 0 or acc_deleted < 0.90:
        deleted_advantage = 0.0
    lib2 = ConceptLibrary.loads(serialized)
    _, acc_after, k_after = lib2.query(d_support, d_audit)
    advantage_after = max(0.0, 1.0 - cost_ratio(k_after + 20 * len(lib2.concepts), k0_del))
    deletion_ok = (
        advantage_before > 0.0 and
        deleted_advantage <= advantage_before * 0.20 and
        advantage_after >= advantage_before * 0.90 and
        acc_before >= 0.90 and
        acc_after >= 0.90
    )

    # Negative control: destroy labels before discovery. It must not acquire a
    # behaviorally valid concept and is not allowed to claim transfer savings.
    rng = random.Random(seed + 12000)
    neg_train = list(dataset_for_pattern(learned[0].atoms, seed + 13000, 80))
    labels = [ep.label for ep in neg_train]
    rng.shuffle(labels)
    neg_train = [replace(ep, label=labels[i]) for i, ep in enumerate(neg_train)]
    neg_lib = ConceptLibrary()
    neg_concept, neg_cost = neg_lib.discover(
        neg_train,
        dataset_for_pattern(learned[0].atoms, seed + 14000, 80),
        min_accuracy=0.90,
    )
    if neg_concept is None:
        neg_acc = 0.0
        neg_savings = 0.0
    else:
        neg_audit = dataset_for_pattern(learned[0].atoms, seed + 15000, 120)
        neg_acc = accuracy(neg_audit, neg_concept.atoms)
        neg_savings = 0.0 if neg_acc < 0.90 else 1.0 - cost_ratio(
            len(neg_audit) + 20,
            fresh_learn(
                dataset_for_pattern(learned[0].atoms, seed + 15000, 60),
                neg_audit,
            )[1],
        )
    negative_ok = neg_savings < 0.05 and neg_acc < 0.90

    all_ok = (
        invention_ok
        and validity_ok
        and transfer_ok
        and recursive_ok
        and composition_ok
        and deletion_ok
        and negative_ok
    )
    return {
        "patterns": [list(p) for p in patterns],
        "acquisition": acquisition_records,
        "invention_ok": invention_ok,
        "validity_ok": validity_ok,
        "transfer_ok": transfer_ok,
        "transfer": transfer_records,
        "recursive_ok": recursive_ok,
        "recursive": compounding,
        "composition_ok": composition_ok,
        "composition": {
            "k0_accuracy": k0_acc,
            "k1_accuracy": k1_acc,
            "k0_cost": k0_comp,
            "k1_cost": k1_comp + 20 * len(lib.concepts),
            "ratio": comp_ratio,
            "k0_formula_found": k0_formula is not None,
            "k1_formula_found": k1_formula is not None,
        },
        "deletion_ok": deletion_ok,
        "deletion": {
            "accuracy_before": acc_before,
            "accuracy_deleted": acc_deleted,
            "accuracy_after": acc_after,
            "advantage_before": advantage_before,
            "advantage_deleted": deleted_advantage,
            "advantage_after": advantage_after,
        },
        "negative_ok": negative_ok,
        "negative_control": {
            "found": neg_concept is not None,
            "audit_accuracy": neg_acc,
            "claimed_savings": neg_savings,
        },
        "verdict": "PASS" if all_ok else "KILLED",
        "reason": "all frozen H4 gates satisfied" if all_ok else "at least one frozen H4 gate failed",
    }


def main() -> int:
    seeds = [
        int(os.environ.get("H4_SEED_A", "431171")),
        int(os.environ.get("H4_SEED_B", "928437")),
    ]
    results = {}
    for seed in seeds:
        results[str(seed)] = run_block(seed)

    killed = [seed for seed, result in results.items() if result["verdict"] != "PASS"]
    verdict = "KILLED" if killed else "PASS"

    print("H4_RESULT")
    print("==========")
    for seed, result in results.items():
        print(f"SEED {seed}: {result['verdict']}")
        print(f"reason={result['reason']}")
        print(f"invention_ok={result.get('invention_ok')}")
        print(f"validity_ok={result.get('validity_ok')}")
        print(f"transfer_ok={result.get('transfer_ok')}")
        print(f"recursive_ok={result.get('recursive_ok')}")
        print(f"composition_ok={result.get('composition_ok')}")
        print(f"deletion_ok={result.get('deletion_ok')}")
        print(f"negative_ok={result.get('negative_ok')}")
        print(f"details={result}")
    print(f"H4 VERDICT: {verdict}")
    return 0 if verdict == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
