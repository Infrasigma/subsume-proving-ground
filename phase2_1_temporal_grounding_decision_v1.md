# PHASE 2.1 TEMPORAL GROUNDING DECISION v1

**Target:** Temporal role-to-provenance semantic closure  
**Status:** `PROTOCOL REDESIGN REQUIRED`  
**Execution authority:** `NONE`  
**Definitive Phase 2.1 experiment:** `NOT EXECUTED`  
**Branch:** `phase2-structural-transfer-20260910`

## 1. Decision

The authoritative Phase 2.1 material does **not** uniquely define the mapping

`grounded role/entity -> selected observable Fact(s) -> provenance timestamp set`.

The mathematical temporal operators are frozen, but their operands are not.

Therefore this target cannot honestly close as `MAPPING ENTAILED` or `MAPPING EXPLICITLY FROZEN` without a substantive prospective protocol decision.

**Authoritative result:** `PROTOCOL REDESIGN REQUIRED`.

No implementation behavior is used as semantic authority, and no benchmark endpoint result is used to choose a mapping.

## 2. Frozen mathematics that is already closed

For two already-selected fact provenance sets A and B:

`BEFORE(A,B) iff max(A) < min(B)`

`AFTER(A,B) iff BEFORE(B,A)`

If either provenance set is empty, the relation is false.

Provenance is a finite sorted duplicate-free set of zero-based append-order event indices. Timestamp/event index is not Fact identity. Consecutive timestamps are not intervals and no interpolation/duration semantics exist.

These rules are established by `phase2_1_fact_semantics_temporal_v1.md` and the SERL learner freeze.

## 3. Authoritative source audit

### 3.1 Parent protocol

`phase2_1_protocol_draft.md@3b5891ec7c45055bc9a8bf7bdf178c07f1e03d78`

Relevant authoritative provisions:

- Section 3 defines the learner-visible five-key observation interface and prohibits timestamps/metadata from being exposed by the environment.
- Section 4 defines auditor-only snapshot/fork and hidden X selection; these are explicitly outside the learner-visible channel.
- Sections 5.2–5.5 define ActionToken-based dependency actions and X/Y/Z semantics.
- Section 5.6 requires causal verification through ordinary observable action outcomes/availability changes.

The parent protocol therefore establishes observable history and action semantics, but does not state which fact predicate supplies the operands of `BEFORE(a,b)` when `a,b` are bound Action roles.

### 3.2 SERL learner freeze

`phase2_1_learner_freeze_v1.md@b2d2f3b09d7f8eb5a4800efbdfa8bfe5f198ea8b`

Section 4.2 defines the legal predicates and Section 4.3–4.4 defines observable base and transition facts. Section 4.5 defines temporal relations over **two fact provenance event sets**, not over role variables directly. It does not define a role-to-fact selector for a syntactic `BEFORE(a,b)` atom.

Section 5.2 defines the role vocabulary `target`, `intervention`, `contrast`, `context`; Section 5.3 defines ActionToken argument sorts. This establishes that the arguments of `BEFORE` are Action-sorted roles, but it does not define whether a role denotes the provenance of `ACTION(role)`, `AVAILABLE(role)`, `BLOCKED(role)`, `INTERVENES(...)`, another predicate instance, or a composite of facts.

Section 6 requires grounding of antecedent atoms but does not add a temporal operand-selection rule.

Section 7 requires temporal/provenance requirements to be satisfied during retrieval but likewise does not define which Fact provenance is selected for a temporal atom.

### 3.3 Fact temporal multiplicity amendment

`phase2_1_fact_semantics_temporal_v1.md`

Sections 1–5 close Fact identity, provenance-set construction, and the max/min temporal operators. Section 5 explicitly starts from already-defined provenance sets A and B. Section 8 states that executable `BEFORE`/`AFTER` candidate matching remained an implementation gap. No role-to-Fact mapping is added.

### 3.4 Grammar

`phase2_1_serl_grammar_v1.json`

