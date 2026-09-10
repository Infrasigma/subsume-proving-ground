# PHASE 2.1 SCIENTIFIC PROTOCOL v3

**Status:** `PROSPECTIVE SCOPE VERSION — NOT EXECUTED`  
**Purpose:** Explicitly remove candidate-level temporal operators from the Phase 2.1 instrument after a formal necessity decision.  
**Repository:** `Infrasigma/subsume-proving-ground`  
**Branch:** `phase2-structural-transfer-20260910`

## 0. Version relationship

`phase2_1_protocol_v3 = phase2_1_protocol_v2 + phase2_1_temporal_scope_decision_v1`

The original parent protocol and v2 remain preserved. This document is a prospective amendment only and does not retroactively change historical protocol meaning or historical results.

Authority for this change:

`phase2_1_temporal_scope_decision_v1.md`

## 1. Candidate grammar amendment

For Phase 2.1 candidate enumeration, the following predicates are removed:

- `BEFORE(Action, Action)`
- `AFTER(Action, Action)`

The remaining candidate predicates are:

- `BLOCKED(Action)`
- `AVAILABLE(Action)`
- `ACTION(Action)`
- `INTERVENES(Action, Action)`
- `OBSERVED_EFFECT(Action, Effect)`
- `ENABLES(Action, Action)`
- `NONENABLES(Action, Action)`
- `SAME_LOCAL_CONTEXT(Action, Action)`

All other grammar restrictions, role vocabulary, binding rules, implication form, candidate-size bounds, canonicalization, duplicate policy, and deterministic enumeration requirements remain inherited unless explicitly contradicted by this amendment.

## 2. What is not removed

Execution chronology remains part of the scientific protocol. Recorded event order is still used wherever required to establish that retrieval and prediction preceded the decisive intervention and that observed consequence followed it.

This procedural chronology is not a candidate predicate and is not available as an additional hidden feature for candidate enumeration.

Fact provenance required solely to evaluate candidate `BEFORE`/`AFTER` atoms is not a Phase 2.1 candidate-semantic dependency.

## 3. Scientific hypothesis

The primary hypothesis remains unchanged:

`C(B+ | K_A) < C(B+ | K_0)`

The bounded capability remains:

`experience -> relational abstraction -> reusable knowledge -> structural transfer -> verified enablement/non-enablement consequence`.

The prospective consequence domain remains exactly:

`{ ENABLES(target), NONENABLES(contrast,target) }`.

## 4. K_A boundary

K_A acquisition uses only rules expressible in the amended grammar. Any candidate requiring `BEFORE` or `AFTER` is inadmissible.

All existing K_A acquisition requirements remain unchanged.

## 5. Retrieval and prediction

Retrieval enumerates and grounds only amended-grammar candidates. No temporal role-to-Fact selector exists in the Phase 2.1 retrieval path.

Prediction remains restricted to the two-value consequence domain and must be created before the decisive intervention. Verification and attribution remain unchanged.

## 6. Attribution

The existing attribution chain and precedence remain unchanged. Event ordering remains an execution-level fact used for causal attribution; it is not promoted into candidate-level temporal predicates.

## 7. Controls and leakage

No hidden simulator state, timestamps exposed by the environment, semantic node IDs, dependency IDs, task seeds, future observations, coordinates, goal distance, or auditor-only metadata become learner-visible or candidate features as a result of this amendment.

## 8. ACE boundary

This amendment does not assert that temporal reasoning is unnecessary for ACE. It asserts only that explicit temporal candidate predicates are not necessary to answer the bounded Phase 2.1 structural-transfer question.

General ACE may later implement typed event/entity grounding, explicit provenance, temporal relations, causal temporal reasoning, and richer temporal abstractions under a separate scientific specification.

## 9. Execution authority

This version does not authorize the definitive Phase 2.1 experiment. Independent final conformance and preflight gates remain mandatory.

**Definitive Phase 2.1 experiment: `NOT EXECUTED`.**
