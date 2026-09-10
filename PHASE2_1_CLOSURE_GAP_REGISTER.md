# PHASE 2.1 CLOSURE GAP REGISTER

**Status:** INCOMPLETE — Target 1B not yet closed.
**Repository:** `Infrasigma/subsume-proving-ground`
**Branch:** `phase2-structural-transfer-20260910`
**Parent scientific protocol:** `phase2_1_protocol_draft.md`
**Parent protocol SHA-256:** `06f39b7ead0dae272094cda82654e834be4cc22c`
**Parent commit:** `3b5891ec7c45055bc9a8bf7bdf178c07f1e03d78`

## Classification

- **A — scientifically consequential:** different choices can change the endpoint, attribution, validity, or interpretation; must be frozen before execution.
- **B — reproducibility consequential:** does not change the scientific question but can change concrete outputs; freeze where practical.
- **C — implementation-equivalent:** alternatives are permitted only if the equivalence test below is satisfied.
- **D — non-consequential engineering:** may remain implementation-defined.

## Gap register

| ID | Ambiguity | Can change endpoint? | Current state | Class | Closure required? |
|---|---|---:|---|---|---:|
| G01 | RNG purpose strings | Yes, via generated corpus/tokens | Namespace and encoding frozen; exact purposes incomplete | A | YES |
| G02 | A generation algorithm | Yes | High-level algorithm specified; exact stream/counter consumption incomplete | A | YES |
| G03 | B generation algorithm | Yes | High-level rejection procedure specified; exact counter allocation incomplete | A | YES |
| G04 | RNG counter allocation | Yes | Not uniquely specified across subprocedures | A | YES |
| G05 | Token generation | Yes | Role-independent pool specified; exact digest-to-token construction incomplete | A | YES |
| G06 | Dependency selection | Yes | Candidate set/count/rejection rules specified; exact draw sequence/counters incomplete | A | YES |
| G07 | Learner architecture | Yes | No actual learner frozen | A | YES — scientifically unresolved |
| G08 | Learner representation | Yes | Only external observation is frozen | A | YES — scientifically unresolved |
| G09 | Learning algorithm | Yes | Not frozen | A | YES — scientifically unresolved |
| G10 | Memory state | Yes | Learner history allowed but storage/update semantics not frozen | A | YES |
| G11 | Search algorithm | Yes | Search namespace exists; procedure not frozen | A | YES |
| G12 | Solver/planner | Yes | Not frozen | A | YES |
| G13 | Action selection | Yes | Not frozen | A | YES |
| G14 | K_A candidate generation | Yes | Grammar and acquisition predicates frozen; mining algorithm not frozen | A | YES |
| G15 | K_A validation | Yes | Predicates frozen conceptually; exact validation pipeline/order not frozen | A | YES |
| G16 | Observation/history → predicates | Yes | Legal grammar exists; executable transformation not fully specified | A | YES |
| G17 | Retrieval applicability | Yes | Applicability concept exists; exact evaluator not frozen | A | YES |
| G18 | Retrieval tie-breaking | Yes | Ranking rule partly specified; complete tie/conflict handling needs closure | A | YES |
| G19 | Prediction generation | Yes | Event fields specified; prediction production is not algorithmically closed | A | YES |
| G20 | K_R control | Yes | Information boundary frozen; exact procedure/decision policy incomplete | A | YES |
| G21 | K_S control | Yes | Boundary described; exact matched search/decision policy incomplete | A | YES |
| G22 | K_P control | Yes | Boundary described; exact matched prior/randomization policy incomplete | A | YES |
| G23 | Direct replay | Yes | Anti-shortcut constraint exists; exact matching/replay procedure incomplete | A | YES |
| G24 | Transfer attribution | Yes | Precedence exists; ablation/equivalent causal evidence is not fully executable | A | YES |
| G25 | Decision-budget accounting | Yes | Decision cutoff is frozen; unit definition/internal accounting incomplete | A | YES |
| G26 | Interaction cost accounting | Yes | E is action attempts; edge cases require exact accounting table | A | YES |
| G27 | Timeout/termination | Yes | Terminal states specified; implementation timeout semantics incomplete | A | YES |
| G28 | Failed-action handling | Yes | Result types exist; illegal/environment-error invalidation ordering needs executable table | A | YES |
| G29 | Deterministic ordering | Yes | Some orders frozen; all enumeration/serialization orders not closed | B/A | YES |
| G30 | Serialization | Potentially | Canonical JSON rules substantially frozen; artifact schemas remain incomplete | B | YES |
| G31 | Artifact format | Yes for auditability/reproducibility | Required artifact classes listed; exact schemas not frozen | B | YES |
| G32 | Statistical implementation | Yes if conventions differ | Test family/count/seeds frozen; exact p-value/bootstrap quantile conventions need reference definition | A | YES |
| G33 | Exclusion rules | Yes | Invalid-task and invalid-pair principles exist; complete decision table incomplete | A | YES |
| G34 | Missing/invalid trials | Yes | Pair invalidation described; exact artifact/reporting and no-imputation rule needs closure | A | YES |
| G35 | A acquisition intervention sampling | Yes | Causal requirements specified; exact intervention-context enumeration is not fully algorithmic | A | YES |
| G36 | K_A provenance representation | Yes | Provenance validity required; exact record schema not frozen | A | YES |
| G37 | K_A support threshold | Yes | At least two independent A tasks stated; exact support counting/contradiction policy incomplete | A | YES |
| G38 | Cross-condition process isolation | Yes | Requirements specified; exact launch/clean-room procedure not frozen | A/B | YES |
| G39 | Learner/search random stream ownership | Yes | Namespaces exist; ownership and call schedule incomplete | A | YES |
| G40 | Control matching/equivalence | Yes | Conceptual matching required; operational equivalence test incomplete | A | YES |
| G41 | B freeze timing | Yes | Freeze-before-transfer principle stated; exact immutable artifact boundary not frozen | A/B | YES |
| G42 | Statistical reference implementation independence | Yes for audit confidence | Required conceptually; reference algorithm/schema not yet committed | B | YES |
| G43 | Two-implementation equivalence oracle | Yes | No executable equivalence oracle exists | A | YES |
| G44 | Adversarial alternative interpretations | Yes | Target 1A exposed unresolved choices; systematic closure red-team not yet committed | A | YES |
| G45 | Independent audit authority | Yes for completion claim | No independent reviewer/reproduction evidence exists in current repo state | A | YES |

## Immediate blockers

The highest-risk unresolved items are G07–G13 (actual learner/search/solver), G14–G19 (K_A construction/retrieval/prediction), G20–G24 (controls and attribution), and G35–G40 (acquisition/control matching). These are not harmless engineering choices. They can directly change which behavior is attributed to K_A and therefore can change the primary endpoint or the nine PASS criteria.

A learner chosen solely because it makes K_A usable would be circular: the experiment would then test the selected inductive bias rather than establish transfer independently of that bias. Conversely, a generic learner that cannot acquire the frozen relational abstraction would make K_A empty or mechanically unusable. The choice therefore requires explicit methodological justification before it can be frozen.

## Scientific closure rule

No implementation may proceed to the definitive Phase 2.1 experiment while any A-class ambiguity remains unresolved. A prototype interpretation is not evidence and must not be promoted to a Phase 2.1 result.