`BEFORE` and `AFTER` are declared as binary predicates over `Action, Action`, while the grammar separately lists `BLOCKED`, `AVAILABLE`, `ACTION`, `INTERVENES`, `OBSERVED_EFFECT`, `ENABLES`, `NONENABLES`, and `SAME_LOCAL_CONTEXT`. The grammar does not declare a temporal operand type such as `FactRef`, `EventRef`, or `PredicateInstance`, nor a function selecting one from an Action role.

### 3.5 Transfer closure and protocol v2

`phase2_1_transfer_closure_v2.md` and `phase2_1_protocol_v2.md` establish the prospective two-value consequence domain and attribution chain, but neither supplies the missing role-to-Fact provenance selector.

The v2 consequence decision is explicitly limited to the prediction consequence domain. It does not retroactively define temporal operands.

### 3.6 Equivalence contract

`phase2_1_serl_equivalence_contract.md` requires identical derived facts, candidates, retrieval, prediction, and attribution inputs between independent implementations. It therefore makes an unresolved temporal operand mapping an A-class specification defect rather than an implementation choice.

## 4. Formal ambiguity

A grounded atom has syntax:

`BEFORE(a,b)` or `AFTER(a,b)`

where `a,b` are Action-sorted role bindings.

The frozen temporal operator requires operands of the form:

`A = provenance(F_A)` and `B = provenance(F_B)`.

The missing function is therefore:

`SelectFacts(atom, grounded_binding, observable_history) -> ordered collection of observable Fact identities`

followed by a defined aggregation rule to obtain the operand provenance set.

No such function is authoritatively specified.

This is scientifically consequential because different selectors can return different provenance sets from the same history, and therefore different truth values for the same grounded temporal atom.

## 5. Plausible interpretations considered

### A — Role denotes an action/event occurrence

Interpretation: `BEFORE(a,b)` compares the provenance of action occurrences represented by the bound action roles, effectively requiring an `ACTION(a)`/`ACTION(b)` event interpretation.

Strengths: directly causal/event-oriented; compatible with Action sorts; deterministic if occurrence selection is uniquely defined.

Problem: the current frozen grammar does not define which occurrence is selected when the same ActionToken appears in multiple observations/actions, nor whether the provenance comes from `ACTION`, `BLOCKED`, or another occurrence-bearing Fact. Selecting an occurrence would be a new substantive rule.

Status: plausible, but **not entailed**.

### B — Role denotes a predicate-specific fact

Interpretation: each temporal operand is supplied by a particular predicate instance such as `BLOCKED(a)`, `AVAILABLE(a)`, or `ACTION(a)`.

Strengths: exactly matches the fact-level definition of temporal operators and permits predicate-specific semantics.

Problem: the atom syntax `BEFORE(a,b)` contains no predicate selectors. Choosing one requires adding a typed predicate association or changing the grammar.

Status: plausible, but **not entailed**.

### C — Role denotes an entity and all facts involving it contribute provenance

Interpretation: collect provenance from every legal observable fact containing the bound role/entity, then compare the union sets.

Strengths: role/entity-centric and deterministic if the fact universe is frozen.

Problems: it conflates semantically different predicates (`AVAILABLE`, `BLOCKED`, `INTERVENES`, etc.) and can make unrelated observations alter temporal truth. It also changes the scientific meaning of a temporal atom from a relation between selected facts to a relation between unions of heterogeneous evidence. Nothing in the frozen protocol authorizes this aggregation.

Status: rejected as an unlicensed composite semantics.

### D — Role denotes a typed role/predicate pair

Interpretation: the role binding carries an additional predicate type that selects one Fact provenance set.

Strengths: precise and independently implementable.

Problem: the frozen role vocabulary and grammar do not carry such a type. Adding it changes the grammar/interface of the hypothesis language.

Status: viable redesign, not currently frozen.

### E — Composite grounding rule determined by candidate atom signature

Interpretation: the candidate's surrounding predicates determine which Fact provenance should feed `BEFORE/AFTER`.

Strengths: could express intended causal/event relationships without exposing hidden simulator data.

Problem: no such signature-to-Fact function is specified. Different composite rules can yield different candidate validity and K_A. It would therefore be a substantive new semantic rule, not an implementation detail.

Status: viable redesign, not currently frozen.

## 6. Why the ambiguity cannot be safely resolved by convenience

