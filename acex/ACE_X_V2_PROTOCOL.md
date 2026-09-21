# ACE-X V2 Scientific Protocol

## Claim boundary

ACE-X V2 is a model-independent cognitive substrate. LLMs, JEPA-style predictors,
neural networks, or external learned models may be plugged into declared interfaces,
but no external model may determine truth, acceptance, hidden evaluation, or promotion.

## Required capabilities

V2-A  Raw grounding -> ID-independent object/relation state.
V2-B  Predictive state model with confidence and contradiction-driven revision.
V2-C  Active hypothesis testing with sequential belief revision.
V2-D  Hierarchical memory: episodic/semantic/procedural/failure/model roles.
V2-E  Selective attention and executive computation allocation.
V2-F  Concept/representation induction and cross-surface transfer.
V2-G  Synthesized search-language induction; a promoted program must be composite,
       independently verified, and transfer to unseen generated tasks.
V2-H  Hierarchical procedural planning using learned action macros.
V2-I  Endogenous curriculum generation from measured capability gaps.
V2-J  Inquiry judgment and critique memory: questions are selected from a Pareto
       frontier; critique must generate falsification/transfer tests.
V2-K  Proof-carrying self-improvement: promotion requires hidden verification, lower
       successor acquisition cost, parent/new version lineage, and rollback.
V2-L  Integrated runtime loop: grounding -> memory/attention -> prediction ->
       planning -> action -> verification -> failure diagnosis -> abstraction ->
       curriculum -> improvement.
V2-M  Model-independent interface ablation: pure substrate functions without any
       external learned model.
V2-N  Resource-normalized future-cost reduction.
V2-O  Adversarial randomized evaluation plus a protected evaluator runtime seed.

## Terminal evidence

The terminal battery must pass on:
- two fixed deterministic seed blocks;
- the 64-case randomized cognitive sweep;
- one evaluator-runtime-derived randomized block in the protected workflow.

A pass is valid only when all V2 capabilities above are exercised by executable tests.

A failure caused by malformed test/CI plumbing is repaired mechanically and rerun.
A failure of a scientific gate after plumbing is known-good is a real negative result.

No threshold, evaluator rule, hidden seed, task-family adapter, or acceptance rule
may be weakened after observing a scientific result.

## Recursive improvement criterion

For successor task T:

R = C(T | K_after) / C(T | K_before)

Promotion requires R < 1 on independent hidden successor tasks and no verified
regression on previously accepted capabilities.

## Interpretation

PASS = this V2 substrate survives its complete frozen battery.

PASS is not equivalent to ASI. ASI requires a subsequent broad, independently
held-out, cross-domain competence and open-ended self-improvement battery.

KILL = any required V2 capability repeatedly fails after mechanical defects are
excluded.
