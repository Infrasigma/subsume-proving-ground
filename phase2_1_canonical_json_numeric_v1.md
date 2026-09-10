# PHASE 2.1 CANONICAL JSON NUMERIC RULE v1

**Status:** `NORMATIVE CONFORMANCE AMENDMENT`
**Execution authority:** `NONE`
**Definitive experiment:** `NOT EXECUTED`

## Purpose

The SERL equivalence contract requires byte-identical canonical boundary serialization, but its existing phrase `ordinary decimal JSON integers` did not define the lexical representation of non-integer JSON numbers. The `0.0` versus `0` divergence therefore exposed a canonicalization gap rather than a scientific difference.

This artifact closes only the serialization layer. It does not change task semantics, learner behavior, attribution, statistics, or any scientific endpoint.

## Numeric domain

1. JSON integer values are represented as base-10 integers with no leading zeroes except `0`, and without a `+` sign.
2. JSON floating-point values are finite IEEE-754 binary64 values.
3. `-0` and `+0` are canonically serialized as `0`.
4. NaN and positive/negative Infinity are forbidden and MUST cause canonicalization failure.
5. Integer-valued floating results in the Phase 2.1 boundary domain MUST use the integer lexical form. Therefore `0.0` and `0` canonicalize to the same bytes: `0`; likewise `1.0` and `1` canonicalize to `1`.
6. Non-integer finite binary64 values MUST use the shortest round-trippable decimal representation for that binary64 value, with no trailing fractional zeroes. Scientific notation uses lowercase `e`, an explicit `+` for positive exponents, and no leading zeroes in the exponent. Fixed notation is used when the exponent is in the ordinary decimal range; otherwise scientific notation is used.
7. Numeric precision is not silently widened or narrowed during canonicalization. Exact JSON integers remain integers; floating values remain binary64 values.

## Scientific boundary

The numeric rule applies only to canonical representation. A parser or implementation MUST NOT use numeric normalization to turn scientifically distinct values into equal values. Semantic comparison occurs before canonical byte comparison.

## Regression requirement

The conformance suite MUST retain a regression in which one implementation would naturally emit `0.0` and another `0`; the canonical boundary MUST be identical after applying this rule. The original failing execution remains historical evidence.

## Provenance

This amendment is prompted by the verified failure in run `34524220718`, case `negative_nonenablement_missing_entity`. It does not authorize the definitive Phase 2.1 experiment.
