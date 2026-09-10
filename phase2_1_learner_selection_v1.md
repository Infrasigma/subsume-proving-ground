# PHASE 2.1 SCIENTIFIC LEARNER SELECTION v1

**Target:** 1B.1 — Scientific Learner Selection
**Status:** `COMPLETE` for learner selection; does **not** authorize definitive Phase 2.1 execution
**Execution authority:** `NONE`
**Definitive Phase 2.1 experiment:** `NOT EXECUTED`
**Repository:** `Infrasigma/subsume-proving-ground`
**Branch:** `phase2-structural-transfer-20260910`
**Selection commit parent:** `46e11cc99e290e6d50f82e1c4f37e695153758eb`
**Parent scientific protocol:** commit `3b5891ec7c45055bc9a8bf7bdf178c07f1e03d78`, SHA-256 `06f39b7ead0dae272094cda82654e834be4cc22c`

## 1. Decision

### `PHASE2.1_LEARNER`

Phase 2.1 will use a **Structured Event-Relational Learner (SERL)**:

> an explicit task-local event ledger over the exact permitted observation/action/result stream; a deterministic observable-transition fact compiler; a deterministic, bounded, exhaustive relational-hypothesis enumerator over the parent protocol's frozen grammar; a provenance-bearing relational knowledge store; and a deterministic action policy in which K_A can affect decisions only through an explicitly retrieved, pre-decisional prediction event.

SERL contains **no neural network, embedding model, learned latent state, learned value function, reinforcement-learning policy, external pretraining, or general-purpose program language**.

This is a deliberate scientific instrument, not a claim that SERL is the final ACE architecture.

### Why this learner was selected

The target question is specifically whether **relational knowledge acquired from A can reduce interaction cost in structurally novel B**. The minimum learner therefore needs a representation in which relational knowledge can be (a) induced from permitted observations, (b) inspected, (c) generalized across opaque token identities, (d) retrieved before an action, and (e) causally attributed to a decision.

A learned latent representation would add an additional unidentifiable variable: the encoder itself could determine what counts as an entity/relation. A full latent world model would add model-learning and planning mechanisms that are not required by the question. A general program-synthesis system would add a second language-design problem beyond the frozen Phase 2.1 grammar. SERL keeps the representation language explicit while leaving the **content** of K_A data-dependent.

The methodological evidence supports this separation. Relational learners and object-relational model-learning systems demonstrate that explicit relational structure can support sparse-data generalization and zero-shot transfer; causal-transfer work explicitly separates abstract structure learning from instance-specific knowledge and planning. citeturn462058search0turn462058search2turn570938search0 Neural relational and neural-symbolic approaches can also learn relations, but they introduce representation-learning choices that are not necessary for the present hypothesis test. citeturn462058search1turn462058search3 Learned-model planners such as MuZero solve a broader problem and introduce substantially more consequential machinery. citeturn875792search0

The selection therefore follows **minimum sufficiency + interpretability + deterministic closure**, not expected endpoint performance.

## 2. What this decision does and does not establish

This decision establishes one preregisterable learner mechanism for Phase 2.1.

It does **not** establish:

- that SERL is the best AGI architecture;
- that relational symbolic systems are universally superior to neural systems;
- that Phase 2.1 will pass;
- that K_A will be nonempty;
- that transfer will occur;
- that the broader ACE architecture should use the same implementation.

Any of those claims require separate evidence.

## 3. Exact learner specification

### 3.1 Input boundary

At decision step `t`, SERL receives only the exact learner-visible observation prescribed by the parent protocol:

```json
{"state":{"location":"<LocationToken>"},"available_actions":["..."],"last_action":null,"last_result":null,"terminated":false}
```

or its exact subsequent instance with the five frozen top-level keys.

The learner may retain its own prior observations, issued actions, and returned results.

The learner is forbidden from receiving or deriving from the harness:

