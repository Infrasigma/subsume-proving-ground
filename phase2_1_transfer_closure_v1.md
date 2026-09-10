# PHASE 2.1 TRANSFER-MECHANISM CLOSURE v1

**Target:** 1B.2 — Freeze K_A / Retrieval / Prediction / Controls / Attribution
**Status:** `SCIENTIFICALLY_UNRESOLVED`
**Execution authority:** `NONE`
**Definitive Phase 2.1 experiment:** `NOT EXECUTED`
**Repository:** `Infrasigma/subsume-proving-ground`
**Branch:** `phase2-structural-transfer-20260910`

## 0. Forensic basis

This artifact is an additive closure audit. It does not replace or modify the frozen parent scientific protocol.

Audited artifacts:

- parent protocol `phase2_1_protocol_draft.md`, commit `3b5891ec7c45055bc9a8bf7bdf178c07f1e03d78`, SHA-256 `06f39b7ead0dae272094cda82654e834be4cc22c`;
- learner freeze `phase2_1_learner_freeze_v1.md`, commit `b2d2f3b09d7f8eb5a4800efbdfa8bfe5f198ea8b`;
- SERL equivalence contract `phase2_1_serl_equivalence_contract.md`, commit `1812f393333a60070dc8715f5f2a024c9bbc8766`;
- closure gap register `PHASE2_1_CLOSURE_GAP_REGISTER.md`.

The parent protocol remains authoritative. The learner freeze explicitly states that its consequence restriction and several transfer/control boundaries depend on this downstream closure; it therefore cannot be treated as independently complete merely because the document exists.

## 1. Scientific non-change

The scientific question remains:

`C(B+ | K_A) < C(B+ | K_0)`

No benchmark performance, endpoint, task corpus, PASS criterion, or post-outcome observation was used to select a procedure.

`SERL = Phase-2.1 scientific instrument` and `SERL != final ACE architecture` remain mandatory.

## 2. Closure matrix

| Boundary | Status | Reason |
|---|---|---|
| Typed grammar | PARTIALLY CLOSED | Existing freeze fixes predicate vocabulary and `1<=k<=6`, but does not provide a machine-exact exhaustive typed atom-construction table from the parent grammar. |
| Candidate enumeration | NOT CLOSED | Exact legal atom construction and all variable/role typing must precede an exhaustive enumeration oracle. |
| Prediction restriction | BLOCKER | The existing SERL freeze restricts predictions to `ENABLES(target)` or `NONENABLES(contrast,target)`. The parent protocol evidence reviewed here does not establish that this restriction is logically implied; treating it as a new restriction would alter the instrument and therefore cannot be silently frozen. |
| Post-intervention window | NOT CLOSED | The learner freeze uses a one-step visible transition for some fact construction, while the parent transfer/attribution machinery requires an exact consequence observation window. Repeated actions, termination, missing observations, competing interventions, timeout, and reset cases still need a single normative table. |
| Counterfactual attribution | NOT CLOSED | Retrieval, prediction, intervention, observed consequence, and causal attribution are separated conceptually, but the exact K0-equivalent counterfactual/ablation procedure is not yet an executable normative algorithm. |
| K_R | NOT CLOSED | Information boundary exists; exact representation, episode selection, candidate access, ordering, budget, and decision procedure are not frozen. |
| K_S | NOT CLOSED | “Search” is not executable until state representation, operators, depth, expansion order, duplicate detection, terminal rules, tie-breaking and cutoff are frozen. |
| K_P | NOT CLOSED | Matching planning/prediction procedure and tie/budget rules remain dependent on downstream closure. |
| Direct replay | NOT CLOSED | Exact trajectory representation, context matching, transformations, partial matching, failure and budget rules remain unspecified. |
| Cost / decision accounting | PARTIALLY CLOSED | `E` and the parent caps are fixed, but an exhaustive decision-unit and edge-case table is still required. |
| Deterministic call order | NOT CLOSED | Parent namespaces exist; exact consumer call schedule cannot be finalized while control/search procedures remain open. |
| Independent reference implementation | NOT DEMONSTRATED | No second implementation has been produced and independently executed against the normative boundaries in the accessible repository state. |
| Differential verification | NOT DEMONSTRATED | No byte-for-byte two-implementation run has been executed. |
| Information-leakage tests | NOT DEMONSTRATED | The prohibition is documented, but executable independent leakage tests are not evidenced by the audited artifacts. |
| Red-team closure | NOT COMPLETE | Material ambiguities remain, so the required `NO MATERIAL UNRESOLVED INTERPRETATION FOUND` condition is false. |

## 3. Typed grammar finding

The current SERL freeze says every atom must use the parent predicate argument types and that every role must be bound. That is necessary but not sufficient for independent reconstruction.

A compliant machine-exact grammar must enumerate, for every predicate:

- predicate name;
- arity;
- each argument's declared sort;
- the complete finite role-variable domain for that sort;
- whether repeated variables are legal in each argument position;
- whether an atom may contain only variables or may contain grammar constants;
- all legal role assignments;
- all illegal combinations;
- canonical atom serialization;
- canonical alpha-normalization;
- duplicate elimination;
- contradiction identity;
- enumeration order.

Until that table is derived directly from the parent grammar, two competent implementers can make different candidate sets while both claiming compliance. Therefore the candidate-space exhaustiveness gate remains open.

## 4. Prediction restriction finding

