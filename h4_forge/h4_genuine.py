from __future__ import annotations

import hashlib
import itertools
import json
import os
import random
from dataclasses import dataclass, replace
from typing import Iterable, Sequence

# Generic relational vocabulary shared by all surfaces.
ATOMS = (
    "same_type",
    "same_aux",
    "neighbor",
    "ordered",
    "distance2",
    "aligned",
    "connected2",
)

PATTERNS = tuple(
    itertools.combinations(ATOMS, k)
    for k in ()
)
PATTERNS = tuple(p for k in (1, 2, 3, 4) for p in itertools.combinations(ATOMS, k))
LOW_PATTERNS = tuple(p for p in PATTERNS if len(p) <= 2)
COMPLEX_PATTERNS = tuple(p for p in PATTERNS if len(p) >= 3)


@dataclass(frozen=True)
class Pair:
    rels: frozenset[str]


@dataclass(frozen=True)
class Episode:
    pairs: tuple[Pair, ...]
    label: int


@dataclass(frozen=True)
class Concept:
    digest: str
    atoms: tuple[str, ...]
    train_accuracy: float
    heldout_accuracy: float

    @property
    def complexity(self) -> int:
        return len(self.atoms)


def digest(atoms: Sequence[str]) -> str:
    return hashlib.sha256("|".join(atoms).encode()).hexdigest()[:16]


def predicts(ep: Episode, atoms: Sequence[str]) -> int:
    need = set(atoms)
    return int(any(need.issubset(pair.rels) for pair in ep.pairs))


def accuracy(data: Sequence[Episode], atoms: Sequence[str]) -> float:
    if not data:
        return 0.0
    return sum(predicts(ep, atoms) == ep.label for ep in data) / len(data)


def search(data: Sequence[Episode], holdout: Sequence[Episode], minimum: float = 0.90) -> tuple[Concept | None, int]:
    best = None
    cost = 0
    for atoms in COMPLEX_PATTERNS:
        cost += len(data)
        train_acc = accuracy(data, atoms)
        if train_acc < minimum:
            continue
        hold_acc = accuracy(holdout, atoms)
        if hold_acc < minimum:
            continue
        cand = Concept(digest(atoms), tuple(atoms), train_acc, hold_acc)
        if best is None or (cand.heldout_accuracy, -cand.complexity, cand.digest) > (
            best.heldout_accuracy, -best.complexity, best.digest
        ):
            best = cand
    return best, cost + len(holdout) * len(COMPLEX_PATTERNS)


def fresh_all_search(data: Sequence[Episode], holdout: Sequence[Episode], minimum: float = 0.90) -> tuple[Concept | None, int]:
    best = None
    cost = 0
    for atoms in PATTERNS:
        cost += len(data)
        train_acc = accuracy(data, atoms)
        if train_acc < minimum:
            continue
        hold_acc = accuracy(holdout, atoms)
        if hold_acc < minimum:
            continue
        cand = Concept(digest(atoms), tuple(atoms), train_acc, hold_acc)
        if best is None or (cand.heldout_accuracy, -cand.complexity, cand.digest) > (
            best.heldout_accuracy, -best.complexity, best.digest
        ):
            best = cand
    return best, cost


def exhaustive_low_cost(data: Sequence[Episode], holdout: Sequence[Episode]) -> tuple[float, float]:
    best_train = max(accuracy(data, p) for p in LOW_PATTERNS)
    best_hold = max(accuracy(holdout, p) for p in LOW_PATTERNS)
    return best_train, best_hold