- semantic node IDs;
- graph topology;
- dependency lists;
- task seed/ID;
- coordinates;
- source/destination metadata;
- hidden enablement variables;
- future state;
- evaluator labels;
- condition identity other than the already-frozen knowledge condition;
- generation counters;
- other-process logs, timing, memory, filesystem, caches, or results.

A token's string identity is usable only as an opaque equality-bearing symbol within its task history.

### 3.2 Internal state

SERL maintains exactly four logical structures during a task:

1. **Observation ledger** `H`: an ordered immutable sequence of the exact observations returned by the environment.
2. **Action/result ledger** `U`: an ordered sequence of learner-issued valid action attempts and the exact returned results. An illegal action invalidates the condition as specified by the parent protocol and is not converted into a learner fact.
3. **Derived fact set** `F(H,U)`: deterministic relational facts compiled from visible history only.
4. **Frozen knowledge store** `K_A`: available only in the K_A condition and immutable during B.

No additional persistent state may affect decisions. Caches are permitted only as implementation accelerators and must be observationally equivalent to recomputation from `H`, `U`, and `K_A`.

### 3.3 Task reset

At the start of every task:

- `H` is empty;
- `U` is empty;
- `F` is empty;
- all task-local token equality maps are empty;
- all per-task search/order iterators are reset;
- K_A, when applicable, is loaded from the immutable pre-B freeze artifact;
- no state from the previous task is retained except the immutable frozen K_A object.

### 3.4 Observable-transition fact compiler

The compiler is a pure function of `(H,U)` and uses no randomness.

For each task-local action token `x` and each event index `t`, it may create only facts that follow from the visible interface:

- `ACTION_t(x)` iff `x` has appeared in an `available_actions` list or as a learner-issued action token by event `t`.
- `AVAILABLE_t(x)` iff `x` occurs in `available_actions` at event `t`.
- `BLOCKED_t(x)` iff the learner issued `x` at event `t` and the exact returned result status is the protocol's blocked status.
- `BEFORE(a,b)` iff the event containing action/effect evidence for `a` precedes the event containing that for `b`.
- `AFTER(a,b)` iff `BEFORE(b,a)`.
- `INTERVENES(y,x)` iff an event executes `y`, `x` is absent from the immediately preceding available-action set, and `x` is present in the immediately following available-action set.
- `OBSERVED_EFFECT(y,ENABLES(x))` iff the same one-step visible transition establishes `x` absent-before/present-after following execution of `y`.
- `NONENABLES(z,x)` iff an event executes `z`, `x` is absent from the immediately preceding available-action set, and `x` remains absent from the immediately following available-action set.
- `SAME_LOCAL_CONTEXT(a,b)` may be emitted only by the exact role-relative context normalization defined in the dependent 1B.2 closure; until that dependent artifact is frozen, SERL does not use this predicate for decisions.

The compiler never accesses hidden enablement variables. It infers only visible availability changes. A transition involving no visible change does not acquire an unobservable causal label.

The compiler retains exact event provenance for every derived fact: task-local event index, source observation indices, source action token, and source result object. Provenance is audit metadata and is not exposed as hidden semantic information to the learner.

### 3.5 Relational representation language versus learned content

The parent protocol's allowed predicate grammar is the **representation language**. It defines which statements are admissible.

The following are **learned content** and must be generated from A evidence:

- which ground token pairs instantiate the variables;
- which relational conjunctions are retained;
- which consequence type is associated with a learned abstraction;
- which cross-instance pattern is supported;
- which A tasks provide provenance.

No concrete A rule is supplied to SERL.

### 3.6 Candidate abstraction form

A candidate abstraction consists of:

```text
ABSTRACTION :=
    canonical_role_binding
    + canonical_conjunction_of_allowed_atoms
    + predicted_consequence_type
```

where:

- roles are restricted to the parent protocol roles (`target`, `intervention`, `contrast`, `context`);
- atom vocabulary and constructors are restricted to the frozen parent grammar;
- no new predicate is introduced;
- no executable code, arbitrary function, embedding, hash lookup, or task-specific table is allowed;
- the predicted consequence type is a protocol-defined consequence category, never a concrete hidden state value.

