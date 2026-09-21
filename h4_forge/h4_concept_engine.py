"""Generic invariant relational concept induction engine for the H4 kill-test.

The engine has no access to the hidden concept generator or audit seeds.
It searches generic relational motifs, validates them on held-out observations,
and stores opaque hashed concept records.
"""

from __future__ import annotations

import hashlib
import itertools
import json
from dataclasses import dataclass
from typing import Iterable, Sequence


ATOMS = (
    "same_shape",
    "same_color",
    "diff_shape",
    "diff_color",
    "adj4",
    "adj8",
    "same_row",
    "same_col",
    "x_lt",
    "y_lt",
)

MAX_PATTERN_SIZE = 4


@dataclass(frozen=True)
class Obj:
    shape: str
    color: str
    x: int
    y: int
    token: str


@dataclass(frozen=True)
class Episode:
    objects: tuple[Obj, ...]
    label: int = 0


@dataclass(frozen=True)
class Concept:
    digest: str
    atoms: tuple[str, ...]
    train_accuracy: float
    heldout_accuracy: float
    support: int

    @property
    def complexity(self) -> int:
        return len(self.atoms)


def candidate_patterns(min_size: int = 2, max_size: int = MAX_PATTERN_SIZE) -> tuple[tuple[str, ...], ...]:
    out: list[tuple[str, ...]] = []
    for size in range(min_size, max_size + 1):
        out.extend(itertools.combinations(ATOMS, size))
    return tuple(out)


PATTERNS = candidate_patterns()


def _rel(a: Obj, b: Obj, atom: str) -> bool:
    if atom == "same_shape":
        return a.shape == b.shape
    if atom == "same_color":
        return a.color == b.color
    if atom == "diff_shape":
        return a.shape != b.shape
    if atom == "diff_color":
        return a.color != b.color
    if atom == "adj4":
        return abs(a.x - b.x) + abs(a.y - b.y) == 1
    if atom == "adj8":
        return max(abs(a.x - b.x), abs(a.y - b.y)) == 1
    if atom == "same_row":
        return a.y == b.y
    if atom == "same_col":
        return a.x == b.x
    if atom == "x_lt":
        return a.x < b.x
    if atom == "y_lt":
        return a.y < b.y
    raise ValueError(atom)


def matches(objects: Sequence[Obj], atoms: Sequence[str]) -> bool:
    if len(objects) < 2:
        return False
    for a, b in itertools.permutations(objects, 2):
        if all(_rel(a, b, atom) for atom in atoms):
            return True
    return False


def predict_dataset(objects: Sequence[Obj], pattern: Sequence[str]) -> int:
    return int(matches(objects, pattern))


def accuracy(dataset: Sequence[Episode], pattern: Sequence[str]) -> float:
    if not dataset:
        return 0.0
    correct = sum(predict_dataset(ep.objects, pattern) == ep.label for ep in dataset)
    return correct / len(dataset)


def concept_digest(atoms: Sequence[str]) -> str:
    payload = "|".join(atoms).encode("utf-8")
    return hashlib.sha256(payload).hexdigest()[:16]


