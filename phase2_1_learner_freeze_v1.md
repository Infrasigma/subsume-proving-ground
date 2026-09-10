# PHASE 2.1 SERL LEARNER FREEZE v1

**Target:** 1B.1A — Complete the SERL Learner Freeze  
**Status:** `COMPLETE` for the learner instrument; this document does **not** authorize the definitive Phase 2.1 experiment.  
**Execution authority:** `NONE`  
**Definitive Phase 2.1 experiment:** `NOT EXECUTED`  
**Repository:** `Infrasigma/subsume-proving-ground`  
**Branch:** `phase2-structural-transfer-20260910`  
**Parent protocol:** `phase2_1_protocol_draft.md`  
**Parent protocol commit:** `3b5891ec7c45055bc9a8bf7bdf178c07f1e03d78`  
**Parent protocol SHA-256:** `06f39b7ead0dae272094cda82654e834be4cc22c`

## 0. Architecture-preservation statement

`SERL = Phase-2.1 scientific instrument`  
`SERL != final ACE architecture`

SERL isolates one capability of the combined ACE research direction: **acquiring an inspectable relational abstraction from permitted experience and using that abstraction before a decision in a structurally novel instance**.

SERL does not instantiate, replace, or establish the long-term ACE requirements for open-ended entity discovery, persistent episodic/semantic/procedural memory, causal hypothesis formation beyond this fixed protocol, active experiment design beyond the protocol's fixed interventions, learned world models, skills, hierarchy, long-horizon planning, self-diagnosis, model revision, self-modeling, capability discovery, architecture search, autonomous implementation, verified self-modification, or recursive capability compounding.

Nothing in this freeze is evidence that SERL is an AGI architecture.

## 1. Scientific role and hard boundary

The parent scientific hypothesis remains unchanged:

`C(B+ | K_A) < C(B+ | K_0)`

SERL is selected because it provides the minimum explicit mechanism needed to test whether **A-derived relational knowledge**, rather than raw A episodes or generic planning machinery, can influence B decisions.

The learner-visible input is exactly the parent protocol's five-key observation object. No additional information channel is permitted.

A compliant implementation MUST NOT expose to SERL, directly or indirectly:

- semantic node IDs;
- graph topology;
- dependency lists or dependency indices;
- task ID or task seed;
- semantic X/Y/Z identity;
- coordinates;
- source/destination metadata;
- hidden enablement state;
- goal distance or path distance;
- future observations/results;
- evaluator labels;
- condition results from another process;
- generation/rejection counters;
- filesystem/cache contents from another condition;
- wall-clock timing as a decision feature;
- process memory inherited from another task or condition.

Opaque tokens are symbols with equality semantics **only within the current task**. A token's hexadecimal spelling has no semantic interpretation.

## 2. SERL state

During one task SERL has exactly these logical state components:

1. `H`: ordered observation ledger.
2. `U`: ordered action/result ledger.
3. `F`: deterministic derived-fact set recomputed from `H,U`.
4. `M`: task-local learned/evaluated relational memory.
5. `K_A`: immutable cross-task knowledge, present only in K_A.

`M` is reset at every task boundary. `K_A` is loaded once before B and is immutable thereafter.

No other mutable state may affect a scientific decision. An implementation may maintain caches, indexes, compiled predicates, or memoized searches only when deleting the cache and recomputing from the frozen state produces byte-identical external behavior.

## 3. Observation/event ledger

### 3.1 Observation records

At every environment response, SERL appends the exact five-key observation object to `H` after canonical validation.

The canonical internal record is:

`OBS(t, state, available_actions, last_action, last_result, terminated)`

where `t` is the zero-based observation index assigned by SERL in append order. `t` is internal provenance only and is never supplied as semantic information by the environment.

The original JSON key order is irrelevant to semantics; the canonical serialization order is the parent protocol order:

`state, available_actions, last_action, last_result, terminated`.

`available_actions` is copied as a set for fact evaluation and retained as the exact parent-specified lexicographically sorted sequence for audit.

### 3.2 Action/result events

Every learner-issued valid action attempt creates exactly one action event:

`ACT(t, x, result_t)`

where `x` is the exact submitted opaque ActionToken and `result_t` is the exact returned ResultObject.

The resulting observation is appended as the next `OBS` record.

An illegal action is **not** converted into a learner fact and invalidates that condition according to the parent protocol's terminal-outcome rules.

A blocked action is a valid action attempt and is retained because its exact result is observable.