The dependent 1B.2 artifact will specify the exact canonical grounding and bounded enumeration details around this fixed learner form. No alternative learner family may be introduced through that downstream closure.

### 3.7 Hypothesis generation

At A-learning time SERL performs a deterministic exhaustive enumeration over the finite, parent-grammar-derived candidate space defined in 1B.2. The enumeration order is canonical serialization byte order after:

1. alpha-renaming role variables;
2. canonical argument ordering;
3. duplicate-atom removal;
4. conjunction operand sorting;
5. canonical implication direction;
6. canonical JSON serialization with UTF-8, sorted ASCII keys, and no insignificant whitespace.

There is no stochastic search. There is no validation-set tuning. There is no performance-guided pruning.

### 3.8 Hypothesis validation

A candidate is retained as A-derived knowledge only when all of the following hold:

1. its predicates are admissible under the parent grammar;
2. every required ground fact is derivable from permitted A histories;
3. its consequence is observed in every claimed positive support instance;
4. it has no contradictory consequence within the same canonical applicability context;
5. its provenance identifies at least two independently generated and independently relabelled A tasks;
6. it contains no B data, B task identifier, B outcome, or future B fact;
7. it is independently inspectable from its canonical serialization and provenance.

No candidate is accepted because it improves B performance; B is never visible during A learning.

### 3.9 K_A memory

After all A tasks have been processed, K_A is frozen as the canonical ordered set of all retained abstractions plus provenance/evidence metadata required by the parent protocol.

Raw A trajectories and concrete task-local labels are **not** part of K_A's usable B content. They may remain in an audit artifact outside the learner's B information boundary, but the K_A condition cannot read them during B.

K_A is immutable after B corpus freeze and before all B condition runs.

### 3.10 Retrieval

At each B decision point, SERL computes the current derived fact set from learner-visible history and evaluates all K_A abstractions for applicability.

Retrieval occurs only when an abstraction's applicability predicates are satisfied by the current visible context. The parent protocol's conflict precedence and specificity ordering remain authoritative.

SERL emits a retrieval event before any associated decisive action. Retrieval that occurs after the action cannot create a transfer attribution.

### 3.11 Prediction

For each retrieved abstraction, SERL emits exactly one candidate prediction event for its frozen predicted consequence type before the intervention/decisive action it recommends.

A prediction event contains the exact protocol-required fields:

- abstraction identifier;
- canonical applicable-context hash;
- normalized-rule hash;
- predicted consequence type;
- event order.

Prediction cannot inspect future observation, result, or hidden simulator state.

### 3.12 Action selection

SERL uses one base deterministic action-selection policy shared by K_A, K0, KR, KS, and KP, with only the protocol-defined knowledge availability differing between conditions.

Base policy:

1. If `terminated=true`, stop.
2. If an admissible K_A prediction event exists and identifies a currently available intervention action, the action is eligible for the knowledge-guided priority path.
3. Otherwise, choose the currently available action with the smallest canonical opaque-token byte ordering among actions not yet successfully executed at the current task-local context.
4. If every currently available action has already been successfully executed at that exact visible context, choose the smallest canonical opaque-token byte ordering among all currently available actions.

The precise event-level equivalence test and condition matching are frozen in 1B.2; K_A may influence only the priority decision supported by an explicit pre-decisional retrieval+prediction chain.

No learned value function, utility estimator, policy gradient, exploration bonus, hidden heuristic, or task-specific tie-breaker is permitted.

### 3.13 Randomness

SERL's learner decision procedure uses **no learner-side randomness**.

Therefore the reserved learner streams in the previous closure artifact remain unused by SERL unless a later dependent specification demonstrates a mathematically necessary role without changing the learner family. The preferred implementation is zero learner-side random calls.

Task generation, token permutation, and statistical randomness remain governed solely by the parent protocol and later implementation-freeze artifacts.

### 3.14 Budget

SERL's external interaction cost is exactly the parent protocol's `E`: learner-issued valid action attempts through success or terminal cutoff.

