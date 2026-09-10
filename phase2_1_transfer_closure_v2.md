# PHASE 2.1 TRANSFER-MECHANISM CLOSURE v2

**Target:** 1B.2
**Status:** `SCIENTIFICALLY_UNRESOLVED`
**Execution authority:** `NONE`
**Definitive experiment:** `NOT EXECUTED`
**Branch:** `phase2-structural-transfer-20260910`

## 1. Forensic restart

Audited normative artifacts:

- `phase2_1_protocol_draft.md` — parent protocol commit `3b5891ec7c45055bc9a8bf7bdf178c07f1e03d78`, SHA-256 `06f39b7ead0dae272094cda82654e834be4cc22c`;
- `PHASE2_1_CLOSURE_GAP_REGISTER.md`;
- `phase2_1_learner_freeze_v1.md` — learner instrument freeze commit `b2d2f3b09d7f8eb5a4800efbdfa8bfe5f198ea8b`;
- `phase2_1_serl_equivalence_contract.md` — equivalence contract commit `1812f393333a60070dc8715f5f2a024c9bbc8766`;
- `phase2_1_transfer_closure_v1.md` — closure audit commit `770c300ec6b81fd5d9badb9dace3c5ad01f9b6e4`.

The parent protocol remains authoritative. No parent artifact was modified.

## 2. Prediction restriction: resolution

**Finding: C — substantive scientific restriction, not proven neutral.**

The parent protocol establishes the causal dependency mechanism: X is initially blocked; Y is an enabling intervention; Z is a non-enabling contrast; and the learner observes only ordinary action outcomes and availability changes. The inspected parent material does not establish, as a logically exhaustive consequence-domain theorem, that the only legal predictions for the scientific question are exactly `ENABLES(target)` and `NONENABLES(contrast,target)`.

The SERL learner freeze nevertheless makes those two consequence types exhaustive. That choice changes the hypothesis space and therefore can change which retrieved rules exist and which events can qualify as transfer. It is therefore not a mere implementation optimization.

**Action:** do not silently inherit this restriction. Before 1B.2 can close, one of these must be established in an authoritative artifact:

1. the parent protocol explicitly defines the prediction domain as exactly these two consequence types; or
2. the Phase 2.1 scientific question is formally narrowed/preregistered to this consequence domain before execution, with an explicit acknowledgement that this is a substantive scope choice; or
3. SERL is changed by a separately authorized scientific design target to support the complete parent consequence domain.

No endpoint result may decide among these alternatives.

## 3. Typed grammar closure

`phase2_1_serl_grammar_v1.json` now provides the machine-readable predicate/sort/composition/canonicalization contract without inventing a larger grammar.

The following are closed at the syntax level:

- ActionToken is the sole Action sort;
- four role variables are permitted: `target`, `intervention`, `contrast`, `context`;
- the ten parent/SERL predicate names and arities are enumerated;
- antecedents are conjunctions of 1..6 atoms;
- disjunction, negation, recursion, arithmetic, function symbols, embeddings, executable code, free variables and task-specific constants are prohibited;
- canonical alpha-normalization, duplicate removal, atom sorting, implication direction and canonical serialization are fixed;
- semantic equivalence beyond byte-identical canonical form is not assumed;
- candidate-count pruning is forbidden.

**Remaining dependency:** exhaustive candidate enumeration cannot be authoritatively frozen until the prediction consequence domain is closed, because `OBSERVED_EFFECT` and the rule consequent range depend on that domain.

## 4. Attribution: executable normative contract

A transfer event is eligible only if all of the following occur in this order for the same B task execution:

1. a frozen K_A rule is retrieved before the decisive action;
2. a legal grounding is recorded;
3. an explicit prediction event is emitted before the decisive action;
4. the prediction names the expected protocol consequence and the grounded target/intervention/contrast roles;
5. the decisive action is causally downstream of the retrieval and prediction and is the first such action in the event trace;
6. the intervention action is actually executed;
7. the expected consequence is observed in the normative post-intervention window;
8. the consequence is verified against the ordinary learner-visible transition, not hidden simulator state;
9. a K0-equivalent ablation/counterfactual shows that the same decisive action was not independently selected without K_A knowledge.

