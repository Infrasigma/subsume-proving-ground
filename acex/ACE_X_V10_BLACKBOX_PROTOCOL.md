# ACE-X V10 Black-Box Cognitive Protocol

The evaluator communicates with the candidate entity only through stdin/stdout JSON Lines.

## Synthesis request

`{"op":"synthesize","task":..., "training":[...], "counterexamples":[...]}`

The evaluator never sends hidden holdout examples in this request.

The candidate returns a serialized executable program. The evaluator owns the interpreter
used to verify that artifact.

A failed hidden check is returned only as a concrete counterexample on a subsequent request.

This creates a true counterexample-guided loop without exposing the hidden test set.

## Interactive request

`{"op":"act","state":<RelationalState>,"actions":["A","B",...]}`

The candidate returns only its chosen action.

No goal, hidden state, target action, task label, or evaluator metadata is transmitted.

## Reset

`{"op":"reset"}`

Clears the candidate runtime state.

## Status

`{"op":"status"}`

Returns only non-authoritative local capability metadata.
