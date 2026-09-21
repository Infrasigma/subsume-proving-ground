# H4 Autonomous Concept-Language Induction Protocol

Hypothesis:
A generic learner can discover reusable, behaviorally testable relational concepts from unlabeled-to-the-learner experience and reuse those concepts across surface changes and novel compositions. The reusable unit is a learned relation motif, not a task-family-specific program or compression macro.

Constraints:
- CPU-only deterministic Python 3 standard library implementation.
- No external model, pretrained model, human labels, or semantic concept names.
- No task-family-specific adapters.
- Training streams expose only observations and binary outcomes; hidden concept identity is never supplied.
- Hidden audits are generated independently by a fixed evaluator from seeds not passed to the learning engine.
- Acceptance logic lives in the evaluator, not in learned data.
- A learned concept must pay its acquisition/verification/storage cost through later reuse.
- The learner may use only generic observation predicates and generic boolean composition.
- Core result must survive concept deletion and rehydration.

Frozen gates:
H4-A Concept invention: on each independent block the learner must retain at least 3 nontrivial concepts whose learned structural signatures contain >=3 relational atoms and whose support is not explainable by any <=2-atom candidate with equal held-out accuracy.
H4-B Behavioral validity: every retained concept must reach >=0.90 accuracy on an independent hidden audit before it can be used for transfer.
H4-C Cross-surface transfer: symbol-renamed, order-shuffled, and distractor-injected audits must retain >=0.80 of the baseline concept detector's accuracy and >=0.50 cost reduction.
H4-D Compositional transfer: a hidden task formed from two previously learned concepts with a generic boolean composition must achieve >=0.90 accuracy and >=0.25 normalized search-cost savings versus a fresh learner.
H4-E Deletion/rehydration: deleting the learned library must remove at least 80% of the measured K1 cost advantage; serializing and reloading the library must restore >=90% of the pre-deletion advantage.
H4-F Recursive compounding: after each of three successive concept acquisitions, the cost ratio R_n = C(T_n|K_(n-1))/C(T_n|K0) must be <0.75 on both independent blocks.
H4-G Negative control: structure-destroying/shuffled training data must yield <0.05 normalized savings on the same hidden audits and must not pass H4-B.
H4-H No family adapters: the source contains one generic learner and generic predicate/boolean grammars; no per-concept or per-domain solver is permitted.

Kill:
If either independent block fails any frozen required gate after implementation defects are excluded, H4 is killed for this substrate. Do not relax thresholds, add domain-specific mechanisms, or alter hidden seeds after seeing outcomes.
