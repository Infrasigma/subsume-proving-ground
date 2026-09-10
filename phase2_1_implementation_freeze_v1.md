# PHASE 2.1 IMPLEMENTATION FREEZE v1 — CLOSURE CANDIDATE

**Status:** `SCIENTIFICALLY_UNRESOLVED`
**Execution authority:** NONE
**Definitive Phase 2.1 experiment:** NOT EXECUTED

## 0. Parent scientific state

- Repository: `Infrasigma/subsume-proving-ground`
- Branch: `phase2-structural-transfer-20260910`
- Parent protocol: `phase2_1_protocol_draft.md`
- Parent protocol SHA-256: `06f39b7ead0dae272094cda82654e834be4cc22c`
- Parent protocol commit: `3b5891ec7c45055bc9a8bf7bdf178c07f1e03d78`
- Closure version: `v1`
- Closure purpose: close reproducibility and implementation ambiguities without silently modifying the parent scientific protocol.

The parent protocol is preserved unchanged. This artifact is additive and does not replace or rewrite the parent scientific record.

## 1. Scientific-scope decision

**Scientific-scope change:** NONE has been incorporated.

However, a critical scientific design choice remains unresolved: the actual learner/search/solver/control procedure. The parent protocol freezes the information boundary and hypothesis but does not identify a unique learner implementation. Selecting one now would be a substantive experimental-design decision because learner inductive bias can materially alter acquisition, transfer, attribution, and interaction cost.

Therefore this v1 artifact is deliberately **not** an execution freeze. It records closed implementation conventions and the exact unresolved decisions that require methodological approval before Target 1A.

## 2. Closure principles

1. A frozen external behavior must be identical across compliant implementations unless an explicit equivalence rule says otherwise.
2. No implementation choice may be selected from Phase 2.1 outcome data.
3. No task, token, novelty, control, or attribution rule may be regenerated after inspection of results.
4. A choice that changes the hypothesis test is A-class and cannot remain implicit.
5. A choice is C-class only if an observational-equivalence test can prove that it cannot change any scientific artifact or endpoint.
6. D-class details may remain implementation-defined.

## 3. Deterministic RNG closure — proposed canonical manifest

The parent encoding, SHA-256 counter construction, rejection sampling, and Fisher-Yates rule remain authoritative.

The following exact purpose strings are frozen for v1 **subject to final audit**:

| Namespace | Purpose string | Consumer | Counter rule | Reuse |
|---|---|---|---|---|
| `TASK_GENERATION` | `family_a_dependency_sources` | A generator | starts 0; counter consumed only by rejection/Fisher-Yates draws | forbidden elsewhere |
| `TASK_GENERATION` | `family_b_dependency_subset` | B generator | starts 0 per B task; sequential draw/rejection consumption | forbidden elsewhere |
| `LABEL_PERMUTATION` | `action_token_pool` | task-local action labels | starts 0 per task; pool generation consumes one counter per token | forbidden elsewhere |
| `LABEL_PERMUTATION` | `action_token_assignment` | semantic-action permutation | starts 0 per task; Fisher-Yates | forbidden elsewhere |
| `LABEL_PERMUTATION` | `location_token_pool` | task-local locations | starts 0 per task; one counter per location token | forbidden elsewhere |
| `LABEL_PERMUTATION` | `location_token_assignment` | semantic-location permutation | starts 0 per task; Fisher-Yates | forbidden elsewhere |
| `A_LEARNER` | `a_learner_decision` | unresolved learner | reserved; cannot be consumed until learner freeze | forbidden |
| `B_LEARNER_KA` | `b_learner_ka_decision` | unresolved learner | reserved; exact schedule to be frozen with learner | forbidden |
| `B_LEARNER_K0` | `b_learner_k0_decision` | unresolved learner | reserved; exact schedule to be frozen with learner | forbidden |
| `B_LEARNER_KR` | `b_learner_kr_decision` | unresolved control | reserved | forbidden |
| `B_LEARNER_KS` | `b_learner_ks_decision` | unresolved control | reserved | forbidden |
| `B_LEARNER_KP` | `b_learner_kp_decision` | unresolved control | reserved | forbidden |
| `SEARCH_KA` | `search_ka_candidate_order` | unresolved search | reserved | forbidden |
| `SEARCH_K0` | `search_k0_candidate_order` | unresolved search | reserved | forbidden |
| `SEARCH_KR` | `search_kr_candidate_order` | unresolved control | reserved | forbidden |
| `SEARCH_KS` | `search_ks_candidate_order` | unresolved control | reserved | forbidden |
| `SEARCH_KP` | `search_kp_candidate_order` | unresolved control | reserved | forbidden |
| `PERMUTATION` | `primary_sign_flip` | statistical reference implementation | counter = permutation index | forbidden elsewhere |
| `BOOTSTRAP` | `paired_mean_percentile` | statistical reference implementation | counter = resample index | forbidden elsewhere |

