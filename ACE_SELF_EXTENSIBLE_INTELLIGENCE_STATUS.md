# ACE Self-Extensible Intelligence Status

## Classification

`PARTIAL`

This milestone crosses a real architectural boundary: acquisition procedures are now representable as executable, serializable programs in a generic search substrate; retained methods can be invoked by later methods; and executable methods can survive process restart. It does **not** establish AGI, open-ended intelligence, or compute-inclusive recursive improvement.

## Exact branch / HEAD

- Branch: `ace-full-system-20260911`
- Exact HEAD at this status update: `721f82a65fcb9a20cadb9f41ecdb2b3b57b99612`
- Pre-mission audited HEAD: `30c2966ca51aed5dab26837404147cac783cefd6`

## Boundary audit

`ACE_SELF_EXTENSIBLE_BOUNDARY_AUDIT.json` records the pre-change execution trace and hidden cognitive decisions. The decisive pre-existing leak was the named-procedure switch in `executeAcquisitionMethod` plus the hand-authored `GenerateMethodCandidates` table.

## New architecture actually implemented

`internal/ace/self_extensible.go` introduces:

1. `CapabilitySearchObject` — common typed artifact envelope for task/capability/method/representation/verifier/decomposer/controller/abstraction objects.
2. `ExecutableAcquisitionMethod` — executable instruction sequence with inputs, outputs, cost, applicability, verifier, parents and version.
3. `SearchLibrary` — library of search primitives, executable methods, abstractions and history.
4. Generic method interpreter with only bootstrap opcode semantics: `search`, `use`, `dedupe`, `reverse`.
5. Bootstrap method search that enumerates executable method programs rather than selecting among hand-authored method names.
6. Independent behavioral verification of executable methods.
7. Serialization round-trip through JSON before installation.
8. Data-driven repeated-instruction abstraction mining.
9. Bounded recursive probe `RunSelfExtensibleExperiment`.

`internal/ace/self_extensible_persistence.go` adds atomic JSON persistence and reload of the executable search library, and the restart test proves a retained method survives a fresh library process boundary.

## Bootstrap primitives

- Search a registered search primitive.
- Invoke a retained executable method.
- Dedupe candidate results.
- Reverse candidate order.
- Serialize/deserialize artifacts.
- Independently verify resulting behavior.

The bootstrap still owns the semantics of these primitives. This is intentional and is the trusted computational floor. It does not contain a task-specific winning method.

## Discovered primitives / methods

The bounded probe synthesizes `M1` by enumerating executable instruction programs and selecting only by hidden behavioral success. The retained method can subsequently be invoked through `use` rather than through a method-name branch.

A second method artifact `M2` is synthesized against a structurally different piecewise task. The current probe establishes executable persistence and reuse, but its M1→M2 causal advantage is not yet strong enough to promote to `DEMONSTRATED` recursive self-improvement.

## Learned abstractions

Repeated instruction suffixes can be mined automatically from verified executable methods. This is a structural library-learning primitive, not a manually named abstraction.

## Search policy

Executable method programs alter the order/content of future mechanism candidates. Historical acquisition-method scoring remains in the older path; a learned predictive applicability model is **not yet demonstrated**.

## Representation / decomposition / verifier search

- Task-program representation: existing `UniversalProgram`.
- Acquisition-procedure representation: new executable method IR.
- Representation search: not yet generalized across independent representation families.
- Decomposition search: not yet independently verified as a learned artifact.
- Verifier search: verifier remains a trusted field/opcode boundary; candidate verifiers are not yet synthesized and independently validated.

## Recursive evidence

Current bounded probe checks:

`T1 -> K1 -> T2 -> M1 -> T3 -> T4 -> M2`

It verifies executable method synthesis, installation/reuse, persistence, and a second method artifact. It does **not** yet prove that M2's discovery depended materially on M1 in a way that lowers future acquisition cost on unseen structure.

## R_n

No valid compute-inclusive `R_n` claim is promoted. The current resource vector exists but does not yet fully account for all method discovery work, serialization, verification, memory operations, wall-clock, retries, and external inference.

## Cost accounting

Partially instrumented via `ResourceVector` and candidate counts. Full discovery-inclusive accounting is unresolved.

## Controls

- Existing historical experiments and the invalidated affine replication result were not rewritten.
- Hidden held-out behavioral cases are used for method verification.
- Serialization round-trip is tested.
- Restart persistence is tested.
- The previous replicated family remains explicitly invalidated because its target was algebraically affine.
- Leakage audit identifies remaining bootstrap/search-strategy knowledge as the main boundary.

## Strongest current endpoint

**The acquisition procedure itself is now an executable searchable artifact rather than metadata interpreted through a method-specific Go branch. A verified method can enter the library, survive restart, and be invoked by later executable methods.**

That is a genuine architectural improvement, but it remains a bounded symbolic self-extension experiment. It is not AGI.

## Next decisive bottleneck

Remove the remaining fixed assumptions at the next boundary:

`fixed bootstrap/search-operator inventory -> acquired search-operator construction`

Then require an ablation showing that the newly acquired operator cannot be reproduced by the original bootstrap at equal or lower discovery-inclusive cost. Only that result would justify promoting recursive improvement beyond `PARTIAL`.
