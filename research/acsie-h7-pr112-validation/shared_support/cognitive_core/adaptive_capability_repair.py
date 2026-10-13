from __future__ import annotations

"""Failure-directed, model-free capability expansion for the native ACSIE core.

The repair loop deliberately treats a failed attempt as evidence:
diagnose structural mismatch -> expand a generic operator vocabulary from observed data
-> synthesize compositionally -> verify on untouched holdout/transfer/adversarial cases
-> persist the diagnosis and the successful repair.

No language model, pretrained model, embedding service, network API, or benchmark-family
router is used.
"""

import copy
import json
import math
import statistics
from dataclasses import asdict, dataclass
from hashlib import sha256
from typing import Any, Mapping, Sequence

from .native_independent_core import Capability, NativeCognitiveCore, Procedure

Json = Any


def _canon(x: Json) -> str:
    return json.dumps(x, sort_keys=True, separators=(",", ":"), default=str, ensure_ascii=True)


def _digest(x: Json) -> str:
    return sha256(_canon(x).encode()).hexdigest()


def _numeric(x: Any) -> bool:
    return isinstance(x, (int, float)) and not isinstance(x, bool)


def _shape(x: Json):
    if isinstance(x, list):
        return ("list", len(x), tuple(_shape(v) for v in x[:4]))
    if isinstance(x, tuple):
        return ("tuple", len(x), tuple(_shape(v) for v in x[:4]))
    if isinstance(x, dict):
        return ("dict", tuple((str(k), _shape(v)) for k, v in sorted(x.items(), key=lambda kv: str(kv[0]))))
    if isinstance(x, bool):
        return "bool"
    if _numeric(x):
        return "number"
    if x is None:
        return "null"
    return type(x).__name__


def _distance(a: Json, b: Json) -> float:
    if a == b:
        return 0.0
    sa, sb = _shape(a), _shape(b)
    base = 0.0 if sa == sb else 0.60
    if isinstance(a, list) and isinstance(b, list):
        common = min(len(a), len(b))
        value = sum(_distance(x, y) for x, y in zip(a, b)) / max(1, common)
        length = abs(len(a) - len(b)) / max(1, len(a), len(b))
        return min(1.0, base + 0.25 * value + 0.15 * length)
    if isinstance(a, dict) and isinstance(b, dict):
        keys = set(a) | set(b)
        value = sum(_distance(a[k], b[k]) if k in a and k in b else 1.0 for k in keys) / max(1, len(keys))
        return min(1.0, base + 0.40 * value)
    if _numeric(a) and _numeric(b):
        scale = max(1.0, abs(float(a)), abs(float(b)))
        return min(1.0, base + abs(float(a) - float(b)) / scale)
    return min(1.0, base + 0.4)


