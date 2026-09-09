# PHASE 2.1 PREREGISTERED SCIENTIFIC PROTOCOL — DRAFT FOR INDEPENDENT AUDIT

**Mode:** Design only. No execution is authorized by this document.

**Repository:** `Infrasigma/subsume-proving-ground`

**Historical references**
- Phase 1 verified commit: `5faebd91cf16ffd7b932ec982c6396c57df138a0`
- Phase 2 branch: `phase2-structural-transfer-20260910`
- Phase 2 tip: `ebe3b99f0a1b6abc85b7c0121229a33d0542ddc7`
- Phase 2 verdict: `FAIL`

**Integrity rule:** Phase 1 and all frozen Phase 2 artifacts remain untouched. This document is a new protocol draft only. It contains no executable experiment code, no generated corpus, and no empirical result.

## 1. Hypothesis

### Scientific hypothesis
An agent can extract a reusable relational abstraction from prior experience in one structurally different task family and use that abstraction to reduce future interaction cost in a novel task family.

### Primary operational hypothesis
\[
C(B^+\mid K_A) < C(B^+\mid K_0)
\]

where:
- `K_A` = genuinely acquired relational knowledge from Family A;
- `K_0` = matched no-A-knowledge counterfactual;
- `B+` = structurally novel Family B;
- `C` = frozen environment interaction cost `E`.

The hypothesis is not broadened to general intelligence, AGI, representation learning in general, or semantic similarity.

## 2. Family A definition

Family A is a deterministic synthetic family of dependency-navigation environments designed so that the reusable mechanism is causally observable through intervention rather than merely encountered as a fixed sequence.

### 2.1 State
An A task consists of a finite transition system. A state contains:
- the current environment location/state;
- latent enablement status for dependency-bearing transitions;
- the task's ordinary observable state representation.

The learner receives only the ordinary observation returned by the environment and the outcome of each attempted action. The learner does not receive the underlying dependency graph or latent prerequisite annotations.

### 2.2 Actions
Each state exposes a finite set of opaque action labels. Labels are task-local and independently permuted for each task. An action may:
- move/transition to another state;
- attempt a dependency-bearing transition;
- perform an intervening action that changes the availability of another transition;
- be irrelevant to the target dependency.

Action labels do not encode semantic roles.

### 2.3 Blocked transition
A target transition `X` is **blocked** when an attempt to execute X produces an ordinary environment outcome indicating that X is currently unavailable and leaves the target transition unavailable.

### 2.4 Enabling action
An action `Y` is an enabling intervention for X when, from a comparable context:
1. X is initially blocked;
2. Y is executed;
3. the environment state changes in a way that makes X available;
4. a subsequent attempt of X succeeds.

Y must not simply be the unique mandatory next step on a fixed path to X. At least one alternative action `Z` must be available at the relevant decision point and must provide a non-enabling contrast for the same target relation.

### 2.5 Ordinary observations
The causal evidence must arise only from interaction outcomes such as:
- action accepted/rejected;
- transition succeeded/failed;
- resulting observable state;
- subsequent availability/success of X.

The environment must not expose prerequisite names, causal labels, answer keys, solution metadata, or an explicit statement that Y enables X.

### 2.6 Recurrence
The same abstract relation must recur across independently generated A instances. The relation is defined by roles and transition consequences, not by concrete identifiers.

Minimum necessary recurrence for acquisition validity: **two independently generated and independently relabeled A tasks**, each providing an independently observed instance of the same dependency relation with an intervention contrast. This is a minimum floor, not a claim that two observations establish a high-confidence causal law. Additional repeated contrasts are an optional strengthening.

### 2.7 Independent relabeling
For every A task, node/state identifiers and action labels are independently permuted using task-local randomization. The learner cannot rely on identifier equality across tasks.

### 2.8 Task randomization
Task topology, concrete identifiers, and placement of dependency relations are generated from the frozen A generator and frozen seed manifest. Randomization rules, seed set, and generator version must be fixed before execution.

No randomization parameter may be selected after inspecting experimental outcomes.

