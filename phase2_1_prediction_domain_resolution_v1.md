# PHASE 2.1 AUTHORITATIVE PREDICTION-DOMAIN RESOLUTION v1

**Target:** 1B.2A — Authoritative prediction-domain resolution

**Status:** `SCIENTIFICALLY_OPEN`

**Execution authority:** `NONE`

**Definitive Phase 2.1 experiment:** `NOT EXECUTED`

**Repository:** `Infrasigma/subsume-proving-ground`

**Branch:** `phase2-structural-transfer-20260910`

**Authoritative parent protocol:** `3b5891ec7c45055bc9a8bf7bdf178c07f1e03d78`

**Parent protocol SHA-256:** `06f39b7ead0dae272094cda82654e834be4cc22c`

## 1. Question

Does the authoritative frozen Phase 2.1 protocol logically entail that the complete prediction/consequence domain is exactly:

`{ENABLES(target), NONENABLES(contrast,target)}`?

## 2. Source priority

The frozen parent protocol at `phase2_1_protocol_draft.md` commit `3b5891ec7c45055bc9a8bf7bdf178c07f1e03d78` is authoritative. Later SERL and closure documents are provenance evidence only and cannot redefine the parent protocol.

## 3. Exhaustive relevant parent definitions

### 3.1 Section 2 — deterministic randomness

Sections 2.1–2.4 define randomness, namespaces, and seeds only. They impose no prediction consequence-domain restriction.

### 3.2 Sections 3.2–3.8 — learner-visible interface

The protocol defines the complete learner observation as state, available actions, last action, last result, and termination. Result status is one of `BLOCKED`, `ACCEPTED`, `SUCCESS`, `ILLEGAL_ACTION`, `ENVIRONMENT_ERROR`. These clauses define the observable interface; they do not enumerate a closed set of prediction objects.

### 3.3 Section 4.1 — auditor snapshot/fork

The auditor may inspect transition-relevant hidden state for instrumentation, but hidden state is inaccessible to the learner. This is an observability/instrumentation rule, not a prediction-domain definition.

### 3.4 Section 4.2 — X selection

X is selected from the hidden task construction table and resolved to an opaque action token. The learner is not told that the token is X. This constrains X identification for the audit; it does not define a complete consequent type system.

### 3.5 Section 4.3 — X-probe presentation

The X probe is presented through the ordinary action interface and yields the ordinary result/observation objects. Again, this defines observable evidence rather than a closed prediction-consequence language.

### 3.6 Sections 5.2–5.6 — Family A dependency and intervention semantics

Section 5.3 defines `Y_r` as an enabling intervention and `Z_r` as a non-enabling contrast. Section 5.4 defines that X is unavailable before Y, becomes available after Y, and remains unavailable after Z. Section 5.6 requires the auditor to establish these positive/negative intervention contrasts.

These clauses establish the causal mechanism under test, including enabling and non-enabling intervention relations. They do not state that these two relation types exhaust all legal prediction consequents.

### 3.7 Section 6.3 — causal observation window

The protocol explicitly records positive and negative intervention sequences and subsequent X outcome/availability. This defines the observation procedure for the causal relation; it does not state that every admissible prediction in Phase 2.1 must belong to exactly two consequence categories.

### 3.8 Sections 7.2–7.7 — acquisition

Section 7.2 defines an operational provenance claim for a qualifying relational record. Section 7.4 defines `Delta_K_A`. Section 7.5 permits an A-derived relational content class. Section 7.7 defines the acquisition gate.

No clause in these sections logically closes the consequent domain to exactly two forms.

### 3.9 Section 8.1 — closed K_A grammar

The parent protocol lists exactly these atomic predicates:

`BLOCKED(x)`
`AVAILABLE(x)`
`ACTION(x)`
`INTERVENES(y,x)`
`BEFORE(a,b)`
`AFTER(a,b)`
`OBSERVED_EFFECT(a,e)`
`ENABLES(y,x)`
`NONENABLES(z,x)`
`SAME_LOCAL_CONTEXT(a,b)`

Section 8.1 says only these atomic predicates exist. This closes the predicate vocabulary. It does **not** state that the consequent of every predicted rule is restricted to exactly `ENABLES(target)` or `NONENABLES(contrast,target)`.

