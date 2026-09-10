# PHASE 2.1 LEARNER CANDIDATES

**Target:** 1B.1 — Scientific Learner Selection
**Status:** `DECISION-READY` research record; no definitive Phase 2.1 endpoint observed
**Repository:** `Infrasigma/subsume-proving-ground`
**Branch:** `phase2-structural-transfer-20260910`
**Parent protocol:** `3b5891ec7c45055bc9a8bf7bdf178c07f1e03d78`

## 1. Decision rule

The learner is selected for scientific adequacy, reproducibility, falsifiability, information-boundary compatibility, small-data suitability, and attribution compatibility. It is **not** selected using Phase 2.1 endpoint results, expected PASS probability, convenience of coding, popularity, or AGI branding.

The selection target is a minimum-sufficient mechanism for:

`opaque observation history -> relational evidence -> reusable A knowledge -> B retrieval/prediction -> action selection`

The learner must not receive graph semantics, semantic node IDs, coordinates, task IDs, seeds, future states, evaluator metadata, or pre-supplied causal rules.

## 2. Evaluation scale

For the comparison matrix below, each criterion uses:

- **0 = incompatible:** violates the frozen interface or cannot perform the required function without forbidden information.
- **1 = conditional:** possible only after adding material assumptions, substantial extra machinery, or an additional scientific choice.
- **2 = direct:** naturally satisfies the criterion with no new scientific information channel.

The scores are a screening aid, not a statistical optimization objective. Any candidate can be rejected despite a high total if it violates a hard scientific constraint.

## 3. Candidate A — Minimal learned relational/sequence learner

**Mechanism.** A trainable sequence model consumes the ordered opaque observation/action/result stream and learns an internal representation used to predict useful next actions or relational consequences. Candidate A includes recurrent/attention-based sequence learners without an explicitly inspectable symbolic relation store.

**Capabilities.** History dependence, sequence abstraction, nonlinear pattern learning, action prediction.

**Information assumptions.** Requires no semantic labels in principle, but its success depends on the model discovering a stable representation of opaque tokens and temporal relations.

**Frozen-interface compatibility.** Yes, provided only the exact observation stream is exposed.

**Evidence.** Neural relational models can infer latent interaction structures from observations; NRI learns latent interaction graphs and dynamics from observational trajectories. Neural logic models can also learn lifted relational rules on small synthetic tasks. citeturn462058search1turn462058search3

**Weaknesses.** Internal representations are hard to audit; architecture, optimizer, initialization, token embedding treatment, context length, and training schedule become consequential. Token permutation can expose whether the learned representation is genuinely relational or simply memorizes identities. The small A sample (12 tasks) creates a substantial risk that performance reflects architectural prior rather than learned transferable content.

**Circularity risk.** Medium-to-high. A sufficiently specialized relational sequence architecture can encode much of the target inductive bias without making the learned content inspectable.

**Phase 2.1 judgment:** Rejected as the primary learner because its implementation degrees of freedom are too large for a narrow, byte-equivalent scientific freeze, and failure/success would be harder to interpret causally.

## 4. Candidate B — Structured latent/event-state learner + deterministic relational memory

**Mechanism.** Maintain an exact, auditable event ledger of the permitted observations, convert only observable transitions into protocol-defined relational facts, retain a deterministic task-local relational memory, and mine reusable relational hypotheses using a bounded exhaustive search over the protocol's already-frozen predicate grammar. No neural embedding is used.

**Capabilities.** Exact history retention, event identity, temporal ordering, relational abstraction, cross-instance generalization, explicit provenance, inspectable learned knowledge.

**Information assumptions.** Uses only the frozen observation/action/result stream. Opaque tokens are treated as equality-bearing symbols, never semantically decoded. Cross-task transfer is obtained only after role abstraction/generalization.

**Evidence.** Relational/logic learners are specifically designed to learn first-order relational concepts from sparse examples; QORA reports strong zero-shot transfer from object-relational model learning with very small observation requirements, while theory-based causal-transfer work explicitly separates abstract structure learning from instance-specific knowledge and uses model-based planning. citeturn570938search0turn462058search0turn462058search2

**Weaknesses.** The explicit relational representation is a strong inductive bias. The learner therefore tests whether *this controlled relational learning mechanism* can support transfer; it does not prove that arbitrary learners would do so. The observation-to-predicate compiler must be frozen exactly to prevent it from becoming hidden oracle access.