### 2.9 Learner information boundary
The learner may observe only information an ordinary agent interacting with the environment would receive: current observation, chosen action, and returned action/environment outcome. It may retain its own interaction history.

The learner may not observe:
- latent prerequisite variables;
- generator state;
- random seed meaning;
- task construction metadata;
- answer keys;
- future states not reached through interaction;
- B information during A acquisition.

## 3. Minimum causal observability evidence

The protocol distinguishes temporal precedence from intervention.

**A. Mere precedence:** Y happens before X in a successful trajectory.

**B. Interventional evidence:** changing whether Y is executed changes whether X becomes available, while an appropriate alternative action does not produce the same change.

A relation qualifies for abstraction acquisition only under B.

### Necessary minimum
Across at least **two independently generated and independently relabeled A instances**, each relation proposed for abstraction must have:
- an observed blocked X state;
- an intervention Y;
- a subsequent X attempt with a successful availability/outcome change;
- at least one non-enabling contrast action Z under a comparable blocked-X condition;
- observation records sufficient to reconstruct the before/after availability difference.

This is the **NECESSARY** minimum. It is selected because one instance can be explained by task-specific coincidence, while two independently relabeled instances provide the minimum cross-instance invariance test without making a claim of statistical generality from a tiny sample.

**OPTIONAL IMPROVEMENT:** multiple positive and negative interventions per relation and more than two independent A instances. These strengthen causal identification but do not alter the hypothesis or minimum gate.

## 4. K_A representation

A candidate abstraction is a compact relational record, not an episode or trajectory.

Conceptual schema:
- `abstraction_id`: unique local identifier;
- `source_A_task_ids`: set of A tasks supporting the abstraction;
- `source_A_observation_ids`: exact observation/event references supporting it;
- `relational_rule`: abstract relation expressed in role terms, e.g. blocked target transition + intervening action + subsequent enabled target transition;
- `applicability_conditions`: observable relational conditions under which the rule may be considered;
- `prediction_rule`: testable predicted consequence of applying the relation;
- `evidence_record`: supporting/contrasting observations and cross-instance support; confidence may be represented only as a record of evidence, not as an outcome-tuned scalar;
- `creation_order`: deterministic order/index at which the abstraction was created;
- `freeze_status`: must become frozen before any B exposure.

The representation must not contain:
- A task ID as the rule itself;
- concrete A state IDs as the reusable mechanism;
- concrete action IDs as semantic roles;
- a concrete A trajectory as the abstraction;
- goal-state lookup tables;
- B task IDs or B metadata;
- hidden answer keys.

A candidate abstraction that is formally nonempty but is task-specific, episodic, answer-key-like, or not label-invariant fails the acquisition gate.

## 5. Strict A acquisition gate G_A

The automated pre-B acquisition gate is:

`G_A = nonempty AND provenance-valid AND cross-instance-supported AND relational/non-episodic AND label-invariant AND B-independent AND independently-inspectable AND frozen-before-B`

Each predicate must be mechanically auditable:

1. **Nonempty:** at least one candidate abstraction record exists.
2. **Provenance-valid:** every rule points to concrete A observation records and task IDs; those records predate rule creation.
3. **Cross-instance-supported:** every qualifying reusable rule has evidence from at least two independent A task instances.
4. **Relational/non-episodic:** the rule is represented in role/relationship terms and cannot be evaluated solely by exact task, state, action, or sequence identity.
5. **Label-invariant:** the same rule matches independently relabeled A instances without relying on shared identifiers.
6. **B-independent:** the rule store and creation process have no access to B identifiers, topology, metadata, generator state, solutions, or B outcomes.
7. **Independently inspectable:** an auditor can reconstruct the rule from its cited A evidence and inspect its contents without hidden state.
8. **Frozen-before-B:** the abstraction store is cryptographically or otherwise immutably frozen before B generation is exposed to the B learner/decision process.

Failure of any predicate means **G2 acquisition validity failure** and requires cancellation before B.

## 6. Family B definition

Family B remains independently generated and structurally novel relative to A.

