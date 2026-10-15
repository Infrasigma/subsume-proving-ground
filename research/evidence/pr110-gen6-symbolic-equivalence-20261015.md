# PR110 Gen6 symbolic-equivalence audit — 2026-10-15

## Provenance

- Repository: Infrasigma/subsume-proving-ground
- Experiment workflow: https://github.com/Infrasigma/subsume-proving-ground/actions/runs/38684349564
- Experiment commit: abc1c86fb223025be5522dda6270156a6dfccca5
- Artifact ID: 11950981949
- Artifact SHA-256: b5d0170a3d7526a805b254d29940fb71ce778f1ce5c3cd1107b01e86d37c6806
- Gen6 raw-log SHA-256: 74caa5a04008a118d897ca41d03e50089a9bfc5fb67ce94579ed02977531f3a2
- Gen6 checkpoint SHA-256: 22828e4d70d7728c87e887dd8e507a4fe43240a154117edcce9f899d218838ee
- Symbolic audit: SymPy 1.14.0

## Results

For every CLOSURE_LINEAGE_MISMATCH event, the post-selection audit reconstructed the hidden closure target from the immutable checkpoint, expanded the selected candidate through the runtime primitive map, normalized both expressions, and simplified candidate minus target.

- 13/13 selected candidates had already passed the task's behavioral evaluator.
- 13/13 failed the expected executable parent-ID lineage check.
- 13/13 symbolic differences simplified to exact Integer(0), and canonical symbolic representation hashes matched.
- All 13 frontiers had at least one competent alternative with expected lineage; 54 candidate instances across frontiers, not 54 unique programs.
- Recomputing the current target-blind cost-first order from the Gen6 checkpoint reproduced the runtime's first candidate on 13/13 events.

This is stronger than a 2,000-point random probe; it is still a symbolic simplifier result, not a machine-checked theorem. It does not alter selection, admission, or gates.

## Retained archive aliasing

Six retained capability IDs reduce to three normalized symbolic functions:

- hidden:0:0 and hidden:5:5: Min(-z, x + y)
- hidden:1:1, hidden:2:2 and hidden:3:3: Min(x, -z, x + y)
- hidden:4:4: Min(x, -z, x + y)**2

The 13 closure targets reduce to five symbolic forms. Hence distinct parent-ID pairs can define the same extensional function. A lower-cost candidate can compute the exact target function while omitting the intended parent IDs. The observable numeric data do not identify which historical IDs the task generator intended.

## Assessment and next scientific step

This diagnostic distinguishes two claims: (1) computes the correct function (all 13 selected candidates did); (2) reuses the exact expected parent IDs in its executable macro ancestry (all 13 failed). The second claim is structurally stronger and is not identified by function behavior alone in this pure DSL.

A target-blind heuristic is not yet a fix: maximum executable ancestry count picked expected lineage in only 6/13 cases; nine other basic feature policies picked it in 0/13. Do not pass hidden parent IDs into runtime selection or weaken the gate. The next predeclared research step is a benchmark-design study: either use a generic target-blind archive deduplication rule before parent sampling, or create a task with a provably observable and leakage-safe parent distinction. Any change requires separately versioned 5 seeds × 12 generations qualification.

## Decision

Gen6: ONE_SEED_DIAGNOSTIC_FAIL. Gen7: lineage qualification failure (20 closure traps, 2 probe traps). Promotion: DO_NOT_PROMOTE. Production ACSIE was not modified; PRs #110–112 remain drafts. AGI, ASI, and open-ended self-extension remain NOT_DEMONSTRATED.
