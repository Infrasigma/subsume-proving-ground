# ACE Recursive Capability-Compounding Status

## Evidence classification

**TESTED BUT LIMITED — a bounded acquisition-method artifact loop is implemented, but autonomous general recursive improvement and AGI remain UNPROVEN.**

No AGI claim is made.

## Exact repository state

Branch: `ace-full-system-20260911`

Latest branch HEAD (test/guard commit): `afd35b741787aad69c7aa648183ac4c48cec401a`

Latest implementation commit covered by this report: `c222d8fd845e0d153df1037ae87507f13eed1693`

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

A search-space failure generates multiple method hypotheses. Candidates are executed and independently evaluated; candidates can be rejected. The currently successful bounded procedure is executable-frontier expansion, which reorders the real synthesis frontier so branching is attempted before the pre-improvement arithmetic path.

### Installation and future causal effect

`InstalledMethodRegistry` refuses incomplete artifacts, records installation, and applies installed procedures to future acquisition. The before/after execution trace is part of the evidence.

### Adaptive runtime

`AdaptiveAcquisitionRuntime.ImproveAndAcquire` performs diagnosis -> method generation -> method evaluation -> installation -> future acquisition and updates method history.

## Decisive bounded result

The repaired recursive protocol no longer contains the previous experiment-side `arithmetic failed -> use conditional search` authority.

It records an arithmetic acquisition failure, diagnoses search-space insufficiency, generates competing method artifacts, independently evaluates them, installs the selected method, and then uses the installed method to change the real future search path. A future piecewise transformation is acquired by that changed path while the pre-improvement arithmetic-only mechanism fails.

This is the strongest currently justified result: **a tested causal acquisition-method-to-future-acquisition loop on a bounded executable substrate.**

## Important scientific correction: prior replication invalidated

The previous `ReplicatedDeepFamily` was algebraically `y = 2x + 5`, despite being described as depth-3 composition. A baseline universal/arithmetic mechanism can solve that function without retained composition. CI exposed this degeneracy. The replication function and test were therefore changed to explicitly record `baseline_degenerate=1`, `valid=0`, `wins=0` rather than fabricate a replication win.

Consequently, no prior `mean_R < 1` result from that family is retained as evidence.

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

1. An initial implementation failed CI because `CapabilitySpecification.Structure` was referenced even though that field does not exist. The error was repaired from the exact CI log.
2. A bootstrap seed accidentally selected the deep-composition task instead of affine acquisition. The protocol was corrected to select task families directly rather than relying on family-index arithmetic.
3. The first replicated task family was scientifically degenerate because its nominal depth-3 computation simplified to an affine function. The evidence was explicitly invalidated instead of weakening the baseline.

These failures are engineering/scientific corrections, not intelligence evidence.

## Validation state

The latest implementation has triggered dedicated ACE compounding, focused, and repository-wide GitHub Actions validation. At the time of this status update, the exact latest source/test SHA validation was queued/in progress; therefore **no green-CI claim is made for `afd35b7` yet**.

Historical Phase 2.1 conformance failures are not rewritten or suppressed.

## Highest-value next boundary

The current bottleneck is now clear and narrower than before:

`hand-authored method hypothesis generator -> autonomously synthesized/verified method generator`

The next decisive experiment must require ACE to construct the method-generation machinery itself, verify it independently, install it, and use it to discover a second acquisition improvement on structurally novel hidden tasks with frozen full cost accounting.

Only after that should `M1 -> M2`, replicated `R_n`, and broader world/representation/causal integration be promoted.

**Bottom line:** the M1 oracle boundary has been crossed only in a bounded research implementation. The genuinely hard recursive boundary — ACE learning the machinery that invents its own acquisition improvements — remains open and is the next target.