def _apply(token: str, x: Json) -> Json | None:
    try:
        if token.startswith("macro:"):
            # Reuse the canonical executable macro representation used by the
            # persistent native program library. Macro contents are self-contained
            # and validated before admission; unknown/broken tokens fail closed.
            from .program_synthesis import apply_program
            return apply_program(x, (token,))
        if token in {"id", "identity"}:
            return copy.deepcopy(x)
        if token == "reverse":
            return list(reversed(copy.deepcopy(x))) if isinstance(x, list) else None
        if token == "unique":
            if not isinstance(x, list):
                return None
            seen = set()
            out = []
            for v in x:
                k = _canon(v)
                if k not in seen:
                    seen.add(k)
                    out.append(copy.deepcopy(v))
            return out
        if token == "sort":
            return sorted(copy.deepcopy(x), key=_canon) if isinstance(x, list) else None
        if token == "flatten":
            if not isinstance(x, list):
                return None
            out = []
            for v in x:
                if not isinstance(v, (list, tuple)):
                    return None
                out.extend(copy.deepcopy(v))
            return out
        if token == "concat":
            if not isinstance(x, list) or len(x) != 2 or not all(isinstance(v, list) for v in x):
                return None
            return copy.deepcopy(x[0]) + copy.deepcopy(x[1])
        if token == "interleave":
            if not isinstance(x, list) or len(x) != 2 or not all(isinstance(v, list) for v in x):
                return None
            if len(x[0]) != len(x[1]):
                return None
            out = []
            for a, b in zip(x[0], x[1]):
                out.extend([copy.deepcopy(a), copy.deepcopy(b)])
            return out
        if token == "reverse_each":
            if not isinstance(x, list) or not all(isinstance(v, list) for v in x):
                return None
            return [list(reversed(copy.deepcopy(v))) for v in x]
        if token in {"prefix_sum", "differences"}:
            if not isinstance(x, list) or not all(_numeric(v) for v in x):
                return None
            if token == "prefix_sum":
                total = 0
                out = []
                for value in x:
                    total += value
                    out.append(total)
                return out
            return [x[i + 1] - x[i] for i in range(len(x) - 1)]
        if token in {"sum", "mean", "min", "max"}:
            if not isinstance(x, list) or not all(_numeric(v) for v in x):
                return None
            if not x:
                return 0 if token != "mean" else None
            if token == "sum":
                return sum(x)
            if token == "mean":
                return sum(x) / len(x)
            if token == "min":
                return min(x)
            return max(x)
        if token == "count":
            return len(x) if isinstance(x, (list, dict, tuple, str)) else None

        if token.startswith("rotate_clamped:"):
            if not isinstance(x, list):
                return None
            k = int(token.split(":", 1)[1])
            if k < 0:
                return None
            if not x:
                return []
            k = min(k, len(x) - 1)
            return copy.deepcopy(x[k:] + x[:k])

        if token.startswith("rotate:"):
            if not isinstance(x, list):
                return None
            k = int(token.split(":", 1)[1])
            if not x:
                return []
            k %= len(x)
            return copy.deepcopy(x[k:] + x[:k])

        if token.startswith(("take:", "drop:", "slice:")):
            if not isinstance(x, list):
                return None
            parts = token.split(":")
            if parts[0] == "take":
                return copy.deepcopy(x[: int(parts[1])])
            if parts[0] == "drop":
                return copy.deepcopy(x[int(parts[1]) :])
            return copy.deepcopy(x[int(parts[1]) : int(parts[2])])

        if token.startswith(("repeat:", "chunk:", "window:")):
            if not isinstance(x, list):
                return None
            parts = token.split(":")
            n = int(parts[1])
            if n <= 0:
                return None
            if parts[0] == "repeat":
                return copy.deepcopy(x * n)
            if parts[0] == "chunk":
                return [copy.deepcopy(x[i : i + n]) for i in range(0, len(x), n)]
            return [copy.deepcopy(x[i : i + n]) for i in range(max(0, len(x) - n + 1))]

        if token.startswith(("map_add:", "map_sub:", "map_mul:")):
            if not isinstance(x, list) or not all(_numeric(v) for v in x):
                return None
            c = float(token.split(":", 1)[1])
            op = token.split(":", 1)[0]
            if op == "map_add":
                return [v + c for v in x]
            if op == "map_sub":
                return [v - c for v in x]
            return [v * c for v in x]

        if token == "map_neg":
            return [-v for v in x] if isinstance(x, list) and all(_numeric(v) for v in x) else None
        if token == "map_abs":
            return [abs(v) for v in x] if isinstance(x, list) and all(_numeric(v) for v in x) else None

        if token in {"transpose", "zip_add", "zip_sub", "zip_mul", "zip_max"}:
            if not isinstance(x, list) or not x or not all(isinstance(r, (list, tuple)) for r in x):
                return None
            if token == "transpose":
                # Match Python's ordinary zip(*rows) semantics: ragged rows are
                # truncated to the shortest row rather than rejected.
                return [list(col) for col in zip(*x)]
            if len(x) != 2 or len(x[0]) != len(x[1]):
                return None
            a, b = x[0], x[1]
            if not all(_numeric(v) for v in a + b):
                return None
            if token == "zip_add":
                return [u + v for u, v in zip(a, b)]
            if token == "zip_sub":
                return [u - v for u, v in zip(a, b)]
            if token == "zip_mul":
                return [u * v for u, v in zip(a, b)]
            return [max(u, v) for u, v in zip(a, b)]

        if token.startswith(("project:", "pair:", "filter_truthy:", "filter_falsy:", "filter_eq:", "filter_neq:")):
            if not isinstance(x, list):
                return None
            head = token.split(":", 1)[0]
            if head == "project":
                i = int(token.split(":", 1)[1])
                out = []
                for row in x:
                    if not isinstance(row, (list, tuple)) or i >= len(row):
                        return None
                    out.append(copy.deepcopy(row[i]))
                return out
            if head == "pair":
                _, ai, bi = token.split(":")
                a, b = int(ai), int(bi)
                out = []
                for row in x:
                    if not isinstance(row, (list, tuple)) or max(a, b) >= len(row):
                        return None
                    out.append([copy.deepcopy(row[a]), copy.deepcopy(row[b])])
                return out
            i, raw = token.split(":", 2)[1], None
            idx = int(i)
            if head in {"filter_truthy", "filter_falsy"}:
                out = []
                want = head == "filter_truthy"
                for row in x:
                    if not isinstance(row, (list, tuple)) or idx >= len(row):
                        return None
                    if bool(row[idx]) == want:
                        out.append(copy.deepcopy(row))
                return out
            raw = token.split(":", 2)[2]
            target = json.loads(raw)
            out = []
            for row in x:
                if not isinstance(row, (list, tuple)) or idx >= len(row):
                    return None
                equal = row[idx] == target
                if (head == "filter_eq" and equal) or (head == "filter_neq" and not equal):
                    out.append(copy.deepcopy(row))
            return out

        if token.startswith("group:"):
            idx = int(token.split(":")[1])
            if not isinstance(x, list):
                return None
            buckets = {}
            for row in x:
                if not isinstance(row, (list, tuple)) or idx >= len(row):
                    return None
                key = _canon(row[idx])
                buckets.setdefault(key, (copy.deepcopy(row[idx]), []))[1].append(copy.deepcopy(row))
            return [[k, rows] for _, (k, rows) in sorted(buckets.items(), key=lambda kv: kv[0])]

        if token.startswith("dict_get:"):
            if not isinstance(x, dict):
                return None
            key = json.loads(token.split(":", 1)[1])
            return copy.deepcopy(x[key]) if key in x else None
        if token == "dict_values":
            return [copy.deepcopy(v) for _, v in sorted(x.items(), key=lambda kv: str(kv[0]))] if isinstance(x, dict) else None
        if token == "dict_keys":
            return [copy.deepcopy(k) for k in sorted(x, key=str)] if isinstance(x, dict) else None
        if token == "dict_items":
            return [[copy.deepcopy(k), copy.deepcopy(v)] for k, v in sorted(x.items(), key=lambda kv: str(kv[0]))] if isinstance(x, dict) else None

        if token == "flatten_pairs":
            if not isinstance(x, list) or not all(isinstance(v, (list, tuple)) and len(v) == 2 for v in x):
                return None
            out = []
            for a, b in x:
                out.extend([copy.deepcopy(a), copy.deepcopy(b)])
            return out
    except (TypeError, ValueError, KeyError, IndexError, ZeroDivisionError):
        return None
    return None


