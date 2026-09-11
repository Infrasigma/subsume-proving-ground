# ACE Autonomous Acquisition-Machinery Status

## Scientific classification

**TESTED BUT LIMITED**

This document is an evidence ledger, not an AGI claim. The current implementation introduces an executable acquisition-procedure IR and blind enumeration over a generic stream-transformation algebra, but the new boundary remains subject to CI and adversarial experimental validation.

## 1. Starting and current HEAD

Starting branch tip inspected at task start: `ee042690b0e6f475ec883fe819018a3b2b2c1052`.

Relevant pre-existing correction: `0393e2a51e7bf27f02ea0d82ca7031984bce8f08` (V2 train/held-out leakage correction).

Engineering commits in this cycle:
- `1e2a265a273067ffa092aa27917038be4f300e3a` — executable acquisition-procedure IR and generic procedure enumeration.
- `43186b3e0660a528520bbdfbafa7e913aa893363` — autonomous method improvement now uses blind executable-procedure enumeration instead of `GenerateMethodCandidates`.
- `7ee069ba9cd9e33e2b4a2a6d508d255b97b1d779` — persistent method registry with restart restore.
- `c565c3c6a7d2777ca2b0ce35c7029c0569404aa1` — acquisition-procedure and restart tests.
- `31c9ecea3fac0e80dd1244d084d969357f5ec4a6` — installed procedures now compose on the live candidate stream rather than restarting static search for each method.
- `3a4d9e6279a9652cf0fd77a9657c77b72a5f9cfd` — runtime integration of persistent acquired methods.

At the time of this ledger update, final HEAD is `3a4d9e6279a9652cf0fd77a9657c77b72a5f9cfd`.

## 2. Trusted bootstrap definition

Trusted bootstrap currently includes the Go runtime, bounded executable-program interpreter, primitive arithmetic/boolean/control-flow representation, deterministic hashing/serialization, resource accounting, persistence primitives, independent behavioral execution checks, and the bounded mechanism-candidate enumeration already represented by `UniversalMechanismSearch`.

The new acquisition layer does **not** contain a table mapping bottleneck classes to named acquisition strategies.

## 3. Acquired-machinery definition

An acquired mechanism is an executable artifact with a deterministic serialized representation, independent behavioral evidence, installation through the same method registry used for future acquisition, execution-visible trace effects, and optional persistence/restart restoration.

The new artifact is `AcquisitionProcedure`: a versioned sequence of generic candidate-stream transformations. Its semantics are executable, not metadata-only.

## 4. Developer-authored machinery audit

### Removed from the causal acquisition path

`GenerateMethodCandidates` is no longer called by `AutonomousMethodImprovement`.

The prior developer-authored named procedures (`expand-executable-frontier`, `revise-representation-then-search`, `decompose-and-compose`, etc.) are no longer the source of method discovery.

### Remaining trusted/searchable machinery

The bootstrap still defines a finite generic procedure algebra (`identity`, `reverse`, `dedupe`, `sort-cost`, `take`, `rotate`) and exhaustively enumerates short compositions. This is intentionally explicit and auditable. It is a remaining bootstrap limitation, not evidence of unrestricted self-improvement.

`UniversalMechanismSearch` remains a finite substrate enumerator for three mechanism families. The new work changes how that candidate stream can be transformed and retained; it does not yet establish open-ended invention of arbitrary new computational primitives.

## 5. Architecture changes

1. Added executable `AcquisitionProcedure` IR.
2. Added generic blind enumeration of procedure compositions.
3. Changed autonomous method improvement to evaluate executable procedure artifacts rather than a hand-authored candidate table.
4. Changed installed-method application so procedures transform the current candidate stream, enabling genuine composition.
5. Added persistent method artifacts and restart restoration.
6. Integrated persistence into `AdaptiveAcquisitionRuntime`.
7. Added regression coverage for executable procedure acquisition and restart.

## 6. Acquisition protocol

The intended loop is:

`failure telemetry -> evidence-based diagnosis -> blind executable procedure enumeration -> independent held-out evaluation -> regression check -> installation -> future acquisition -> trace comparison`.

The candidate procedure itself is not told the desired algorithm or named decomposition. The verifier remains separate from proposal enumeration.

## 7. Train / held-out protocol

Training observations are used only to construct the capability specification. Held-out cases are supplied only to evaluation. The previous V2 future-example leakage was corrected before this cycle.

No acceptance claim should be made unless held-out data are demonstrably absent from candidate construction and procedure generation.

## 8. Leakage audit

Required checks:
- no held-out values in procedure enumeration;
- no target-specific procedure names in the enumerator;
- no benchmark-specific lookup tables;
- serialized procedure contains only generic operations;
- independent evaluator is outside procedure construction;
- future task distribution is generated independently of the learned procedure.