The existing Phase 2 structural novelty definition is frozen and is not changed:
- unlabeled degree profile;
- reachable-distance profile;
- dependency-placement structure;
- multiset Jaccard-based structural comparison;
- no string/name similarity contribution.

The novelty threshold remains:

`mean structural novelty > 0.50`

No threshold or metric change is permitted in Phase 2.1.

### B independence
B generation must be performed by a separately specified B generator whose inputs do not include:
- A trajectories;
- A learned abstractions;
- A task-specific A solution;
- expected B results;
- B-specific answer keys supplied to the learner;
- post-hoc selection based on novelty or performance.

The generator may use the preregistered family-level design and its frozen seed manifest. It must not condition individual B tasks on observed A learning outcomes.

Before B exposure, the following must be frozen:
- B generator source and hash;
- B seed manifest;
- generated B corpus or deterministic corpus-generation record;
- novelty implementation;
- novelty threshold;
- A/K_A freeze artifact;
- control configuration;
- leakage-audit result.

## 7. Pre-B novelty gate

Required order:

`generate B -> calculate frozen novelty -> evaluate threshold -> freeze B -> permit transfer exposure`

If `mean novelty <= 0.50`, the run cannot be interpreted as a valid test of the preregistered B-transfer hypothesis. **G3 fails and transfer interpretation is cancelled.**

Required gate artifacts:
- B generator source hash;
- seed manifest hash;
- B corpus/configuration hash;
- per-task novelty values;
- aggregate mean novelty;
- exact frozen metric definition/version;
- threshold value and comparison result;
- timestamp/order proving novelty was evaluated before transfer exposure;
- immutable B freeze marker.

No task may be removed, regenerated, or substituted after novelty inspection to obtain a passing mean.

## 8. Genuine transfer event

A transfer event exists only when all of the following occur in order:

1. an A-derived abstraction exists and passed G2;
2. that abstraction is retrieved during B;
3. it is applied to a B situation matching its applicability conditions;
4. it generates a testable prediction;
5. an intervention/action is taken on the B environment on the basis of that application;
6. the predicted environmental consequence is observed;
7. the prediction is verified against the observed consequence;
8. attribution audit finds that the event cannot be explained merely by a B-only heuristic available equally to K0.

### Conceptual machine-readable transfer event schema
- `abstraction_id`
- `source_A_task_ids`
- `source_A_observation_ids`
- `B_task_id`
- `B_local_context`
- `retrieval_event`
- `application_event`
- `prediction`
- `intervention`
- `predicted_outcome`
- `observed_outcome`
- `verification_result`
- `attribution_audit`
- `interaction_cost_before`
- `interaction_cost_after`
- `event_order`

`retrieval` means the stored A abstraction was selected for consideration. It is not itself transfer.

`prediction` means the abstraction generated a falsifiable expected B consequence. It is not itself transfer.

`verification` means the observed B consequence matched the prediction under the predefined verification rule. It is necessary but not sufficient for transfer attribution.

`transfer` means the complete chain occurred and attribution passed.

## 9. Controls

### K0 — no-A-knowledge counterfactual
Receives the same B tasks, observations, action space, solver procedure, budgets, and randomization framework, but no A-derived knowledge. Controls for the baseline interaction cost of solving B without prior knowledge.

Cannot receive A abstractions, A episodic traces, or A-derived B-specific information.

**Necessary.** It is the primary counterfactual for the hypothesis.

### KR — episodic-retention control
Retains A episodic records in the learner object but does not supply extracted abstraction rules to the B decision process.

Cannot receive the relational abstraction store.

Controls whether any observed effect is explainable by raw A episode retention/replay rather than abstraction.

**Necessary.** It distinguishes relational reuse from episodic memory.

### KS — search/computation-budget control
Uses the same no-knowledge B solver and fixed action/search budget as K_A.

Cannot receive A-derived knowledge.

Controls for an advantage caused merely by extra computation or search budget.

**Necessary.**

### KP — solver-prior control
Uses the same solver, randomization, and task prior as K_A but without A-derived B knowledge.

Cannot receive B-specific answer information or A-derived abstraction.