def apply_extended_program(x: Json, program: Sequence[str]) -> Json | None:
    out = copy.deepcopy(x)
    for token in program:
        out = _apply(token, out)
        if out is None:
            return None
    return out


def _observed_behavior_signature(
    program: Sequence[str],
    rows: Sequence[tuple[Json, Json]],
) -> tuple[str, ...] | None:
    """Canonical output signature over currently observed rows.

    Programs with identical observed behavior are treated as one search state;
    the simpler candidate wins. This compresses syntactic aliases without
    exposing untouched external evaluation data to synthesis.
    """
    signature: list[str] = []
    for x, _ in rows:
        z = apply_extended_program(x, program)
        if z is None:
            return None
        signature.append(_canon(z))
    return tuple(signature)


def _program_complexity(program: Sequence[str]) -> int:
    """Count executable complexity, expanding persisted macros instead of hiding their cost."""
    total = 0
    for token in program:
        if isinstance(token, str) and token.startswith("macro:"):
            try:
                from .program_synthesis import apply_program
                import base64
                payload = base64.urlsafe_b64decode(token.split(":", 1)[1].encode()).decode()
                steps = tuple(json.loads(payload))
                if all(isinstance(step, str) and not step.startswith("macro:") for step in steps):
                    total += max(1, len(steps))
                    continue
            except Exception:
                pass
        total += 1
    return total


def _normalize_program(program: Sequence[str]) -> tuple[str, ...]:
    """Apply semantics-preserving local simplifications before scoring/searching."""
    out: list[str] = []
    for raw in program:
        token = str(raw)
        if token in {"id", "identity"}:
            continue
        if out and out[-1] == "reverse" and token == "reverse":
            out.pop()
            continue
        if out and token.startswith("rotate:") and out[-1].startswith("rotate:"):
            try:
                a = int(out.pop().split(":", 1)[1])
                b = int(token.split(":", 1)[1])
                out.append(f"rotate:{a + b}")
                continue
            except ValueError:
                pass
        if out and token.startswith(("map_add:", "map_sub:", "map_mul:")) and out[-1].startswith(token.split(":", 1)[0] + ":"):
            op = token.split(":", 1)[0]
            try:
                a = float(out.pop().split(":", 1)[1])
                b = float(token.split(":", 1)[1])
                if op == "map_add":
                    v = a + b
                elif op == "map_sub":
                    v = a + b
                else:
                    v = a * b
                if abs(v - round(v)) < 1e-12:
                    v = int(round(v))
                out.append(f"{op}:{v}")
                continue
            except ValueError:
                pass
        out.append(token)
    return tuple(out)


