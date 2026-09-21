# ICS v2 — Interventional Schema Falsification Protocol

Hypothesis:
A generic interventional learner can discover reusable transition schemas over an object-relation state representation, transfer them to unseen structures, and compose previously verified schemas without task-family-specific solvers.

Frozen constraints:
- no paid compute;
- no external foundation model;
- no human task labels during runs;
- evaluator code is outside the learned schema library;
- no per-family solver;
- hidden target worlds are generated independently;
- results are recorded before interpretation.

V2 gates:
I2-A Primitive identification on identifiability-controlled worlds >= 0.95.
I2-B Transfer of learned primitive schema to new object counts/topologies >= 0.95.
I2-C Two-schema composition accuracy on hidden worlds >= 0.90.
I2-D Active interaction cost / random interaction cost <= 0.70 on composition.
I2-E All four gates reproduce on two disjoint deterministic seed blocks.

Kill rule:
If the same implementation fails any required gate in both independent seed blocks, the ICS hypothesis is killed for this substrate. A single implementation defect does not justify rewriting the acceptance threshold; repairs must be generic and applied before reruns.

No ASI/AGI claim follows from a pass.