**Circularity risk.** Low-to-medium if, and only if, the protocol grammar is treated as a representation language and candidate rule content is learned from observed evidence rather than supplied. This distinction is mandatory.

**Phase 2.1 judgment:** Preferred candidate.

## 5. Candidate C — Neural representation learner + explicit symbolic/relational hypothesis store

**Mechanism.** A learned encoder converts observation histories into latent objects/relations; an explicit symbolic store then learns and retrieves relational rules from those latent structures.

**Capabilities.** Potentially combines flexible representation learning with inspectable rules and transfer.

**Evidence.** Neural-symbolic systems such as Neural Logic Machines combine neural function approximation with symbolic relational processing and have demonstrated relational generalization on synthetic reasoning tasks. citeturn462058search3 Neural relational inference demonstrates that learned latent interaction structures can emerge from raw trajectories. citeturn462058search1

**Weaknesses.** The encoder becomes a hidden scientific variable: two encoders can induce different object identities or relation structures from the same opaque history. Training data, optimization, and representation collapse become materially consequential. In Phase 2.1 this adds flexibility without being necessary to answer the target question.

**Circularity risk.** Medium-to-high because the encoder can encode the expected relational decomposition.

**Phase 2.1 judgment:** Rejected as unnecessarily composite; suitable for a later ACE representation-learning study.

## 6. Candidate D — Learned world model + explicit model-based planning

**Mechanism.** Learn a latent transition model from experience and use search/planning over the learned model to select actions.

**Capabilities.** Model-based prediction, internal simulation, planning, adaptation.

**Evidence.** MuZero demonstrates that a learned model can support tree-based planning without access to the environment's underlying dynamics. citeturn875792search0 Dreamer-style work similarly uses learned latent dynamics for planning, while recent work notes instability and representation problems when jointly learning representations, dynamics, and policy. citeturn462058search4

**Weaknesses.** This is substantially larger than required. Latent-state identifiability, model rollouts, value learning, search budget, and planning policy introduce multiple additional choices. A success could arise from generic model-based planning rather than reusable relational knowledge unless attribution is extremely strict.

**Circularity risk.** Medium. The model could encode the B dynamics directly in latent form without producing protocol-valid K_A relational knowledge.

**Phase 2.1 judgment:** Rejected for scope. Model-based planning is a later ACE layer; a deterministic bounded planner may be retained as a matched decision mechanism once the learner's relational effect is isolated.

## 7. Candidate E — Hybrid learned representation + explicit relational program induction

**Mechanism.** Induce symbolic programs/rules from learned or symbolic representations, using bounded synthesis or Bayesian/MDL-style program search.

**Capabilities.** Compositional abstractions, reusable programs, transfer through learned libraries, interpretable structure.

**Evidence.** Bayesian program learning and DreamCoder show that program induction can produce compact, transferable abstractions from sparse examples; DreamCoder learns both symbolic abstractions and search guidance. citeturn570938search10turn570938search3 MDL-based relational program learning has also shown robustness to noisy data. citeturn570938search4

**Weaknesses.** A general program language is broader than the Phase 2.1 hypothesis grammar. Introducing a program DSL, synthesis semantics, or learned library would add scientific scope and can silently encode the expected solution structure.

**Circularity risk.** Medium-to-high unless the language is restricted to the frozen Phase 2.1 grammar.

**Phase 2.1 judgment:** Rejected as the full mechanism. A bounded enumerative rule-search component is retained inside Candidate B only because it is constrained to the parent grammar and therefore does not introduce a new programming language.

## 8. Comparative matrix

| Criterion | A: neural sequence | B: structured event + relational memory | C: neural + symbolic | D: world model + planning | E: program induction |
|---|---:|---:|---:|---:|---:|
| Frozen-interface compatibility | 2 | 2 | 2 | 2 | 2 |
| Relational discovery | 1 | 2 | 2 | 1 | 2 |
| Structural transfer | 1 | 2 | 2 | 1 | 2 |
| Causal testability | 1 | 2 | 1 | 1 | 2 |
| Reproducibility | 1 | 2 | 1 | 1 | 2 |
| Small-data suitability | 1 | 2 | 1 | 0 | 2 |
| Leakage resistance | 1 | 2 | 1 | 1 | 2 |
| Attribution compatibility | 1 | 2 | 1 | 1 | 2 |
| Budget closure | 1 | 2 | 1 | 1 | 1 |
| Implementation complexity | 0 | 2 | 0 | 0 | 1 |
| Circularity risk | 0 | 2 | 0 | 1 | 1 |
| Scientific interpretability | 1 | 2 | 1 | 1 | 2 |