def _parameter_tokens(examples: Sequence[tuple[Json, Json]], library: Sequence[str] = ()) -> tuple[str, ...]:
    numeric_constants = {0, 1, -1, 2, -2, 3, -3}
    indices = set()
    keys = set()
    lengths = {1, 2, 3}
    row_widths = set()

    def visit(v: Json):
        if _numeric(v) and abs(float(v)) <= 16:
            numeric_constants.add(v)
        elif isinstance(v, dict):
            keys.update(v.keys())
            for z in v.values():
                visit(z)
        elif isinstance(v, list):
            lengths.add(len(v))
            for z in v:
                if isinstance(z, (list, tuple)):
                    row_widths.add(len(z))
                visit(z)
        elif isinstance(v, tuple):
            lengths.add(len(v))
            for z in v:
                visit(z)

    for x, y in examples:
        visit(x)
        visit(y)
    for w in row_widths:
        indices.update(range(min(6, max(0, w))))
    for n in list(lengths):
        if 0 < n <= 8:
            lengths.update({n - 1, n + 1})

    toks = {
        "id", "reverse", "unique", "sort", "flatten", "flatten_pairs",
        "concat", "interleave", "reverse_each", "prefix_sum", "differences",
        "sum", "mean", "min", "max", "count", "map_neg", "map_abs",
        "transpose", "zip_add", "zip_sub", "zip_mul", "zip_max",
        "dict_values", "dict_keys", "dict_items",
    }
    # Learned executable macros are reusable capabilities, not task-family routes.
    toks.update(str(token) for token in library if isinstance(token, str) and token.startswith("macro:"))
    for k in sorted(keys, key=lambda z: _canon(z)):
        toks.add("dict_get:" + json.dumps(k, sort_keys=True, separators=(",", ":")))
    for n in sorted(lengths):
        if 0 < n <= 8:
            toks.update({f"take:{n}", f"drop:{n}", f"chunk:{n}", f"window:{n}", f"repeat:{n}"})
    for k in sorted(lengths):
        if 0 <= k <= 8:
            toks.add(f"rotate_clamped:{k}")
        if -8 <= k <= 8:
            toks.add(f"rotate:{k}")
    for c in sorted(numeric_constants, key=lambda z: (float(z), str(type(z)))):
        toks.update({f"map_add:{c}", f"map_sub:{c}", f"map_mul:{c}"})
    for i in sorted(indices):
        toks.update({f"project:{i}", f"filter_truthy:{i}", f"filter_falsy:{i}", f"group:{i}"})
        for j in sorted(indices):
            toks.add(f"pair:{i}:{j}")
        for x, _ in examples:
            if isinstance(x, list):
                for row in x:
                    if isinstance(row, (list, tuple)) and i < len(row):
                        raw = json.dumps(row[i], sort_keys=True, separators=(",", ":"))
                        toks.add(f"filter_eq:{i}:{raw}")
                        toks.add(f"filter_neq:{i}:{raw}")
    for a in range(0, 6):
        for b in range(a, 7):
            if a < b:
                toks.add(f"slice:{a}:{b}")
    return tuple(sorted(toks))


@dataclass(frozen=True)
class FailureDiagnosis:
    diagnosis_id: str
    category: str
    observed_gap: str
    structural_signals: tuple[str, ...]
    candidate_score: float
    train_score: float
    holdout_score: float
    transfer_score: float
    suggested_capabilities: tuple[str, ...]


@dataclass(frozen=True)
class RepairAttempt:
    attempt: int
    diagnosis_id: str
    operator_count: int
    depth: int
    beam: int
    train_score: float
    holdout_score: float
    transfer_score: float
    adversarial_score: float
    accepted: bool
    program: tuple[str, ...]


@dataclass(frozen=True)
class RepairResult:
    solved: bool
    program: tuple[str, ...]
    train_score: float
    holdout_score: float
    transfer_score: float
    adversarial_score: float
    diagnosis: FailureDiagnosis | None
    attempts: tuple[RepairAttempt, ...]


