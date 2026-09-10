# PHASE 2.1 SCIENTIFIC PROTOCOL v2

**Status:** `PROSPECTIVE SCOPE VERSION — NOT EXECUTED`  
**Purpose:** Explicitly adopt the consequence-domain decision required to close the parent protocol's prediction-domain ambiguity.  
**Repository:** `Infrasigma/subsume-proving-ground`  
**Branch:** `phase2-structural-transfer-20260910`

## 0. Version relationship

This document is a versioned scientific amendment to the frozen parent protocol:

`phase2_1_protocol_v2 = phase2_1_protocol_draft.md@3b5891ec7c45055bc9a8bf7bdf178c07f1e03d78 + explicit prospective consequence-domain decision`

The original parent file is preserved unchanged and remains the historical authority for every provision not explicitly changed below.

This version does **not** retroactively claim that v1 contained the consequence-domain restriction.

Decision authority:

`phase2_1_consequence_domain_decision_v1.md`

Decision commit: `c5874afe7f9d8f1701cdaf8dc2cf40c45fe5cfd7`

Parent protocol SHA-256: `06f39b7ead0dae272094cda82654e834be4cc22c`

## 1. Explicit prospective scientific scope

For the Phase 2.1 experiment only, the prediction consequence domain is now explicitly frozen as exactly:

`ConsequenceType := { ENABLES(target), NONENABLES(contrast,target) }`

No other consequence type is admissible in a Phase 2.1 prediction event.

This is a **scientific scope restriction**, not an implementation convenience and not a claim about the universal prediction language of ACE.

## 2. Formal consequence typing

### 2.1 `ENABLES(target)`

`target` is a bound action role identifying the target action whose availability is being predicted.

The prediction is satisfied only when the specified intervention causes the target action to become available/executable under the frozen environment semantics, with the corresponding protocol-defined positive causal verification.

### 2.2 `NONENABLES(contrast,target)`

`contrast` is a bound intervention role and `target` is the bound target action.

The prediction is satisfied only when the specified contrast intervention does not make the target action available/executable under the frozen negative-control semantics.

### 2.3 Legal instantiation

The only legal consequence predicate instantiations are:

1. `ENABLES(target)`
2. `NONENABLES(contrast,target)`

The roles must be bound using the closed K_A role vocabulary and canonical alpha-renaming rules inherited from the parent/SERL grammar after this decision.

Concrete task IDs, semantic node IDs, seeds, opaque token literals as semantic roles, hidden enablement variables, future outcomes, coordinates, and executable code are not legal consequence arguments.

## 3. Prediction validity

A Phase 2.1 prediction event is valid only if:

- its consequence type is one of the two types above;
- its canonical role bindings are valid;
- it is created before the decisive action/intervention;
- its applicability context is learner-visible and protocol-admissible;
- its predicted consequence is falsifiable by the subsequent frozen intervention/observation procedure;
- its verification event is available to the attribution chain.

A prediction outside this domain is `PROTOCOL_VIOLATION`, not an alternative scientific prediction.

## 4. Attribution compatibility

The parent attribution chain remains:

`retrieval -> application -> prediction -> intervention -> observed consequence -> verification -> attribution`

A `KA_TRANSFER` event requires the predicted consequence to be one of the two scoped consequence types and to pass the existing verification and K0-equivalent attribution replay.

The restriction does not alter the parent attribution precedence:

`PROTOCOL_VIOLATION > UNATTRIBUTABLE > CONFLICT > RETRIEVAL_ONLY > PREDICTION_ONLY > COINCIDENTAL > KA_TRANSFER`

## 5. What remains unchanged

All parent protocol provisions remain unchanged unless explicitly overridden by Sections 1–4 of this version, including:

- primary hypothesis and endpoint;
- Family A and Family B task definitions;
- X/Y/Z causal semantics;
- learner-visible interface;
- acquisition boundary;
- novelty algorithm and threshold;
- retrieval ordering;
- controls;
- isolation;
- pairing;
- terminal outcomes and E;
- statistics and seeds;
- leakage invariants;
- reproducibility requirements;
- stopping/cancellation rules.

The parent protocol's explicit two-value consequence restriction was absent; this v2 document is the prospective point at which it becomes authoritative for Phase 2.1.

## 6. Scientific interpretation boundary

Phase 2.1 now tests the bounded capability:

`experience -> relational abstraction -> reusable knowledge -> structural transfer -> verified enablement/non-enablement consequence`

It does **not** test:

- general prediction;
- arbitrary causal consequence representation;
- unrestricted causal discovery;
- general representation learning;
- semantic understanding;
- AGI.

A positive result is evidence only for the bounded capability under the frozen learner and task families.

A negative result does not establish that relational abstraction or structural transfer is generally impossible.

## 7. ACE architecture boundary

The two-value consequence domain is not a constraint on the combined ACE architecture.

Future ACE consequence representation must remain capable of expressing arbitrary falsifiable consequences appropriate to the operating domain, subject to whatever future scientific specification is independently established.

No ACE architectural component is added or removed by this protocol version.

## 8. Execution authority

This v2 protocol does **not** authorize execution by itself.

Independent conformance and differential verification remain mandatory. All implementation-frozen boundaries must be completed and independently checked before any definitive Phase 2.1 run.

**Definitive Phase 2.1 experiment: `NOT EXECUTED`.**

## 9. Provenance

Parent protocol:

`phase2_1_protocol_draft.md`

commit `3b5891ec7c45055bc9a8bf7bdf178c07f1e03d78`

Decision:

`phase2_1_consequence_domain_decision_v1.md`

commit `c5874afe7f9d8f1701cdaf8dc2cf40c45fe5cfd7`

This v2 artifact is the prospective scientific scope version; its own commit SHA is the authoritative version identifier once created.
