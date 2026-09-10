# PHASE 2.1 SERL TWO-IMPLEMENTATION EQUIVALENCE CONTRACT

**Target:** 1B.1A — Complete the SERL Learner Freeze  
**Status:** `NORMATIVE`  
**Execution authority:** `NONE`  
**Definitive Phase 2.1 experiment:** `NOT EXECUTED`  
**Repository:** `Infrasigma/subsume-proving-ground`  
**Branch:** `phase2-structural-transfer-20260910`

## 1. Purpose

This contract defines when two independently written SERL implementations are scientifically equivalent.

Passing ordinary unit tests is insufficient. Matching average performance is insufficient. Producing the same final success rate is insufficient.

Two implementations are equivalent only when identical frozen inputs produce identical outputs at every scientifically consequential boundary, or when the boundary is explicitly defined as observationally equivalent below.

`SERL = Phase-2.1 scientific instrument`  
`SERL != final ACE architecture`

## 2. Frozen-input identity

For an equivalence comparison, both implementations receive byte-identical:

1. parent protocol artifact and SHA-256;
2. SERL learner-freeze artifact and version;
3. task semantic definition/corpus artifact;
4. opaque token assignment artifact;
5. initial learner-visible observation;
6. condition identifier;
7. immutable K_A artifact when the condition is K_A;
8. fixed budget limits;
9. fixed terminal rules;
10. fixed serialization specification.

The second implementation must not be given any internal state produced by the first implementation.

## 3. Boundary model

The following sequence is the normative observable pipeline:

`INPUT -> H/U -> F -> CANDIDATES -> K_A -> RETRIEVAL -> PREDICTION -> ACTION -> E/TERMINAL -> ATTRIBUTION INPUT`

Each arrow is a comparison boundary.

## 4. Exact-equality boundaries

### 4.1 Input

Canonical input bytes must be identical.

### 4.2 Event ledger

For every event, both implementations must produce byte-identical canonical records:

- event kind;
- sequence index;
- observation bytes;
- action token;
- result bytes;
- provenance indices.

### 4.3 Derived facts

The canonical set of observable facts and each fact's provenance must be identical.

Fact order in an internal data structure is irrelevant, but canonical serialized order must be identical.

### 4.4 Candidate set

The complete canonical candidate set must be identical. No implementation may omit a candidate because it appears unhelpful.

Canonical candidate serialization, hash, role bindings, and predicted consequence type must match exactly.

### 4.5 K_A

The frozen K_A contents, ordering, hashes, provenance, support-task list, and contradiction-task list must be byte-identical.

### 4.6 Retrieval

At each decision boundary, both implementations must agree exactly on:

- applicable candidates;
- canonical grounding of each applicable candidate;
- conflict state;
- selected candidate;
- retrieval event.

### 4.7 Prediction

Prediction events must be byte-identical, including event order, abstraction identifier, applicable-context hash, normalized-rule hash, and predicted consequence type.

### 4.8 Action

The next learner-issued ActionToken must be byte-identical.

### 4.9 Terminal/cost

Both implementations must produce identical:

- valid/illegal action classification;
- interaction cost `E` after every action;
- terminal status;
- decision-cutoff state;
- interaction-cap state;
- invalidation status.

### 4.10 Attribution inputs

The event trace presented to the attribution procedure must be byte-identical.

## 5. Observational equivalence for internal representations

Internal representations may differ in:

- programming language;
- data structure;
- memory layout;
- indexing strategy;
- object identity;
- cache layout;
- compiled representation;
- process scheduling;
- storage location.

They are equivalent only if deleting those implementation details and recomputing from the frozen inputs produces the same canonical boundary artifacts.

A hash map, database, array, graph object, or custom index may therefore be used internally only if iteration-order nondeterminism cannot affect a boundary output.

## 6. No hidden state

An implementation fails equivalence if any decision depends on:

- uninitialized memory;
- memory address/object identity;
- wall-clock time;
- process ID;
- host-specific filesystem state;
- inherited environment variables not included in the frozen input;
- cross-task cache contents;
- cross-condition cache contents;
- random seed not declared by the protocol;
- thread scheduling;
- network state;
- external model/service state;
- previous experimental results.

## 7. Determinism requirement

SERL has zero learner-side random draws.

Given identical frozen input and state, repeated execution must yield:

`same H/U -> same F -> same candidates -> same K_A -> same retrieval -> same prediction -> same action`.

If repeated executions differ, the implementation is non-conformant until the source of nondeterminism is eliminated.

## 8. Canonical ordering contract

Both implementations must apply the same ordering rules:

- role order: `target, intervention, contrast, context`;
- opaque token order: bytewise ascending;
- event order: append/sequence order;
- candidate order: canonical UTF-8 serialization bytes;
- K_A order: canonical rule serialization, then SHA-256 hash;
- action ties: bytewise ActionToken order;
- replay sequences: literal recorded sequence order.