Internal hypothesis enumeration is not included in `E`.

The parent decision cutoff remains authoritative. A computational timeout may invalidate a trial as `INSTRUMENTATION_FAILURE` only under the exact environment rule later frozen in 1B.2; an implementation cannot silently turn excessive internal work into a counted external action.

### 3.15 Termination

SERL terminates when:

- the environment returns `terminated=true` after a successful goal state;
- the parent decision cutoff is exhausted;
- the parent interaction cap is exhausted;
- a protocol/environment/instrumentation invalidation occurs.

No learner-specific early-success criterion is permitted.

### 3.16 Failure handling

The parent terminal-outcome precedence is immutable. SERL must not reinterpret blocked actions, illegal actions, environment errors, or instrumentation failures as successful knowledge updates.

A blocked action is a valid interaction and updates the visible history. An illegal action invalidates the condition as specified by the parent protocol.

### 3.17 Serialization

The frozen learner state is serialized only for audit/restart and must be reconstructible exactly from:

`protocol identifier + task-local H + task-local U + K_A (when applicable) + fixed SERL version`

No hidden runtime object identity, memory address, process-local pointer, or noncanonical map order may affect a decision.

## 4. Circularity audit

**Result: PASS, conditional on the dependent closure preserving these prohibitions.**

The representation grammar is supplied by the scientific protocol, but the learned relational content is not. SERL does not receive the intended A→B rule, B graph structure, semantic relation labels, or hidden causal variables.

The strongest remaining circularity risk is the observation-to-predicate compiler. It is therefore treated as a separately auditable dependency rather than hidden inside the learner.

## 5. Minimum-sufficiency classification

### Required for Phase 2.1

- finite history of visible observations/actions/results;
- task-local opaque token identity;
- observable transition extraction;
- relational hypothesis representation;
- cross-instance abstraction;
- provenance;
- pre-decisional retrieval/prediction;
- deterministic action choice;
- exact reset and terminal accounting.

### Optional but excluded from SERL

- neural embeddings;
- probabilistic latent states;
- learned value functions;
- learned exploration policies;
- deep model-based rollouts;
- neural search guidance;
- generic program libraries;
- offline pretraining.

These could improve some future systems but are not needed to test the present hypothesis.

### Later-stack ACE mechanisms

- unsupervised learned object discovery;
- open-ended causal hypothesis formation;
- active experiment design beyond the protocol's fixed interventions;
- learned world models;
- hierarchical skill discovery;
- self-diagnosis/model revision;
- architecture search;
- autonomous implementation;
- verified self-modification;
- recursive capability compounding.

## 6. Kill test for SERL

SERL is scientifically rejected before execution if any of the following can be established on non-definitive fixtures:

1. a forbidden input is necessary for a required decision;
2. two equivalent opaque-token relabelings produce different frozen relational content for reasons other than exact token equality semantics;
3. the learner cannot produce auditable K_A provenance from at least two independent A tasks;
4. K_A contains raw A task-specific answer tables or concrete B solutions;
5. retrieval or prediction can occur only after the decisive B action;
6. the same frozen learner cannot be instantiated for controls without weakening or strengthening the base decision mechanism;
7. deterministic recomputation from serialized state does not reproduce the same next action;
8. two independent implementations disagree at a frozen external boundary after applying the agreed equivalence contract.

A favorable Phase 2.1 result can never waive a failed kill test.

## 7. Red-team

### Objection 1: another competent researcher could choose a different learner.

**Yes.** Neural relational learning, causal-transfer Bayesian learners, or program-induction systems are defensible research mechanisms. The resolution is not that SERL is universally correct; it is that Phase 2.1 is explicitly a controlled test of *observable relational abstraction and transfer*. SERL is the smallest mechanism that isolates that mechanism without adding an encoder/world-model/search-policy confound.

### Objection 2: does SERL privilege K_A?

