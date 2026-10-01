# H-R14 — Balanced Second-Order Composition Reservoir

## Hypothesis
H-R13 preserves a very large fraction of the final candidate frontier for distinct second-order structural classes. The seed 2026100110 failure at p6 indicates that broad second-order diversity can interact adversely with noisy finite-sample selection: a structurally novel candidate can enter the pool and win selection while failing fresh holdout.

H-R14 keeps the generic second-order structural reservoir but reduces its reserved quota to one quarter of the final beam, with a minimum of 32 slots at the current 128-candidate frontier. This is a quality-preserving diversity intervention: most frontier capacity remains under the existing global discovery ranking.

## Runtime mechanism
Only syntax-derived structure is used:
- root boolean operator
- child relation operators
- child predicates
- unordered field-pair structure

No target identity, task family, benchmark identifier, holdout, transfer result, or generation label enters the mechanism.

## Scientific prediction
Compared with H-R13, H-R14 should:
1. preserve enough second-order structural diversity for p7 acquisition;
2. reduce selection interference from excessive low-ranked second-order candidates;
3. preserve the existing H-R12 first-order reservoir and V7/retention mechanisms.

## Falsifier
Any one fresh seed with a valid scientific failure blocks qualification.

## Provenance
ACSIE research commit: 007c90d2e83ed5e0796b1e94c5f974b3bcc446cb
Base ACSIE main: 34ce504f4f51f963ce1443cfe974f57adca4f223
Harness: research/run_l2c_balanced_second_order_reservoir_20261001.sh
Seeds: 2026100111–2026100115
