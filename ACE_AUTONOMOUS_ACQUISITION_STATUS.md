# ACE Autonomous Acquisition-Machinery Status

## Scientific classification

**TESTED BUT LIMITED**

This is an evidence ledger, not an AGI claim. The cycle introduced executable acquisition-procedure machinery, but the full recursive chain is not yet demonstrated.

## 1. Starting / final HEAD

Starting HEAD: `ee042690b0e6f475ec883fe819018a3b2b2c1052`.
Final HEAD for this cycle: `e2e027ca14eabfd5e36a5ecbf1f142e6d841b89b`.

Relevant commits:
- `1e2a265a273067ffa092aa27917038be4f300e3a` — executable acquisition-procedure IR.
- `43186b3e0660a528520bbdfbafa7e913aa893363` — autonomous method path no longer calls `GenerateMethodCandidates`.
- `7ee069ba9cd9e33e2b4a2a6d508d255b97b1d779` — persistent acquired-method registry.
- `c565c3c6a7d2777ca2b0ce35c7029c0569404aa1` — procedure/restart tests.
- `31c9ecea3fac0e80dd1244d084d969357f5ec4a6` — live candidate-stream composition.
- `3a4d9e6279a9652cf0fd77a9657c77b72a5f9cfd` — runtime persistence integration.
- `1593e7bfce3f6c61d6f2dd3787c781709cbe7888` — independent generic-builder verification.
- `e2e027ca14eabfd5e36a5ecbf1f142e6d841b89b` — live-search/restart regression assertions.

Pre-existing scientific correction: `0393e2a51e7bf27f02ea0d82ca7031984bce8f08` removed V2 future-example leakage.

## 2. Trusted bootstrap

The trusted bootstrap includes bounded execution, primitive arithmetic/boolean/control flow, deterministic serialization/hash, persistence, resource accounting, independent behavioral checking, and the finite mechanism enumeration represented by `UniversalMechanismSearch`.

The new autonomous method path no longer contains a bottleneck-class -> named-method table.

## 3. Acquired machinery

`AcquisitionProcedure` is a versioned executable artifact containing generic candidate-stream transformations. It can be serialized, installed, composed on a live candidate stream, independently evaluated through a separate builder/evaluator path, and persisted/restored.

## 4. Developer-authored machinery audit

`GenerateMethodCandidates` is no longer in the causal path of `AutonomousMethodImprovement`.

The old named procedures such as `expand-executable-frontier`, `revise-representation-then-search`, and `decompose-and-compose` are no longer the source of future acquisition strategy.

Remaining limitation: the bootstrap defines a small generic algebra (`identity`, `reverse`, `dedupe`, `sort-cost`, `take`, `rotate`) and exhaustively enumerates short programs. This is explicit bootstrap/search substrate, not an open-ended learned generator.

## 5. Architecture changes

1. Executable search-procedure IR.
2. Blind enumeration of generic procedure programs.
3. Autonomous method evaluation over executable artifacts.
4. Live candidate-stream composition.
5. Persistent method storage and restart restore.
6. Runtime integration.
7. Independent generic builder used for method verification.
8. Adversarial/restart regression coverage.

## 6. Acquisition protocol

`failure telemetry -> diagnosis -> blind procedure enumeration -> independent held-out evaluation -> regression check -> installation -> future acquisition -> execution trace`.

No target algorithm, method name, or decomposition is encoded in the new procedure enumerator.

## 7. Train / held-out and leakage

The V2 leakage flaw is corrected. The procedure enumerator receives no held-out examples. Hidden evaluation is used only after a procedure is constructed.

The source-level audit checks for target-specific method tables, hard-coded method names, and held-out data in procedure construction. Final scientific acceptance still requires execution-level leakage audits.

## 8. CI evidence

A CI run on the earlier implementation commit `fb35b444e9f7a1b7944551910736d76a31f71055` failed `go test ./...`. Failures included the recursive compounding tests, `TestAutonomousMethodImprovementFromTelemetry`, and `TestSelfExtensibleCapabilityProbe`. This negative evidence is preserved and was not relabeled as success.