**Potentially, by design, but symmetrically.** The protocol asks whether a condition containing valid K_A knowledge reduces E. K0 and controls must use the same base action policy and budgets. K_A gains only the information the hypothesis under test claims exists.

### Objection 3: does the predicate language encode the answer?

**The language encodes the hypothesis class, not the learned instance.** The parent protocol already freezes the relational vocabulary. Supplying a representation language is not equivalent to supplying a rule. The boundary fails only if the compiler inserts the intended rule as fact rather than deriving facts from observations.

### Objection 4: could a simpler learner answer the question?

A purely reactive opaque-token policy cannot produce protocol-valid reusable relational K_A without an additional abstraction mechanism. A simple transition table can memorize instances but cannot satisfy the required cross-instance, label-invariant knowledge gate. SERL is therefore close to the minimum mechanism that can satisfy the stated target.

### Objection 5: could a more powerful learner provide stronger evidence?

Not for this specific experiment. A more powerful learner would make a positive result harder to attribute to the narrow relational mechanism because success could arise from additional representation, prediction, planning, or optimization capacities. Such alternatives belong in later comparative experiments.

### Objection 6: can failure still be informative?

Yes. If the protocol and closure are valid, a SERL failure is evidence that this specific minimal relational mechanism did not reduce B cost under the frozen task construction. It is not evidence that all relational learners or ACE architectures fail.

### Objection 7: can success be informative?

Yes, provided the transfer attribution chain, controls, token invariance, and all nine Phase 2.1 PASS criteria remain valid. Success would support the narrower proposition that cross-instance relational knowledge acquired under this learner can causally reduce the measured interaction cost.

## 8. Two-implementation reproducibility contract

Two independent implementations of SERL are equivalent only if, for the same frozen inputs, they produce byte-identical values at these boundaries:

1. task-local observation ledger;
2. derived fact set;
3. canonical candidate abstraction order;
4. A acceptance/rejection decisions;
5. frozen K_A serialization;
6. B applicability set;
7. retrieved abstraction;
8. prediction event;
9. next external action;
10. terminal outcome and E;
11. attribution inputs.

Internal implementation differences are allowed only when the boundaries above are identical. Statistical agreement on aggregate averages is insufficient.

## 9. Dependency graph after learner selection

```text
SERL
  |
  +--> observable event/fact compiler
  |       |
  |       +--> K_A candidate enumeration
  |       |       |
  |       |       +--> provenance / cross-task support / freeze
  |       |
  |       +--> B retrieval evaluator
  |               |
  |               +--> prediction
  |                       |
  |                       +--> attribution
  |
  +--> matched base action policy
          |
          +--> K_R / K_S / K_P controls
          |
          +--> interaction + decision budget
```

### Downstream components now mechanically constrained

- K_A must be produced from SERL's observable event ledger and frozen parent grammar.
- Retrieval must inspect only current SERL-derived visible facts plus frozen K_A.
- Prediction must precede any knowledge-attributed decisive action.
- Controls must retain the same SERL base decision procedure and differ only in their explicitly frozen knowledge access.
- Direct replay cannot add any transformation unavailable to SERL.
- Attribution must be defined over the same SERL action-policy boundary.
- Decision and interaction accounting must count the same external action events.

### Still independently unresolved

These are intentionally deferred to Target 1B.2 rather than hidden inside learner selection:

- exact finite candidate-space enumeration limits and ordering over the full parent grammar;
- exact role-relative `SAME_LOCAL_CONTEXT` normalization;
- full retrieval partial-match/conflict semantics;
- exact control procedures for KR/KS/KP;
- direct-replay matching/transformation rule;
- exact attribution ablation procedure;
- exhaustive event-level budget table;
- statistical reference implementation;
- two-implementation executable equivalence harness.

Those are dependent closure decisions, not reasons to reopen the learner family choice unless they reveal that SERL cannot satisfy the parent protocol without a forbidden assumption.

## 10. Randomness decision

The learner itself is deterministic and consumes no random stream.

