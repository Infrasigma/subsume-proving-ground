# H-R13 + CV-Partition: Composition Coverage with Selection Robustness

Date: 2026-10-01

## Diagnosis

H-R13 successfully preserves second-order composition classes, but the five-seed qualification exposed a new interaction: on seed 2026100110, p6 selected a structurally second-order program that passed discovery/selection gates yet failed the fresh holdout. This means the reservoir is increasing candidate availability but can also increase the number of high-scoring finite-sample compositions that compete with simpler first-order programs.

The failure is therefore not evidence that second-order coverage is useless. It is evidence that frontier coverage and candidate selection must be separated.

## Hypothesis

A generic second-order composition reservoir plus discovery/selection-only cross-validation and semantic-partition selection will:

1. retain second-order candidates needed for later composition,
2. reduce selection of finite-sample overfitting compositions,
3. preserve the previously demonstrated p7 acquisition mechanism,
4. avoid holdout/transfer/task-label/target-specific routing,
5. leave retention isolated and unchanged.

## Intervention

Keep the H-R13 native composition reservoir unchanged.

Change only candidate selection:

- 4-fold cross-validation over discovery + selection rows,
- no holdout or transfer access,
- semantic partition quotient over the same pre-holdout observations,
- deterministic canonical representative within an equivalence class,
- rank by CV mean, then discovery fit and complexity.

## Falsification

Reject the combined mechanism if any completed seed fails acquisition/holdout/transfer, retention, or integrity, or if the protocol no longer executes the intended mechanism.

One scientific seed failure blocks promotion.

## Scientific boundary

This is a combined L2-C acquisition/selection hypothesis. It does not establish unrestricted continual learning, AGI, ASI, or general intelligence.