### Post-intervention observation window

For this protocol, the minimal causal consequence is the first ordinary observation immediately following the executed intervention action. A one-step availability change is therefore observable at `OBS(t+1)` and may be used for attribution. No later observation may be substituted for a missing immediate consequence. Later observations can corroborate but cannot repair a missing first-step causal observation.

For consequences requiring a subsequent target execution, the target execution is a separate action event and its result is recorded, but it does not retroactively enlarge the intervention's immediate observation window.

If the intervention terminates the task, the resulting terminal observation is still the one-step post-intervention observation. If the required consequence is not present there, attribution fails.

### Counterfactual / ablation

The attribution counterfactual is a paired replay of the identical pre-decision B learner-visible history with the frozen K_A artifact removed while retaining the same task, initial state, interface, solver family, and condition-specific deterministic stream allocation. The counterfactual may be implemented by an auditor fork, but no fork/reset/hidden state is learner-visible.

If the K0-equivalent counterfactual independently selects the same decisive action for the same visible context before K_A can influence the decision, the event is `COINCIDENTAL`, not `KA_TRANSFER`.

If the counterfactual cannot be constructed without changing the visible history or the decision procedure, attribution is `UNATTRIBUTABLE`.

The authoritative precedence remains:

`PROTOCOL_VIOLATION > UNATTRIBUTABLE > CONFLICT > RETRIEVAL_ONLY > PREDICTION_ONLY > COINCIDENTAL > KA_TRANSFER`.

No human causal judgment is permitted.

## 5. Controls: minimum matched procedures

Controls are comparison instruments, not intentionally handicapped baselines.

### K_R — raw episodic control

- Information: complete frozen raw A episodes only; no cross-episode aggregation or relational abstraction.
- Representation: exact learner-visible A histories.
- Episode selection: deterministic lexicographic selection among eligible stored episodes using the fixed K_R stream.
- Forbidden: rule induction, cross-episode alignment, relational generalization, semantic abstraction, B-result feedback.
- B action selection: the same base deterministic action policy available to the corresponding solver, except that no K_A rule may influence it.
- Budget/termination: identical parent B caps and terminal rules.
- Any attempt to derive a cross-episode rule invalidates the K_R condition.

### K_S — matched search control

K_S receives no A-derived information. It may search only over information available from the current B episode. To remain a scientifically credible control, search must use the same visible state/action history and the same B action budget as K_A.

Search is breadth-first over finite action-history nodes generated from the learner-visible interface; successor order is lexicographic opaque-token order; duplicate identity is canonical learner-visible history; goal is `terminated=true`; expansion stops at the first depth containing a goal and then selects the lexicographically smallest complete action sequence. No hidden state, future outcome, graph topology, heuristic distance, or A information is permitted.

If the finite search space cannot be exhausted within the parent decision cutoff, the condition terminates at the frozen cutoff rather than silently changing to heuristic search.

### K_P — matched prior/policy control

K_P receives no A-derived knowledge and uses the same task-independent solver family and fixed decision procedure as K_A, including the same deterministic ordering and budgets, with the K_A memory input removed. It may not reconstruct A-derived rules from B observations. Its purpose is to test whether the solver/prior itself, rather than K_A, accounts for the effect.

### Direct replay

Replay has access only to literal recorded A action sequences. A replay candidate is applicable only on exact byte-identical learner-visible local context. No semantic substitution, variable renaming, path compression, partial matching, rule extraction, token mapping, or trajectory transformation is allowed.

A replay either emits the exact next recorded action or is inapplicable. It receives the same B interaction and decision cutoffs. A replay success is never evidence of relational transfer.

