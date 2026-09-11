# ACE Recursive Capability-Compounding Status

## Evidence classification

**TESTED BUT LIMITED — a bounded acquisition-method artifact loop is implemented, but autonomous general recursive improvement and AGI remain UNPROVEN.**

No AGI claim is made.

## Exact repository state

Branch: `ace-full-system-20260911`

Latest branch HEAD for this mission checkpoint: `0393e2a51e7bf27f02ea0d82ca7031984bce8f08`

Mission starting HEAD supplied by the request: `b53756c8bb36c1f33c5aabfcdb9657def7d6cad7`.

Engineering commits in this checkpoint:
- `ad7a47734e95ce89afd9dea43c551e9019fd7740` — narrow compile repair preserving V2 verification-cost accounting.
- `0393e2a51e7bf27f02ea0d82ca7031984bce8f08` — enforce a genuine training/held-out split for the V2 future task.

The status file itself is documentation-only.

## Actual executed data flow

`behavioral task evidence -> capability specification -> pre-improvement attempt -> independent failure/counterexample telemetry -> bottleneck diagnosis -> competing acquisition-method hypotheses -> executable method evaluation -> verified method artifact -> installation -> changed candidate ordering -> future capability acquisition -> retention/history`

The independent evaluator remains outside the cognitive mechanism.

## Implemented breakthrough

### First-class acquisition method

`AcquisitionMethodArtifact` records identity, preconditions, expected strengths/failure modes, input capability specification, executable procedure, representation policy, candidate policy, verification policy, resource model, provenance, dependencies, performance statistics, regression constraints, transfer evidence, and an artifact representation.

### Evidence-driven diagnosis

`DiagnoseBottleneck` derives a bottleneck from candidate failures, counterexamples, search path, representation trace and verification outcomes. It does not receive the hidden solution or an experiment-side instruction such as "use conditional search."

### Competing method hypotheses

A search-space failure generates multiple method hypotheses. Candidates are executed and independently evaluated; candidates can be rejected. The currently intended successful bounded procedure is executable-frontier expansion, which changes the real synthesis ordering so branching can be attempted before the pre-improvement arithmetic path.

### Installation and future causal effect

`InstalledMethodRegistry` refuses incomplete artifacts, records installation, and applies installed procedures to future acquisition. The before/after execution trace is part of the evidence.

### Adaptive runtime

`AdaptiveAcquisitionRuntime.ImproveAndAcquire` performs diagnosis -> method generation -> method evaluation -> installation -> future acquisition and updates method history.

## Latest empirical red-team result

GitHub Actions run `34550286312` at the pre-split checkpoint established that the isolated universal-branching diagnostic passed, but repository tests failed in three places:

- `TestAdaptiveAcquisitionRuntimeCausalCompounding`: all acquisition-method candidates rejected.
- `TestRecursiveCapabilityCompoundingCore`: autonomous method improvement failed because all acquisition-method candidates were rejected.
- `TestSelfExtensibleCapabilityProbe`: acquired operator did not reduce future search cost.

This is important negative evidence. It means the earlier apparent recursive loop cannot be promoted merely because its classes and test scaffolding exist.

The diagnostic passed, so the earlier universal-branching synthesis ceiling is not the immediate blocker at this checkpoint. The remaining failures are acquisition/experimental-design failures, not evidence of AGI.

## Scientific correction applied

The V2 future task previously used the same examples as both `KnownExamples` and `hidden`. That permits a synthesizer to exploit finite-example interpolation and makes the before/after cost comparison scientifically weak.

Commit `0393e2a51e7bf27f02ea0d82ca7031984bce8f08` changes V2 to use:

- future training examples: `(-7,-6),(0,0),(4,8)`;
- genuinely held-out future examples: `(-1,0),(9,18)`.

The primary future acquisition measurement therefore cannot simply succeed by fitting the same cases supplied to synthesis.

## What remains UNPROVEN

- The acquisition-method hypothesis generator is still hand-authored and keyed by evidence classes; it is not itself autonomously synthesized.
- The executable substrate remains bounded integer/boolean synthesis.
- Open-ended self-generated task discovery is not demonstrated.
- General representation invention is not demonstrated.
- Autonomous causal-model discovery is not integrated into meta-acquisition.
- Broad structural transfer across relational, planning, causal and program-transformation domains is not demonstrated.
- Full compute-inclusive `(E,I,S,D)` accounting is not frozen and replicated.
- No defensible compute-inclusive `R_n < 1` claim exists.
- A second recursively discovered method `M1 -> M2` is not demonstrated.
- Recursive architecture self-modification is not demonstrated.
- AGI is unproven.

