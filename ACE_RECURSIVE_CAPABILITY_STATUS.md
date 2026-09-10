# ACE Recursive Capability-Compounding Status

## Evidence classification

**TESTED BUT LIMITED — autonomous acquisition-method capability is implemented, but true recursive capability improvement and AGI remain UNPROVEN.**

No AGI claim is made.

## Source milestone

Source-code HEAD at the time of this report:

`70dd64d4e1ef6bf9f2ba1fb3bb92c33e55f26572`

This report is documentation-only and must not be treated as source evidence.

## Actual architecture executed

The current compounding path contains:

`behavioral task evidence -> capability specification -> bounded executable mechanism search -> independent counterexample/held-out verification -> failure telemetry -> evidence-driven bottleneck diagnosis -> competing acquisition-method hypotheses -> executable method candidates -> independent method evaluation -> verified method installation -> changed future search order -> future capability acquisition`

The installed-method registry records executable method artifacts and before/after execution traces.

## What changed in this milestone

### 1. First-class acquisition-method artifact

`AcquisitionMethodArtifact` records identity, applicability/preconditions, expected strengths/failure modes, input capability specification, executable procedure, representation/candidate/verification policies, resource model, provenance, dependencies, performance statistics, regression constraints, transfer evidence, and an artifact representation.

### 2. Evidence-driven bottleneck diagnosis

`DiagnoseBottleneck` consumes acquisition telemetry rather than a task-family label or hidden solution. The telemetry includes candidate failures, counterexamples, representation/search traces, verification outcomes and cost.

### 3. Competing method generation

`GenerateMethodCandidates` produces multiple executable method hypotheses from the diagnosed evidence class. The harness does not select a method by name.

### 4. Independent method evaluation

Candidates are executed through the acquisition substrate and tested against held-out behavioral cases. Failed candidates are retained as rejected hypotheses rather than being silently converted into success.

### 5. Installation with real causal effect

`InstalledMethodRegistry.Apply` executes the installed method. The frontier-expansion method changes the actual acquisition candidate order by moving executable branching ahead of the pre-improvement arithmetic-only order. A trace records the before/after path.

### 6. Adaptive acquisition runtime

`AdaptiveAcquisitionRuntime.ImproveAndAcquire` closes diagnosis -> method generation -> method evaluation -> installation -> future acquisition in one executable path and records method history.

## Decisive experiment boundary

The repaired recursive protocol now removes the previous experiment-side `arithmetic failed -> use conditional search` authority.

The protocol instead observes an arithmetic failure, constructs telemetry, diagnoses search-space insufficiency, generates competing method artifacts, independently evaluates them, installs the selected method, and uses the installed method to change future acquisition behavior.

The future task is a piecewise transformation that the pre-improvement arithmetic-only mechanism cannot acquire but the installed branching search can acquire.

This demonstrates a **causal method-to-future-acquisition path on a bounded executable substrate**.

## What is NOT demonstrated

- Fully autonomous invention of arbitrary new acquisition algorithms.
- General typed executable mechanism construction.
- Open-ended task generation beyond the bounded task lab.
- General representation invention.
- Autonomous causal hypothesis generation/discrimination integrated with method discovery.
- Compute-inclusive replicated `R_n < 1`.
- Structurally broad cross-domain transfer (arithmetic -> relational/planning/causal/program transformation).
- A second independently discovered recursive improvement `M1 -> M2`.
- Recursive self-modification of the cognitive architecture.
- Broad general intelligence or AGI.

## Cost accounting

The current compounding protocol exposes only bounded substrate costs and candidate/search counts. It does **not** yet provide a frozen, compute-inclusive `(E,I,S,D)` ledger covering experimentation, inference/search, synthesis, verification, compute, wall-clock time, retries and failed candidates for a defensible recursive `R_n` comparison.

Therefore no current method-compounding result is promoted to a full compute-inclusive `R_n < 1` claim.

## Leakage boundary

The task harness may define task generators and independent evaluators. It must not provide hidden solution programs or the correct acquisition method to ACE. The new recursive path contains no `conditionalCandidate` oracle or equivalent experiment-side method-selection authority.

The present tests are still source-level research protocols rather than a hardened blind external evaluation environment; leakage robustness therefore remains LIMITED.

## Validation

The branch has repeatedly used repository-wide Go CI and dedicated ACE compounding workflows. A source failure discovered during this milestone was a concrete compile error (`CapabilitySpecification.Structure` does not exist); it was diagnosed from CI logs and repaired without weakening acceptance criteria.

The newest source milestone is subject to its own GitHub Actions validation. A green result must be recorded against the exact source SHA before being treated as engineering evidence.

Historical Phase 2.1 conformance failures involving the frozen legacy development gate are not rewritten or reclassified by this milestone.

## Remaining highest-value work

1. Make method artifacts persistent first-class knowledge objects rather than an in-memory registry only.
2. Replace the hand-authored bottleneck-to-method candidate mapping with a learned/general program-transformation mechanism.
3. Make method discovery itself subject to blind, structurally novel transfer tests.
4. Freeze and measure the complete acquisition cost vector before comparing `K_n` and `K_{n+1}`.
5. Build self-generated challenge tasks from the self-model and feed their failures into the same method-discovery loop.
6. Attempt a second-order improvement only after the first method survives adversarial replication.

**Bottom line:** the missing boundary has been crossed at the level of a bounded, executable research mechanism: acquisition methods are now artifacts that can be diagnosed, proposed, independently evaluated, installed, and causally used by a future acquisition path. The stronger claim — that ACE autonomously improves its own general capability-acquisition competence in an open-ended, compute-inclusive, structurally broad setting — remains UNPROVEN.
