# PHASE 2.1 PARENT-LEVEL CONSEQUENCE DOMAIN DECISION v1

**Target:** 1B.2A.1 — Parent-Level Scientific Consequence-Domain Decision  
**Status:** `A — EXPLICIT NARROW SCOPE`  
**Definitive Phase 2.1 experiment:** `NOT EXECUTED`  
**Repository:** `Infrasigma/subsume-proving-ground`  
**Branch:** `phase2-structural-transfer-20260910`

## 1. Parent wording

The authoritative parent is `phase2_1_protocol_draft.md` at commit `3b5891ec7c45055bc9a8bf7bdf178c07f1e03d78`, SHA-256 `06f39b7ead0dae272094cda82654e834be4cc22c`.

The parent defines the scientific question as structural transfer:

`C(B+ | K_A) < C(B+ | K_0)`

and defines A/B causal dependencies through X/Y/Z interventions. In Family A, `Y_r` changes `e_r:0->1`, `Z_r` changes no enablement variable, and `X_r` is unavailable while `e_r=0` and becomes available after Y. The same causal pattern is specified for Family B. See parent Sections 5.2–5.6 and 9.4–9.7.

Parent Section 12.3 requires every prediction to contain a `predicted consequence type` and says that a prediction must specify the expected `availability/outcome consequence` of applying the relation. Parent Section 13.1 requires `retrieval -> application -> prediction -> intervention -> observed consequence -> verification -> attribution`.

The parent does not state that the complete consequence domain is exactly `{ENABLES(target), NONENABLES(contrast,target)}`. That exhaustiveness was introduced later by the SERL learner freeze.

## 2. Formal gap

The parent protocol specifies:

- that a prediction has a consequence type;
- that the consequence is an expected availability/outcome consequence;
- the causal mechanisms that produce positive enablement and negative non-enablement contrasts;
- the verification/attribution chain.

It does not formally specify an exhaustive type system for all possible prediction consequents.

Therefore the parent alone does not entail a unique complete consequence domain.

## 3. Options

### A — Explicitly narrow Phase 2.1

Prospectively define this Phase 2.1 experiment as testing one deliberately bounded causal-transfer capability:

`ConsequenceType := { ENABLES(target), NONENABLES(contrast,target) }`

This is not called general prediction. It is a scope restriction for this experiment only.

### B — Define a complete broader domain

Would require a new complete formal consequence type system, including predicate typing, legal instantiation, canonicalization, admissibility, and attribution compatibility. The parent task families do not currently instantiate a broader outcome vocabulary, so doing this would materially expand the scientific question rather than merely close an existing implementation detail.

### C — Redesign Phase 2.1

Would be required if the narrow scope failed to preserve the intended relational-transfer question or if the experiment required broader consequence types to make its hypothesis scientifically meaningful. The audit does not establish that requirement.

## 4. Methodological evidence

The methodological literature supports treating relational generalization as transfer of role-structured relations to novel instances, not as requiring an unrestricted universal prediction language. Hummel and Holyoak describe relational inference/generalization in terms of acquiring schemas and applying them to new relational configurations. Doumas et al. likewise frame cross-domain generalization as inference over structured relational representations. citeturn0search0turn0search1

Causal-abstraction literature emphasizes that abstraction is tested by preserving causal/interventional structure across levels or domains; it does not require every conceivable consequence predicate to be represented in a single benchmark. Beckers and Halpern formalize increasingly strong notions of causal abstraction, while later work extends such mappings to soft and lossy interventions. citeturn0search7turn0search2turn0search8

These sources support the scientific legitimacy of a deliberately bounded relational-transfer test, but they do not specifically mandate the two-value domain. The choice therefore remains a prospective Phase 2.1 scope decision, not a literature-derived entailment.

## 5. Scientific tests of Option A

### Necessity

The two-value restriction preserves the central causal-transfer contrast actually instantiated by both task families: an intervention changes whether a target transition is enabled, while a matched contrast leaves it non-enabled. The B family uses the same dependency semantics. Thus the experiment can still ask whether an A-derived relational rule about intervention → enablement/non-enablement transfers to structurally novel B instances.

### Sufficiency

A successful result would demonstrate only the following bounded capability:

`A-derived relational abstraction about causal enablement/non-enablement -> prediction -> verified intervention consequence -> structural transfer`

It would not demonstrate general prediction, arbitrary causal reasoning, general representation learning, or AGI.

### Bias

The restriction does bias the experiment toward the enablement relation instantiated by the benchmark. That bias is acceptable only because it is now explicit and prospective. It must not be described as a neutral discovery of the full prediction language.

