# H-R13 bounded second-order frontier repair

## Failure diagnosis

The original H-R13 second-order composition reservoir was qualified against the frozen ACSIE commit `6435c270af085380aed7e06135d98471429b2ac9`.

Seed `2026100110` failed at p6 acquisition under the original H-R13 run and failed again under the CV-partition selection variant.

The two seed artifacts are byte-level consistent in the critical p6 diagnosis:

- selected p6 program: `or(neq(parity(y), parity(z)), eq(sign(y), sign(z)))`
- selected program digest: `44e1f60a12e5851aa25b5c5a175e28f9aa193f55893e7ea3833f17bb13f0fddf`
- selection rows: 8
- selection successes for the incumbent bank: 4/8
- fresh holdout: 0.375 versus prior-best 0.5
- transfer: 0.833333...
- failure boundary: p6 acquisition gate
- integrity flags clear

The CV-partition mechanism therefore did not change the failing frontier sufficiently to remove the failure.

## Causal hypothesis

The second-order reservoir reserves 75% of the final beam:

`max(32, 3*beam/4)`

With `beam=128`, this reserves 96 final slots for second-order structural groups and leaves only 32 slots to the ordinary quality frontier.

This final-stage reservation is downstream of the earlier first-order structural search. It can therefore displace first-order opportunities that were reachable before the H-R13 modification.

The failure is consequently classified as:

`generation/frontier interaction -> active acquisition selection`

rather than retention failure.

## Patch

Bound the second-order final reservation to 25% of the active beam:

`max(16, beam/4)`

For the standard beam of 128 this reserves at most 32 second-order slots and leaves at least 96 slots to the ordinary scored frontier.

The rule remains target-independent and depends only on the beam width.

## Qualification requirement

The patch is not accepted from code review alone.

It must survive the full five-seed qualification:

- fresh holdout
- fresh transfer
- acquisition gate
- p0-p6 retention non-regression
- route invariance
- replay/integrity checks
- no external model
- no target/task-family routing

A single scientific seed failure blocks promotion.