Controls for solver priors/randomization differences rather than knowledge transfer.

**Necessary.**

### Direct A-trajectory replay
Attempts literal A action sequences on B under the predefined replay procedure.

Cannot transform those trajectories into relational rules.

Controls whether any B performance could be explained by direct trajectory reuse.

**Necessary.** under the existing Phase 2 contract.

No additional control is required unless a new concrete confound is identified before protocol freeze; adding a control may not be used to alter the primary hypothesis or endpoint.

## 10. Leakage audit contract

Every item below must be audited before transfer interpretation. A failure invalidates scientific transfer interpretation.

| Leakage route | Exact audit question | Pass condition | Failure consequence |
|---|---|---|---|
| Task IDs | Can IDs encode family, solution, role, or correct action? | IDs are opaque/randomized and carry no semantic answer information. | Invalid interpretation. |
| Node labels | Can node names reveal roles or matching across tasks? | Labels are independently permuted and semantically opaque. | Invalid interpretation. |
| Action labels | Can action names encode Y/X/Z roles? | Action labels are independently permuted and opaque. | Invalid interpretation. |
| Generator metadata | Can learner inspect generator internals or latent variables? | No metadata is exposed through the interaction boundary. | Invalid interpretation. |
| Seeds | Can seed values encode task answers or enable direct reconstruction by learner? | Seeds are only reproducibility identifiers and are inaccessible to the learner. | Invalid interpretation. |
| Ordering artifacts | Does generation/order reveal the correct intervention? | Ordering has no deterministic answer-carrying relation. | Invalid interpretation. |
| Topology shortcuts | Does local B topology directly identify the correct action without K_A? | Any B-only structural heuristic is equally available to K0 and cannot encode A-derived relation. | Invalid transfer attribution; if unavoidable, redesign before execution. |
| Hidden solutions | Does any condition receive an answer key or latent solution? | No learner condition receives hidden solution information. | Invalid interpretation. |
| A trajectories | Can B decisions replay concrete A trajectories? | No concrete trajectory is exposed as a B solution; replay is separately controlled. | Invalid interpretation. |
| B metadata | Does K_A receive B task IDs, topology summaries, generator state, or future outcomes before retrieval? | None available before ordinary B observation. | Invalid interpretation. |
| B topology-derived answer keys | Does topology contain a deterministic shortcut that is effectively an answer key? | No unique answer encoding unavailable to K0. | Invalid transfer attribution. |
| Solver priors | Do K_A and controls differ in fixed priors unrelated to A learning? | Priors are matched or explicitly audited. | Invalid comparison. |
| Randomization | Do conditions receive different random streams in a way that affects outcome? | Randomization protocol is fixed and matched as specified. | Invalid comparison. |
| Cross-task memorization | Can exact A task structure be recognized in B? | Independent relabeling and structural novelty prevent identity matching. | Invalid transfer attribution. |
| State/action indexing | Do numerical indices carry stable role information? | Indices are task-local and independently permuted. | Invalid interpretation. |
| Deterministic generator quirks | Does a fixed generator artifact correlate with the answer? | No learner-visible deterministic quirk uniquely supplies the answer; audited before execution. | Invalid interpretation or cancellation. |

## 11. Statistical endpoint

The primary endpoint remains exactly the Phase 2 endpoint:

\[
D = E_{K_0} - E_{K_A}
\]

Positive D favors the hypothesis. Negative D opposes it.

Preserved statistical procedure:
- paired two-sided sign-flip permutation test;
- 100,000 permutations;
- deterministic permutation seed `20260911`;
- 20,000 deterministic percentile-bootstrap resamples;
- deterministic bootstrap seed `20260912`;
- original Phase 2 PASS criteria unchanged.

A statistically significant result with `D < 0` is **not** evidence for the hypothesis. It is evidence in the opposite direction under a valid experiment.

A validity-gate failure means the transfer hypothesis was not validly tested by that run, regardless of any nominal p-value.

## 12. Validity hierarchy

The frozen ordering is:

`G0 repository/provenance validity -> G1 A observability validity -> G2 A acquisition validity -> G3 B novelty validity -> G4 leakage validity -> G5 control validity -> primary transfer analysis`