The matrix favors B because B uniquely minimizes additional scientific degrees of freedom while directly supporting the target mechanism. The choice is not based on an expected transfer score.

## 9. What each candidate would answer

A mainly asks: *can a learned sequence architecture discover enough latent structure to transfer?*

B asks: *can explicitly observable relational evidence be abstracted into reusable, protocol-valid knowledge and then reduce B interaction cost?*

C asks a larger question involving both representation learning and symbolic induction.

D asks whether latent model learning and planning can solve the task efficiently, which is broader than relational transfer.

E asks whether a program-induction system can discover transferable rules, but risks conflating language design with the scientific result.

The frozen Phase 2.1 question is closest to B without requiring later-stack machinery.

## 10. Kill tests applicable to all candidates

A candidate is rejected before definitive evaluation if any of the following is true:

1. it requires forbidden simulator metadata or semantic labels;
2. it cannot expose an auditable K_A representation satisfying the frozen grammar and provenance requirements;
3. it changes behavior under an otherwise equivalent opaque-token relabeling in a way not explained by the frozen task-local identity semantics;
4. its result depends on undocumented randomness, hidden initialization state, or uncontrolled process memory;
5. two independent implementations cannot produce equivalent frozen-boundary artifacts;
6. it contains task-specific lookup tables or pre-specified A→B rules;
7. it makes attribution impossible because the transfer-relevant decision cannot be identified and counterfactually ablated.

## 11. Sanity checks permitted before definitive evaluation

Only non-endpoint checks are permitted:

- schema conformance of the learner-visible observation;
- exact replay of the frozen deterministic RNG primitives;
- proof/tests that no forbidden metadata is reachable from the learner API;
- parser/validator tests for the already-frozen K_A grammar;
- token-permutation invariance tests using synthetic/non-Phase-2.1 fixtures;
- two-implementation equivalence tests on fixtures that are not the definitive A/B corpus;
- termination and budget-accounting tests on synthetic fixtures.

These checks may establish interface correctness and determinism only. They may not compare Phase 2.1 K_A versus K0 endpoint cost, inspect definitive B outcomes, or select/tune the learner from observed transfer performance.

## 12. Conclusion

The literature supports an explicit structured relational learner as the minimum-sufficient choice for this narrow experiment. It is more interpretable and reproducible than a learned latent architecture, while avoiding the extra representational and planning variables introduced by full world-model or program-synthesis systems. citeturn462058search0turn462058search2turn570938search0

This conclusion does **not** imply that the same learner is the correct long-term ACE architecture. The Phase 2.1 mechanism is an experimental instrument, not the final system design.

## References

1. Kipf et al., “Neural Relational Inference for Interacting Systems,” ICML 2018. https://proceedings.mlr.press/v80/kipf18a.html
2. Stella & Loguinov, “QORA: Zero-Shot Transfer via Interpretable Object-Relational Model Learning,” ICML 2024. https://proceedings.mlr.press/v235/stella24a.html
3. Edmonds et al., “Theory-based Causal Transfer: Integrating Instance-level Induction and Abstract-level Structure Learning,” AAAI 2020. https://mjedmonds.com/projects/OpenLock/AAAI20_OpenLockLearner.html
4. Dong et al., “Neural Logic Machines,” ICLR 2019. https://research.google/pubs/neural-logic-machines/
5. Schrittwieser et al., “Mastering Atari, Go, chess and shogi by planning with a learned model,” Nature 2020. https://www.nature.com/articles/s41586-020-03051-4
6. Das et al., “Few-Shot Induction of Generalized Logical Concepts via Human Guidance,” Frontiers in Robotics and AI 2020. https://pmc.ncbi.nlm.nih.gov/articles/PMC7805948/
7. Lake et al., “Human-level concept learning through probabilistic program induction,” Science 2015. https://pubmed.ncbi.nlm.nih.gov/26659050/
8. Ellis et al., “DreamCoder: growing generalizable, interpretable knowledge with wake-sleep Bayesian program learning,” Philosophical Transactions B 2023. https://pubmed.ncbi.nlm.nih.gov/37271169/
9. Hocquette et al., “Learning MDL Logic Programs from Noisy Data,” AAAI 2024. https://ojs.aaai.org/index.php/AAAI/article/view/28925
10. He & Geng, “Active Learning of Causal Networks with Intervention Experiments and Optimal Designs,” JMLR 2008. https://www.jmlr.org/papers/v9/he08a.html