def diagnose_failure(
    examples: Sequence[tuple[Json, Json]],
    holdout: Sequence[tuple[Json, Json]],
    transfer: Sequence[tuple[Json, Json]],
    *,
    train_score: float = 0.0,
    holdout_score: float = 0.0,
    transfer_score: float = 0.0,
    candidate_score: float = 0.0,
) -> FailureDiagnosis:
    signals = set()
    for x, y in examples:
        if isinstance(x, list) and isinstance(y, list):
            if len(x) == len(y) and x != y:
                signals.add("sequence_or_elementwise_transform")
            if any(isinstance(v, (list, tuple)) for v in x):
                signals.add("nested_sequence_composition")
            if len(x) == len(y) and x == list(reversed(y)):
                signals.add("sequence_reversal")
        if isinstance(x, dict) and isinstance(y, dict):
            signals.add("structured_record_transform")
        if _numeric(x) and _numeric(y) and x != y:
            signals.add("scalar_arithmetic")
        if isinstance(x, list) and x and all(isinstance(v, (list, tuple)) for v in x):
            signals.add("row_projection_filter_grouping")

    if train_score < 0.50:
        category = "train_fit_gap"
        observed = "current search cannot express the demonstrated mapping"
    elif holdout_score < 0.75:
        category = "generalization_gap"
        observed = "a fitted rule does not survive untouched examples"
    elif transfer_score < 0.75:
        category = "transfer_gap"
        observed = "the learned rule does not transfer to structurally related cases"
    else:
        category = "verification_gap"
        observed = "candidate exists but independent verification is insufficient"

    suggestions = []
    if "sequence_or_elementwise_transform" in signals:
        suggestions += ["sequence_reordering", "elementwise_arithmetic"]
    if "nested_sequence_composition" in signals:
        suggestions += ["flattening", "transpose", "chunking", "windowing", "zipped_composition"]
    if "structured_record_transform" in signals:
        suggestions += ["dictionary_projection", "dictionary_selection"]
    if "row_projection_filter_grouping" in signals:
        suggestions += ["row_projection", "filtering", "grouping"]
    if "scalar_arithmetic" in signals:
        suggestions += ["numeric_constants", "arithmetic_composition"]
    if not suggestions:
        suggestions.append("broadened_compositional_search")

    return FailureDiagnosis(
        diagnosis_id="gap:" + _digest((tuple(sorted(signals)), category, train_score, holdout_score, transfer_score))[:20],
        category=category,
        observed_gap=observed,
        structural_signals=tuple(sorted(signals)),
        candidate_score=float(candidate_score),
        train_score=float(train_score),
        holdout_score=float(holdout_score),
        transfer_score=float(transfer_score),
        suggested_capabilities=tuple(dict.fromkeys(suggestions)),
    )



def _prefix_family(
    program: Sequence[str],
    width: int = 2,
) -> tuple[str, ...]:
    """Return the first ``width`` operator families of a candidate program."""
    heads = tuple(str(token).split(":", 1)[0] for token in program)
    return heads[: max(1, int(width))]


def _select_diverse_frontier(
    candidates: Sequence[tuple[float, tuple[str, ...], float]],
    beam: int,
    *,
    prefix_width: int = 2,
) -> list[tuple[str, ...]]:
    """Preserve distinct prefix skeletons before global score fill."""
    if not candidates:
        return []
    limit = max(1, int(beam))
    ordered = sorted(candidates, key=lambda q: (q[0], q[1]))
    representatives: dict[tuple[str, ...], tuple[float, tuple[str, ...], float]] = {}
    for item in ordered:
        representatives.setdefault(_prefix_family(item[1], prefix_width), item)
    chosen = sorted(representatives.values(), key=lambda q: (q[0], q[1]))[:limit]
    chosen_programs = {item[1] for item in chosen}
    if len(chosen) < limit:
        for item in ordered:
            if item[1] in chosen_programs:
                continue
            chosen.append(item)
            chosen_programs.add(item[1])
            if len(chosen) == limit:
                break
    return [item[1] for item in chosen]

