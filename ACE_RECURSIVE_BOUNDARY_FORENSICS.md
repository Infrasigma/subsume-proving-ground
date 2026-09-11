# ACE RECURSIVE BOUNDARY FORENSICS

## Truth boundary
HEAD is `2218ce62e8fd6dc4c2b9dd43196f07b117637cf1`; branch is `ace-full-system-20260911`. This report is code-forensic, not an executed experiment. `go test ./...` was requested but cannot be run through the connected GitHub API. Existing CI evidence on this HEAD is a failed `go test ./internal/ace` run with failures in endogenous abstraction learning, recursive method improvement, and self-extensible held-out use; no fresh run is asserted here.

## Pipeline reconstruction
`AutonomousMethodImprovementWithLibrary` diagnoses telemetry, then ignores the diagnosis for candidate generation and calls `EnumerateAcquisitionProceduresWithLibrary(2,lib)`. Each procedure is wrapped as an `AcquisitionMethodArtifact`. Execution decodes the procedure, calls `UniversalMechanismSearch`, then applies the procedure to the resulting candidate stream. Universal search returns exactly three fixed strategies. Verification compares the resulting mechanism order/length against a fresh universal baseline, then tries `UniversalProgramBuilder` and hidden/regression cases. Selection ranks only candidates that already passed all required booleans.

## Reachability decision
The generic procedure IR has a real `call` opcode and recursive executor. A procedure containing `call(A1)` is therefore representable and executable when A1 is installed. But the actual recursive V3 experiment calls `AutonomousMethodImprovement` without a library, so its acquisition candidate generator cannot emit any acquired-reference call. This is an actual boundary between generic library capability and the exercised recursive path.

More importantly, even with a non-nil library, `call` does not introduce a new primitive semantic. It recursively invokes an installed procedure whose leaf vocabulary is fixed to identity/reverse/dedupe/sort-cost/take/rotate. Universal search remains the same three developer-defined strategies. Thus current code does not establish reachability of a genuinely new acquisition/search operator.

## Nearest M1
Under deterministic atom ordering, the first depth-1 candidate is `identity`, which is rejected by the future-order-change gate because it preserves the fixed search trace. The next is `reverse`, which changes order and can therefore pass that gate. This makes `reverse` the nearest likely M1 in the recursive V3 path, but this is a code-path inference, not an executed candidate trace. Its leaf is a trusted bootstrap stream operation; acquired leaf count is zero.

## A2 construction
A true A2 that *uses A1 as an acquisition primitive* can be syntactically represented only as a call to an already installed A1. The actual V3 path has no installed library at method-improvement time, so it cannot generate such a form. Even when a library is supplied, the call merely reuses fixed procedure semantics. Therefore `A2_GENERABLE_USING_A1=NO` for a genuinely new operator; generic call-form representation is YES only in the broader library-aware grammar.

## Future-search effect
Current code's `FutureTraceChanged` is only `!sameMechanismOrder(base,cs)`. That predicate detects length/order changes, not whether the effective search space, candidate construction semantics, pruning, ranking, or selected solution set has materially expanded. Therefore a passing future-change predicate is insufficient to establish real frontier expansion. The current architecture explicitly permits ORDER_ONLY_EFFECT.

## Objective boundary
The selector has Compute and ExperimentBudget cost terms, history, Gain, and the FutureSearchChanged eligibility gate. It has no MDL/complexity term and no separate verification/regression/transfer/discovery/implementation-cost objective. Gain is currently hard-set to 1 on a successful candidate. Candidate generation ignores the bottleneck diagnosis. These are code facts, not proposed repairs.

## Architectural-theater assessment
- False recursion: SUPPORTED RISK. A call can recurse, but the exercised V3 path has no acquired-call dependency; nearest M1 is a bootstrap stream operator.
- False novelty: SUPPORTED. Universal search remains three fixed developer-defined strategies and procedure atoms are developer-defined.
- False search expansion: SUPPORTED RISK. Future change is order/length equality only; it can classify cosmetic/order effects as change.
- False transfer: NOT ESTABLISHED. Current code contains held-out/regression checks, but no fresh raw trace is available here.
- False verification: NOT ESTABLISHED as a general claim. Acquired abstraction verification has a separate reference interpreter; the recursive method verifier uses the universal builder for behavioral checking, so independence must be assessed at the concrete candidate level.
- False self-improvement: SUPPORTED RISK. History affects selector scores, but candidate generation itself is fixed and ignores the diagnosis; no evidence here proves that learned history expands the search language.

## Primary boundary
`MULTIPLE_INTERACTING_LIMITATIONS` is the defensible code-level classification, with strongest concrete components: (a) exercised recursive path has no acquired-reference library; (b) procedure language is fixed to stream transforms; (c) UniversalMechanismSearch is a fixed three-strategy generator; (d) future-search change measures order/length rather than effective search-space expansion; (e) selector does not use diagnosis to alter the candidate language.

This report deliberately does NOT claim that one of these is the experimentally proven root cause because the requested fresh raw execution/rejection matrix was not available.

## Minimal counterfactual, not implemented
The smallest architecture delta should be a *verified acquired search-constructor operator* with explicit representation, execution, and reference semantics. It must consume prior verified capability objects plus current task evidence, emit a new executable search operator/program constructor whose behavior is not reducible to the fixed UniversalMechanismSearch strategies, and expose a trace-level causal effect on candidate generation (new candidate family/branch/pruning rule), not merely order. Independent verification must use a separate semantics/reference implementation and held-out tasks. Existing recursive acceptance, leakage controls, restart persistence, and multi-seed ablations remain unchanged.

## Scientific decision
`CAN_CURRENT_SYSTEM_REPRESENT_A2_USING_A1=YES in library-aware grammar; NO in exercised V3 path without a library`
`CAN_CURRENT_SYSTEM_GENERATE_A2_USING_A1=NO for a genuinely new search operator`
`CAN_CURRENT_SYSTEM_VERIFY_A2_INDEPENDENTLY=UNKNOWN without fresh execution; generic abstraction verifier is independent`
`CAN_A2_CHANGE_FUTURE_SEARCH=UNKNOWN for genuine expansion; current predicate can detect only order/length change`
`CAN_A2_ENABLE_STRUCTURALLY_NEW_TASKS=UNKNOWN`
`PRIMARY_BLOCKING_BOUNDARY=MULTIPLE_INTERACTING_LIMITATIONS`
`EVIDENCE=static execution graph and generator semantics; fresh raw execution absent`

## Status
`REACHABILITY PARTIALLY ESTABLISHED`

This status means the generic acquired-call mechanism is expressible, but reachability of a genuinely new recursive acquisition operator is not established. No AGI claim follows.

NO DECISIVE FRONTIER EXPANSION