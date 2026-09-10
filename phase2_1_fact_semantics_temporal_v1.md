# PHASE 2.1 FACT SEMANTICS / TEMPORAL MULTIPLICITY v1

**Status:** `NORMATIVE CONFORMANCE AMENDMENT`
**Execution authority:** `NONE`
**Definitive experiment:** `NOT EXECUTED`
**Scope:** SERL observable fact representation only

## 1. Authoritative rule

One stored Fact is identified by:

`(predicate, canonical arguments)`

Temporal occurrence is **not** part of Fact identity. Temporal provenance is a separate field attached to that fact.

Therefore the required model is:

`Fact = (predicate, arguments, provenance_event_set)`

and not a sequence of independent facts whose identities differ only by timestamp.

This follows directly from the SERL learner freeze rule that duplicate facts with identical canonical predicate arguments and identical provenance are stored once, while the same canonical fact with different provenance retains the union of provenance records. The same freeze separately defines temporal predicates from provenance event sets.

## 2. Timestamp-set semantics

For every canonical `(predicate, arguments)` key:

1. collect every provenance event index at which the fact is observed;
2. represent provenance as a finite set of zero-based event indices;
3. remove duplicate event indices;
4. sort event indices numerically ascending;
5. do not infer intervals from consecutive timestamps;
6. do not discard non-consecutive timestamps;
7. do not preserve source insertion order as semantic order;
8. do not merge different predicate/argument keys merely because their timestamps overlap.

Thus `[0,1]` and two separate occurrences at `0` and `1` denote the same fact with the same provenance set **only after semantic grouping**, not by serialization normalization.

## 3. Duplicate semantics

A duplicate occurrence with the same canonical predicate, arguments, and event index contributes no additional semantic information and is represented once in the provenance set.

Two occurrences with the same predicate and arguments but different event indices are one Fact with multiple provenance members.

Example:

`AVAILABLE(a,0)` + `AVAILABLE(a,1)` + `AVAILABLE(a,1)` + `AVAILABLE(a,4)`

becomes:

`AVAILABLE(a, at=[0,1,4])`.

The duplicate at `1` is removed; `0`, `1`, and `4` remain distinct provenance members.

## 4. Ordering

The canonical provenance representation is strictly ascending numeric event-index order.

For valid SERL observations, event indices are zero-based append-order indices. They are provenance identifiers, not learner-visible semantic variables.

## 5. Temporal matching

The learner freeze defines temporal relations over provenance event sets:

`BEFORE(A,B)` is true iff:

`max(A) < min(B)`.

`AFTER(A,B)` is true iff:

`BEFORE(B,A)`.

An empty provenance set makes either relation false.

Consecutive timestamps have no special semantics. `[0,1]`, `[0,4]`, and `[4,9]` are evaluated by the same max/min rule.

This amendment does **not** introduce interval semantics, duration, interpolation, persistence, or a new temporal predicate.

## 6. Information-preservation requirement

Compression from occurrence records to timestamp-set provenance is permitted because it preserves exactly the provenance information required by the frozen temporal operators.

Compression is invalid if it:

- drops any event index;
- changes event-index ordering;
- converts a set into an interval;
- collapses distinct predicate/argument keys;
- causes temporal relations to be evaluated from insertion order rather than provenance sets;
- removes provenance needed by candidate support, causal ordering, or attribution.

## 7. Implementation conformance

Independent implementations MUST emit the same semantic Fact model and canonical timestamp-set representation.

The conformance suite MUST include:

- same predicate/arguments at consecutive times;
- same predicate/arguments at non-consecutive times;
- repeated identical event occurrences;
- timestamp ordering;
- multiple timestamps in one Fact;
- temporal candidate cases where `BEFORE`/`AFTER` semantics are exercised.

A byte-level comparison is necessary but not sufficient: tests MUST inspect the semantic timestamp-set model.

## 8. Current implementation limitation

The current conformance reference/production surfaces preserve timestamp sets, but their compact retrieval matcher presently checks predicate/argument membership and does not yet implement the frozen `BEFORE`/`AFTER` provenance-set operators as executable candidate matching.

Therefore this amendment resolves the Fact identity and multiplicity question but does **not** claim that full temporal candidate semantics are already implemented or closed.

That remaining gap is scientific infrastructure, not a benchmark-performance result.