This eliminates a major reproducibility ambiguity from the previous closure candidate. Any future use of learner-side randomness would constitute a new scientific-design choice and therefore reopen Target 1B.1 rather than being introduced silently.

## 11. Why this is not performance-tuned

No Phase 2.1 A/B endpoint, B novelty outcome, K_A transfer count, or E result was used to choose SERL.

The selection was made from the task definition and methodological evidence concerning relational abstraction, causal transfer, small-data relational learning, and model-based planning. No candidate was run on the definitive 100-task Phase 2.1 evaluation.

## 12. `LONG_TERM_ACE_ARCHITECTURE`

The long-term ACE architecture remains the broader research program:

`representation -> persistent entities/relations -> world model -> causal hypothesis formation -> active experimentation -> abstraction -> memory -> skills -> hierarchy -> self-diagnosis -> model revision -> transfer -> self-model -> capability discovery -> architecture search -> autonomous implementation -> verified self-modification -> recursive capability compounding`

SERL is not asserted to be that architecture. It is one controlled experimental mechanism for the narrower Phase 2.1 transfer question and may later be replaced or subsumed.

## 13. Completion checklist

- [x] candidate learner classes researched
- [x] relevant primary evidence collected
- [x] circularity checked
- [x] minimum-sufficiency analyzed
- [x] candidate comparison completed
- [x] scientific selection rationale documented
- [x] learner specification is implementation-exact at the learner-family level
- [x] randomness rule closed: learner-side randomness = none
- [x] budget boundary identified; exact dependent event table deferred to 1B.2
- [x] failure modes documented
- [x] kill test defined
- [x] future ACE architecture separated from Phase 2.1 learner
- [x] dependency impacts documented
- [x] no definitive Phase 2.1 endpoint observed
- [x] no post-hoc model selection occurred
- [x] decision is independently auditable

## 14. Status

`COMPLETE`

This completion applies **only to Target 1B.1 — Scientific Learner Selection**.

It does not mark the overall Phase 2.1 implementation freeze complete, and it does not authorize Target 1A or the definitive experiment.

## 15. Next target

`TARGET 1B.2 — Freeze dependent K_A / retrieval / prediction / controls / attribution procedures`

Target 1A remains prohibited until the broader closure gate is satisfied, including exact dependent specifications and equivalence validation.

## References

1. Stella, G. & Loguinov, D. “QORA: Zero-Shot Transfer via Interpretable Object-Relational Model Learning.” ICML 2024. https://proceedings.mlr.press/v235/stella24a.html
2. Edmonds, M. et al. “Theory-based Causal Transfer: Integrating Instance-level Induction and Abstract-level Structure Learning.” AAAI 2020. https://mjedmonds.com/projects/OpenLock/AAAI20_OpenLockLearner.html
3. Kipf, T. et al. “Neural Relational Inference for Interacting Systems.” ICML 2018. https://proceedings.mlr.press/v80/kipf18a.html
4. Dong, H. et al. “Neural Logic Machines.” ICLR 2019. https://research.google/pubs/neural-logic-machines/
5. Das, M. D. et al. “Few-Shot Induction of Generalized Logical Concepts via Human Guidance.” Frontiers in Robotics and AI 2020. https://pmc.ncbi.nlm.nih.gov/articles/PMC7805948/
6. Lake, B. M. et al. “Human-level concept learning through probabilistic program induction.” Science 2015. https://pubmed.ncbi.nlm.nih.gov/26659050/
7. Ellis, K. et al. “DreamCoder: growing generalizable, interpretable knowledge with wake-sleep Bayesian program learning.” Philosophical Transactions B 2023. https://pubmed.ncbi.nlm.nih.gov/37271169/
8. Schrittwieser, J. et al. “Mastering Atari, Go, chess and shogi by planning with a learned model.” Nature 2020. https://www.nature.com/articles/s41586-020-03051-4
9. He, Y.-B. & Geng, Z. “Active Learning of Causal Networks with Intervention Experiments and Optimal Designs.” JMLR 2008. https://www.jmlr.org/papers/v9/he08a.html
