# ACE full-system foundation

This branch adds an executable, contract-first ACE kernel on top of the preserved research history. The kernel separates raw experience, interpretation, hypothesis, evidence, knowledge level, skill, capability, planning, execution, verification, diagnosis, transfer, self-model, capability specification, architecture candidates, construction, sandbox validation, integration and rollback.

## Scientific status

This is an implementation foundation, not an AGI result. Several mechanisms are intentionally deterministic baselines behind replaceable interfaces. In particular, neural perception/world models, rich causal inference, open-ended abstraction, general structural transfer, autonomous source construction and recursive capability compounding remain scientifically unproven.

## Safety invariants

- Raw experience remains distinct from derived interpretation.
- Provenance is carried through derived objects and knowledge promotion.
- C4 promotion requires verified evidence.
- Planning is separated from simulation and execution.
- Execution requires an explicit capability authorization boundary.
- Verification is independent of the internal prediction path and can reject it.
- Candidate mechanisms require declared interfaces and tests before sandbox validation.
- Failed candidates cannot be integrated.
- Integration keeps the previous candidate for rollback.
- Historical phase artifacts are not rewritten.

## Runtime

`cmd/ace` is the canonical executable entrypoint for the foundation. Persistent state is stored as an atomically replaced JSON snapshot. The package has no new external dependencies.

The runtime intentionally returns `inconclusive` rather than manufacturing success when the current capability set is insufficient. This is a key distinction between architecture plumbing and an empirical AGI claim.
