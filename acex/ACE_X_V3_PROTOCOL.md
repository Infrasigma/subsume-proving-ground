# ACE-X V3 Scientific Protocol

## Claim boundary

V3 is a model-independent cognitive substrate. No LLM, JEPA, neural network, reward model,
or external learned model is required for the V3 core. External models may propose optional
representations, but they cannot determine truth, hidden acceptance, promotion, or rollback.

V3 targets the missing capability exposed by V2: reusable concepts must be *invented* from
experience rather than selected from a fixed hand-authored search grammar.

## Frozen V3 capabilities

V3-A  Typed symbolic program induction from input/output evidence.
V3-B  Cross-task library induction: discover reusable subprograms that were not named or
       supplied as candidate concepts.
V3-C  MDL/resource-bounded admission: a new library item is admitted only when its definition
       plus reuse measurably lowers verified acquisition cost.
V3-D  Recursive composition: later abstractions may use earlier learned abstractions.
V3-E  Hidden successor transfer: hidden tasks use independently generated inputs/holdouts;
       hidden verification is separate from training and never supplied to the learner.
V3-F  Model-independent core: the above loop must run with the standard library only.
V3-G  Persistence/rehydration and lineage: learned library state has a deterministic digest,
       can be restored, and records parent/child versions.
V3-H  Proof-carrying self-improvement: promotion requires hidden success, lower successor
       cost, no accepted regression, and rollback availability.
V3-I  Endogenous challenge generation: successor tasks are derived by composing learned
       abstractions under a resource budget; no candidate-visible target program is supplied.
V3-J  Adversarial randomized protected evaluation: hidden input seeds include evaluator-runtime
       entropy and fixed seed blocks.

## Terminal evidence

PASS requires:
- first-generation library invention on both deterministic seed blocks;
- second-generation abstraction that references the first learned abstraction;
- at least three independently generated hidden successor task pairs;
- a positive resource-normalized cost reduction for the learned library versus a fresh learner;
- deletion/rehydration with identical library digest and behavior;
- rollback after promotion;
- pure-substrate execution without external learned models.

A mechanical defect may be corrected only when the acceptance rule and scientific target remain
unchanged. A scientific failure is preserved and kills the V3 route.

## Recursive improvement criterion

For successor task T:

R = C(T | K_after) / C(T | K_before)

Promotion requires R < 0.80 on every hidden successor task in the terminal block and
no regression in previously verified tasks.

## Interpretation

A V3 PASS establishes recursive symbolic library growth under the frozen task family and
resource model. It is not, by itself, a claim of AGI or ASI. A later terminal battery must
cross independent domains, richer causal environments, open-ended grounding, and broader
self-directed research loops.
