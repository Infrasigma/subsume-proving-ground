# H-R14: H-R13 second-order reservoir + discovery-only CV semantic-partition selection

Date: 2026-10-01

## Observed failure

Fresh H-R13 qualification produced 4/5 scientific PASS and one scientific FAIL at p6 on seed 2026100110. The selected p6 program was `or(neq(parity(y),parity(z)), eq(sign(y),sign(z)))`; it reached validation, passed transfer, but failed fresh holdout at 0.375 versus prior-bank best 0.5.

## Hypothesis

H-R13 improves compositional frontier coverage but can increase competition among candidates. The p6 result is consistent with a selection/generalization interaction: more structural candidates become available, but the current raw selection score can prefer a candidate that does not generalize to the untouched holdout.

A discovery-only four-fold CV selector plus semantic-partition quotient may reduce this selection variance without using holdout, transfer, benchmark IDs, task labels, or target identity.

## Intervention

Keep H-R13 second-order composition coverage unchanged, then:

1. evaluate candidates with four folds over discovery + selection only;
2. rank eligible candidates by CV mean, then train score;
3. quotient candidates by their observed behavior partition (including Boolean inverse) over discovery + selection;
4. deterministically choose a canonical representative within each partition;
5. keep existing holdout, transfer, replay, retention, regression, and integrity gates unchanged.

This is a new combined hypothesis. It does not inherit H-R13 qualification automatically.

## Falsification

One scientific failure blocks promotion. Any integrity violation, holdout contamination, incumbent route change, retention regression, or execution crash blocks promotion.

## Scope

L2-C acquisition/selection mechanism only. It does not establish AGI, ASI, or general capability.
