# H-R13: Second-Order Composition Coverage Reservoir

Date: 2026-10-01

## Failure diagnosis

The combined L2-C qualification failed on seed 2026100103 at p7 acquisition. A post-hoc diagnostic found zero exact and zero behaviorally target-equivalent candidates in the final 126-candidate frontier. The failure is therefore downstream of first-order discovery and upstream of candidate selection.

## Hypothesis

H-R12 preserves structurally diverse first-order relations, but the second-order composition pool is still globally discovery-ranked and then truncated to max_programs=128. A useful second-order composition can therefore be eliminated even when both required first-order relations survive.

## Intervention

Preserve a generic reservoir of second-order boolean compositions before final frontier truncation. The reservoir key contains only:
- root boolean operator;
- relation operator of each child;
- child predicate;
- unordered field-pair structure.

For each structural composition class, retain its highest discovery-scoring representative. Reserve up to 3/4 of the output beam for distinct second-order structural classes, then fill remaining slots using the existing discovery-quality ordering.

No target identity, task label, benchmark ID, holdout outcome, transfer outcome, or p7-specific rule enters the mechanism.

## Falsification

The mechanism is rejected if it fails the fresh five-seed qualification under the existing acquisition and retention gates, or if it causes any incumbent retention regression/intrusion/integrity violation.

## Scope

This is an L2-C composition-frontier mechanism only. It does not establish AGI, ASI, or general capability.

Execution trigger synchronized 2026-10-01.

Workflow activation base verified at 6e800edeb456d66e182ff6d5a2ba2c7ac9a4d218.