class Library:
    def __init__(self) -> None:
        self.items: dict[str, Concept] = {}

    @property
    def concepts(self) -> tuple[Concept, ...]:
        return tuple(sorted(self.items.values(), key=lambda c: (c.complexity, c.digest)))

    def add(self, c: Concept) -> None:
        self.items[c.digest] = c

    def dumps(self) -> str:
        return json.dumps(
            [
                {
                    "digest": c.digest,
                    "atoms": list(c.atoms),
                    "train_accuracy": c.train_accuracy,
                    "heldout_accuracy": c.heldout_accuracy,
                }
                for c in self.concepts
            ],
            sort_keys=True,
            separators=(",", ":"),
        )

    @classmethod
    def loads(cls, payload: str) -> "Library":
        out = cls()
        for item in json.loads(payload):
            out.add(
                Concept(
                    item["digest"],
                    tuple(item["atoms"]),
                    float(item["train_accuracy"]),
                    float(item["heldout_accuracy"]),
                )
            )
        return out

    def clear(self) -> None:
        self.items.clear()

    def query(self, support: Sequence[Episode], audit: Sequence[Episode]) -> tuple[Concept | None, float, int]:
        best = None
        best_support = -1.0
        for c in self.concepts:
            a = accuracy(support, c.atoms)
            if a > best_support:
                best, best_support = c, a
        if best is None:
            return None, 0.0, 0
        return best, accuracy(audit, best.atoms), len(support) * len(self.concepts) + len(audit)

    def compose(
        self, support: Sequence[Episode], audit: Sequence[Episode]
    ) -> tuple[tuple[str, str, str] | None, float, int]:
        concepts = self.concepts
        if len(concepts) < 2:
            return None, 0.0, 0
        preds = {c.digest: [predicts(ep, c.atoms) for ep in support] for c in concepts}
        best = None
        best_train = -1.0
        cost = 0
        for a_i, a in enumerate(concepts):
            for b in concepts[a_i + 1 :]:
                for op in ("and", "or", "xor"):
                    vals = []
                    for x, y in zip(preds[a.digest], preds[b.digest]):
                        vals.append(int((x and y) if op == "and" else (x or y) if op == "or" else (bool(x) ^ bool(y))))
                    cost += len(support)
                    acc = sum(v == ep.label for v, ep in zip(vals, support)) / len(support)
                    if acc > best_train:
                        best_train = acc
                        best = (a.digest, b.digest, op)
        if best is None:
            return None, 0.0, cost
        da, db, op = best
        ac = {c.digest: c for c in concepts}
        vals = []
        for ep in audit:
            x = predicts(ep, ac[da].atoms)
            y = predicts(ep, ac[db].atoms)
            vals.append(int((x and y) if op == "and" else (x or y) if op == "or" else (bool(x) ^ bool(y))))
        return best, sum(v == ep.label for v, ep in zip(vals, audit)) / len(audit), cost + len(audit)


def _entity(rng: random.Random, idx: int, n_types: int = 5, n_aux: int = 4) -> tuple[int, int]:
    return rng.randrange(n_types), rng.randrange(n_aux)


def make_surface(kind: str, rng: random.Random, n: int = 9) -> tuple[tuple[int, int, int, int], ...]:
    # Hidden raw surface state. The learner never sees this.
    if kind == "spatial":
        cells = rng.sample([(x, y) for x in range(6) for y in range(6)], n)
        return tuple((i, *cells[i], *_entity(rng, i)) for i in range(n))
    if kind == "sequence":
        return tuple((i, i, 0, *_entity(rng, i)) for i in range(n))
    if kind == "graph":
        return tuple((i, 0, 0, *_entity(rng, i)) for i in range(n))
    raise ValueError(kind)


def encode_surface(kind: str, raw: Sequence[tuple[int, int, int, int]], rng: random.Random) -> tuple[Pair, ...]:
    # The output is only an unordered list of generic relational pair features.
    # Surface-specific geometry is deliberately hidden behind this fixed encoder.
    type_map = list(range(10))
    aux_map = list(range(10))
    rng.shuffle(type_map)
    rng.shuffle(aux_map)

    n = len(raw)
    edges: set[tuple[int, int]] = set()
    if kind == "graph":
        for i in range(n):
            for j in range(i + 1, n):
                if rng.random() < 0.22:
                    edges.add((i, j))
        # ensure each node has at least a chance of connectivity
        for i in range(n - 1):
            if rng.random() < 0.35:
                edges.add((min(i, i + 1), max(i, i + 1)))

    pairs: list[Pair] = []
    for i, j in itertools.combinations(range(n), 2):
        _, xi, yi, ti, ai = raw[i]
        _, xj, yj, tj, aj = raw[j]
        rels: set[str] = set()
        if type_map[ti] == type_map[tj]:
            rels.add("same_type")
        if aux_map[ai] == aux_map[aj]:
            rels.add("same_aux")
        if xi < xj or (xi == xj and yi < yj):
            rels.add("ordered")

        if kind == "spatial":
            man = abs(xi - xj) + abs(yi - yj)
            if man == 1:
                rels.add("neighbor")
            if man == 2:
                rels.add("distance2")
            if xi == xj or yi == yj:
                rels.add("aligned")
            if man == 2:
                rels.add("connected2")
        elif kind == "sequence":
            d = abs(xi - xj)
            if d == 1:
                rels.add("neighbor")
            if d == 2:
                rels.add("distance2")
                rels.add("connected2")
            if ti == tj or ai == aj:
                rels.add("aligned")
        else:
            key = (min(i, j), max(i, j))
            if key in edges:
                rels.add("neighbor")
            common = 0
            for k in range(n):
                if k != i and k != j and ((min(i, k), max(i, k)) in edges) and ((min(j, k), max(j, k)) in edges):
                    common += 1
            if common > 0:
                rels.add("connected2")
                rels.add("distance2")
            if (i % 2) == (j % 2):
                rels.add("aligned")
        pairs.append(Pair(frozenset(rels)))
    rng.shuffle(pairs)
    return tuple(pairs)