### 3.3 Event identity and ordering

Event identity is the tuple `(episode_id=0, sequence_index, event_kind)`, with `sequence_index` equal to append order. There is exactly one episode per task execution.

The total order is:

`OBS_0 < ACT_0 < OBS_1 < ACT_1 < OBS_2 ...`

No event is inserted retroactively. Derived facts may reference event indices but cannot change event order.

Repeated observations and repeated actions are distinct events even when byte-identical.

### 3.4 Null events

The initial observation is an observation event with `last_action=null` and `last_result=null`. It is evidence of the initial visible interface only.

There are no synthetic action, transition, causal, or success events. A learner may not invent an event for an unobserved transition.

### 3.5 History retention

The complete current episode `H,U` is retained until task termination. No truncation, decay, sampling, summarization, or forgetting is permitted.

Thus any bounded-context or finite-memory alternative is not SERL.

### 3.6 Reset

At task start, all task-local state is empty. At task end, all task-local state is destroyed. Cross-task equality of token strings is ignored. Only the immutable pre-B K_A artifact may survive between A and B in the K_A condition.

## 4. Observable relational fact construction

The fact compiler is a pure deterministic function:

`F = Compile(H,U)`.

It may use only exact visible history. It may not query an environment object, simulator object, hidden state, task generator, or evaluator.

### 4.1 Legal ground entities

Legal entities are:

- ActionToken values observed in `available_actions` or `last_action`;
- LocationToken values observed in `state.location`.

Entity identity is exact byte equality within the current task. No semantic identity is inferred across different token strings. No cross-task token equality is used.

### 4.2 Legal predicates

Only the parent protocol predicates are legal:

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

No new predicate, feature, distance, count, arithmetic expression, graph relation, embedding, or latent-state symbol may be introduced.

### 4.3 Base observable facts

For every observation index `t`:

- `AVAILABLE_t(x)` iff x occurs in that observation's `available_actions`.
- `ACTION(x)` iff x occurs anywhere in `available_actions` or as `last_action` in the episode.
- `BLOCKED_t(x)` iff the immediately preceding valid action event submitted x and its exact result status is the protocol's blocked status.

The suffix `_t` is event provenance notation; canonical stored predicates use the parent grammar and retain event provenance separately.

### 4.4 Observable transition facts

For an action event `ACT(t,y,result)` with preceding observation `OBS_t` and following observation `OBS_{t+1}`:

`INTERVENES(y,x)` is derivable for an action token x iff:

1. x is absent from `OBS_t.available_actions`; and
2. x is present in `OBS_{t+1}.available_actions`.

`OBSERVED_EFFECT(y,ENABLES(x))` is derivable under exactly the same visible-transition condition.

`NONENABLES(y,x)` is derivable iff:

1. x is absent from `OBS_t.available_actions`; and
2. x remains absent from `OBS_{t+1}.available_actions`.

No hidden enablement variable is consulted.

### 4.5 Temporal facts

For two fact provenance event sets A and B:

`BEFORE(A,B)` is true iff `max_event(A) < min_event(B)`.

`AFTER(A,B)` is true iff `BEFORE(B,A)`.

If either provenance set is empty, the temporal relation is false.

Temporal relations never assert causality; they assert only observed event order.

### 4.6 Local-context equality

For an action occurrence at observation index `t`, its local observable context is the canonical tuple:

`LC_t = (state.location, available_actions)`

with `available_actions` serialized in parent-defined lexicographic token order.

`SAME_LOCAL_CONTEXT(a,b)` is derivable iff the two referenced action occurrences have byte-identical `LC` tuples.

No hidden node identity or graph position is used.

### 4.7 Location relations

LocationToken values may be retained as observable symbols, but SERL does not introduce any predicate connecting two different locations unless such a connection is explicitly expressible by the parent grammar. There is no inferred adjacency, distance, direction, or semantic location type.

### 4.8 Equality

Equality means exact equality of opaque token bytes within one task. Equality is not semantic equivalence and is never carried across tasks.

### 4.9 Derived hypothesis boundary

A fact is **observable evidence** only when it follows directly from the definitions above and visible history.

A rule/candidate is a **derived relational hypothesis** when it combines facts through the frozen hypothesis grammar. Hypotheses are never inserted into `F` as though they were observations.

This separation is mandatory for provenance and circularity auditing.

### 4.10 Duplicate and contradiction handling

Duplicate facts with identical canonical predicate arguments and identical provenance are stored once.