## Failure record

1. The V2 compile blocker was an unused `cost1`; it was repaired without deleting verification-cost accounting.
2. The isolated universal branching diagnostic passed, while the broader acquisition-method tests failed. This localizes the current bottleneck away from the basic branching synthesizer.
3. The adaptive runtime rejected all generated acquisition-method candidates. This indicates the method-generation/evaluation contract is not yet producing a verified winner on the current telemetry/task split.
4. The original V2 future comparison reused training examples as hidden examples. That design flaw was corrected with a strict training/held-out split before accepting any future-cost claim.
5. The previous replicated task family was algebraically degenerate; its claimed replication win remains explicitly invalidated rather than reused.

These failures are engineering/scientific corrections, not intelligence evidence.

## Current architecture bottleneck

The highest-leverage unresolved boundary is:

`hand-authored acquisition-method hypothesis generator -> executable, independently acquired method-generation machinery`

The current code can represent and retain executable methods, but that does not yet mean ACE discovered a structurally new search procedure. In particular, `executeAcquisitionMethod` still routes procedures into `UniversalMechanismSearch`, and the current candidate procedures are developer-authored names/semantics.

The next architecture should therefore make search-procedure construction itself a normal capability acquisition problem rather than adding another `MethodGeneratorV3` special case.

## Required decisive M1/M2/M3 experiment

M1 must be acquired from a trusted bootstrap whose searchable machinery does not already contain the target mechanism. M1 must be independently verified, persisted, and shown to change acquisition on unseen tasks.

M2 must then be acquired using M1, from a structurally different task family, with an ablation removing M1.

M3 must be acquired using M1+M2, with ablations removing either mechanism and with discovery-inclusive cost accounting.

The primary tests must use multiple seeds, independent task generation, held-out transfer, restart, distributions, and paired comparisons.

## Cost definition

The primary metric must include discovery and verification rather than only final execution:

`C = candidate generation + candidate evaluation + failures + execution/simulation + verification + memory + wall time + compute + acquisition overhead + installation + regression + transfer testing`.

Scalarized cost may be reported, but component-wise costs must remain visible. Any discovery-excluded result is sensitivity analysis only.

## Acceptance ladder

The exact permitted classifications are:

- `UNPROVEN`
- `TESTED BUT LIMITED`
- `PARTIAL`
- `BOOTSTRAP ESCAPE DEMONSTRATED`
- `RECURSIVE CAPABILITY IMPROVEMENT DEMONSTRATED`
- `AGI-RELEVANT EVIDENCE`

Current classification: **TESTED BUT LIMITED**.

Reason: a bounded executable acquisition-method loop is implemented and the universal branching diagnostic passes, but the broader acquisition-method tests currently fail and the stronger causal M1 -> M2 -> M3 evidence has not been established.

## Central scientific questions

### Question 1
Did ACE acquire a computational mechanism absent from its initial trusted/searchable acquisition substrate, independently verify it, retain it, and then use it to make acquisition of a genuinely novel capability measurably cheaper or more successful?

**Current answer: NOT ESTABLISHED.** The current retained artifact is executable, but its structural novelty relative to the searchable universal substrate remains unresolved; the broader acquisition-method tests also fail.

### Question 2
Did the acquired mechanism itself enable acquisition of another structurally different acquisition mechanism, producing measurable recursive capability compounding under discovery-inclusive cost?

**Current answer: NO.** No valid M1 -> M2 -> M3 result with the required causal ablations, replication, restart, structural transfer, and full discovery-inclusive cost has been demonstrated.

## Ultimate invariant

The implementation should converge toward:

`Experience -> Representation -> Hypothesis -> Experiment -> Knowledge -> Capability -> Failure -> Diagnosis -> Missing Capability -> Mechanism Search -> Verified Mechanism -> Integration -> New Acquisition Power -> Repeat`

No AGI claim is made by this status file. A future success must earn a stronger classification through the acceptance ladder rather than through naming, module count, or green CI.