### G0 — repository/provenance
Pass requires all pre-execution protocol/source/generator/configuration/seed/environment artifacts to be frozen and independently inspectable.

Failure: **INVALID EXPERIMENT; CANCEL before execution.**

### G1 — A observability
Pass requires the generated A environment to permit the specified blocked-X -> Y -> enabled-X intervention and non-enabling contrast, using ordinary observations.

Failure: **INVALID EXPERIMENT; CANCEL before acquisition interpretation.**

### G2 — A acquisition
Pass requires the complete G_A conjunction.

Failure: **CANCEL BEFORE B.** No B transfer exposure or transfer inference is permitted.

### G3 — B novelty
Pass requires mean structural novelty > 0.50 under the unchanged metric.

Failure: **CANCEL BEFORE TRANSFER INTERPRETATION.**

### G4 — leakage
Pass requires the complete leakage audit with no material leakage route.

Failure: **INVALIDATE SCIENTIFIC INTERPRETATION.** A numerical result cannot rescue a leakage failure.

### G5 — controls
Pass requires control conditions to have received the specified information boundaries and matched budgets/procedures.

Failure: **INVALID EXPERIMENT for causal transfer interpretation.** Do not reinterpret the remaining conditions to obtain a result.

## 13. PASS/FAIL classification

Three outcomes are mandatory.

### 13.1 INVALID EXPERIMENT
One or more scientific validity gates fail. No transfer hypothesis inference is permitted.

### 13.2 VALID NEGATIVE RESULT
All validity gates pass, but one or more frozen transfer criteria fail.

This includes a statistically significant effect in the wrong direction.

### 13.3 VALID POSITIVE RESULT
All validity gates pass and every frozen Phase 2 transfer criterion passes:
- novelty > 0.50;
- mean(E_KA) < mean(E_K0);
- p < 0.05;
- positive transfer >= 70%;
- negative transfer <= 20%;
- K_A final performance >= K0;
- retrieved+verified transfer >= 80%;
- direct replay <= 10%;
- KR does not reproduce the K_A effect.

No partial PASS is permitted.

## 14. Parameter-selection discipline

All numerical parameters already frozen in Phase 2 remain unchanged, including the novelty threshold, statistical resample counts, seeds, and PASS criteria.

Any new Phase 2.1 numerical parameter must be justified before execution according to:
1. **Scientific reason:** why the parameter is required to instantiate the hypothesis or its validity test;
2. **Classification:** NECESSARY REPAIR or OPTIONAL IMPROVEMENT;
3. **Role:** whether it affects hypothesis meaning, statistical power, or implementation feasibility;
4. **Pre-outcome freeze:** value/range fixed before execution and not changed after any result is observed;
5. **No outcome optimization:** expected probability of PASS is not an admissible justification.

The only new numerical minimum explicitly required by this protocol is the causal-observability recurrence floor of **two independent, independently relabeled A instances per qualifying abstraction**, classified **NECESSARY**. Its justification is minimum cross-instance invariance, not expected performance.

Optional stronger repetition is not allowed to replace the minimum after seeing outcomes.

## 15. Implementation-neutral pseudocode

```text
FREEZE protocol, hypotheses, metrics, thresholds, seeds, controls, audit rules
VERIFY G0 repository/provenance prerequisites
IF G0 fails: CANCEL

GENERATE Family A from frozen A generator and seed manifest
VERIFY G1 causal observability requirements
IF G1 fails: CANCEL

RUN A learning under the frozen information boundary
BUILD candidate K_A abstractions from A observations only
VERIFY G2 acquisition conjunction
IF G2 fails: CANCEL BEFORE B
FREEZE K_A

GENERATE Family B independently from frozen B generator and seed manifest
CALCULATE frozen structural novelty metric
VERIFY mean novelty > 0.50
IF G3 fails: CANCEL BEFORE TRANSFER INTERPRETATION
FREEZE B corpus/configuration

RUN leakage audit and verify G4
IF G4 fails: INVALIDATE SCIENTIFIC INTERPRETATION

INSTANTIATE K0, KR, KS, KP and direct-replay controls
VERIFY G5 information boundaries, matching and budgets
IF G5 fails: INVALID EXPERIMENT

RUN B transfer conditions under frozen protocol
LOG every candidate transfer event using the transfer schema
VERIFY retrieval -> application -> prediction -> intervention -> observation -> verification -> attribution

COMPUTE E and paired D = E_K0 - E_KA
RUN frozen sign-flip test and frozen bootstrap
APPLY original PASS criteria without modification
CLASSIFY as INVALID EXPERIMENT, VALID NEGATIVE RESULT, or VALID POSITIVE RESULT

PRESERVE all required artifacts and hashes
```

