# PR110 Gen6 parent-function-class coverage — 2026-10-15

## Result

An additional evaluator-only audit compared expected parent functions with the symbolic functions of macros that actually occur in each selected candidate's executable lineage.

- Function-class coverage: 13/13 events.
- Exact expected parent-ID lineage: 0/13 events.
- Events where all distinct expected parent function classes occur in executable ancestry but exact IDs still mismatch: 13/13.
- Audit errors: 0.
- SymPy version: 1.14.0.
- Runtime selection and admission were not modified; the strict qualification gate was not reinterpreted.

This means every selected candidate had executable macro dependencies whose normalized functions matched each distinct expected parent function class, even though those macros were not always the exact hidden parent IDs. In cases where two expected parent IDs represented the same function, class coverage counts that as one distinct function class; it must not be misread as proving two separate parent identities were reused.

## Interpretation

This is useful evidence that bounded functional macro reuse occurred in these 13 diagnostics, but it is not a pass for the stricter exact-ID lineage objective. Both results should be preserved: the behaviorally correct function class is present in executable ancestry, and exact generator-assigned parent IDs are not preserved. Do not turn the new diagnostic into a replacement gate.

## Provenance

- Workflow run: https://github.com/Infrasigma/subsume-proving-ground/actions/runs/38684349564
- Run commit: abc1c86fb223025be5522dda6270156a6dfccca5
- Artifact ID: 11950981949
- Artifact SHA-256: b5d0170a3d7526a805b254d29940fb71ce778f1ce5c3cd1107b01e86d37c6806
- Raw Gen6 log SHA-256: 74caa5a04008a118d897ca41d03e50089a9bfc5fb67ce94579ed02977531f3a2
- Gen6 checkpoint SHA-256: 22828e4d70d7728c87e887dd8e507a4fe43240a154117edcce9f899d218838ee

Next experiment: make an explicitly versioned, leakage-safe benchmark-design candidate that either (a) prevents functional aliases from being treated as distinct lineage targets, or (b) assigns an independently observable role to the intended parent dependency. Compare it against the unchanged exact-ID gate on fresh frozen seeds; never change selection using hidden expected IDs.