## 6. Cost and decision accounting

Primary interaction cost is exactly:

`E = number of valid learner-issued action attempts from initial observation through success or terminal cutoff`.

- `ACCEPTED`: count +1.
- `BLOCKED`: count +1.
- `SUCCESS`: count +1 and terminates the episode.
- `ILLEGAL_ACTION`: count 0 and invalidates the condition; no retry is scientifically substituted.
- `ENVIRONMENT_ERROR`: condition invalidated; no imputation.
- Internal computation, candidate enumeration, search expansion, serialization, and cache lookup are not E.
- Auditor snapshot/fork/restore operations are not E.
- A retry is a new learner-issued action attempt and counts if the prior action was valid; an illegal action cannot be converted into a retryable valid attempt.
- Interaction cap is 200. A valid action at the 200th permitted interaction counts and may succeed.
- Decision cutoff is 120 decision units. Internal search work is counted only for the separate decision-budget mechanism; it cannot alter E.
- `terminated=true` occurs only after a successful goal-reaching action.
- Invalid task, instrumentation failure, environment error, protocol violation and invalid pair are reported as invalid units, never assigned a fabricated E.

The exact implementation must emit both E and terminal status, so a reviewer can distinguish success-at-cap from cap exhaustion and invalidation.

## 7. Deterministic call-order closure rule

No new RNG purpose string may be introduced during implementation. Every randomized operation must consume its already-authorized namespace, condition and purpose. Within a condition, calls occur in this normative order:

`task materialization -> label materialization -> initial observation -> observation/action ledger update -> fact compilation -> candidate generation/evaluation -> K_A load/retrieval -> prediction -> action selection -> environment action -> result observation -> attribution inputs -> terminal accounting`.

Search-control operations occur only inside the action-selection stage and use `SEARCH_*`; statistical randomization occurs only after all task records are frozen and uses `PERMUTATION`/`BOOTSTRAP`.

If a required implementation operation has no authorized namespace/purpose, implementation stops and the namespace contract must be amended before execution. No implicit RNG stream is permitted.

## 8. Independent verification status

The repository integration available for this closure pass permits source inspection and repository writes but does not expose a verified mechanism for executing a genuinely independent second implementation and comparing its outputs.

Therefore:

`INDEPENDENT_EXECUTION_NOT_AVAILABLE`

is the honest current status.

No documentation, fixture definition, or primary implementation is treated as independent evidence.

Required future differential boundaries remain:

`F/H/U -> facts -> candidates -> K_A -> retrieval -> prediction -> action -> E/terminal -> attribution inputs -> statistics`.

## 9. AGI relevance audit

### CORE AGI PRIMITIVE

- reusable relational abstraction from experience;
- explicit provenance/evidence for knowledge;
- prediction-before-action;
- causal intervention/verification rather than correlation-only transfer;
- structural transfer across changed surface representations.

### EXPERIMENTAL INSTRUMENT

- SERL;
- fixed synthetic A/B families;
- K0/KR/KS/KP controls;
- transfer attribution ledger;
- deterministic permutation/bootstrap machinery.

### BENCHMARK-SPECIFIC MACHINERY

- the exact 12-task/100-task corpus construction;
- opaque 16-character tokenization;
- fixed six-atom candidate limit;
- protocol-specific interaction/decision caps;
- exact direct-replay anti-shortcut control.

The benchmark-specific layer must not be promoted into the final ACE architecture merely because it is useful for this test.

## 10. Completion gate

Target 1B.2 remains `SCIENTIFICALLY_UNRESOLVED` because the prediction consequence domain is still a material scientific ambiguity and independent differential execution has not been demonstrated.

No definitive experiment is authorized.

The next operation is **not** another generic documentation pass: resolve the prediction-domain question directly from the authoritative parent text; then, if closed, implement the independent conformance/differential fixtures and stop closure if their outputs diverge materially.