### 3.10 Section 8.2 — constructors

The grammar permits bounded EXISTS/FORALL, AND, implication, and role-variable binding, while prohibiting disjunction, negation, arithmetic, recursion, arbitrary functions, executable code, embeddings, hash lookup, and unbounded quantifiers.

These clauses close composition operators, not the semantic range of the prediction consequent.

### 3.11 Section 8.3 — canonical role vocabulary and reusable relation

The protocol restricts role variables to `target`, `intervention`, `contrast`, `context` and gives one reusable structural relation as an example/form: blocked target, intervention, observed enabling effect, temporal precedence, and non-enabling contrast.

Crucially, the text says the reusable rule **may express** this structural relation. It does not formally state that this consequent is the complete prediction domain for all admissible predictions.

### 3.12 Section 12.1 — retrieval

The protocol defines retrieval as matching frozen K_A abstractions against the learner-visible relational context. No hidden metadata may be used. This defines applicability and retrieval; it does not enumerate all possible predicted consequence types.

### 3.13 Section 12.2 — applicability

Applicability is evaluated only in the closed learner-visible relational language and excludes hidden state, generator metadata, task IDs, and future outcomes. This closes the information boundary for applicability, not the complete semantic range of consequences.

### 3.14 Section 12.3 — prediction schema

The protocol defines a prediction event containing:

- `abstraction_id`;
- canonical applicable context hash;
- normalized rule hash;
- `predicted consequence type`;
- event order.

It then states:

`A prediction is falsifiable: it must specify the expected availability/outcome consequence of applying the relation.`

This is the decisive parent-protocol text. It requires a falsifiable expected availability/outcome consequence, but it does not enumerate the complete set of legal consequence values, nor does it state that the set is exactly `{ENABLES(target), NONENABLES(contrast,target)}`.

### 3.15 Section 13 — transfer attribution

Section 13.1 requires the chain `retrieval -> application -> prediction -> intervention -> observed consequence -> verification -> attribution`.

Sections 13.2–13.5 define decisive action timing, K0-equivalent attribution replay, precedence, and adversarial cases. These clauses constrain causal attribution; they do not provide an exhaustive two-value consequence-domain definition.

### 3.16 Sections 22–23 — protocol/implementation boundary

Section 22.1 calls the K_A grammar, retrieval, attribution, controls, and related items scientifically immutable. Section 22.2 says the concrete learner implementation is frozen separately. Section 23 requires independent implementers to agree without making a scientific choice about K_A grammar, retrieval, and attribution.

These sections make the distinction between protocol and implementation explicit. They do not retroactively make a later SERL consequence restriction parent-authoritative.

## 4. Formal result

### Conclusion

`SCIENTIFICALLY_OPEN`

### Proof obligation

To obtain `ENTAILED`, the parent protocol would need to contain a logical chain equivalent to:

1. the prediction event has a consequent;
2. the consequent must be an availability/outcome consequence;
3. the protocol defines the complete availability/outcome consequence set;
4. that set contains exactly `ENABLES(target)` and `NONENABLES(contrast,target)`;
5. no other consequence form is admissible.

The frozen parent protocol establishes (1) and (2), and it defines `ENABLES`/`NONENABLES` as members of the causal relational vocabulary. It does **not** establish (3)–(5).

Therefore the implication

`allowable consequence = {ENABLES(target), NONENABLES(contrast,target)}`

is not logically entailed by the parent protocol.

No contradiction was found in the parent definitions. The issue is under-specification, not inconsistency.

## 5. Counterinterpretations

### Interpretation 1 — exactly two consequences

`{ENABLES(target), NONENABLES(contrast,target)}`.

**Compliance:** not established by the parent protocol alone.

**Reason:** the parent defines both relations but never declares them to be the exhaustive consequent domain.

**Status:** cannot be treated as parent-derived.

### Interpretation 2 — all protocol-defined effect/consequence predicates

A prediction consequent could range over consequence forms explicitly represented by the protocol's causal/effect vocabulary, subject to the applicable predicate typing and observability rules.

**Compliance:** not uniquely established either.

**Reason:** the protocol never gives a formal exhaustive typing rule saying that all effect-like predicates are legal consequents. This interpretation is broader than the two-value SERL restriction but still requires an additional formal choice about which predicates may occupy the consequent position.