No implementation may rely on language-specific map/set iteration order.

## 9. Serialization contract

Canonical JSON is UTF-8 with:

- ASCII-sorted object keys;
- no insignificant whitespace;
- arrays in normative order;
- ordinary decimal JSON integers;
- JSON booleans/null;
- opaque tokens retained as strings.

Scientific hashes are SHA-256 over exact canonical UTF-8 bytes.

## 10. State-reconstruction test

For any decision boundary `t`, an implementation must be able to reconstruct its decision from:

`protocol_version + SERL_version + H_t + U_t + K_A + frozen constants`.

Deleting all process-local caches and recomputing must reproduce the same next action and boundary artifacts.

If it cannot, hidden mutable state exists and equivalence fails.

## 11. Token-permutation test

On non-definitive fixtures, apply a bijective opaque-token permutation consistently to all task-local occurrences while preserving the underlying observable transition sequence.

A compliant SERL implementation must preserve role-generalized learned content and decision behavior up to the corresponding token permutation.

It may not interpret hexadecimal spelling, prefixes, numeric magnitude, lexical substrings, or hash-derived visual patterns as semantics.

This test is not a Phase 2.1 endpoint measurement.

## 12. Independent implementation protocol

Two implementations must be written independently from the normative artifacts. Neither implementer may inspect the other's internal source before producing the comparison artifacts.

For each frozen fixture:

1. initialize both implementations from identical canonical input;
2. run to the next boundary;
3. serialize both boundary artifacts canonically;
4. compare byte-for-byte;
5. if different, stop and classify the discrepancy;
6. repair the specification or implementation only before any definitive endpoint run;
7. rerun the equivalence suite.

No discrepancy may be resolved by choosing whichever implementation produces the more favorable later endpoint.

## 13. Discrepancy classification

Every mismatch must be classified as one of:

`SPECIFICATION_AMBIGUITY` — the documents permit two behaviors.  
`IMPLEMENTATION_DEFECT_A` — implementation A violates the specification.  
`IMPLEMENTATION_DEFECT_B` — implementation B violates the specification.  
`NONDETERMINISM` — same implementation gives different boundary outputs from identical input.  
`UNDEFINED_BOUNDARY` — the specification failed to define a comparison boundary.

Any `SPECIFICATION_AMBIGUITY` or `UNDEFINED_BOUNDARY` is an A-class closure defect and blocks definitive execution.

## 14. Equivalence is not algorithmic identity

Two implementations do not need identical source code or internal algorithms.

They do need identical external behavior at every scientifically frozen boundary.

This permits legitimate implementation optimization, including:

- memoization;
- precompiled candidate indexes;
- compact storage;
- different traversal implementations;
- parallel internal computation.

Such optimization is allowed only when the resulting canonical outputs remain identical and no timing/process state becomes a decision variable.

## 15. Resource exhaustion

Resource exhaustion cannot be converted into a scientific algorithmic choice.

If one implementation completes and another cannot complete exhaustive candidate evaluation under the agreed execution environment, the comparison is not declared equivalent. It is recorded as a resource/conformance issue.

No candidate pruning, approximate search, timeout-based rule selection, or fallback heuristic may be introduced solely to force equivalence.

## 16. Scientific-equivalence theorem used by the harness

For frozen input `I`, define the boundary trace:

`T(I) = (H,U,F,C,K_A,R,P,A,E,Z)`

where `H,U,F,C,K_A,R,P,A,E,Z` are respectively the canonical event history, derived facts, complete candidate set, frozen knowledge, retrieval, prediction, action, cost/terminal state, and attribution inputs.

Two implementations `S1,S2` are SERL-equivalent iff:

`∀ I ∈ F : T_S1(I) = T_S2(I)`

for the finite conformance fixture set `F`, and both implementations satisfy the deterministic reconstruction property for every boundary.

For the definitive experiment, the same equality requirement applies to every executed task, not merely to the preflight fixture set.

## 17. What this contract does not prove

Passing equivalence does not prove:

- that SERL is a good learner;
- that SERL transfers relational knowledge;
- that Phase 2.1 will pass;
- that the benchmark is representative of AGI;
- that the long-term ACE architecture is correct.

It proves only that two implementations instantiate the same frozen experimental instrument.

## 18. Pre-execution gate

Before Target 1A is authorized, an independent implementation/equivalence review must verify:

- no unresolved A-class learner ambiguity;
- no hidden semantic input;
- identical candidate-space closure;
- identical K_A construction;
- identical retrieval/prediction/action boundaries;
- identical cost/terminal accounting;
- deterministic reconstruction;
- token-permutation sanity behavior;
- no cross-condition state leakage.

This contract itself authorizes no definitive experiment.