### Important limitation

The exact purpose strings above are deterministic conventions, but freezing them does not close the scientific learner problem. They therefore do not authorize experiment execution.

## 4. A/B generation closure status

The following invariants are accepted as mandatory implementation rules from the parent protocol:

- A seeds are exactly `0..11`; B seeds exactly `0..99`.
- Seed values are never learner-visible.
- Every random draw uses the parent SHA-256 counter construction.
- Rejection sampling never silently changes a task seed.
- B generation must terminate with `INVALID_TASK` if the candidate counter reaches the frozen limit without a valid task.
- A and B corpora are frozen before transfer exposure.
- Structural novelty is computed before transfer exposure and from the unrounded per-task values.
- No graph/task is regenerated because novelty is inconvenient.
- Semantic action/location records are generated before opaque token permutation.
- Token generation cannot depend on semantic role.
- Available-action ordering is token-byte lexicographic order.

### Still unresolved

The exact mapping from suboperation to counter sequence, and the exact semantic enumeration order used before token assignment, must be made explicit in the final executable freeze. This is A-class because it can alter token mappings and generated B novelty.

## 5. Learner closure — BLOCKED

No actual learner architecture is frozen by the parent protocol or current repository.

The following must be fixed before implementation:

- representation of internal history;
- memory data structure and update rule;
- model/algorithm family;
- initialization;
- learning/update schedule;
- search procedure;
- planning/solver procedure;
- action-selection rule;
- deterministic tie-breaking;
- decision-unit definition;
- random-stream consumption;
- termination behavior;
- maximum internal computation per decision;
- forbidden simulator/task metadata;
- serialization/checkpoint behavior.

### Why this cannot be guessed

The experiment is explicitly testing whether A-derived relational knowledge reduces B interaction cost. A learner with a hard-coded relational-rule induction mechanism, a learner with a generic tabular history representation, and a neural sequence learner are not observationally equivalent and can have radically different transfer behavior. The methodological literature itself treats the choice of abstraction/relational representation and transfer mechanism as a substantive part of the experiment, not a harmless coding detail. Work on causal-transfer agents has used explicit abstract causal structure learning plus model-based planning, while object-relational model-learning work likewise treats representation/model induction as a core algorithmic choice. These demonstrate that there is no literature-supported universal learner that can be silently substituted here. citeturn1academia24turn1search0

A learner must therefore be selected through an explicit methodological decision, with its information boundary and inductive bias preregistered before any Phase 2.1 outcome exists.

## 6. K_A closure — BLOCKED pending learner/representation closure

The following pipeline is mandatory:

`raw A observations -> canonical history -> candidate generation -> normalization -> grammar validation -> provenance validation -> cross-instance support -> contradiction handling -> frozen K_A`

The grammar, allowed predicates, role variables, canonical normalization, provenance requirement, and minimum two independently generated/relabelled A-task support remain inherited from the parent protocol.

Still missing:

- complete candidate enumeration domain;
- exact bounded candidate-length/order;
- candidate generation from observations;
- treatment of duplicate derivations;
- contradiction resolution;
- support counting;
- incomplete-candidate handling;
- exact provenance record schema;
- freeze serialization;
- maximum candidate count and failure behavior.

These cannot be safely closed until the learner/history representation is fixed because the legal observation-to-predicate transformation determines what candidate evidence exists.

## 7. Observation/history → predicate closure — BLOCKED

The parent protocol freezes the learner-visible observation and the legal predicate grammar, but does not fully specify how a learner history becomes predicate facts.

The final freeze must specify:

- exact retained event history;
- event identity and lifetime;
- entity creation rules;
- role binding;
- equality semantics;
- action/result relation construction;
- temporal relation construction;
- context segmentation;
- duplicate fact handling;
- canonical ordering;
- forbidden simulator-derived facts.

No hidden state, semantic node ID, task seed, dependency index, graph topology, or harness metadata may enter this transformation.

## 8. Retrieval/prediction/attribution closure — BLOCKED

The parent precedence ordering remains authoritative:

