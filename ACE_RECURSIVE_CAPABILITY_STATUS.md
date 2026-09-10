# ACE Recursive Capability-Compounding Status

## Evidence classification

**TESTED BUT LIMITED — acquisition-method artifacts and a causal future-acquisition runtime are implemented; true recursive capability improvement and AGI remain UNPROVEN.**

No AGI claim is made.

## Source milestone

Latest source-code milestone covered by this report:

`2eb9c9213bc7de1d994e4ab6131bd3f04f32430f`

The branch may advance with documentation-only commits; this SHA identifies the implementation under evaluation.

## Actual executed data flow

`behavioral task evidence -> capability specification -> pre-improvement attempt -> independent failure/counterexample telemetry -> bottleneck diagnosis -> competing acquisition-method hypotheses -> executable method evaluation -> verified method artifact -> installation -> changed candidate ordering -> future capability acquisition -> retention/history`

The independent evaluator remains outside the cognitive mechanism.

## Implemented breakthrough

### First-class acquisition method

`AcquisitionMethodArtifact` contains identity, preconditions, expected strengths/failure modes, input capability specification, executable procedure, representation policy, candidate policy, verification policy, resource model, provenance, dependencies, learned performance statistics, regression constraints, transfer evidence, and an executable artifact representation.

### Evidence-driven diagnosis

`DiagnoseBottleneck` derives a bottleneck from candidate failures, counterexamples, search path, representation trace and verification outcomes. It does not receive the hidden target or an experiment-side instruction such as "use conditional search."

### Competing method hypotheses

For a search-space failure the runtime generates multiple method hypotheses, including executable-frontier expansion, representation revision and decomposition/composition. Candidates are actually executed and independently evaluated; a candidate can be rejected.

### Verified installation

`InstalledMethodRegistry` accepts only complete method artifacts and records installation plus before/after execution traces. The frontier-expansion method materially changes future search by moving branching synthesis ahead of the pre-improvement arithmetic path.

### Adaptive runtime

`AdaptiveAcquisitionRuntime.ImproveAndAcquire` closes diagnosis -> method generation -> method evaluation -> installation -> future acquisition and updates method history.

## Decisive bounded experiment

Stage 1 acquires an affine capability from behavioral examples.

Stage 2 deliberately demonstrates failure of the arithmetic-only mechanism on a conditional task. Observable telemetry is then passed to autonomous method diagnosis/generation/evaluation. The harness does not select the winning method.

Stage 3 is an independently specified piecewise transformation. The pre-improvement arithmetic-only search fails. The installed method changes the real search path and branching synthesis acquires the future capability.

The source-level test suite also checks reuse on a distinct latent threshold instance.

## Scientific classification

The strongest justified statement is:

> ACE now contains a tested bounded mechanism in which an acquisition failure produces telemetry, telemetry produces competing acquisition-method artifacts, a method is independently verified and installed, and the installed method causally changes a subsequent acquisition path that succeeds where the pre-improvement mechanism fails.

This is **not** yet evidence of open-ended recursive intelligence.

## Known limitations

- Method-hypothesis generation is still a hand-authored generic transformation library keyed by evidence classes. It is not yet a learned universal method-discovery process.
- The executable substrate remains bounded integer/boolean synthesis; it is not yet a general typed programming environment.
- The task laboratory remains small and source-defined rather than an open-ended self-generated challenge distribution.
- Method verification currently uses source-level held-out behavioral cases, not a hardened external blind evaluation service.
- Transfer is presently within the scalar executable domain; broad cross-domain transfer is unproven.
- Method performance history exists in the adaptive runtime but is not yet a persistent knowledge object with calibrated predictive uncertainty.
- Full `(E,I,S,D)` accounting is not frozen across experimentation, inference/search, synthesis, verification, compute, wall-clock time, retries and failed candidates.
- Therefore no compute-inclusive replicated `R_n < 1` claim is made.
- A second-order recursive improvement `M1 -> M2` has not been demonstrated.
- Recursive architecture self-modification has not been demonstrated.
- AGI is unproven.

## Failure record

A previous implementation run failed because `adaptive_runtime.go` referenced `CapabilitySpecification.Structure`, which does not exist. The CI log identified the exact compile error; the code was repaired by deriving the history key from available specification fields. A later experiment failure showed that the bootstrap task-family seed selected deep composition instead of affine acquisition; the seed was corrected rather than weakening the test. These are recorded as engineering/debugging failures, not evidence of intelligence.

The historical Phase 2.1 conformance failure is intentionally not rewritten or made to pass.

## Validation state

Dedicated ACE and repository-wide GitHub Actions are required for each source milestone. The implementation under `2eb9c9213bc7de1d994e4ab6131bd3f04f32430f` has a dedicated compounding run and repository validation runs in progress/queued at the time of this documentation update; until those exact-SHA runs complete green, they are **not** counted as validated evidence.

## Next decisive work

1. Replace the evidence-class switch with learned, executable method generation whose candidates are themselves synthesized and verified.
2. Persist method artifacts and calibrated performance evidence in the knowledge lifecycle.
3. Freeze full cost accounting before any recursive `R_n` claim.
4. Generate structurally novel hidden tasks without source-visible solution structure.
5. Stress method transfer under representation, mechanism and causal distribution shift.
6. Only then attempt `M1 -> M2` and measure whether acquisition competence continues to improve.

**Bottom line:** the experiment-side M1 oracle has been removed from the bounded protocol, and acquisition methods now exist as executable artifacts with a real installation effect. The remaining scientific gap is much harder: ACE must learn the machinery that generates those method artifacts rather than relying on the currently hand-authored generic method hypothesis generator.
