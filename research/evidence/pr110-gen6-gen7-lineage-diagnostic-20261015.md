# PR110 Gen6/Gen7 lineage diagnostic — 2026-10-15

## Provenance

- Repository: `Infrasigma/subsume-proving-ground`
- Research branch before this evidence-tool commit: `research/pr110-gen6-lineage-priority-20261014`
- Experiment commit: `abc1c86fb223025be5522dda6270156a6dfccca5`
- Workflow run: [38684349564](https://github.com/Infrasigma/subsume-proving-ground/actions/runs/38684349564)
- Artifact ID: `11950981949`
- Artifact SHA-256: `b5d0170a3d7526a805b254d29940fb71ce778f1ce5c3cd1107b01e86d37c6806`
- Gen6 raw-log SHA-256: `74caa5a04008a118d897ca41d03e50089a9bfc5fb67ce94579ed02977531f3a2`
- Frozen Gen5 input checkpoint SHA-256: `f21654220651001f3ac63f67a983a516ff8ba236732bb58e47f611c41dd87ced`
- Gen6 result checkpoint SHA-256: `22828e4d70d7728c87e887dd8e507a4fe43240a154117edcce9f899d218838ee`
- Gen6 result state digest: `369cf45b1d12afdb831eb75d91e63ae38dfba118a8898f2b9fa5117ba1b6602a`
- Gen7 result checkpoint SHA-256: `eb7a901ee360f1ec6af6783d150ca78fe2a59a0ea3bc0702b34b4064c3008964`
- Gen7 result state digest: `57c21f54aea5f7a362465ef62e1a121d88a5eb5fe5e5a80b7cf3cb8b30536e2f`

The GitHub job and focused preflight tests completed successfully. This is infrastructure/process success, not a scientific pass.

## Gen6 result — scientific fail, one seed only

Seed `2026100305`; source checkpoint resumed at generation 6 and validated to `generation_next=7`.

- Closure behavior: 14/15 = 93.33%.
- Exact executable-parent reuse: 1/15 = 6.67%.
- Closure lineage traps: 13.
- Probe behavior: 100%.
- Probe exact-parent reuse: 25%; probe traps: 3.
- Candidate acceptance: false.
- Transfer error: 0.6253149732; OOD error: 3.0309140855.
- Classification: `ONE_SEED_DIAGNOSTIC_FAIL`; `DO_NOT_PROMOTE`.

The legacy `*_parent_fidelity=0.0` metrics are representation MAE (zero means no representation error), not successful lineage reuse.

## Fresh-input identifiability audit

The raw Gen6 log contains 13 closure-lineage mismatch events. For every event, the evaluator-only audit sampled 2,000 deterministic fresh inputs over x, y and z in [-10, 10].

- Audit results: 13/13 `EMPIRICALLY_AMBIGUOUS`.
- Candidate instances across the pair audits: 159.
- Wrong-lineage, validation-competent candidate instances matching the target exactly on all sampled inputs: 105 (recorded and independently recomputed from candidate-level rows).
- Expected-lineage exact candidate instances across rows: 54.
- Audit flags state that these results did not affect selection or admission; runtime selection was not changed.

Counts are candidate instances across separate target-pair audits, not 105 unique programs. Candidate IDs recur across pairs. The fresh-input observations are finite-sample evidence, not a formal equivalence proof.

**Research implication:** in this seed/generation, the current behavior-based evaluation cannot reliably distinguish the selected wrong-lineage representatives from correct-lineage representatives, because both kinds match the hidden target exactly over the sampled fresh inputs. Do not fix this by passing expected parent IDs into runtime ranking, by selecting based on holdout/transfer/OOD, or by weakening the gate silently. The next research step is a predeclared challenge design that tests a clearly stated, leakage-safe notion of recursive reuse and identifies when exact parent lineage is or is not observable.

## Gen7 continuation — completed diagnostic, still fails the local lineage criterion

The workflow resumed exactly from the validated Gen6 checkpoint and stopped at the requested generation-7 boundary. It completed in 1,648.095 seconds (~27.47 minutes); checkpoint validation passed and `generation_next=8`.

- Generation 7 marked accepted by the learner: true.
- Closure behavior: 21/21 = 100%.
- Exact closure-parent reuse: 1/21 = 4.76%; closure traps: 20.
- Probe behavior: 100%; exact probe-parent reuse: 50%; probe traps: 2.
- Target depth stayed at 7 (same as Gen6); runtime max depth increased to 9.
- One-seed diagnostic classification: `GEN7_DIAGNOSTIC_COMPLETED_REQUIRES_REVIEW` in the emitted summary; given 20 closure traps and 2 probe traps, the exact-lineage qualification criteria are not met.

The return object's `finite_open_ended_growth_gate=false` is **not by itself a valid full-gate conclusion**: this segment deliberately stopped after generation 7, while `run_seed` was asked for 12 generations, so its full-run gate cannot pass. A complete frozen qualification has not been run.

## Target-blind ranking counterfactuals — retrospective diagnostic

To investigate whether a generic search-order/structure signal could solve the Gen6 failures without leaking expected parent IDs, a read-only replay scored each saved frontier using only candidate structure/dependency-count features and original frontier order for ties. Expected-parent match and competency labels were consulted only after each policy chose its candidate.

| Target-blind policy | Exact expected-lineage selections |
|---|---:|
| Discovery-first (current recorded choice) | 0/13 |
| Maximum direct primitive count | 0/13 |
| Maximum ancestor-lineage count | 6/13 (46.2%) |
| Minimum ancestor-lineage count | 0/13 |
| Minimum AST nodes | 0/13 |
| Maximum AST nodes | 0/13 |
| Maximum macro nodes | 0/13 |
| Minimum macro nodes | 0/13 |
| Shallowest AST | 0/13 |
| Deepest AST | 0/13 |

These are **post-hoc results on the same 13 cases**, not an independent test or qualification. They show that simply maximizing the count of ancestors helps on some rows but does not solve the mismatch (7/13 still fail); the other tested generic features do not rescue any row. Historical bounded tie-break experiments also failed the qualification gate. Do not promote this heuristic based on this replay. A separate frozen seed/generation evaluation is required, and exact parent identities may be unobservable from behavior in the current task design.

The reproducible utility `lab/pr110_gen6_blind_selection_replay.py` and tests are added alongside the audit aggregator. Both are diagnostic-only; no runtime ranker was changed.

## Gate and promotion decision

- Frozen qualification: 5 seeds × 12 generations — **not run here**.
- This experiment: one seed, one Gen6 rerun plus one Gen7 continuation.
- Scientific status: Gen6 fails; Gen7 also has severe lineage-reuse failures.
- Production ACSIE changes: none.
- PRs #110–112: do not merge on this evidence.
- AGI, ASI and open-ended self-extension: `NOT_DEMONSTRATED`.
- Promotion: `DO_NOT_PROMOTE`.

The added `lab/pr110_gen6_lineage_identifiability_report.py` aggregates the raw JSONL audits without changing runtime selection or gates. Its test suite covers ambiguous, non-ambiguous, missing-audit, malformed-line, and selection-invariance cases.