def episode(kind: str, pattern: Sequence[str], seed: int, label_override: int | None = None, n: int = 9) -> Episode:
    rng = random.Random(seed)
    raw = make_surface(kind, rng, n)
    pairs = encode_surface(kind, raw, rng)
    label = predicts(Episode(pairs, 0), pattern)
    if label_override is not None:
        label = label_override
    return Episode(pairs, label)


def balanced(kind: str, pattern: Sequence[str], seed: int, count: int = 100) -> tuple[Episode, ...]:
    rng = random.Random(seed)
    pos: list[Episode] = []
    neg: list[Episode] = []
    for attempt in range(count * 2500):
        ep = episode(kind, pattern, rng.randrange(10**12))
        (pos if ep.label else neg).append(ep)
        if len(pos) >= count // 2 and len(neg) >= count // 2:
            break
    if len(pos) < count // 2 or len(neg) < count // 2:
        raise RuntimeError(f"could not balance {kind} {pattern}: {len(pos)}/{len(neg)}")
    return tuple(pos[: count // 2] + neg[: count // 2])


def prevalence(kind: str, pattern: Sequence[str], seed: int, trials: int = 300) -> float:
    rng = random.Random(seed)
    hits = 0
    for _ in range(trials):
        ep = episode(kind, pattern, rng.randrange(10**12))
        hits += ep.label
    return hits / trials


def choose_hidden(seed: int, count: int = 3) -> list[tuple[str, ...]]:
    rng = random.Random(seed)
    candidates = list(COMPLEX_PATTERNS)
    rng.shuffle(candidates)
    chosen: list[tuple[str, ...]] = []
    surfaces = ("spatial", "sequence", "graph")
    for p in candidates:
        if all(0.12 <= prevalence(s, p, seed + i * 101) <= 0.55 for i, s in enumerate(surfaces)):
            source = surfaces[len(chosen) % len(surfaces)]
            src_hold = balanced(source, p, seed + 1000 + len(chosen) * 31, 120)
            if max(exhaustive_low_cost(src_hold, src_hold)) < 0.90:
                chosen.append(p)
        if len(chosen) == count:
            return chosen
    raise RuntimeError("failed to select hidden cross-surface concepts")


def shuffled_control(data: Sequence[Episode], seed: int) -> tuple[Episode, ...]:
    rng = random.Random(seed)
    labels = [ep.label for ep in data]
    rng.shuffle(labels)
    return tuple(replace(ep, label=labels[i]) for i, ep in enumerate(data))


def transfer_dataset(kind: str, pattern: Sequence[str], seed: int, count: int = 140) -> tuple[Episode, ...]:
    return balanced(kind, pattern, seed, count)


def run_block(seed: int) -> dict:
    surfaces = ("spatial", "sequence", "graph")
    hidden = choose_hidden(seed)
    library = Library()
    snapshots: list[Library] = []
    discoveries = []

    # Each concept is learned on one surface and immediately audited there.
    for i, pattern in enumerate(hidden):
        source = surfaces[i % len(surfaces)]
        train = balanced(source, pattern, seed + 2000 + i * 37, 120)
        hold = balanced(source, pattern, seed + 3000 + i * 41, 120)
        c, cost = search(train, hold)
        if c is None:
            return {"verdict": "KILLED", "reason": f"discovery failed at {i}", "hidden": hidden}
        low_train, low_hold = exhaustive_low_cost(train, hold)
        if c.complexity < 3 or low_hold >= 0.90:
            return {"verdict": "KILLED", "reason": f"minimality failed at source {i}", "hidden": hidden}
        if c.heldout_accuracy < 0.90:
            return {"verdict": "KILLED", "reason": f"source validity failed at {i}", "hidden": hidden}
        library.add(c)
        snapshots.append(Library.loads(library.dumps()))
        discoveries.append({
            "source": source,
            "atoms": list(c.atoms),
            "source_accuracy": c.heldout_accuracy,
            "search_cost": cost,
            "low_source_holdout": low_hold,
        })

    if len(library.concepts) < 3:
        return {"verdict": "KILLED", "reason": "discovery produced fewer than 3 unique retained concepts", "hidden": hidden, "discoveries": discoveries}

    # Cross-surface transfer to both surfaces never used for source acquisition.
    transfer = []
    for i, c in enumerate(library.concepts):
        source = surfaces[i % len(surfaces)]
        targets = [s for s in surfaces if s != source]
        for j, target in enumerate(targets):
            support = transfer_dataset(target, c.atoms, seed + 5000 + i * 53 + j * 17, 100)
            audit = transfer_dataset(target, c.atoms, seed + 6000 + i * 59 + j * 19, 140)
            fresh, k0 = fresh_all_search(support, audit)
            _, k1_acc, k1 = library.query(support, audit)
            low_train, low_hold = exhaustive_low_cost(support, audit)
            ratio = k1 / max(k0, 1)
            transfer.append({
                "concept": c.digest,
                "source": source,
                "target": target,
                "accuracy": k1_acc,
                "fresh_accuracy": None if fresh is None else fresh.heldout_accuracy,
                "cost_ratio": ratio,
                "low_holdout": low_hold,
            })
            if k1_acc < 0.90 or low_hold >= 0.90 or ratio >= 0.50:
                return {"verdict": "KILLED", "reason": f"cross-surface transfer failed {source}->{target}", "hidden": hidden, "discoveries": discoveries, "transfer": transfer}

    # Sequential compounding uses only the prefix library at each step.
    recursive = []
    for i, c in enumerate(library.concepts):
        source = surfaces[i % len(surfaces)]
        target = surfaces[(i + 1) % len(surfaces)]
        support = transfer_dataset(target, c.atoms, seed + 7000 + i * 61, 100)
        audit = transfer_dataset(target, c.atoms, seed + 8000 + i * 67, 140)
        _, k0 = fresh_all_search(support, audit)
        prefix = snapshots[i]
        _, acc, k1 = prefix.query(support, audit)
        ratio = k1 / max(k0, 1)
        recursive.append({"step": i + 1, "ratio": ratio, "accuracy": acc})
        if ratio >= 0.75 or acc < 0.90:
            return {"verdict": "KILLED", "reason": f"recursive compounding failed at step {i+1}", "hidden": hidden, "discoveries": discoveries, "transfer": transfer, "recursive": recursive}

    # Higher-order composition on a surface that was not the source of either concept.
    a, b = library.concepts[0], library.concepts[1]
    target = "graph"
    if target == surfaces[0]:
        target = "sequence"
    def compose_episode(kind: str, pa: Sequence[str], pb: Sequence[str], op: str, seed0: int) -> Episode:
        rng = random.Random(seed0)
        raw = make_surface(kind, rng)
        pairs = encode_surface(kind, raw, rng)
        ea = Episode(pairs, 0)
        xa, xb = predicts(ea, pa), predicts(ea, pb)
        label = int((xa and xb) if op == "and" else (xa or xb) if op == "or" else (bool(xa) ^ bool(xb)))
        return Episode(pairs, label)
    def compose_balanced(kind: str, pa: Sequence[str], pb: Sequence[str], op: str, seed0: int, count: int) -> tuple[Episode, ...]:
        rng = random.Random(seed0)
        pos, neg = [], []
        for _ in range(count * 4000):
            ep = compose_episode(kind, pa, pb, op, rng.randrange(10**12))
            (pos if ep.label else neg).append(ep)
            if len(pos) >= count // 2 and len(neg) >= count // 2:
                break
        if len(pos) < count // 2 or len(neg) < count // 2:
            raise RuntimeError("composition balancing failed")
        return tuple(pos[: count // 2] + neg[: count // 2])
    ctrain = compose_balanced(target, a.atoms, b.atoms, "xor", seed + 9000, 100)
    caudit = compose_balanced(target, a.atoms, b.atoms, "xor", seed + 9100, 140)
    k0_concept, k0_cost = fresh_all_search(ctrain, caudit)
    k0_acc = 0.0 if k0_concept is None else k0_concept.heldout_accuracy
    _, k1_acc, k1_cost = library.compose(ctrain, caudit)
    comp_ratio = k1_cost / max(k0_cost, 1)
    composition = {"accuracy": k1_acc, "fresh_accuracy": k0_acc, "cost_ratio": comp_ratio}
    if k1_acc < 0.90 or comp_ratio >= 0.75:
        return {"verdict": "KILLED", "reason": "higher-order composition failed", "hidden": hidden, "discoveries": discoveries, "transfer": transfer, "recursive": recursive, "composition": composition}

    # Deletion/rehydration on a blinded cross-surface task.
    target_c = library.concepts[2]
    target = surfaces[0]
    support = transfer_dataset(target, target_c.atoms, seed + 10000, 100)
    audit = transfer_dataset(target, target_c.atoms, seed + 10100, 140)
    _, k0 = fresh_all_search(support, audit)
    _, before_acc, before_cost = library.query(support, audit)
    before_adv = max(0.0, 1.0 - before_cost / max(k0, 1))
    payload = library.dumps()
    library.clear()
    deleted, deleted_acc, deleted_cost = library.query(support, audit)
    deleted_adv = 0.0 if deleted is None or deleted_acc < 0.90 else max(0.0, 1.0 - deleted_cost / max(k0, 1))
    restored = Library.loads(payload)
    _, after_acc, after_cost = restored.query(support, audit)
    after_adv = max(0.0, 1.0 - after_cost / max(k0, 1))
    deletion = {"before_acc": before_acc, "deleted_acc": deleted_acc, "after_acc": after_acc, "before_adv": before_adv, "deleted_adv": deleted_adv, "after_adv": after_adv}
    if before_adv <= 0 or deleted_adv > before_adv * 0.20 or after_adv < before_adv * 0.90:
        return {"verdict": "KILLED", "reason": "deletion/rehydration failed", "hidden": hidden, "discoveries": discoveries, "transfer": transfer, "recursive": recursive, "composition": composition, "deletion": deletion}

    # Three independently shuffled-label negative controls.
    controls = []
    source = surfaces[0]
    for j in range(3):
        p = hidden[j % len(hidden)]
        data = balanced(source, p, seed + 12000 + j * 73, 120)
        hold = balanced(source, p, seed + 13000 + j * 79, 120)
        decoy = shuffled_control(data, seed + 14000 + j * 83)
        c, _ = search(decoy, hold)
        acc = 0.0 if c is None else c.heldout_accuracy
        controls.append({"found": c is not None, "audit_accuracy": acc})
        if acc >= 0.90:
            return {"verdict": "KILLED", "reason": f"negative control passed at {j}", "hidden": hidden, "discoveries": discoveries, "transfer": transfer, "recursive": recursive, "composition": composition, "deletion": deletion, "controls": controls}

    return {
        "verdict": "PASS",
        "reason": "all frozen genuine gates passed",
        "hidden": hidden,
        "discoveries": discoveries,
        "transfer": transfer,
        "recursive": recursive,
        "composition": composition,
        "deletion": deletion,
        "controls": controls,
    }


def main() -> int:
    seeds = [int(os.environ.get("H4G_SEED_A", "610117")), int(os.environ.get("H4G_SEED_B", "830921"))]
    results = {}
    for seed in seeds:
        results[str(seed)] = run_block(seed)
    verdict = "PASS" if all(v["verdict"] == "PASS" for v in results.values()) else "KILLED"
    print("H4_GENUINE_RESULT")
    print("==================")
    for seed, r in results.items():
        print(f"SEED {seed}: {r['verdict']}")
        print(f"reason={r['reason']}")
        print(f"details={r}")
    print(f"H4 GENUINE VERDICT: {verdict}")
    return 0 if verdict == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