### Generality

The tested primitive remains relevant to ACE because reusable relational knowledge must often encode intervention-to-consequence relations. However, the general ACE architecture must not inherit the two-value limit.

### Failure interpretation

Failure remains informative but narrow. It could falsify the claim that this bounded SERL-style relational abstraction transfers under these controls; it could not establish that relational abstraction generally fails.

## 6. Strongest counterargument

**Objection:** The two-value domain may hard-code the causal relation that the benchmark was designed to reward. A positive result could therefore show only that the learner can acquire and reuse the benchmark's predefined enablement schema, rather than discover broadly useful causal consequences.

**Response:** This objection is valid as a limitation and is now made explicit rather than hidden. The parent task family itself defines the causal intervention as an enable/non-enable contrast. The experiment is therefore retained as a narrowly scoped test of reusable relational transfer, not as a test of open-ended causal prediction. The result must be interpreted at exactly that level.

A second objection is that Option B would be more AGI-like. That does not by itself make it scientifically superior here: introducing a broad consequence ontology absent from the frozen task families would change the question and create additional arbitrary design degrees of freedom. A larger benchmark is not automatically a better controlled experiment.

## 7. Decision

**Decision: Option A.**

For Phase 2.1 only, the complete prediction consequence domain is explicitly and prospectively frozen as:

`ConsequenceType := { ENABLES(target), NONENABLES(contrast,target) }`

with the following interpretation:

- `ENABLES(target)` predicts that the intervention makes the target action available/executable under the protocol's causal semantics;
- `NONENABLES(contrast,target)` predicts that the contrast intervention does not make the target action available/executable.

The two-value restriction is an experimental scope boundary, not a claim about the complete prediction language of ACE.

No endpoint data were used in making this decision.

## 8. Limitations

This decision does not establish:

- general prediction;
- arbitrary consequence representation;
- general causal discovery;
- open-ended relational learning;
- broad model-based planning;
- semantic understanding;
- general intelligence;
- AGI.

A positive Phase 2.1 result would be evidence for one bounded reusable causal-relational-transfer capability only.

A negative result would invalidate or weaken that bounded claim under the preregistered implementation and controls, not the broader ACE thesis.

## 9. SERL impact

The existing SERL two-value consequent restriction is now scientifically authorized **prospectively for Phase 2.1**, but its provenance is corrected: it was not entailed by the original parent freeze. Any SERL implementation freeze must reference this decision as the explicit scope authority.

The earlier SERL grammar remains downstream of this decision. It must not be treated as evidence that the parent protocol originally contained the restriction.

## 10. Combined ACE impact

The combined ACE architecture remains open-ended with respect to consequence representation.

ACE must ultimately support domain-appropriate falsifiable consequences beyond enablement/non-enablement, including richer state changes, relational effects, quantitative or structured outcomes, and counterfactual consequences where appropriate. Those capabilities belong to future ACE architecture research and are not to be injected into Phase 2.1 merely to make it more general.

Thus:

`Phase 2.1 consequence domain != ACE universal consequence domain`.

## 11. Protocol versioning

The original frozen `phase2_1_protocol_draft.md` is preserved unchanged.

This decision creates an explicit prospective scope delta. Therefore a versioned protocol artifact is required before definitive execution:

`phase2_1_protocol_v2.md = parent v1 + this explicit consequence-domain scope decision`

No definitive experiment is authorized merely by this decision artifact; all remaining closure and independent conformance requirements still apply.

## 12. Provenance

Parent protocol:

- file: `phase2_1_protocol_draft.md`
- commit: `3b5891ec7c45055bc9a8bf7bdf178c07f1e03d78`
- SHA-256: `06f39b7ead0dae272094cda82654e834be4cc22c`

SERL learner freeze where the two-value restriction first appears explicitly as exhaustive:

- file: `phase2_1_learner_freeze_v1.md`
- commit: `b2d2f3b09d7f8eb5a4800efbdfa8bfe5f198ea8b`

Decision artifact:

- file: `phase2_1_consequence_domain_decision_v1.md`
- initial commit: `c5874afe7f9d8f1701cdaf8dc2cf40c45fe5cfd7`
- finalized commit: this update commit

Protocol v2:

- file: `phase2_1_protocol_v2.md`
- commit: `8a0e2b26e9a1f1e4dc3d23bf56bd29047c94cc7d`

## 13. Execution status

**Definitive Phase 2.1 experiment: `NOT EXECUTED`.**

This decision is scientific scope closure only. Independent conformance/differential verification remains mandatory before execution.