The same canonical fact with different provenance retains the union of provenance records.

Contradictory conclusions are never merged. They remain distinct candidate outcomes and are handled by hypothesis validation; no majority vote, confidence heuristic, or human semantic rescue is permitted.

## 5. Exact candidate hypothesis space

The candidate space is finite and exhaustive.

### 5.1 Rule form

Each candidate has the form:

`IF A1 AND A2 AND ... AND Ak THEN C`

where:

- `1 <= k <= 6`;
- every `Ai` is one legal parent-grammar atomic predicate after legal role binding;
- `C` is exactly one of `ENABLES(target)` or `NONENABLES(contrast,target)` as a predicted consequence type;
- no disjunction;
- no negation;
- no recursion;
- no arithmetic;
- no function symbols;
- no free variables;
- no literal task-specific token constants;
- no seed, task ID, graph ID, dependency ID, coordinate, or hidden-state constant.

The consequence types are representation-level consequence categories already present in the parent causal grammar; the concrete relation instances are learned from A evidence.

### 5.2 Role variables

Only parent roles are permitted:

`target`, `intervention`, `contrast`, `context`.

Every role used must be bound by the candidate. Alpha-renaming is canonical and no anonymous role is allowed.

### 5.3 Atom admissibility

An atom is admissible only if every argument position has the type required by its parent predicate and all variables are among the candidate's bound roles.

`ENABLES` and `NONENABLES` may appear only in the argument position permitted by the parent grammar; they are not arbitrary user-defined functions.

### 5.4 Constants

No ground token constant is legal inside a reusable K_A rule.

The only constants are the finite grammar symbols themselves and the two permitted consequence categories.

### 5.5 Enumeration

Generate every syntactically admissible candidate satisfying Sections 5.1–5.4.

Canonicalize each candidate before insertion into the enumeration set:

1. alpha-normalize roles in first-appearance order using `target`, `intervention`, `contrast`, `context`;
2. canonicalize predicate argument order according to the parent grammar;
3. remove duplicate identical atoms;
4. sort conjunction atoms by canonical atom serialization;
5. reject any candidate with an unbound role;
6. canonicalize the implication direction;
7. serialize as UTF-8 JSON with sorted ASCII keys and no insignificant whitespace.

Candidates are enumerated in ascending bytewise order of this canonical serialization.

### 5.6 Equivalence and duplicates

Two candidates are equivalent iff their canonical serializations are byte-identical. No semantic equivalence beyond this canonicalization is assumed.

### 5.7 Candidate count and compute limit

The finite grammar enumeration is exhaustive. There is no arbitrary candidate-count cutoff.

A compliant implementation MUST materialize the canonical candidate set, verify finiteness, sort it, and evaluate every candidate. Internal computation is not part of the external interaction cost `E`.

If an implementation cannot complete exhaustive candidate evaluation within its own machine resources, the run is an `INSTRUMENTATION_FAILURE`; it must not prune the candidate set or silently substitute a heuristic search.

This makes resource failure explicit rather than scientifically changing the hypothesis space.

## 6. Hypothesis generation and validation

### 6.1 Per-A-task evidence

For each A task, compile `F` from its complete visible episode. For every candidate, enumerate every legal grounding of its roles over the task-local observable entities and determine whether its antecedent is satisfied at an observed event context and whether its consequence is subsequently observed within the same task.

Grounding order is lexicographic by canonical opaque-token bytes, with role order `target, intervention, contrast, context`.

### 6.2 Positive support

A candidate has positive support in an A task iff at least one legal grounding satisfies every antecedent atom and the predicted consequence is observed in the permitted post-intervention observation window.

A task contributes at most one unit to cross-instance support, regardless of the number of groundings.

### 6.3 Contradiction

A candidate is contradictory in an A task iff there exists a legal grounding satisfying the antecedent but the predicted consequence is falsified by the corresponding observable transition under the same local context.

A contradictory task contributes zero positive support and marks the candidate as contradicted for that task.

No contradiction is resolved by confidence, majority, or performance.

### 6.4 Cross-instance acquisition gate

A candidate enters K_A iff all conditions hold:

1. positive support in at least **two** A tasks;
2. the supporting A tasks are independently generated under distinct task seeds;
3. support remains after independent opaque token relabeling because the canonical rule is role-based rather than token-constant;
4. no supported A task is contradictory;
5. all provenance is valid and inspectable;
6. the candidate uses no B information;
7. the candidate is not a literal replay or task-specific table;
8. the candidate is valid under the parent grammar.

