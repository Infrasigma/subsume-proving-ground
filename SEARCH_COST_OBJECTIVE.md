# SEARCH COST OBJECTIVE

Evidence mode: CODE_DERIVED_NO_EXECUTION.

The selector is `selectMethod(evals, history)`.

| Field | Status | Source/formula | Consumed by | Learned? |
|---|---|---|---|---|
| Compute | PRESENT | `eval.Cost.Compute`; denominator `1+Compute+ExperimentBudget` | selector score | no |
| ExperimentBudget | PRESENT | `eval.Cost.ExperimentBudget`; same denominator | selector score | no |
| VerificationCost | ABSENT | no separate selector field | none | no |
| RegressionCost | ABSENT | no separate selector field | none | no |
| ImplementationCost | ABSENT | no explicit field in selector | none | no |
| DiscoveryCost | PRESENT elsewhere in abstraction discovery, not in `selectMethod` | observation score denominator includes Compute/Memory/TimeMS/ExperimentBudget | `DiscoverReusableAbstraction` | no |
| ExecutionCost | PRESENT as `MethodEvaluation.Cost`, but selector only consumes Compute + ExperimentBudget | selector | no |
| TransferCost | ABSENT | no selector term | none | no |
| HistoricalSuccess | PRESENT | verified history adds `1/(SearchAttempts+1)`, failed history subtracts `.25` | selector | history-derived |
| Gain | PRESENT | `eval.Gain`; current verifier hard-sets successful gain to `1` | selector | not learned |
| FutureSearchChange | PRESENT as Boolean gate | must be true | candidate eligibility | no |
| Complexity | ABSENT | no complexity term | none | no |
| MDL Penalty | ABSENT | no MDL implementation | none | no |
| FinalFitness | PRESENT | accumulated map score per method name | ranking | partly history/evaluation-derived |
| Ranking | PRESENT | stable descending sort by score | winner selection | no |
| TieBreak | PRESENT implicitly | `sort.SliceStable`, preserving evaluation order on equal scores | ranking | no |

Critical upstream fact: `AutonomousMethodCandidatesWithLibrary` ignores the diagnosed bottleneck (`_ = d`) and always enumerates exactly the same depth-2 procedure space. `verifyMethodCandidateWithLibrary` rejects before behavioral verification if `sameMechanismOrder(base, cs)` is true. A successful evaluation is assigned `Gain:1`, `Transfer:true`, `Regression:true`, `LeakFree:true`, `FutureTraceChanged:true` after hidden/regression checks.

UniversalMechanismSearch itself has no objective optimization: it returns three fixed strategies in fixed order. Therefore the current objective ranks procedures that already passed the fixed search and behavioral gates; it does not learn a new search-space-generating operator.

No raw numeric objective trace is claimed because the recursive experiment was not executed in this environment.