## 16. Reproducibility contract

### Required before execution
The complete bundle must exist and be independently inspectable before any experiment execution:
- this protocol/specification;
- experiment source;
- Family A generator source;
- Family B generator source;
- exact configuration;
- seed manifest;
- dependency lock/environment description;
- corpus-generation procedure;
- acquisition-gate implementation;
- transfer-event schema;
- statistical analysis specification.

No experiment source or generator may remain only in an unversioned local working tree at execution time.

### Required after execution
Preserve:
- raw stdout;
- raw stderr;
- structured results;
- event logs;
- source hash;
- A/B generator hashes;
- configuration/corpus hash;
- output hash;
- exact command;
- runtime;
- OS/environment information;
- commit SHA.

The implementation must make it possible for an independent auditor to reconstruct exactly what was executed and distinguish protocol artifacts from generated results.

## 17. Cancellation rules

Cancellation is mandatory, not discretionary:

1. **Before execution:** any G0 failure cancels execution.
2. **During A:** any G1 failure cancels the run as an invalid test.
3. **After A learning:** any G2 failure cancels **before B**. B must not be exposed to the transfer learner.
4. **After B generation:** any G3 failure cancels **before transfer interpretation**. No threshold or metric may be changed.
5. **At leakage audit:** any material G4 failure invalidates scientific interpretation.
6. **At control validation:** any G5 failure invalidates the causal comparison.
7. **Any post-hoc tuning, task deletion, seed substitution, threshold change, metric change, or selective event exclusion:** invalidates the affected run and cannot be used to establish a positive result.
8. **Any inability to establish provenance from A observations to K_A:** treated as G2 failure.
9. **Any inability to establish that B was independent and frozen before transfer exposure:** treated as G3/G4 failure as appropriate.

## 18. Phase-3 advancement requirements

Phase 3 is not authorized by this protocol.

It may only be considered after a future Phase 2.1 run has:
- passed G0-G5;
- produced a valid positive result under every unchanged transfer criterion;
- demonstrated non-episodic A abstraction acquisition with inspectable provenance;
- demonstrated B structural novelty > 0.50 under the frozen metric;
- demonstrated actual retrieval, prediction, intervention, observed consequence, verification, and attribution;
- shown mean E_KA < mean E_K0 with p < 0.05;
- met positive-transfer >=70% and negative-transfer <=20%;
- met retrieved+verified transfer >=80%;
- kept direct replay <=10%;
- shown KR does not reproduce the K_A advantage;
- preserved complete source, generator, configuration, seed, environment, event, statistical, and output artifacts for independent audit;
- been independently reproduced from the frozen source/protocol without post-hoc changes.

Even a valid positive Phase 2.1 result would support only the tested synthetic structural-transfer hypothesis under the specified conditions. It would not establish AGI, general intelligence, or broad real-world transfer.

## 19. Freeze statement

This document is a protocol draft for independent audit only. It does not report empirical evidence. It does not authorize execution. It does not modify or reinterpret Phase 2. It does not alter the Phase 2 hypothesis, structural novelty metric, threshold, seeds, controls, endpoint, statistics, or PASS criteria.

The minimum scientific repair is limited to making the A dependency causally observable through intervention, requiring genuine cross-instance A abstraction acquisition before B, and enforcing the unchanged B novelty gate. These are validity repairs, not result-optimization steps.
