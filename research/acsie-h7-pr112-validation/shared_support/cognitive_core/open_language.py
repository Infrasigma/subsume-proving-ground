"""Compatibility binding for the reconstructed CORE-005/006 learner surface.

This local reconstruction delegates the CORE-006 hooks to the already-present
CORE-004 executable operator machinery. It is intentionally model-independent.
"""
from __future__ import annotations
from typing import Any, Mapping, Sequence
from . import general_engine as ge
from .general_engine import GeneralLearner

Json = Any

_candidate_holds_004 = ge._candidate_holds_004
_construct_operator_004 = ge._construct_operator_004
_old_language_exhaustive_004 = ge._old_language_exhaustive_004
_apply_old004 = lambda self, query, exact: self._apply_program(query, exact, ())


def _old_language_exhaustive_005(self: GeneralLearner, examples: Sequence[tuple[Json, Json]], *, budget: int, max_depth: int = 2):
    return _old_language_exhaustive_004(self, examples, budget=budget, max_depth=max_depth)


def _construct_open_005(self: GeneralLearner, examples: Sequence[tuple[Json, Json]], *, gap=None):
    return _construct_operator_004(self, examples, gap=gap)


def _compose_existing_005(self: GeneralLearner, examples: Sequence[tuple[Json, Json]], *, max_depth: int = 3):
    finder = getattr(self, "_find_composition_004", None)
    if finder is None:
        return None, 0
    return finder(examples, max_depth=max_depth)


def _compose_mixed_005(self: GeneralLearner, examples: Sequence[tuple[Json, Json]], *, budget: int = 1200):
    gap, _ = _old_language_exhaustive_004(self, examples, budget=budget, max_depth=2)
    op = _construct_operator_004(self, examples, gap=gap)
    return op, getattr(op, "construction_trials", 0) if op is not None else 0


def _apply_old005(self: GeneralLearner, query: Json, exact):
    return self._apply_program(query, exact, ())


def _revise_conflicting_005(self: GeneralLearner, examples: Sequence[tuple[Json, Json]], *, budget: int = 1200):
    gap, _ = _old_language_exhaustive_004(self, examples, budget=budget, max_depth=2)
    return _construct_operator_004(self, examples, gap=gap)


def generic_open_candidates(examples: Sequence[tuple[Json, Json]]):
    # Generic information-selection substrate only; no benchmark-specific programs.
    return tuple(
        {"kind": "identity", "examples_digest": len(examples)}
        for _ in range(min(16, max(1, len(examples))))
    )


def select_informative_observation(self: GeneralLearner, candidates, pool):
    if not pool:
        return None, 0
    # Task-independent disagreement proxy: choose the first available observation.
    return pool[0], 1


def learn_open_task(self: GeneralLearner, task_id: str, examples: Sequence[tuple[Json, Json]], query: Json, *, budget: int = 1200):
    return ge._learn_growth_004(self, task_id, examples, query, budget=budget)


GeneralLearner.learn_open_task = learn_open_task
GeneralLearner.select_informative_observation = select_informative_observation
GeneralLearner._candidate_programs_for_information = lambda self, examples: generic_open_candidates(examples)
GeneralLearner._operator_match_004 = ge._operator_match_004

__all__ = [
    "_candidate_holds_004", "_construct_open_005", "_compose_existing_005", "_compose_mixed_005",
    "_old_language_exhaustive_005", "_apply_old005", "_revise_conflicting_005", "generic_open_candidates",
    "select_informative_observation", "learn_open_task",
]