The threshold is exactly two supporting A tasks. There is no confidence threshold, learned score, validation-set selection, or B-based tuning.

### 6.5 Provenance

Each retained candidate records:

- canonical rule serialization;
- canonical rule hash using SHA-256;
- supporting A task seed list;
- for each supporting task: canonical event indices, role bindings, and source fact provenance;
- contradiction-task seed list, which must be empty for K_A admission;
- acquisition timestamp is forbidden as a decision field;
- learner-freeze version identifier.

Task seeds in provenance are audit metadata and are not supplied to the B decision procedure.

### 6.6 K_A ordering and freeze

K_A is the lexicographically sorted set of canonical retained candidates by `(canonical_rule_serialization, canonical_rule_hash)`.

Duplicate hashes with identical canonical serialization collapse to one object.

After the final A task is processed, K_A is serialized and frozen. No B observation, novelty value, B task, B outcome, or B result can modify it.

The frozen K_A artifact is the only A-derived semantic memory available to K_A during B.

## 7. Retrieval

Retrieval is a deterministic pure function of current visible history and frozen K_A.

### 7.1 Applicability

A K_A candidate is applicable iff there exists at least one legal role grounding over the current task's observable entities such that **every antecedent atom** is satisfied by current/history-derived facts and all temporal/provenance requirements are satisfied.

No partial match is applicable.

A missing atom, missing role binding, or ambiguous unresolved conflict makes the candidate non-applicable.

### 7.2 Grounding order

Enumerate role groundings in role order `target, intervention, contrast, context`, with candidate entity domains sorted lexicographically by opaque-token bytes.

The first grounding in canonical order is the canonical grounding for that candidate.

### 7.3 Ranking

Applicable candidates are ranked by:

1. greater number of antecedent atoms;
2. greater number of distinct role variables;
3. lexicographically smaller canonical rule serialization.

Only the highest-ranked candidate is selected.

If two candidates remain tied after criterion 3, their canonical serializations are identical and they are duplicates, so only one exists.

### 7.4 Conflicts

A conflict exists when two maximally ranked non-equivalent candidates require incompatible predicted consequence types for the same canonical applicable context.

On conflict, SERL emits `CONFLICT`, performs no knowledge-guided decisive action, and falls back to the base no-knowledge policy.

### 7.5 Retrieval count

At most one candidate is retrieved for a decision. The complete applicable set is evaluated to establish ranking and conflict status; only the selected candidate is exposed to the action policy.

### 7.6 Retrieval timing

Retrieval is evaluated before action selection. A retrieval discovered after an action cannot retroactively influence that action or receive transfer attribution for it.

## 8. Prediction

Prediction is an explicit protocol event, not a synonym for successful action.

### 8.1 Prediction input

Input is exactly:

`(selected K_A rule, canonical grounding, current visible context, selected consequence type)`.

No future observation/result is accessible.

### 8.2 Prediction output

The prediction is the selected consequence type for the grounded target, represented only by the protocol-defined consequence category.

The prediction event contains exactly the parent-required fields:

- abstraction identifier;
- canonical applicable-context hash;
- normalized-rule hash;
- predicted consequence type;
- event order.

### 8.3 Timing

Prediction is emitted immediately after retrieval and before the next learner-issued action.

If no rule is retrieved, no prediction event is emitted.

### 8.4 Verification

After the next relevant environment observation, verification checks whether the predicted consequence is observed exactly under the prediction's grounding and protocol-defined observation window.

Verification reads only the newly returned visible observation and retained provenance. It cannot inspect hidden state.

### 8.5 Mismatch

A mismatch does not revise K_A during B. The event is recorded as `PREDICTION_MISMATCH`, the candidate is not silently repaired, and the action policy falls back to the non-knowledge-guided path for subsequent decisions unless another independently applicable frozen candidate exists.

K_A remains immutable.

## 9. Action selection

### 9.1 Common base policy

All conditions use the same deterministic base policy.

At each decision:

1. if `terminated=true`, stop;
2. construct the current visible action set from `available_actions`;
3. if a valid pre-decisional knowledge-guided candidate identifies an intervention action that is currently available, that action is the knowledge-priority choice;
4. otherwise select the lexicographically smallest available ActionToken that has **not** already been successfully executed at the current local context;
5. if all currently available actions have already been successfully executed at that local context, select the lexicographically smallest currently available ActionToken.