Choosing `ACTION(role)` would be tempting because roles are Action-sorted and the intended example uses `BEFORE(intervention,target)`. However, that inference is insufficient for scientific closure:

1. the temporal section defines operands as Fact provenance sets, not ActionToken provenance;
2. `ACTION(x)` is an episode-level fact whose provenance may contain multiple occurrence indices;
3. repeated action occurrences can produce a multi-timestamp provenance set;
4. `BEFORE(ACTION(a), ACTION(b))` can therefore differ from temporal relations based on `BLOCKED(a)`, `AVAILABLE(a)`, or a transition fact;
5. the protocol does not state which one is intended.

Likewise, choosing `AVAILABLE(role)` merely because availability is central to the benchmark would inject benchmark semantics into the grammar.

No current implementation is permitted to decide this question by precedent.

## 7. Scientific consequence

The mapping can alter every downstream stage named in the equivalence contract:

- **Candidate validity:** a temporal antecedent may switch between satisfied and unsatisfied depending on selected provenance.
- **K_A:** support/contradiction status can change, changing which relational rules pass the two-A-task acquisition gate.
- **Retrieval:** a frozen K_A rule may become applicable or inapplicable on B history.
- **Prediction:** retrieval changes which prediction event is emitted.
- **Attribution:** retrieval/prediction timing and grounding can change whether an event qualifies as `KA_TRANSFER`, `COINCIDENTAL`, or another precedence class.

Therefore this is a scientific protocol boundary, not an implementation detail.

## 8. Information boundary

No proposed mapping may use hidden simulator state, semantic node IDs, dependency IDs, X/Y/Z identity, future observations, coordinates, goal distance, task seeds, or auditor-only metadata.

The only admissible inputs are the learner-visible history and its deterministic observable-derived Facts, plus their provenance event sets as defined by the fact-semantics amendment.

The existing frozen interface therefore provides enough raw observable information to implement a future mapping, but it does **not** select which observable Fact(s) are the operands.

## 9. Temporal reasoning fixtures to define after redesign

These are semantic fixture classes, not executed endpoint experiments. Their expected answers must be derived only after the role-to-Fact selector is explicitly frozen:

1. one fact per role;
2. multiple candidate facts for one role;
3. repeated occurrences of the same fact;
4. multiple timestamps on one Fact;
5. strict-before provenance;
6. equal-time provenance;
7. strict-after provenance;
8. interleaving provenance;
9. duplicate timestamps collapsed to a set;
10. empty/missing provenance;
11. unrelated predicates;
12. multiple predicates involving the same bound Action entity.

The fixture suite must specifically distinguish `first`, `last`, `all`, `interval`, and predicate-specific provenance selection where these produce different answers.

## 10. Required prospective redesign boundary

Before executable temporal operators are implemented, the scientific protocol must add exactly one authoritative definition of:

`TemporalOperand(atom, grounding, F) -> Fact-selection set`

and, if multiple Facts may be selected, exactly one provenance aggregation rule.

The rule must be deterministic, observable, language-independent, compatible with the frozen grammar or accompanied by an explicit grammar amendment, and reproducible by an independent implementation.

Until that decision exists, implementing temporal evaluation would necessarily encode an unstated scientific assumption.

## 11. Anti-toy / ACE boundary

The reusable ACE-level primitive is **temporal grounding over explicit observable event/fact provenance**, not any Phase 2.1-specific selection trick.

The benchmark may use a deliberately narrow temporal grounding rule once scientifically frozen, but that rule must remain classified as experimental-instrument machinery unless separately justified for ACE.

The broader ACE architecture must support richer typed event grounding, temporal relations, causal provenance, and heterogeneous event/fact references without inheriting this benchmark's fixed grammar limitations.

## 12. Execution status

No definitive Phase 2.1 experiment was executed.

No benchmark endpoint was used to choose a mapping.

No toy cognitive machinery was built.

Earlier protocol artifacts remain unchanged.

## 13. Next target

Because the mapping is not yet frozen, the next target remains this semantic closure problem. Once a prospective mapping is explicitly approved/frozen, the next executable target is:

`TARGET 1B.2B-T2 — IMPLEMENT + DIFFERENTIAL-TEST FROZEN TEMPORAL OPERATORS`