def synthesize_extended_program(
    examples: Sequence[tuple[Json, Json]],
    holdout: Sequence[tuple[Json, Json]],
    transfer: Sequence[tuple[Json, Json]],
    *,
    depth: int = 5,
    beam: int = 192,
    budget: int = 60000,
    library: Sequence[str] = (),
) -> RepairResult:
    if not examples:
        return RepairResult(False, (), 0.0, 0.0, 0.0, 0.0, None, ())

    split = max(1, int(len(examples) * 0.67))
    train = tuple(examples) if holdout else tuple(examples[:split])
    verification_pool = tuple(holdout) or tuple(examples[split:])
    transfer_pool = tuple(transfer) or tuple(examples[: max(1, len(examples) // 4)])
    tokens = _parameter_tokens(examples, library)
    attempts = []
    frontier: list[tuple[str, ...]] = [()]
    seen = {()}
    trials = 0
    best = (float("inf"), (), 0.0)
    # Quotient the search space by behavior on data available before hidden
    # evaluation. This is deliberately stronger than syntax-level deduplication
    # and prevents equivalent numeric aliases from consuming the beam.
    observed_rows = tuple(train) + tuple(verification_pool) + tuple(transfer_pool)
    behavior_best: dict[tuple[str, ...], tuple[float, int, int, tuple[str, ...]]] = {}

    for d in range(1, max(1, int(depth)) + 1):
        candidates = []
        for prefix in frontier:
            for token in tokens:
                if prefix and token in {"id", "identity"} and prefix[-1] == token:
                    continue
                program = _normalize_program(prefix + (token,))
                if program in seen:
                    continue
                seen.add(program)
                trials += 1
                if trials > budget:
                    break
                distances = []
                valid = True
                for x, y in train:
                    z = apply_extended_program(x, program)
                    if z is None:
                        valid = False
                        break
                    distances.append(_distance(z, y))
                if not valid:
                    continue
                score = statistics.mean(distances) if distances else 1.0
                complexity = _program_complexity(program)
                complexity_penalty = 0.004 * complexity
                rank = score + complexity_penalty
                signature = _observed_behavior_signature(program, observed_rows)
                if signature is None:
                    continue
                candidate_key = (rank, complexity, len(program), program)
                previous = behavior_best.get(signature)
                if previous is not None and candidate_key >= previous:
                    continue
                behavior_best[signature] = candidate_key
                candidates.append((rank, program, score))
                if rank < best[0]:
                    best = (rank, program, score)
                if score == 0.0:
                    break
            if trials > budget:
                break

        candidates.sort(key=lambda q: (q[0], q[1]))
        frontier = _select_diverse_frontier(
            candidates,
            max(8, int(beam)),
            prefix_width=min(3, d),
        )

        # Evaluate all exact-fit candidates in the current frontier before accepting.
        # This prevents the first internally verified candidate from winning merely
        # because it was enumerated earlier. Selection is still training/internal-only.
        verified_candidates = []
        for _, program, train_distance in candidates[: min(len(candidates), 256)]:
            if train_distance > 0.0:
                continue
            hold_values = [
                float(apply_extended_program(x, program) == y)
                for x, y in verification_pool
            ]
            transfer_values = [
                float(apply_extended_program(x, program) == y)
                for x, y in transfer_pool
            ]
            hs = statistics.mean(hold_values) if hold_values else 1.0
            ts = statistics.mean(transfer_values) if transfer_values else 1.0
            if hs >= 0.75 and ts >= 0.75:
                verified_candidates.append((
                    hs + ts,
                    -_program_complexity(program),
                    -len(program),
                    program,
                    hs,
                    ts,
                ))
        if verified_candidates:
            verified_candidates.sort(key=lambda q: (-q[0], -q[1], -q[2], q[3]))
            _, _, _, program, hs, ts = verified_candidates[0]
            attempts.append(RepairAttempt(
                1, "pending", len(tokens), d, beam, 1.0, hs, ts, 0.0, True, program
            ))
            return RepairResult(True, program, 1.0, hs, ts, 0.0, None, tuple(attempts))
        if trials > budget:
            break

    return RepairResult(
        False,
        best[1],
        max(0.0, 1.0 - best[2]),
        0.0,
        0.0,
        0.0,
        None,
        tuple(attempts),
    )



class RepairDrivenCognitiveCore(NativeCognitiveCore):
    """Native core whose failed solves become persistent, actionable capability evidence."""

    SCHEMA = "ACSIE.native-repair-driven-core.v1"

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.failure_archive: list[dict[str, Any]] = []
        self.capability_gaps: dict[str, dict[str, Any]] = {}
        self.repair_cache: dict[str, tuple[str, ...]] = {}

    def _repair_profile(self, examples: Sequence[tuple[Json, Json]]) -> str:
        # Cache identity must include the observed mapping, not only its shape.
        # Otherwise unrelated tasks with the same tensor/list shape can inherit a
        # previously synthesized program and bypass genuine capability acquisition.
        signature = tuple(
            (_shape(x), _shape(y), _canon(x), _canon(y))
            for x, y in examples[:12]
        )
        return _digest(("repair-profile-v2", signature))[:20]

    def _store_failure(self, diagnosis: FailureDiagnosis) -> None:
        bucket = self.capability_gaps.setdefault(
            diagnosis.diagnosis_id,
            {"count": 0, "categories": {}, "capabilities": list(diagnosis.suggested_capabilities)},
        )
        bucket["count"] += 1
        bucket["categories"][diagnosis.category] = int(bucket["categories"].get(diagnosis.category, 0)) + 1
        self.failure_archive.append(asdict(diagnosis))
        self.failure_archive = self.failure_archive[-256:]

    def _verify_program(
        self,
        program: Sequence[str],
        rows: Sequence[tuple[Json, Json]],
    ) -> float:
        values = [
            float(apply_extended_program(x, program) == y)
            for x, y in rows
        ]
        return statistics.mean(values) if values else 1.0

    def solve_with_repair(
        self,
        examples: Sequence[tuple[Json, Json]],
        holdout: Sequence[tuple[Json, Json]] = (),
        transfer: Sequence[tuple[Json, Json]] = (),
        adversarial: Sequence[tuple[Json, Json]] = (),
        *,
        max_repairs: int = 3,
        depth: int = 5,
        beam: int = 192,
    ) -> RepairResult:
        # Strict information-flow rule:
        # hidden holdout/transfer/adversarial rows are NEVER supplied to the
        # search/synthesis process. They are evaluation-only inputs after a
        # candidate is frozen.
        native = self.solve_examples_anytime(
            examples, (), (), budget=max(256, beam)
        )

        def verify_frozen(program: Sequence[str]) -> tuple[float, float, float]:
            hs = self._verify_program(program, holdout) if holdout else 1.0
            ts = self._verify_program(program, transfer) if transfer else 1.0
            av = self._verify_program(program, adversarial) if adversarial else 1.0
            return hs, ts, av

        if native.verified and isinstance(native.result, Procedure):
            tokens = tuple(
                step.get("token")
                for step in native.result.program
                if isinstance(step, Mapping) and step.get("token")
            )
            hs, ts, av = verify_frozen(tokens)
            if hs >= 0.75 and ts >= 0.75 and av >= 0.75:
                return RepairResult(
                    True, tokens, 1.0, hs, ts, av, None, ()
                )

        profile = self._repair_profile(examples)
        cached = self.repair_cache.get(profile)
        if cached:
            hs, ts, av = verify_frozen(cached)
            if hs >= 0.75 and ts >= 0.75 and av >= 0.75:
                return RepairResult(True, cached, 1.0, hs, ts, av, None, ())

        # Diagnosis is based only on the information available before hidden
        # verification. In particular, no hidden score is used to choose a repair.
        baseline_train = max(
            (float(p.train_score) for p in self.procedures.values()), default=0.0
        )
        diagnosis = diagnose_failure(
            examples, (), (),
            train_score=baseline_train,
            holdout_score=0.0,
            transfer_score=0.0,
            candidate_score=float(native.score),
        )
        self._store_failure(diagnosis)

        attempts: list[RepairAttempt] = []
        candidates: list[tuple[float, float, float, int, tuple[str, ...]]] = []
        for attempt in range(1, max(1, int(max_repairs)) + 1):
            widened_depth = min(8, depth + attempt - 1)
            widened_beam = min(512, beam + attempt * 64)

            # Crucially, synthesis sees training examples only.
            result = synthesize_extended_program(
                examples, (), (),
                depth=widened_depth,
                beam=widened_beam,
                budget=60000 + attempt * 20000,
                library=tuple(self.program_library),
            )
            program = tuple(result.program)
            candidates.append((
                float(result.holdout_score),
                float(result.transfer_score),
                float(result.train_score),
                -len(program),
                program,
            ))
            attempts.append(RepairAttempt(
                attempt,
                diagnosis.diagnosis_id,
                len(_parameter_tokens(examples, self.program_library)),
                widened_depth,
                widened_beam,
                float(result.train_score),
                0.0,
                0.0,
                0.0,
                False,
                program,
            ))

        # Select only from training/internal-validation evidence. Hidden evaluation
        # is untouched until this choice is frozen.
        candidates.sort(reverse=True, key=lambda x: (x[0], x[1], x[2], x[3], x[4]))
        best_program = candidates[0][4] if candidates else ()
        best_train = candidates[0][2] if candidates else 0.0

        # Hidden data is touched exactly once, after the training-only candidate
        # has been selected. It cannot steer search, widening, or candidate choice.
        hs, ts, av = verify_frozen(best_program) if best_program else (0.0, 0.0, 0.0)
        accepted = bool(best_program) and hs >= 0.75 and ts >= 0.75 and av >= 0.75
        attempts.append(RepairAttempt(
            len(attempts) + 1,
            diagnosis.diagnosis_id,
            len(_parameter_tokens(examples)),
            min(8, depth + max(0, len(attempts))),
            min(512, beam + (len(attempts) + 1) * 64),
            float(best_train),
            float(hs),
            float(ts),
            float(av),
            accepted,
            best_program,
        ))

        final = RepairResult(
            accepted,
            best_program,
            float(best_train),
            float(hs),
            float(ts),
            float(av),
            diagnosis,
            tuple(attempts),
        )
        if not accepted:
            return final

        # An accepted repair becomes a persistent executable capability. Its
        # externally visible representation must therefore be the canonical
        # persisted macro, not the transient primitive search sequence. Existing
        # macro tokens are kept as-is so nested macros are never created.
        persisted_program = tuple(best_program)
        if persisted_program and not any(
            isinstance(token, str) and token.startswith("macro:")
            for token in persisted_program
        ):
            admitted_macro = self._admit_program_macro(
                persisted_program,
                family="failure-repair",
                procedure_id=None,
            )
            if admitted_macro is not None:
                persisted_program = (admitted_macro,)

        self.repair_cache[profile] = tuple(persisted_program)
        pid = "repair:" + _digest((profile, tuple(persisted_program), self.generation))[:20]
        procedure = Procedure(
            pid,
            tuple({"token": t} for t in persisted_program),
            float(best_train),
            float(hs),
            float(ts),
            len(persisted_program),
            self.generation,
            None,
            {
                "origin": "native_failure_directed_program_repair",
                "external_model": False,
                "network": False,
                "manual_solution": False,
                "diagnosis_id": diagnosis.diagnosis_id,
                "capability_gap": list(diagnosis.suggested_capabilities),
                "hidden_data_used_for_search": False,
                "hidden_data_used_for_selection": False,
                "persisted_representation": True,
            },
            "validated",
        )
        self.procedures[pid] = procedure
        cap_id = "cap:" + _digest((diagnosis.diagnosis_id, tuple(persisted_program)))[:20]
        self.capabilities[cap_id] = Capability(
            cap_id,
            "failure-recovered-capability",
            float(best_train),
            float(ts),
            1.0,
            len(persisted_program),
            pid,
            self.generation,
        )
        self.change_history.append({
            "type": "capability_repair",
            "diagnosis": asdict(diagnosis),
            "repair": asdict(attempts[-1]),
            "persisted_program": list(persisted_program),
        })
        return RepairResult(
            True,
            tuple(persisted_program),
            float(best_train),
            float(hs),
            float(ts),
            float(av),
            diagnosis,
            tuple(attempts),
        )


        self.repair_cache[profile] = tuple(best_program)
        pid = "repair:" + _digest((profile, tuple(best_program), self.generation))[:20]
        procedure = Procedure(
            pid,
            tuple({"token": t} for t in best_program),
            float(best_train),
            float(hs),
            float(ts),
            len(best_program),
            self.generation,
            None,
            {
                "origin": "native_failure_directed_program_repair",
                "external_model": False,
                "network": False,
                "manual_solution": False,
                "diagnosis_id": diagnosis.diagnosis_id,
                "capability_gap": list(diagnosis.suggested_capabilities),
                "hidden_data_used_for_search": False,
                "hidden_data_used_for_selection": False,
            },
            "validated",
        )
        self.procedures[pid] = procedure
        # Successful failure-directed acquisition becomes a persistent executable
        # capability, so later repair/search can reuse it without rediscovery.
        self._admit_program_macro(
            best_program,
            family="failure-repair",
            procedure_id=pid,
        )
        cap_id = "cap:" + _digest((diagnosis.diagnosis_id, tuple(best_program)))[:20]
        self.capabilities[cap_id] = Capability(
            cap_id,
            "failure-recovered-capability",
            float(best_train),
            float(ts),
            1.0,
            len(best_program),
            pid,
            self.generation,
        )
        self.change_history.append({
            "type": "capability_repair",
            "diagnosis": asdict(diagnosis),
            "repair": asdict(attempts[-1]),
        })
        return final

    def export_state(self) -> dict[str, Any]:
        state = super().export_state()
        state["schema"] = self.SCHEMA
        state["failure_archive"] = copy.deepcopy(self.failure_archive)
        state["capability_gaps"] = copy.deepcopy(self.capability_gaps)
        state["repair_cache"] = {
            k: list(v) for k, v in self.repair_cache.items()
        }
        return state

    @classmethod
    def from_state(cls, state: Mapping[str, Any]) -> "RepairDrivenCognitiveCore":
        out = cls(seed=int(state.get("seed", 0)))
        restored = NativeCognitiveCore.from_state(state)
        out.policy = restored.policy
        out.learning_kernel = restored.learning_kernel
        out.experience = restored.experience
        out.hypotheses = restored.hypotheses
        out.procedures = restored.procedures
        out.capabilities = restored.capabilities
        out.goal_archive = restored.goal_archive
        out.change_history = restored.change_history
        out.prediction_history = restored.prediction_history
        out.semantic_memory = restored.semantic_memory
        out.solver_history = restored.solver_history
        out.program_library = restored.program_library
        out.generation = restored.generation
        out.step = restored.step
        out.failure_archive = copy.deepcopy(list(state.get("failure_archive", ())))
        out.capability_gaps = copy.deepcopy(dict(state.get("capability_gaps", {})))
        out.repair_cache = {
            k: tuple(v) for k, v in dict(state.get("repair_cache", {})).items()
        }
        return out