A subsequent run for the independent-builder correction also failed in the existing Phase 2.1 conformance job. A fresh `ACE Capability Compounding` run for final HEAD `e2e027ca14eabfd5e36a5ecbf1f142e6d841b89b` was still in progress at ledger finalization; therefore no green full-system claim is made.

## 9. M0 / A1 / M1 / A2 / M2

- **M0:** bounded trusted executable substrate — present.
- **A1:** limited acquired capability evidence — present in the earlier system.
- **M1:** executable acquired search-procedure architecture — implemented and directly targeted; empirical promotion remains pending.
- **A2:** structurally different capability acquired through M1 — unproven.
- **M2:** second acquisition mechanism discovered through M1 — unproven.

## 10. Cost accounting

`ResourceVector` tracks compute, memory, storage, time, and experiment budget. The new procedure search itself must be counted as discovery cost, along with failed candidates, evaluation, verification, regression, installation, and transfer.

No recursive-improvement claim is accepted without discovery-inclusive cost.

## 11. Causal ablations still required

Required controls:
- M0 vs M0+M1;
- remove M1;
- identity/no-op procedure control;
- shuffled procedure control;
- restart with persisted artifact only;
- independently generated task family;
- structurally distinct capability family.

## 12. Restart

`PersistentMethodRegistry` serializes executable procedures and restores them into the runtime registry. Restart tests exist. Scientific restart evidence is still pending successful CI plus a future-task behavioral trace.

## 13. Structural transfer / replication

The procedure representation is task-stream oriented rather than value-memorizing, but current mechanism candidates remain bounded to the existing universal substrate. Cross-family transfer and multi-seed replication are unproven.

## 14. Known failures / invalidated evidence

- Earlier recursive-depth evidence was rejected as insufficient.
- Acquisition-method candidates previously failed independently.
- V2 future-example leakage was identified and corrected.
- The developer-authored acquisition-method table was identified as a confound and removed from the new causal path.
- CI failures in this cycle remain negative evidence.

## 15. Open bottlenecks

1. Finite, hand-defined mechanism substrate.
2. Finite bootstrap procedure algebra.
3. Non-open-ended representation invention.
4. Bounded diagnosis.
5. No demonstrated internal active experiment selection.
6. No broad world/entity/causal transfer.
7. Incomplete discovery-inclusive cost telemetry.
8. No demonstrated M1 -> A2 -> M2 recursion.

## 16. Acceptance ladder

Current classification: **TESTED BUT LIMITED**.

`BOOTSTRAP ESCAPE DEMONSTRATED` requires an executable acquired procedure absent from the initial installed set, independent verification, changed unseen-task acquisition, and restart survival.

`RECURSIVE CAPABILITY IMPROVEMENT DEMONSTRATED` additionally requires M1-caused later acquisition machinery, held-out evaluation, discovery-inclusive cost, causal ablation, restart, replication, and structural transfer.

`AGI-RELEVANT EVIDENCE` is reserved for substantially broader cross-domain and increasingly open-ended acquisition.

## 17. Decision test

Q1 — capability not explicitly supplied: **limited evidence; broad claim unproven**.
Q2 — machinery itself acquired: **architecture implemented; scientific proof pending**.
Q3 — machinery changes future computation: **instrumented/tested path; final CI evidence pending**.
Q4 — structural capability transfer: **unproven**.
Q5 — second acquisition mechanism: **unproven**.
Q6 — causal impairment when M1 removed: **unproven**.
Q7 — restart survival: **implemented/tested; final execution evidence pending**.
Q8 — independent task generation: **unproven for recursive chain**.
Q9 — structural transfer: **unproven for recursive chain**.
Q10 — discovery-inclusive improvement: **unproven**.

## Bottom line

This cycle crossed an important architectural boundary in representation: acquisition machinery is now an executable, serializable, composable object that can be searched over generically rather than selected by a developer-authored method table.

That is a real engineering leap, but **not yet a scientific demonstration of recursive intelligence or AGI**. The next decisive target remains `M0 -> A1 -> M1 -> A2 -> M2` with causal ablation, restart, independent generation, structural transfer, replication, and discovery-inclusive cost.
