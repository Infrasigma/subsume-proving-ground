# PHASE 2.1 SCIENTIFIC PROTOCOL v3

**Status:** `PROSPECTIVE SCOPE VERSION — NOT EXECUTED`

`phase2_1_protocol_v3 = phase2_1_protocol_v2 + phase2_1_temporal_scope_decision_v1`

## Candidate grammar amendment

Phase 2.1 candidate predicates are exactly:

- `BLOCKED(Action)`
- `AVAILABLE(Action)`
- `ACTION(Action)`
- `INTERVENES(Action, Action)`
- `OBSERVED_EFFECT(Action, Effect)`
- `ENABLES(Action, Action)`
- `NONENABLES(Action, Action)`
- `SAME_LOCAL_CONTEXT(Action, Action)`

Candidate-level `BEFORE` and `AFTER` are permanently removed from Phase 2.1. They MUST NOT be recreated under another name or implementation path.

All inherited grammar restrictions, bounded candidate size, canonicalization, provenance, acquisition, retrieval, prediction, attribution, controls, generation, novelty, isolation, cost, and reproducibility requirements remain unchanged unless explicitly contradicted here.

## Execution chronology

Actual event order remains part of execution instrumentation and causal attribution. It is not a learner-visible feature and is not a candidate predicate.

## Scientific scope

The hypothesis remains `C(B+ | K_A) < C(B+ | K_0)`.

The bounded capability remains:

`experience -> relational abstraction -> reusable knowledge -> verified enablement/non-enablement prediction -> structural transfer`.

The prediction consequence domain is exactly `{ ENABLES(target), NONENABLES(contrast,target) }`.

## Execution authority

This version does not authorize the definitive 100-task experiment. Final conformance, leakage, determinism, differential verification, and preflight gates remain mandatory.

**Definitive Phase 2.1 experiment: `NOT EXECUTED`.**