Current implementation passes the source-level structural checks above; execution-level CI evidence is still required for a final classification change.

## 9. M0 / A1 / M1 / A2 / M2

Current state does not yet establish the complete chain.

- **M0:** bounded trusted executable substrate — present.
- **A1:** acquired task capability — present in the earlier acquisition subsystem, with limited domain evidence.
- **M1:** executable acquired search procedure — newly implemented and experimentally targeted; not yet promoted beyond `TESTED BUT LIMITED` pending CI and causal ablation evidence.
- **A2:** structurally different capability acquired using M1 — not yet established.
- **M2:** second acquisition mechanism discovered using M1 — not yet established.

## 10. Cost accounting

The existing `ResourceVector` records compute, memory, storage, time, and experiment budget. The new procedure enumeration itself must be charged as discovery work; candidate evaluation, verification, regression, and transfer must also be counted.

A downstream speedup without discovery-inclusive cost is not accepted as a capability-compounding result.

## 11. Causal ablations required

Required comparisons:
- bootstrap only vs bootstrap + M1;
- M1 removed after acquisition;
- learned procedure replaced by identity/no-op control;
- learned procedure replaced by shuffled procedure control;
- persistent method removed after restart;
- procedure retained but candidate-generation evidence withheld;
- structurally distinct task family.

The decisive test is whether M1 causally reduces the discovery-inclusive cost of later acquisition rather than merely changing an in-memory registry.

## 12. Restart

`PersistentMethodRegistry` serializes acquired executable procedures and can restore them into `InstalledMethodRegistry`. A restart test has been added. Full scientific restart evidence requires a successful CI execution of the test and a future-task trace showing the restored procedure changes acquisition behavior.

## 13. Structural transfer

Current method artifacts operate on candidate streams rather than on task-specific values. This provides a clean route to transfer, but current mechanism candidates remain limited to the existing universal substrate. Transfer beyond that substrate is unproven.

## 14. Replication

Replication across seeds, independently generated task instances, and at least one structurally distinct task family remains required before any recursive-improvement promotion.

## 15. Adversarial failures and invalidated evidence

Known prior invalidations remain preserved:
- earlier recursive-depth evidence was not accepted as proof;
- earlier acquisition-method experiments exposed candidate rejection;
- V2 future-example leakage was identified and corrected;
- a static developer-authored method generator was identified as a scientific confound and removed from the new autonomous method path.

No historical negative evidence is overwritten by this cycle.

## 16. Open bottlenecks

1. The searchable mechanism substrate is still finite and largely hand-defined.
2. The generic procedure algebra is finite and bootstrap-defined.
3. Representation invention is not open-ended.
4. Failure diagnosis remains bounded and evidence-rule based.
5. Active experiment selection is not yet demonstrated as independently causal.
6. No broad world/entity/causal transfer result exists.
7. Discovery-inclusive cost telemetry is not yet complete enough for a recursive-capability claim.
8. M1 -> A2 -> M2 causal recursion remains unproven.

## 17. Exact classification rule

Current classification: **TESTED BUT LIMITED**.

Promotion to `BOOTSTRAP ESCAPE DEMONSTRATED` requires an acquired executable procedure absent from the initial installed method set, independent verification, future unseen-task acquisition change, and restart survival.

Promotion to `RECURSIVE CAPABILITY IMPROVEMENT DEMONSTRATED` additionally requires M1-caused later acquisition machinery, held-out evaluation, discovery-inclusive cost, causal ablation, restart, replication, and structural transfer.

`AGI-RELEVANT EVIDENCE` remains reserved for substantially broader cross-domain and increasingly open-ended capability acquisition.

## 18. Final decision questions

Q1 capability not explicitly supplied: **limited evidence exists; broad claim unproven**.

Q2 acquisition machinery itself acquired: **new executable procedure path implemented; scientific proof pending**.

Q3 acquired machinery changes future computation: **instrumented and test-covered; final CI evidence pending**.

Q4 structurally different capability using acquired machinery: **unproven**.

Q5 second acquisition mechanism: **unproven**.

Q6 first mechanism causally enables later acquisition: **unproven**.

Q7 restart: **implemented/test added; execution evidence pending**.

Q8 independent task generation: **not yet established for recursive chain**.

Q9 structural transfer: **not yet established for recursive chain**.

Q10 discovery-inclusive cost improvement: **not yet established**.

## Bottom line

This cycle attacks the correct bottleneck: the acquisition strategy is now representable as an executable artifact and can be generated by blind search over a generic procedure algebra rather than by a developer-authored method table. That is a meaningful architectural change.

It is **not yet evidence of recursive intelligence or AGI**. The decisive remaining experiment is M0 -> A1 -> M1 -> A2 -> M2 with causal ablation and discovery-inclusive accounting on independently generated, structurally distinct tasks.
