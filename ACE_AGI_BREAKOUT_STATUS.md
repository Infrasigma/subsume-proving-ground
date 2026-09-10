# ACE AGI breakout status

Date: 2026-09-11
Branch: `ace-full-system-20260911`
Current head: `9ade99cc533669a5f20886f34fdde527cd7cfe4d`

## Evidence boundary

This document deliberately does **not** claim AGI or open-ended capability acquisition. The breakout phase is incomplete.

## Changes with adversarial evidence

| Target | Status | Evidence |
|---|---|---|
| Representation revision | PARTIAL + TESTED | `TestRepresentationRevisionInventsMissingVariableAndRejectsIrrelevantDistinction`; `TestRepresentationRevisionWorksForDifferentWorldStructure`. Evidence-driven candidate generation selects the distinction that reduces residual aliasing. |
| General executable mechanism construction | PARTIAL + TESTED | Universal compositional program substrate added with arithmetic, boolean predicates, branching, iteration and serialized executable artifacts. |
| Mechanism search | PARTIAL + TESTED | Multiple generic construction strategies are generated rather than a single affine candidate. |
| Causal hypothesis generation | PARTIAL + TESTED | Existing Bayesian competing-hypothesis backend remains tested, but hypotheses are still externally supplied in the strongest causal test. |
| Self-generated counterexample testing | BLOCKED | Current adversarial-case generator does not independently derive expected outcomes; an oracle/model-grounded counterexample loop is still required. |
| General skill composition | PARTIAL + TESTED | Existing skill/planner substrate is executable but composition is not yet demonstrated as autonomous capability discovery. |
| Adaptive meta-control | PARTIAL + TESTED | Controller remains heuristic and fixed-choice; evidence-driven action-value control is not demonstrated. |
| Architectural learning | PARTIAL + TESTED | Architectural outcomes persist and reorder known strategies, but broad predictive learning is not demonstrated. |
| Self-model invalidation/recovery | PARTIAL + TESTED | Persistence exists; dependency-grounded automatic invalidation is not demonstrated end-to-end. |
| End-to-end open-ended acquisition | UNPROVEN | No replicated independently generated task-family evidence yet. |
| Structural novelty transfer | UNPROVEN | Existing bounded transfer evidence does not establish the new general mechanism substrate transfers autonomously. |
| Recursive acquisition improvement | UNPROVEN | No validated acquisition-process improvement has yet been demonstrated. |
| `R_n < 1` across replicated novel families | UNPROVEN | No preregistered replicated measurement has been completed. |
| AGI | UNPROVEN | Scientific firewall remains active. |

## Exact validation evidence

Previous focused ACE run `34535822272` passed:

- `go test ./internal/ace`
- `go test -race ./internal/ace`
- `go vet ./internal/ace`
- `go build ./cmd/ace`

Breakout validation exposed a genuine construction bug before the correction:

- `TestUniversalSynthesisEscapesAffineCeiling` failed because the serialized synthesized branch contained an invalid boolean operand.
- The correction added serialized-artifact round-trip validation before candidate acceptance.

A subsequent repository-wide run against the corrected head `9ade99cc533669a5f20886f34fdde527cd7cfe4d` is currently executing on GitHub Actions. Its final result is not yet available, so no green result is claimed here.

## Direct answers from evidence

1. Can ACE invent a representation distinction not explicitly supplied? **Partially demonstrated in bounded structured-state evidence; not yet demonstrated in open-ended task acquisition.**
2. Can ACE generate competing causal explanations and select informative interventions? **Competing-model reweighting and intervention scoring are tested; autonomous hypothesis generation remains incomplete.**
3. Can ACE construct an executable mechanism outside the original affine substrate? **The new universal substrate is designed and adversarially tested, but corrected-head CI evidence is pending.**
4. Can ACE generate its own counterexample tests? **No.**
5. Can ACE compose previously acquired capabilities into a new capability? **Not yet demonstrated autonomously.**
6. Can ACE improve its own acquisition process? **No demonstrated validated recursive improvement yet.**
7. Can that improvement reduce acquisition cost on structurally novel capabilities? **Unproven.**
8. Is there replicated evidence of `R_n < 1` across independent novel task families? **No.**
9. Has ACE crossed from bounded into genuinely open-ended capability acquisition? **No evidence sufficient to make that claim.**

## Next decisive work

Do not enlarge the primitive list merely to pass another benchmark. After the corrected-head validation is green, the next required work is:

1. integrate representation candidates into the acquisition loop;
2. generate causal hypotheses from experience rather than accepting them externally;
3. implement model-grounded self-generated counterexample search;
4. make universal mechanism synthesis consume acquired skills as compositional operators;
5. make candidate search select on held-out behavior, resource cost and regression evidence;
6. demonstrate an acquired non-affine capability through the actual persistent acquisition loop;
7. only then run independently generated structural task families and measure `R_n`.