The set of successfully executed actions at a local context is derived solely from `U` and `SAME_LOCAL_CONTEXT`; it is not hidden state.

### 9.2 Knowledge-guided eligibility

A K_A rule may influence action selection only if all of the following have occurred in this order:

`retrieval -> prediction -> action eligibility`

The predicted consequence must identify the currently available intervention role's concrete token through the current grounding.

No retrieved rule may directly inject an arbitrary action token.

### 9.3 Tie-breaking

All token ties use bytewise lexicographic order. There is no random tie-breaker.

### 9.4 Failed actions

Blocked actions are valid and consume one interaction-cost unit `E` under the parent protocol. Their resulting visible state/result enters `H,U`.

Illegal actions invalidate the condition and are never used as positive learner evidence.

Environment/instrumentation errors follow the parent terminal precedence.

### 9.5 Termination

SERL has no learner-specific success shortcut. It terminates only under the parent protocol's success, interaction cap, decision cutoff, task invalidation, environment error, or instrumentation-failure rules.

## 10. Controls

The controls are not weakened baselines. They share the same frozen visible interface, task reset, action-token ordering, terminal rules, and external interaction accounting.

### 10.1 K0

K0 has exactly the same SERL runtime and base action policy but receives an empty immutable K_A store.

It may not read A episodes, A facts, A candidate rules, or A provenance.

Its action selection therefore always follows Section 9.1 steps 4–5.

### 10.2 KR

KR receives the same raw A episodic records permitted by the parent control definition but is forbidden to aggregate across episodes, construct cross-episode relational abstractions, or form K_A-style reusable rules.

KR may inspect **one** stored A episode at a time. It may use only literal episode-local facts and exact token identities from that episode.

During B, KR cannot use any A episode whose task-local token vocabulary is not present in the current B context to construct a new relation. It cannot synthesize a token-independent rule.

Action selection is the same deterministic base policy as K0, with a literal episodic match permitted only when an exact currently observable token/context sequence exists. No generalized abstraction is permitted.

### 10.3 KS

KS receives no A-derived knowledge. It uses the same current B history and same deterministic base action policy as K0, plus the **fixed exhaustive search mechanism** specified for the common decision procedure. Search may operate only on current observable B history and candidate action sequences; it may not access A data or hidden B state.

Search tie-breaking is canonical bytewise action-sequence order. Search is capped by the parent decision budget; exhaustion produces the parent cutoff outcome rather than an unreported heuristic fallback.

### 10.4 KP

KP receives no A-derived knowledge. It uses the same current B interface and same decision procedure as K_A, but its knowledge store is empty and it uses only the fixed task-local action prior: uniform rank over currently available actions, resolved deterministically by canonical token order.

No learned A prior or cross-task statistic is available.

### 10.5 Direct replay

Replay stores literal A action sequences exactly as observed, including opaque token strings and sequence order. It may replay a sequence in B only if the exact sequence is legally executable from the current B visible state without token transformation.

No role substitution, token remapping, sequence edit, graph inference, or rule induction is permitted.

Replay success is counted only when the literal replay reaches the protocol goal under the predefined replay cutoff. It is a shortcut control, not a learner.

## 11. Decision units and budget

The scientific external cost `E` remains exactly the parent definition: learner-issued **valid action attempts** through success or terminal cutoff.

The following are distinct:

- action attempt: external interaction; if valid, increments `E`;
- blocked action: valid action attempt; increments `E`;
- successful action: valid action attempt; increments `E`;
- illegal action: not counted in `E`; condition invalidates;
- retry: a new action attempt and therefore a new `E` unit;
- retrieval: internal computation; does not increment `E`;
- hypothesis evaluation: internal computation; does not increment `E`;
- search/planning: internal computation; does not increment `E`;
- prediction: internal event; does not increment `E`;
- timeout: handled as instrumentation/environment failure according to the parent terminal precedence; it cannot be silently treated as success.

The parent decision cutoff of 120 decision units and interaction cap of 200 remain immutable.

A decision unit is one learner policy invocation following an environment observation. Internal enumeration/search within that invocation does not create additional decision units.

## 12. Randomness and deterministic execution

SERL itself uses no randomness.

The learner consumes **zero** values from `A_LEARNER`, `B_LEARNER_KA`, `B_LEARNER_K0`, `B_LEARNER_KR`, `B_LEARNER_KS`, `B_LEARNER_KP`, or `SEARCH_*` streams.

Task generation, label permutation, and statistical randomization remain governed by the parent protocol and their separately frozen manifests.