class ConceptLibrary:
    """Opaque concept memory. Only generic motif discovery is implemented."""

    def __init__(self) -> None:
        self._concepts: dict[str, Concept] = {}

    @property
    def concepts(self) -> tuple[Concept, ...]:
        return tuple(sorted(self._concepts.values(), key=lambda c: (c.complexity, c.digest)))

    def add(self, concept: Concept) -> None:
        self._concepts[concept.digest] = concept

    def remove_all(self) -> None:
        self._concepts.clear()

    def dumps(self) -> str:
        payload = [
            {
                "digest": c.digest,
                "atoms": list(c.atoms),
                "train_accuracy": c.train_accuracy,
                "heldout_accuracy": c.heldout_accuracy,
                "support": c.support,
            }
            for c in self.concepts
        ]
        return json.dumps(payload, sort_keys=True, separators=(",", ":"))

    @classmethod
    def loads(cls, payload: str) -> "ConceptLibrary":
        lib = cls()
        for item in json.loads(payload):
            lib.add(
                Concept(
                    digest=item["digest"],
                    atoms=tuple(item["atoms"]),
                    train_accuracy=float(item["train_accuracy"]),
                    heldout_accuracy=float(item["heldout_accuracy"]),
                    support=int(item["support"]),
                )
            )
        return lib

    def discover(
        self,
        train: Sequence[Episode],
        heldout: Sequence[Episode],
        *,
        min_accuracy: float = 0.90,
        min_support: int = 8,
    ) -> tuple[Concept | None, int]:
        best: Concept | None = None
        evaluations = 0
        for atoms in PATTERNS:
            evaluations += len(train)
            train_acc = accuracy(train, atoms)
            if train_acc < min_accuracy:
                continue
            held_acc = accuracy(heldout, atoms)
            support = sum(predict_dataset(ep.objects, atoms) for ep in train)
            if support < min_support:
                continue
            candidate = Concept(
                digest=concept_digest(atoms),
                atoms=atoms,
                train_accuracy=train_acc,
                heldout_accuracy=held_acc,
                support=support,
            )
            if candidate.heldout_accuracy < min_accuracy:
                continue
            if best is None or (candidate.heldout_accuracy, -candidate.complexity, candidate.support) > (
                best.heldout_accuracy,
                -best.complexity,
                best.support,
            ):
                best = candidate
        if best is not None:
            self.add(best)
        return best, evaluations

    def query(
        self,
        support: Sequence[Episode],
        audit: Sequence[Episode],
    ) -> tuple[Concept | None, float, int]:
        """Find the stored concept whose detector best explains support labels."""
        best: Concept | None = None
        best_acc = -1.0
        evaluations = 0
        for concept in self.concepts:
            evaluations += len(support)
            acc = accuracy(support, concept.atoms)
            if acc > best_acc:
                best = concept
                best_acc = acc
        if best is None:
            return None, 0.0, evaluations
        pred_acc = accuracy(audit, best.atoms)
        return best, pred_acc, evaluations + len(audit)

    def query_composition(
        self,
        support: Sequence[Episode],
        audit: Sequence[Episode],
    ) -> tuple[tuple[str, str, str] | None, float, int]:
        """Generic boolean composition over stored opaque concepts."""
        concepts = self.concepts
        if len(concepts) < 2:
            return None, 0.0, 0
        best = None
        best_support = -1.0
        evaluations = 0
        formulas = []
        for a_idx, a in enumerate(concepts):
            for b in concepts[a_idx + 1 :]:
                for op in ("and", "or", "xor"):
                    formulas.append((a.digest, b.digest, op))
        predictions = {
            c.digest: [predict_dataset(ep.objects, c.atoms) for ep in support]
            for c in concepts
        }
        for da, db, op in formulas:
            pa, pb = predictions[da], predictions[db]
            out = []
            for a, b in zip(pa, pb):
                if op == "and":
                    out.append(int(a and b))
                elif op == "or":
                    out.append(int(a or b))
                else:
                    out.append(int(bool(a) ^ bool(b)))
            evaluations += len(support)
            acc = sum(x == ep.label for x, ep in zip(out, support)) / len(support)
            if acc > best_support:
                best_support = acc
                best = (da, db, op)
        if best is None:
            return None, 0.0, evaluations
        by_digest = {c.digest: c for c in concepts}
        a, b, op = best
        audit_preds_a = [predict_dataset(ep.objects, by_digest[a].atoms) for ep in audit]
        audit_preds_b = [predict_dataset(ep.objects, by_digest[b].atoms) for ep in audit]
        audit_out = []
        for x, y in zip(audit_preds_a, audit_preds_b):
            if op == "and":
                audit_out.append(int(x and y))
            elif op == "or":
                audit_out.append(int(x or y))
            else:
                audit_out.append(int(bool(x) ^ bool(y)))
        evaluations += len(audit)
        audit_acc = sum(x == ep.label for x, ep in zip(audit_out, audit)) / len(audit)
        return best, audit_acc, evaluations


def fresh_learn(
    train: Sequence[Episode],
    heldout: Sequence[Episode],
    *,
    top_k: int = 12,
) -> tuple[Concept | None, int]:
    """Fresh K0 search. It knows no previously learned concept library."""
    ranked: list[tuple[float, int, tuple[str, ...]]] = []
    evaluations = 0
    for atoms in PATTERNS:
        evaluations += len(train)
        acc = accuracy(train, atoms)
        ranked.append((acc, -len(atoms), atoms))
    ranked.sort(reverse=True)
    for train_acc, neg_size, atoms in ranked[:top_k]:
        held_acc = accuracy(heldout, atoms)
        if train_acc >= 0.90 and held_acc >= 0.90:
            return (
                Concept(
                    digest=concept_digest(atoms),
                    atoms=atoms,
                    train_accuracy=train_acc,
                    heldout_accuracy=held_acc,
                    support=sum(predict_dataset(ep.objects, atoms) for ep in train),
                ),
                evaluations + top_k * len(heldout),
            )
    return None, evaluations + top_k * len(heldout)


def fresh_composed_search(
    train: Sequence[Episode],
    heldout: Sequence[Episode],
    *,
    top_k: int = 8,
) -> tuple[tuple[tuple[str, ...], tuple[str, ...], str] | None, float, int]:
    """Generic K0 composition search over high-scoring candidate motifs."""
    ranked: list[tuple[float, int, tuple[str, ...]]] = []
    evaluations = 0
    for atoms in PATTERNS:
        evaluations += len(train)
        acc = accuracy(train, atoms)
        ranked.append((acc, -len(atoms), atoms))
    ranked.sort(reverse=True)
    candidates = ranked[:top_k]
    best = None
    best_support = -1.0
    for _, _, a in candidates:
        for _, _, b in candidates:
            if a >= b:
                pass
            for op in ("and", "or", "xor"):
                predictions_a = [predict_dataset(ep.objects, a) for ep in train]
                predictions_b = [predict_dataset(ep.objects, b) for ep in train]
                out = []
                for x, y in zip(predictions_a, predictions_b):
                    if op == "and":
                        out.append(int(x and y))
                    elif op == "or":
                        out.append(int(x or y))
                    else:
                        out.append(int(bool(x) ^ bool(y)))
                evaluations += len(train)
                acc = sum(x == ep.label for x, ep in zip(out, train)) / len(train)
                if acc > best_support:
                    best_support = acc
                    best = (a, b, op)
    if best is None:
        return None, 0.0, evaluations
    a, b, op = best
    out = []
    for ep in heldout:
        x, y = predict_dataset(ep.objects, a), predict_dataset(ep.objects, b)
        if op == "and":
            out.append(int(x and y))
        elif op == "or":
            out.append(int(x or y))
        else:
            out.append(int(bool(x) ^ bool(y)))
    held_acc = sum(x == ep.label for x, ep in zip(out, heldout)) / len(heldout)
    evaluations += len(heldout)
    return best, held_acc, evaluations
