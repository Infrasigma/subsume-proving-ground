# PR110 Gen6 lineage-selection replay — 2026-10-14

**Disposition:** diagnostic analysis only; no production promotion.  
**Source run:** [run 38602663903](https://github.com/Infrasigma/subsume-proving-ground/actions/runs/38602663903).  
**Input checkpoint:** raw SHA-256 `f21654220651001f3ac63f67a983a516ff8ba236732bb58e47f611c41dd87ced`; state digest `e284699d06649788b58071783651f087c57e0511ee10f93948b878d68f49e28a`; seed `2026100305`, generation_next=6.  
**Evidence inspected:** `bitset-full-gen6-raw.txt` and final checkpoint from the run artifact. Candidate-frontier replay below was performed offline in analysis only; expected parent IDs were not added to runtime ranking.

## Confirmed observation

The original Gen6 run emitted 16 `CLOSURE_LINEAGE_MISMATCH` records. All 16 logged candidate frontiers contained at least one candidate whose *executable* lineage included the two expected parent primitive IDs, but the runtime-selected candidate did not. Frontier sizes were 9–32 (median 15). The count of expected-lineage candidates per frontier was 1–8 (median 3, mean 3.56). This falsifies “the closure search simply never generated the right executable representative” as the explanation for these 16 failures; it does not rule out generation/coverage issues in other phases or generations.

The first divergence is therefore candidate ordering/selection from a set of behaviorally competent alternatives. Candidate correctness on train/transfer/OOD is not enough to establish correct executable-parent reuse.

## Offline ranking sensitivity test

For each of the same 16 old frontiers, I recomputed expanded execution depth/node count from the frozen end-of-Gen6 primitive definitions and replayed several generic, non-target-aware lexicographic rank orders. This is a post-hoc sensitivity analysis, **not** a proposed production objective and **not** a qualification result; it uses the same 16 failures that motivated the analysis.

| Generic rank order | Selected candidates with expected executable parent lineage | Mean expanded depth | Mean expanded nodes |
|---|---:|---:|---:|
| Expanded cost first, then smaller direct vocabulary, then executable-lineage cardinality | 0/16 | 8.94 | 97.56 |
| Direct-vocabulary cardinality first, then surface AST size, then executable-lineage cardinality | 5/16 | 11.56 | 136.44 |
| Executable-lineage cardinality first, then direct-vocabulary count and expanded cost | 3/16 | 10.81 | 120.69 |

The current run for the cost-first aligned patch is still the authoritative test for its actual Gen6 trajectory, because its Gen6 primitive acquisition may differ from the old run and change the subsequent closure pairs. The offline replay only shows that a cost-first re-rank applied to the *old* frontiers would not itself resolve their lineage gates.

## Research implications

1. Preserve the explicit split between **selection** and **validation**: choose a candidate using a predeclared discovery-only objective before inspecting holdout/transfer/OOD results. A failed selected candidate must fail admission, not silently promote a later candidate.
2. Keep executable cost and executable ancestry separate: the H7 R2 default in the private ACSIE PR112 line is expanded-cost-first; this is not equivalent to maximizing direct vocabulary. Cost-first alone may trade away recursive reuse, so its scientific effect needs direct measurement.
3. Do not encode expected-parent IDs, hidden target construction, or OOD outcomes into runtime ranking. They are evaluator diagnostics only.
4. If the cost-first Gen6 rerun still fails lineage, run a separately labeled, predeclared generic multi-objective/lineage-retention hypothesis (cost depth, expanded nodes, direct vocabulary and executable ancestry kept as distinct dimensions), from the same immutable checkpoint. Report the cost/reuse trade-off, do not choose a scalar ordering by tuning against these 16 failures, and require a new frozen seed/generation evaluation after the hypothesis is frozen.
5. Keep production unchanged. Promotion requires the frozen 5-seed × 12-generation qualification with every seed/generation gate satisfied; a one-seed Gen6 diagnostic cannot substitute for it.

Current statuses remain: `H7=UNRESOLVED`; `OPEN_ENDED_SELF_EXTENSION=NOT_DEMONSTRATED`; `GENERAL_CAPABILITY=NOT_DEMONSTRATED`; `AGI=NOT_DEMONSTRATED`; `ASI=NOT_DEMONSTRATED`; `PROMOTION=DO_NOT_PROMOTE`.

## Fresh-input identifiability audit

A second evaluator-only audit used the saved hidden-capability expressions from the checkpoint (not the learned primitive expressions as target surrogates) and compared every old closure-frontier candidate with the actual hidden closure target on **2,000 fresh deterministic random inputs** over `x,y,z ∈ [-10,10]`. It did not participate in ranking or admission.

- Each of the 16 mismatch frontiers contained at least one wrong-lineage candidate with **exact zero error on all 2,000 inputs**.
- In 15/16 frontiers, every candidate in the recorded frontier had exact zero error on all 2,000 inputs; the remaining frontier had 20/32 candidates with exact zero error.
- In the saved Gen6 primitive archive, 22 primitive pairs (out of 105 pairs) also had exact zero error on the same 2,000-point sample, despite distinct primitive IDs and different stored expressions.

These are strong finite-sample behavioral-equivalence observations, not formal equivalence proofs. They show that exact hidden parent-ID membership is frequently not identifiable from task input/output behavior in this fixture. Therefore a candidate-selection patch alone cannot be assumed to solve the exact-lineage gate: it may only choose among observationally indistinguishable alternatives. The next scientific issue is the test construction / representation-identity contract. Do not weaken the existing lineage gate silently; instead create a predeclared challenge set that separates parent functions behaviorally (or otherwise provides an explicit, permitted structural requirement), while still denying runtime access to hidden target identities. The evaluator should report ambiguity separately from ordinary search failure, but must not use the diagnostic to change runtime selection.