The current learner freeze defines the candidate consequence as exactly one of:

- `ENABLES(target)`;
- `NONENABLES(contrast,target)`.

This is an experimental restriction, not merely serialization. It determines which hypotheses can exist and which predictions can be attributed.

The closure rule is therefore:

1. If the parent protocol explicitly defines the consequence domain as exactly these two consequence types, record the formal mapping and inherit it without changing scope.
2. If the parent protocol permits a wider consequence domain and SERL narrows it to these two types, classify the narrowing as a new scientific design decision. Do not silently retain it merely because it simplifies SERL.
3. Until case 1 is proved from the normative parent artifact, prediction closure remains blocked.

No endpoint result may be used to choose between these cases.

## 5. Attribution closure requirement

The normative causal chain must remain:

`knowledge exists -> retrieval -> application -> prediction -> intervention -> observed consequence -> verification -> attribution`

A successful B trajectory without this chain is not a transfer event.

The final attribution algorithm must define, without interpretation:

- the exact pre-intervention decision boundary;
- the retrieved abstraction and grounding;
- the prediction event;
- the decisive action;
- the intervention identity as an observable action token occurrence;
- the post-intervention observation window;
- the expected visible consequence;
- the observed consequence;
- the K0-equivalent counterfactual state/history;
- the counterfactual action;
- the attribution eligibility predicate;
- precedence when evidence is missing or conflicting.

The parent precedence remains authoritative:

`PROTOCOL_VIOLATION > UNATTRIBUTABLE > CONFLICT > RETRIEVAL_ONLY > PREDICTION_ONLY > COINCIDENTAL > KA_TRANSFER`.

No “looks causal” or human semantic judgment is permitted.

## 6. Controls closure rule

The controls must be matched scientific controls, not performance-shaped baselines.

### K_R

Must preserve raw A episodic information while forbidding cross-episode abstraction, aggregation, relational generalization, and rule construction during B. The exact single-episode selection rule and action procedure must be frozen before execution.

### K_S

Must use the same permitted B observations and no A knowledge. “Exhaustive search” must be expanded into an exact finite procedure: state encoding, operators, successor generation, depth, expansion order, duplicate-state identity, terminal condition, action validity, cutoff, and tie-break.

### K_P

Must preserve the specified solver/task prior/randomization relationship while removing A knowledge. The exact prediction/planning algorithm, available information, action policy, budget and tie-break must be normative.

### Direct replay

Must replay only literal recorded A action trajectories under an exact matching rule. No transformation may encode the intended relational abstraction. Partial matching or semantic substitution must be explicitly prohibited unless already authorized by the parent protocol.

## 7. Cost closure rule

The primary external cost remains exactly `E`: valid learner-issued action attempts through success or terminal cutoff.

Internal computation is not charged to `E`, but it cannot become an alternate unmeasured external channel.

The final table must give one outcome for every combination of:

- valid accepted action;
- valid blocked action;
- success action;
- illegal action;
- environment error;
- interaction-cap boundary;
- decision-cutoff boundary;
- instrumentation timeout;
- retry;
- task invalidation;
- pair invalidation.

There is no imputation of an invalid pair and no substitution of a cheaper action merely because an internal procedure is expensive.

## 8. Independent implementation requirement

The equivalence contract is normative but is not evidence that an independent implementation exists.

For completion, a second implementation must be written from the normative artifacts without copying the first implementation's source logic and must independently emit canonical boundary artifacts for fixed conformance fixtures.

Required comparison boundaries:

`H/U -> F -> candidates -> K_A -> retrieval -> prediction -> action -> E/terminal -> attribution inputs -> statistics`

A mismatch must stop closure. It cannot be resolved by selecting the implementation with the desired endpoint behavior.

## 9. Definitive-experiment firewall

This closure artifact authorizes no definitive experiment.

Specifically prohibited during closure:

- executing the definitive 100-task Phase 2.1 evaluation;
- inspecting endpoint performance to select a learner/control procedure;
- regenerating B tasks after novelty inspection;
- tuning K_A thresholds or retrieval from B outcomes;
- post-hoc learner modification;
- performance-based candidate pruning;
- claiming any closure fixture as Phase 2.1 evidence.

## 10. Current verdict

`SCIENTIFICALLY_UNRESOLVED`

The target cannot honestly be marked COMPLETE from the present repository evidence. The prediction restriction and several downstream transfer/control boundaries remain scientifically consequential, and the required independent differential implementation has not been demonstrated.

The correct next operation is to close these normative gaps, then independently implement and execute the conformance/differential suite. Only after those gates pass can Target 1B.2 become COMPLETE.

## 11. Next closure sequence

1. derive the typed atom table directly from the parent protocol;
2. prove or reject the two-consequence prediction restriction;
3. freeze the exact post-intervention observation window and counterfactual attribution algorithm;
4. freeze K_R, K_S, K_P and direct replay as matched executable procedures;
5. freeze the exhaustive cost/decision/error table and deterministic call schedule;
6. implement an independent reference stack without copying the primary implementation;
7. run fixed non-definitive differential and leakage fixtures;
8. red-team again for two reasonable compliant implementations with different scientific outputs;
9. stop if any A-class ambiguity remains.

**Required completion statement:** `NO MATERIAL UNRESOLVED INTERPRETATION FOUND`.

Until that statement is supported by actual differential evidence, this target remains scientifically unresolved.
