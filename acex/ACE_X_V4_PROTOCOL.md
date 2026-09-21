# ACE-X V4 Scientific Protocol

## Claim boundary

V4 is a model-independent symbolic cognitive substrate. No LLM, JEPA, neural network,
learned judge, or external model is required for induction, verification, promotion, or rollback.

V4 specifically targets the V3 failure: exact-subtree library induction breaks when equivalent
solutions have different syntax. V4 therefore learns parameterized typed concepts by
anti-unification plus resource-normalized compression.

## Frozen capabilities

V4-A  Typed expression IR with independent Int and Bool semantics.
V4-B  Training-only program synthesis with independent holdout verification.
V4-C  Parameterized concept invention by anti-unification of independently solved programs.
V4-D  Compression/MDL admission: a concept is retained only when reusable calls lower
       verified acquisition cost.
V4-E  Recursive composition: later concepts may contain calls to earlier learned concepts.
V4-F  Hidden successor generation with evaluator-owned constants and independent holdouts.
V4-G  Cross-domain typed transfer across Int and Bool task families.
V4-H  Deterministic persistence/rehydration with library digest.
V4-I  Proof-carrying runtime promotion and rollback through the existing improvement ledger.
V4-J  Protected evaluator runtime seeding; the seed changes latent task parameters.
V4-K  Model-independence: standard-library Go only.

## Terminal acceptance

Two fixed seed blocks plus one evaluator-runtime-derived seed must each demonstrate:
- Int concept invention;
- Bool concept invention;
- recursive Int concept invention referencing the earlier Int concept;
- at least six hidden successor cases across both domains;
- every hidden cost ratio R = C(T|K_after) / C(T|K_before) < 0.75;
- no accepted visible regression;
- deterministic library digest after rehydration;
- successful proof-carrying promotion and rollback.

The learner receives only task observations and outcomes. Hidden holdouts are verifier-only:
a rejected hypothesis may be discarded, but holdout contents are never used to rank candidates.

## Kill rule

A capability failure after mechanical defects are excluded kills V4. No threshold, task label,
hidden seed, or acceptance condition may be weakened to obtain PASS.

## Interpretation

V4 PASS demonstrates parameterized typed symbolic library growth under hidden transfer.
It is not evidence of AGI/ASI by itself. Subsequent gates must remove the remaining
hand-designed DSL/task-family assumptions, add open-world grounding and causal experimentation,
and demonstrate self-improvement of the cognitive mechanisms themselves rather than only
library contents.
