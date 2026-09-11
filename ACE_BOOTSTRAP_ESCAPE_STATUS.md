# ACE Bootstrap Escape Status

## Status

**PARTIAL — bootstrap ceiling diagnosed; adaptive synthesis installed; executable acquired-search experiment added; full recursive causal evidence not yet established.**

This file is an evidence ledger, not an AGI claim. Historical failures are preserved elsewhere and are not overwritten.

## Mission boundary

Branch: `ace-full-system-20260911`

Starting HEAD supplied for this mission: `fb302372eb2a705a459a58b804cb4580b17ddf75`

Current source line after this mission's implemented changes: `b53756c8bb36c1f33c5aabfcdb9657def7d6cad7`.

Key implementation commits:
- `6af8559d32076979e875e691595423e434fd278d` — adaptive semantics-guided synthesis substrate.
- `451a9fe53694cd9cbb91783fef48eb063c89fdcd` — recursive protocol routes future acquisition through adaptive synthesis.
- `1bb5694bf19761acc90924cfd68194125f712aff` — legacy acquisition-method evaluation routes through adaptive synthesis.
- `716e4284e771eb62aaba8a0a1d9d73a9b4e1898a` — executable bootstrap-escape V2 experiment.
- `b53756c8bb36c1f33c5aabfcdb9657def7d6cad7` — V2 probe promoted to the focused self-extensible test.

## Forensic diagnosis

The failing path was:

`task -> CapabilitySpecification -> BootstrapMethodSearch -> SearchLibrary.Execute -> UniversalMechanismSearch -> UniversalProgramBuilder -> verification`

The critical ceiling was in synthesis, not execution or serialization.

The existing `UniversalProgramBuilder` used a depth-1 branching condition frontier whose base constants were only `-2..2`. The smallest diagnostic conditional requires a predicate equivalent to `x == 5` for examples `(-8,0),(0,0),(1,0),(5,1)`. The existing branching IR can represent this program, but the builder's bounded frontier did not contain the required constant/predicate structure. Therefore the representation was expressive enough while the synthesizer was not.

This is why increasing the number of named acquisition methods would not address the actual ceiling.

The existing diagnostic file explicitly constructs the threshold task and invokes the universal branching strategy before behavioral validation. See `internal/ace/universal_branching_diagnostic_test.go`.

## Initial searchable substrate

Initial `SearchLibrary` primitives:
- `search:straight-line -> universal:straight-line`
- `search:branching -> universal:branching`

Initial method opcodes:
- `search`
- `use`
- `dedupe`
- `reverse`

The trusted execution IR already supported integer variables/constants, arithmetic, comparisons, boolean expressions, conditionals and bounded repetition. The experimentally demonstrated limitation was search, not basic conditional execution.

## Synthesis change

`internal/ace/adaptive_synthesis.go` adds a semantics-deduplicated behavioral synthesizer. It derives candidate constants from observed input/output evidence, enumerates arithmetic expressions by bounded structural depth, collapses equivalent observed semantics, and searches conditional partitions induced by candidate predicates. Acceptance remains separate: synthesized artifacts are serialized and independently behaviorally checked.

This is deliberately not presented as a new computational primitive. It is a stronger acquisition mechanism that removes the demonstrated brittle-search ceiling without changing the trusted execution semantics.

Program synthesis research supports this separation of candidate generation from independent verification and the use of CEGIS-style feedback; SyGuS formalizes synthesis as finding an implementation satisfying a semantic specification within a syntactic search space. See the external research record used during this mission.

## Acquired-search experiment

`internal/ace/self_extensible_v2.go` introduces `RunSelfExtensibleExperimentV2`.

The experiment:
1. establishes a K0 arithmetic ceiling on a conditional task;
2. derives a capability specification from behavioral examples;
3. searches executable acquisition-method programs rather than selecting a named method;
4. synthesizes a behavioral solution with the adaptive substrate;
5. independently verifies and persists the executable method;
6. installs an acquired primitive whose strategy points to the retained executable method;
7. applies that acquired primitive to a structurally different held-out piecewise task;
8. compares future candidate evaluations before and after installation.

The promoted test requires the acquired artifact to solve the held-out task and reduce the measured future search-evaluation count.

## Scientific interpretation

If CI confirms the V2 probe, the defensible claim is:

> **Executable bootstrap escape / acquired search reuse demonstrated on a bounded symbolic domain.**

It is stronger than storing metadata because the installed artifact is invoked through the executable search substrate and changes future search evaluation cost.

It is **not** yet proof of M1 -> M2 recursive capability improvement, open-ended self-improvement, broad transfer, or AGI.

## Outstanding decisive gaps

1. The V2 experiment still begins with a fixed symbolic primitive inventory; it does not yet prove acquisition of an operator whose construction changes the generator itself in a broad domain-independent way.
2. M1 -> M2 causal dependence requires a second independently discovered acquisition mechanism with ablations removing M1 and M2.
3. The current cost ledger is still incomplete: discovery, execution, verification, memory, wall-clock, and all retries must be counted.
4. The held-out family is still symbolic/programmatic; cross-domain transfer is unproven.
5. The old `BootstrapMethodSearch` path remains in the repository and should be unified with the adaptive substrate only after the new path is validated, rather than silently deleting historical machinery.
6. Representation revision, decomposition acquisition, verifier synthesis, self-model prediction and active challenge generation remain unproven.

## Acceptance ladder

- `BOOTSTRAP ESCAPE DEMONSTRATED`: only after CI confirms the V2 executable acquired primitive changes future held-out search behavior under independent verification.
- `RECURSIVE CAPABILITY IMPROVEMENT DEMONSTRATED`: only after M1 materially changes M2 discovery, with M1 ablation and discovery-inclusive cost.
- `AGI EVIDENCE`: reserved for broad transfer across substantially different domains.

Current classification remains **PARTIAL** pending the exact CI result for the latest promoted test.
