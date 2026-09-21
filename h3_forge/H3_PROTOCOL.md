# H3 Compression-Driven Predictive Induction Protocol

Hypothesis:
A generic lossless compression/prediction learner can turn recurring experience into reusable representations that reduce the cost of genuinely new future experience, including novel compositions and renamed surface symbols.

Constraints:
- CPU-only deterministic Go implementation.
- No external model or pretrained intelligence.
- No task-family-specific solver.
- No human labels or semantic annotations.
- Hidden audit data are generated independently from training streams.
- Learned library must pay its own storage cost.
- Evaluator and acceptance logic are outside learned data.

Frozen gates:
H3-A Hidden composition savings: learned K_n must reduce audited bit cost by >=25% versus a fresh K0 learner on each independent generator block.
H3-B Recursive compounding: at least 3 successive future-generation ratios R_n=C(T_n|K_(n-1))/C(T_n|K0) must be <0.75 in each block.
H3-C Surface invariance: renamed-symbol audits must retain >=80% of learned-library savings of unrenamed audits.
H3-D Higher-order composition: audit sequences built from newly learned composite macros must retain >=25% net savings.
H3-E Negative control: libraries learned from structure-destroying shuffled streams must obtain <5% savings on the same hidden audits.
H3-F Two independent blocks must satisfy all gates.

Kill:
If either independent block fails any required gate after implementation defects are excluded, H3 is killed for this substrate. Do not relax thresholds or add domain-specific mechanisms.