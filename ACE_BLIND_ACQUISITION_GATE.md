# ACE Blind Acquisition-Transfer Gate

## Purpose

This gate tests whether verified retained capabilities reduce the cost of acquiring independently generated future tasks without exposing the future-task construction rule to the acquisition policy.

It is a transfer/acquisition-efficiency gate. It is not a proof of autonomous mechanism invention, recursive self-improvement, AGI, or ASI.

## Protocol

1. Bootstrap three unary capabilities from behavioral examples only.
2. Independently verify every retained capability on held-out examples.
3. Freeze the retained library.
4. Generate future tasks from a deterministic external generator.
5. Expose each future task to acquisition only as input/output examples.
6. Compare K0: direct bounded universal synthesis with its existing three strategy frontiers, against K1: generic role-compatible composition over the independently verified retained library.
7. Require hidden-case verification for every claimed success.
8. Compute the measured acquisition-strategy ratio:

R_probe = K1 strategy evaluations / K0 strategy evaluations.

A gate pass requires R_probe < 1 and K1 to solve all blind future tasks while K0 solves none.

## Evidence boundary

The current cost is deliberately narrower than the project-wide cost vector C(T).

It measures strategy-frontier evaluations only. It does not yet include wall-clock time, memory, failed internal expression expansions, I/O, retries, or full verification cost. Therefore R_probe < 1 must not be promoted to the canonical compute-inclusive R_n < 1.

Likewise, K1 reuses a human-provided composition mechanism. The test therefore establishes verified transfer through retained composition, not autonomous invention of the acquisition mechanism.

## Next gate

Replace the fixed composition operator with an acquired acquisition-policy artifact:

failure evidence -> bottleneck diagnosis -> candidate acquisition mechanisms -> independent verification -> policy installation -> blind future improvement

The next experiment must also include genuinely different future task families so that the result cannot be explained by repeated exposure to one composite transformation family.