# H7 experiment: invocation-local lineage memoization

Status: laboratory hypothesis under test. No production ACSIE source is modified.

## Motivation
On pinned repaired-B runtime `9b0c42eb4394aee457f8b7adecc45a3f4ea7c4be`, the secondary Gen5 behavioral search for `pair_idx=14, probe_idx=1` took about 1,009 seconds to reach depth 2 and about 1,016 seconds to reach depth 3. In the same invocation the evaluator recorded only 1,428 expressions through depth 2 and 7,262 through depth 3. The observed counters point to repeated work inside ranking/dominance, not simply expression count.

The source repeatedly calls `executable_lineage_ids(used_primitives)` from `rank`, `dominates`, and `lineage_frontier`; each call rebuilds the same executable lineage closure by traversing the unchanged primitive archive. Provenance closure is recomputed in a similar way. This variant caches those two pure results locally to one `find_exact_expression` invocation.

## Hypothesis H7-LC1
Memoizing `executable_lineage_ids` and `provenance_lineage_ids` by the exact direct-macro tuple is observationally transparent and substantially reduces runtime for lineage-heavy Gen5 search.

## Required gates
1. Full frontier representatives and search statistics match the original snapshot exactly for the same checkpoint, seed, generation, probe and hash seed. Only elapsed-time fields and the separately recorded candidate-module identity may differ.
2. The selected output, lineage sets, target match counts, dominance counts and ordering remain unchanged.
3. A measured speedup is reported descriptively; it is not accepted if parity fails.
4. Follow-up 5-seed × 12-generation qualification remains separate and unchanged.

No runtime compression/deletion is performed. No holdout, transfer, or OOD data enter the selector. The validation bundle is analyzed only after capture.