`PROTOCOL_VIOLATION > UNATTRIBUTABLE > CONFLICT > RETRIEVAL_ONLY > PREDICTION_ONLY > COINCIDENTAL > KA_TRANSFER`.

Still required before execution:

1. exact applicability evaluator;
2. exact partial-match semantics;
3. missing-field behavior;
4. conflict detection;
5. candidate ranking and complete tie handling;
6. maximum retrieval count;
7. prediction representation;
8. prediction timing;
9. prediction verification rule;
10. decisive-action identification;
11. attribution counterfactual/ablation procedure;
12. exact event serialization.

Retrieval presence alone and successful outcome alone are never sufficient evidence of transfer.

## 9. Controls — BLOCKED

`K_R`, `K_S`, and `K_P` require independent procedure specifications. Their information boundaries are inherited from the parent protocol but their exact decision policies are not.

Each control must define:

- accessible information;
- inaccessible information;
- internal memory;
- search;
- solver;
- action selection;
- randomness;
- budget accounting;
- termination;
- tie-breaking.

The controls must be matched alternatives, not deliberately weakened baselines.

Direct replay must additionally specify exact trajectory storage, matching, allowable transformations, and success criterion.

## 10. Cost/termination closure

The parent endpoint `E` remains immutable: learner-issued valid action attempts through success or terminal cutoff.

The final implementation freeze must add an exhaustive event table for:

- accepted action;
- blocked action;
- success action;
- illegal action;
- environment error;
- retries;
- internal search;
- internal computation;
- decision cutoff;
- interaction cap;
- task invalidation;
- instrumentation failure.

No internal computation may be converted into uncounted external work in a way that changes the frozen meaning of `E`.

## 11. Statistics closure

No statistical hypothesis or threshold is changed.

The final reference implementation must freeze:

- exact sign-flip enumeration/stream mapping;
- two-sided p-value convention;
- whether the observed statistic is included in the permutation count;
- percentile interpolation rule;
- bootstrap index ordering;
- invalid-pair handling;
- no-imputation rule;
- reference output serialization.

Independent reference calculations are required before the experiment.

## 12. Two-implementation equivalence

The minimum equivalence contract is:

**Exact identity required:**

- A/B semantic task definitions;
- task seeds;
- dependency sets;
- opaque token assignments;
- novelty values;
- learner-visible observations;
- K_A serialized contents;
- retrieval selection;
- cost accounting;
- attribution eligibility;
- statistical inputs.

**Allowed equivalence:** internal data structures, programming language, memory layout, process orchestration, and other details are permitted only when they produce byte-identical values at every externally frozen boundary.

No “same result on average” criterion is sufficient.

An equivalence harness must compare two independent implementations at each frozen boundary before Target 1A is authorized.

## 13. Adversarial audit status

The prior Target 1A forensic pass already demonstrated that a plausible implementation can make materially consequential choices not fixed by the parent protocol. In particular, an implementation probe found that a reasonable interpretation of task-generation details can affect the B novelty gate; this probe was not repository evidence and produced no Phase 2.1 result.

The remaining red-team question is therefore not whether ambiguity exists; it is whether the final closure can remove it without introducing a favorable post-hoc learner or control design.

## 14. Methodological research conclusion

Existing causal-transfer work supports treating abstract structure learning, representation, and planning as explicit experimental components rather than hidden implementation details. citeturn1academia24turn1search5

Existing object-relational transfer work likewise shows that representation/model induction is itself an algorithmic choice affecting zero-shot transfer. citeturn1search0

Therefore this closure process must not pretend that there is a neutral default learner. Selecting the learner is the central remaining scientific design decision.

## 15. Completion gate

Target 1B cannot be marked COMPLETE until all of the following are true:

- parent protocol preserved;
- gap register complete;
- learner/search/control algorithms frozen;
- K_A construction executable and deterministic;
- predicate mapping executable and deterministic;
- retrieval/prediction/attribution executable and deterministic;
- exact cost/decision accounting frozen;
- statistical reference implementation frozen;
- deterministic vectors committed;
- two-implementation equivalence demonstrated;
- adversarial audit finds no remaining A-class ambiguity;
- independent audit/reproduction completed;
- no definitive experiment executed during closure.

## 16. Current verdict

`SCIENTIFICALLY_UNRESOLVED`

The correct next action is **not** to implement Target 1A. The remaining work is to obtain an explicit methodological decision for the learner/solver/control family and then close the dependent acquisition/retrieval/attribution specifications around that frozen learner.

## 17. Definitive experiment

`NOT EXECUTED`