All ordering is deterministic:

- opaque-token bytes: ascending lexicographic;
- role order: `target, intervention, contrast, context`;
- event order: append order;
- candidate order: canonical serialization bytes;
- K_A order: canonical rule serialization then hash;
- action tie-break: token bytes;
- replay sequence order: literal recorded order.

No hash-map/set iteration order may be observable. Implementations must sort before any operation whose order can affect output.

## 13. Canonical serialization

Canonical JSON is UTF-8, sorted ASCII object keys, no insignificant whitespace, arrays in their explicitly specified order, integers represented in ordinary decimal JSON form, booleans as JSON booleans, and null as JSON null.

Opaque tokens remain strings and are never numerically parsed.

All hashes used for scientific identity are SHA-256 of the exact canonical UTF-8 byte representation.

## 14. Circularity audit

The following routes to pre-encoding the expected A→B result are explicitly prohibited and tested:

- semantic decoding of token hex values;
- use of hidden graph/task generator APIs;
- task/seed identifiers in decisions;
- hard-coded A→B rules;
- semantic X/Y/Z labels;
- preloaded K_A content;
- literal B solution tables;
- goal-distance/path heuristics;
- future observations;
- action-role metadata from the environment;
- cross-condition logs or caches;
- candidate pruning based on B outcomes;
- B-based K_A mutation;
- token constants inside reusable rules.

The parent grammar is a **representation language**, not supplied knowledge. K_A content is generated solely from A-visible evidence.

A circularity violation is an immediate scientific invalidation, irrespective of endpoint performance.

## 15. Scientific independence and no-toy-drift check

SERL deliberately has a narrow scope. Its explicit relational grammar is a controlled inductive bias; therefore the experiment can establish, at most, that this specified mechanism can or cannot acquire and transfer the tested relational abstraction under the frozen interface.

It cannot establish that the representation is universal, that symbolic relational learning is the unique route to intelligence, or that SERL contains the mechanisms needed for autonomous capability compounding.

The experiment therefore remains isolated as a **Phase 2.1 structural-transfer instrument** rather than a miniature final ACE system.

## 16. Kill tests

Before definitive execution, non-endpoint specification tests must reject SERL if any of the following occurs:

1. forbidden information is reachable from the learner API;
2. token relabeling changes role-generalized learned content for reasons other than within-task equality;
3. K_A provenance cannot be reconstructed exactly;
4. a K_A rule contains a task-specific token constant or literal B solution;
5. candidate enumeration is incomplete or order-dependent;
6. retrieval/prediction can occur only after the decisive action;
7. K_A and K0 do not share the same base policy;
8. a serialized state cannot reproduce the same next action;
9. two independent implementations disagree at a frozen boundary;
10. hidden process/cache/filesystem state changes a decision;
11. internal resource failure is silently converted into an algorithmic shortcut;
12. any rule is admitted because of B performance.

A favorable endpoint can never override a failed kill test.

## 17. Implementation-equivalence consequence

The separate `phase2_1_serl_equivalence_contract.md` is normative with this document. The contract defines the exact frozen boundaries at which two independent implementations must agree and the observational-equivalence rule for internal implementations.

## 18. Dependency closure

This learner freeze closes the learner-side choices that previously blocked downstream specification:

`SERL state -> observable facts -> finite candidate grammar -> A evidence -> K_A freeze -> retrieval -> prediction -> common action policy -> controls -> E accounting`.

The downstream scientific procedures may still be audited as separate artifacts, but they may not alter this learner definition, the parent protocol, or the learner's information boundary.

## 19. Independent-implementer self-test

An independent implementer must be able to reconstruct the learner from this document plus the parent protocol without asking the original author to choose:

- a history length;
- an entity definition;
- a predicate definition;
- a candidate grammar;
- an enumeration order;
- an acquisition threshold;
- a support rule;
- a provenance rule;
- a retrieval ranking;
- a prediction representation;
- an action tie-break;
- a learner RNG policy;
- a task reset policy;
- an E accounting rule.

Any such unresolved question is a conformance defect, not an invitation to choose a local default.

## 20. Completion statement

This document freezes SERL as the Phase 2.1 scientific instrument. It does not freeze the final ACE architecture and does not authorize the definitive experiment.

**Definitive Phase 2.1 experiment:** `NOT EXECUTED`  
**Phase 2.1 endpoint observed during this freeze:** `NONE`  
**Selection/tuning using endpoint results:** `NONE`
