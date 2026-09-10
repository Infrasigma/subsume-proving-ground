# ACE AGI breakout status

Date: 2026-09-11
Branch: `ace-full-system-20260911`
Current head at status creation: `9ade99cc533669a5f20886f34fdde527cd7cfe4d`

## Evidence boundary

This document deliberately does **not** claim AGI or open-ended capability acquisition. The breakout phase is incomplete.

## Targets

| Target | Status |
|---|---|
| Representation revision | PARTIAL + TESTED |
| General executable mechanism construction | PARTIAL + TESTED |
| Mechanism search | PARTIAL + TESTED |
| Autonomous causal-hypothesis generation | UNPROVEN |
| Self-generated counterexample testing | BLOCKED |
| General skill composition | UNPROVEN |
| Adaptive meta-control | PARTIAL + TESTED |
| Predictive architectural learning | PARTIAL + TESTED |
| Evidence-derived self-model invalidation | PARTIAL + TESTED |
| End-to-end open-ended acquisition | UNPROVEN |
| Structural novelty transfer beyond bounded substrate | UNPROVEN |
| Recursive acquisition improvement | UNPROVEN |
| Replicated `R_n < 1` | UNPROVEN |
| AGI | UNPROVEN |

## Adversarial evidence

Previous focused ACE run `34535822272` passed `go test ./internal/ace`, `go test -race ./internal/ace`, `go vet ./internal/ace`, and `go build ./cmd/ace`.

Breakout run `34536701718` against an intermediate head failed `TestUniversalSynthesisEscapesAffineCeiling` because a serialized synthesized branch contained an invalid boolean operand. That failure was retained as evidence and led to serialized-artifact round-trip validation in commit `9ade99cc533669a5f20886f34fdde527cd7cfe4d`.

The corrected-head repository-wide validation was still running when this status was written. Therefore this file makes no claim that the corrected head has passed `go test ./...`, `go test -race ./...`, `go vet ./...`, or `go build ./cmd/ace`.

## Direct evidence answers

1. Representation invention: **bounded structured-state candidate generation is implemented and adversarially tested; open-ended acquisition use is unproven.**
2. Competing causal explanations: **competing-model reweighting/intervention scoring is tested; autonomous hypothesis generation is incomplete.**
3. Executable mechanism outside affine substrate: **a compositional integer/boolean program substrate exists and has adversarial tests; corrected-head execution evidence is pending.**
4. Self-generated counterexamples: **not demonstrated.**
5. Skill composition: **not demonstrated as autonomous acquisition.**
6. Acquisition-process improvement: **not demonstrated.**
7. Reduced cost from such an improvement: **unproven.**
8. Replicated `R_n < 1`: **no.**
9. Open-ended capability acquisition: **no evidence sufficient to claim it.**

## Required next gates

Do not expand the primitive set merely to fit a benchmark. Next work must integrate representation revision into acquisition, generate causal hypotheses from experience, ground counterexample generation in independent models/metamorphic constraints, make acquired skills compositional search operators, and then evaluate independently generated structural task families with preregistered acquisition-cost measurements.
