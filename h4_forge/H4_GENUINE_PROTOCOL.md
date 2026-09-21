# H4 Genuine Adversarial Replication Protocol

This test is intended to distinguish the earlier single-surface H4 result from genuine
surface-independent concept reuse.

Frozen hypothesis:
A generic learner can discover reusable relational concepts from one observation
surface, then apply the same learned concept to previously unseen structural
surfaces without being told the surface identity or hidden concept identity.

Surfaces:
- spatial scenes on a 2-D grid,
- ordered sequences,
- sparse graphs.

The learner receives only a common relation-set representation. It is never given
the surface name, hidden generator, concept name, or target labels.

Required gates, on two independent hidden seed blocks:

G1 Discovery:
At least 3 hidden concepts are independently induced. Each must be >=90% accurate
on a source holdout and contain >=3 generic relational atoms.

G2 Exhaustive minimality:
No 1- or 2-atom generic candidate may achieve >=90% accuracy on either the source
holdout or either hidden cross-surface audit for a retained concept.

G3 Cross-surface transfer:
Each retained concept must achieve >=90% accuracy on two unseen surface types,
after independent symbol renaming, pair-order permutation, and distractor injection,
with >=50% search-cost reduction versus a fresh learner.

G4 Sequential compounding:
For the three successive acquisitions, the prefix-library transfer cost ratio
R_n = C(T_n | K_(n-1)) / C(T_n | K0) must be <0.75, and target accuracy >=90%.

G5 Higher-order composition:
A hidden target formed from two previously learned concepts using generic AND/OR/XOR
composition must reach >=90% accuracy and >=25% search-cost reduction versus fresh
composition search.

G6 Deletion/rehydration:
Clearing the learned library must remove >=80% of the measured transfer advantage;
serializing and reloading it must restore >=90% of the pre-deletion advantage.

G7 Negative controls:
Three independent label-shuffled training controls must fail concept admission
on the corresponding hidden audits. No shuffled control may obtain >=90% hidden audit
accuracy.

G8 Independence:
Two deterministic seed blocks must satisfy every required gate. No threshold,
seed, surface, or candidate family may be changed after inspecting a result.

Terminal rule:
Any failure after mechanical implementation defects are fixed => KILLED.
All gates on both blocks => PASS.
This protocol does not claim ASI; it tests whether H4 concept reuse survives stronger
representation and negative-control challenges.

Frozen test revision: G1-G8; no post-result threshold changes permitted.

Trigger revision 2: evaluator logic unchanged.