**Status:** plausible but not closed.

### Interpretation 3 — all directly observable availability/outcome consequences

A prediction may specify any falsifiable consequence about the next ordinary observable availability/outcome that satisfies the protocol's information boundary and causal attribution rules.

**Compliance:** consistent with Section 12.3's phrase “expected availability/outcome consequence,” but not uniquely formalized.

**Reason:** the protocol names availability/outcome as the required prediction content but does not provide a grammar or type system enumerating every legal such consequence.

**Status:** plausible but not closed.

### Interpretation 4 — only the showcased reusable relation

A prediction must be exactly the structural relation displayed in Section 8.3.

**Compliance:** rejected as an entailment.

**Reason:** Section 8.3 presents the relation as something the reusable rule “may express,” not as an exhaustive prediction grammar. Treating that example as exhaustive would add a semantic restriction not stated in the parent protocol.

## 6. Why different compliant interpretations matter

Interpretations 1–3 differ in candidate hypothesis space. The candidate space determines which rules can be acquired into K_A, which predictions can be emitted, which events can qualify for transfer attribution, and potentially which B action can be knowledge-guided.

Therefore the distinction is scientifically material, not merely an implementation convenience.

A later implementation that silently chooses Interpretation 1 changes the scientific hypothesis space relative to an implementation that honestly follows a broader interpretation. Two independent implementers could therefore produce different K_A candidate sets while each claiming to follow the current parent wording.

That violates the closure requirement in Section 23 for any disagreement capable of changing K_A acquisition, transfer attribution, or the endpoint.

## 7. SERL provenance

The later SERL learner freeze commit is:

`b2d2f3b09d7f8eb5a4800efbdfa8bfe5f198ea8b`

File: `phase2_1_learner_freeze_v1.md`

Section 5.1 states explicitly that each candidate has:

`C is exactly one of ENABLES(target) or NONENABLES(contrast,target) as a predicted consequence type.`

This is the first verified later artifact in the audited chain that explicitly makes the two-element consequence domain exhaustive. It is not present as such in the authoritative parent protocol.

Section 4.4 of the same SERL freeze also defines:

`OBSERVED_EFFECT(y,ENABLES(x))`

under the visible-transition condition, and

`NONENABLES(y,x)`

for the negative contrast.

The later SERL text therefore operationalizes the parent intervention mechanism into a two-consequence candidate space. That is a substantive scientific restriction, not a neutral implementation detail.

The later grammar artifact `phase2_1_serl_grammar_v1.json` explicitly records its status as `PARTIALLY_FROZEN` and identifies the same issue as a blocker. Its blocker statement says that the parent protocol does not establish that SERL's two-element consequence domain is complete.

## 8. Scientific consequence

The current SERL grammar and candidate-enumeration path must remain non-authorizing.

Do **not**:

- silently freeze the two-value domain;
- broaden it opportunistically in code;
- choose the domain from expected benchmark utility;
- run the definitive experiment to see which interpretation performs better;
- claim that implementation has been fully closed.

A defensible next scientific decision requires one of:

1. an authoritative parent-level amendment/preregistration that explicitly narrows the Phase 2.1 prediction domain to exactly the two values;
2. an authoritative parent-level closure defining the complete consequence domain and its typing rules;
3. a separately authorized redesign showing that SERL can test the complete parent-defined domain without adding an outcome-driven restriction.

No endpoint result may decide among these.

## 9. AGI architectural warning

`AGI_ARCHITECTURAL_WARNING`

The broader ACE goal requires prediction machinery that is not intrinsically limited to one benchmark-specific pair of consequence categories. The audit therefore reinforces a distinction already present elsewhere in the ACE plan: SERL's two-value prediction space must not be promoted into the final ACE architecture.

This is an architectural warning, not evidence that Phase 2.1 itself requires a general prediction engine. No new architecture is authorized by this document.

## 10. Definitive experiment

`NOT EXECUTED`

## 11. Next target

Continue scientific resolution before execution.

The two-value domain cannot be promoted to parent-authoritative status from the present frozen protocol alone. A parent-level scientific decision is required before `TARGET 1B.2B — INDEPENDENT CONFORMANCE + DIFFERENTIAL VERIFICATION` can honestly become the next closure target.
