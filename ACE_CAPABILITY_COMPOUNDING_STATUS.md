# ACE Capability-Compounding Status

Date: 2026-09-11

## Status

**TESTED BUT LIMITED. Recursive capability compounding remains UNPROVEN.**

No AGI claim is made.

## Source

Branch: `ace-full-system-20260911`

Current source/documentation HEAD: `e98c07528f033abfeb738aa3ffdce9406b86c3ee`

The milestone contains a latent-task laboratory, behavior-driven parameterized mechanism search, hidden-task evaluation, structural program composition, and a replicated compounding protocol. It deliberately does not replace the canonical runtime with a hand-authored AGI loop.

## Evidence

### Latent task generation — TESTED BUT LIMITED

Discovery and hidden evaluation are separate generator calls. Current families are affine, threshold, deep composition, and replicated deep composition. They are still hand-designed and bounded; this is not an open-ended task generator.

### Capability acquisition — TESTED BUT LIMITED

`ParameterizedMechanismSearch` infers candidate constants from observed input/output behavior and searches generic arithmetic mechanisms. It does not receive the hidden oracle while constructing the program. The search space is still bounded and arithmetic, not a general typed executable substrate.

### Verified retention and composition — TESTED BUT LIMITED

Previously acquired programs can be structurally composed by substitution. The hidden depth-3 protocol is designed so the existing depth-2 universal builder cannot directly solve the target while retained primitives can be composed to solve it.

### Acquisition-method change — PARTIAL

A separate conditional search is invoked after the arithmetic-only acquisition method fails on a structurally different threshold family. This is evidence of a method boundary, but the method upgrade is still authored by the experiment harness rather than discovered, verified, and installed autonomously by ACE.

### Replication — TESTED BUT LIMITED

The repository contains a 12-instance hidden-task replication gate. Its intended metric compares a frozen K0 mechanism-search boundary with two structural composition operations after primitive retention. This is a conditional search-cost metric, not yet the project's full compute-inclusive `R_n` ledger.

## Adversarial findings

1. The compounding protocol is not integrated into `Runtime.Acquire` as an evidence-driven controller.
2. The experiment harness supplies several acquisition mechanisms; therefore it cannot yet establish autonomous meta-capability invention.
3. The task-family universe is bounded and hand-authored.
4. General counterexample search remains narrower than required for arbitrary executable mechanisms.
5. Representation revision remains limited to observed-variable/relation candidates.
6. Causal hypothesis generation still starts from hypotheses already present in the model.
7. The predictive method model is not yet the execution authority that changes the canonical acquisition policy.
8. No verified self-modification of the acquisition algorithm has been demonstrated.

## CI evidence

A first compounding validation run (`34542828179`) failed at Go compilation because the initial draft contained malformed syntax. The failure was inspected rather than hidden, and the malformed drafts were removed/replaced.

The subsequent source repair was pushed as a parse-safe core. GitHub Actions validation for the newest HEAD is still being scheduled/executed; therefore this document intentionally makes **no green-CI claim** for HEAD `e98c07528f033abfeb738aa3ffdce9406b86c3ee`.

## Exact justified claim

> ACE has a tested, bounded substrate for behavior-driven capability acquisition and structural reuse across hidden task instances. The repository does not yet demonstrate that ACE autonomously discovers a missing acquisition capability, constructs and verifies a meta-capability, installs it into its own acquisition policy, and thereby improves subsequent capability acquisition.

## Next decisive milestone

Make the acquisition policy itself a learned, verifiable object:

`failure telemetry -> bottleneck diagnosis -> competing acquisition-method candidates -> independent verification -> policy installation -> changed future behavior -> replicated hidden-task improvement`.

The candidate method must be represented as a verifiable artifact, not a branch in the test harness. Full cost accounting must include search, candidate failures, verification, experiments, retries, and compute. Only then should a replicated `R_n < 1` result be promoted to demonstrated capability compounding.
