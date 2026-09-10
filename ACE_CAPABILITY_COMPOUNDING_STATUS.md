# ACE Capability-Compounding Status

Date: 2026-09-11

## Evidence boundary

**Overall status: TESTED BUT LIMITED / RECURSIVE IMPROVEMENT UNPROVEN.**

This milestone deliberately does **not** claim AGI, open-ended intelligence, or general recursive self-improvement.

## Current source

Branch: `ace-full-system-20260911`

Source milestone before this document: `d51e15158561f8105c1c2d004664f9b70718b9ac`

The milestone adds a latent task lab, a parameterized mechanism search, an expanding verified-primitive library, structural program composition, and replicated hidden-task protocols. The current canonical `Runtime` path has not yet been rewritten to make the learned meta-controller the mandatory cognitive loop.

## What is actually demonstrated by the new tests

### 1. Latent task generation — TESTED BUT LIMITED

`LatentTaskLab` generates tasks from family generators rather than selecting a named benchmark answer. Discovery and hidden generation are separate calls and hidden evaluation cases are not supplied to the mechanism builder.

Families currently implemented include affine, threshold, composition, deep composition, and replicated deep composition. This is still a small hand-designed family universe, so it is not an open-ended task generator.

### 2. Parameterized mechanism acquisition — TESTED BUT LIMITED

`ParameterizedMechanismSearch` infers arithmetic constants from behavioral examples and searches generic parameterized mechanisms. It does not inspect the hidden oracle during construction.

The implementation remains a bounded arithmetic search space. It is not a general executable substrate with arbitrary types, collections, environment interaction, recursion, or learned external primitives.

### 3. Verified primitive retention — TESTED BUT LIMITED

Verified programs can be retained and structurally composed. The hidden depth-3 composition protocol acquires primitive mechanisms from independent behavioral examples and composes them on hidden cases.

### 4. Structural transfer / composition — TESTED BUT LIMITED

`composePrograms` performs structural substitution rather than a family-specific transfer rule. The replicated hidden protocol requires a depth-3 composition that the existing depth-2 universal builder cannot solve, while the retained primitives can solve it.

### 5. Conditional acquisition method — PARTIAL

A generic conditional candidate search was added in the protocol after the arithmetic-only method fails on a threshold family. This demonstrates a method-level expansion, but the expansion is still authored by the research code rather than discovered and installed autonomously by ACE.

### 6. Recursive capability compounding — UNPROVEN

The required scientific loop

`T1 -> K1 -> T2 -> diagnosis -> M1 -> K2 -> T3`

is only partially represented. The decisive missing evidence is that ACE itself must infer the acquisition bottleneck from telemetry, construct or select the acquisition improvement, verify that improvement as a reusable meta-capability, and then alter its subsequent acquisition behavior without a hand-authored protocol choosing the improvement.

### 7. R_n — UNPROVEN

The repository does not yet have a preregistered, compute-complete cost ledger for the full loop. Candidate-count ratios in the structural composition gate are diagnostic only and must not be promoted to the project's full `R_n = C(T_{n+1}|K_n)/C(T_{n+1}|K_0)` evidence until cost accounting covers search, verification, experiments, retries, and compute consistently.

## Adversarial findings

- The new compounding code is not yet integrated into the canonical `Runtime.Acquire` controller.
- Several protocol primitives are supplied by the experiment harness; therefore the experiment cannot yet be called autonomous meta-capability discovery.
- Task families remain hand-designed and bounded.
- Counterexample generation in the general acquisition path is still predominantly numeric-boundary based.
- Representation invention remains limited to observed-variable and pairwise-relation candidates.
- Causal hypothesis generation remains dependent on hypotheses already present in the causal model.
- The predictive method model currently has useful data structures but is not yet the sole execution authority for acquisition decisions.
- No evidence here establishes self-modification of the acquisition algorithm itself.

## CI

GitHub Actions was triggered for the milestone through the repository's existing workflows and the new focused capability-compounding workflow. The validation run was still in progress when this document was authored; therefore no green CI result is asserted here.

## Exact claim justified

> ACE now contains a tested, bounded experimental substrate for latent-task generation, parameterized mechanism acquisition, verified primitive retention, and structural composition across hidden task instances. It does **not** yet demonstrate autonomous discovery and verified installation of a meta-capability that measurably improves subsequent capability acquisition.

## Next decisive milestone

Replace the hand-authored protocol with an integrated evidence-driven acquisition controller. It must:

1. observe acquisition telemetry;
2. diagnose a bottleneck without a family-specific branch;
3. generate competing acquisition-method candidates;
4. verify the candidate method on held-out tasks;
5. install the verified method into the acquisition policy;
6. demonstrate changed future search behavior;
7. replicate the effect across independently generated structural families;
8. measure the full cost vector and preregistered `R_n` against a frozen K0 control.

Until that is done, the project remains on the capability-acquisition side of the scientific boundary.
