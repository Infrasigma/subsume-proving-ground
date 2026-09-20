
# ACE Frontier Manifesto

**Artificial Cognitive Engine (ACE)**  
**Architectural Blueprint: T1–T15**

**Status:** T10 sealed; T11–T15 defined as future proof obligations  
**T10 baseline:** `26809d51546a92107fa8a0f38f10f7df7e0abe74`  
**Repository:** `Infrasigma/subsume-proving-ground`

---

## 0. Mission

The Artificial Cognitive Engine (ACE) is not defined as an omnipotent machine.

Literal omnipotence is not a scientifically meaningful engineering target: physical systems remain constrained by finite energy, matter, time, causality, information bandwidth, and the environments in which they operate.

The ACE objective is therefore:

> **Sustained, independently verified, open-world autonomous general intelligence with recursive capability improvement and closed-loop economic and physical agency.**

ACE is designed around a stricter proposition:

> A capability is not considered acquired merely because the system produced an answer, executed an action, or reported success. It is acquired only when the relevant consequence is independently verifiable and its provenance is durably committed to a protected evidence substrate.

The architecture therefore separates:

`experience
≠ knowledge

confidence
≠ verification

search
≠ learning

retrieval
≠ abstraction

provider claim
≠ consequence

execution
≠ successful effect`

The system's governing question is not:

> "Did the agent say it succeeded?"

It is:

> "What independently verifiable state change occurred, under which authority, and can that claim be reconstructed and audited?"

---

# 1. Architectural Doctrine

ACE is governed by five foundational principles.

## 1.1 Zero-Trust Authority

No autonomous component receives authority merely because it belongs to the system.

Authority is:

`identified
→ bounded
→ time-limited
→ capability-scoped
→ authenticated
→ independently verified
→ durably recorded`

The system must fail closed when authority cannot be established.

Unbounded authority is a protocol violation.

---

## 1.2 Two Independent Truths

For consequential actions, ACE requires two logically distinct channels:

`Channel A:
executor / provider
    ↓
actual actuation

Channel B:
independent observer
    ↓
consequence observation`

The executor's report is never itself sufficient evidence of the effect.

The system commits a consequential state only when the independent observation satisfies the declared expected effect.

The same principle extends from software to infrastructure, economics, logistics, manufacturing, and physical experimentation.

---

## 1.3 Cryptographic Determinism

Anything used as cryptographic evidence must have deterministic representation.

The current canonicalization boundary is deliberately fail-closed.

Floating-point JSON representations are rejected.

Durable protocol quantities therefore use explicit deterministic representations such as:

`int64
integer micro-units
absolute byte counts
nanoseconds / milliseconds
cryptographic hashes
canonical strings`

Dynamic JSON decoding must preserve integer semantics through canonicalization rather than silently converting values into `float64`.

Canonicalization is a security boundary, not a formatting preference.

---

## 1.4 Protected Evaluation

The candidate system must not control the machinery that decides whether it succeeded.

A candidate cannot:

`modify evaluator
→ change acceptance criteria
→ rewrite benchmark
→ declare itself successful`

Evaluation authority is external to candidate control.

Acceptance remains:

`PASS
FAIL
INSUFFICIENT`

with provenance sufficient to reconstruct why the result was obtained.

---

## 1.5 Evidence Before Escalation

Capability escalation follows verified evidence.

A later layer cannot inherit authority merely because an earlier layer exists.

The progression is therefore:

`mechanism
→ measurement
→ verification
→ durable evidence
→ capability admission
→ higher-order capability`

This prevents architectural ambition from being mistaken for empirical achievement.

---

# 2. F0 — The Cryptographic Trust Substrate

F0 is the protected execution and evidence boundary.

Its purpose is to ensure that consequential state changes are represented as an auditable chain rather than an informal application event.

Core lifecycle:

`AUTHORIZED
    ↓
DISPATCHED
    ↓
EFFECT_OBSERVED
    ↓
VERIFIED
    ↓
COMMITTED`

Exceptional states include:

`INDETERMINATE
RECONCILIATION_REQUIRED
ABORTED
COMPENSATED
QUARANTINED`

The critical invariant is:

> **An externally consequential state change cannot become trusted simply because the execution path returned success.**

The ledger cryptographically binds:

`execution identity
sequence
event type
canonical payload
previous event hash
current event hash`

Recovery states create explicit barriers against accepting late or contradictory observations.

F0 is therefore the root of operational truth.

---

# 3. T1–T10 Software Substrate

The T1–T10 hierarchy represents the current ACE software substrate.

These layers must be understood as progressively verified mechanisms rather than as claims of already-achieved general intelligence.

---

## T1 — Deterministic Cognitive Substrate

T1 establishes deterministic execution and evidence primitives.

Core properties include:

`canonical representations
deterministic hashing
signed envelopes
explicit domains
bounded contracts
durable state
reproducible evaluation`

The objective is to prevent hidden nondeterminism from entering the epistemic substrate.

---

## T2 — Hostile Scientific Evaluation

T2 establishes adversarial scientific testing.

The system is exposed to:

`hostile inputs
negative cases
boundary violations
recovery cases
tampered evidence
invalid transitions
adversarial protocol states`

A mechanism that succeeds only under friendly assumptions is not considered verified.

---

## T3 — Capability-Bounded Action

T3 establishes the ActionContract principle:

`identity
provider
resource
operation
arguments
delegation
precondition
expected effect
maximum scope
expiry
nonce
policy hash`

A capability describes what may be done, not what the system wishes to do.

Authority is therefore constrained before execution.

---

## T4 — Verification and Evidence

T4 establishes independent consequence checking.

The system distinguishes:

`requested effect
actual effect
verified effect
recorded effect`

A successful command without a corresponding verified consequence is insufficient.

---

## T5 — Cognitive Acquisition and Composition

T5 represents the knowledge and abstraction substrate.

Candidate knowledge must carry:

`provenance
evidence
version
dependencies
validity
composition relationships`

Capability acquisition is not equivalent to memorization.

A candidate abstraction must survive verification before it becomes reusable system knowledge.

---

## T6 — Distributed / Swarm Execution

T6 introduces AXON-style distributed execution.

The system can coordinate multiple bounded workers while retaining common authority and evidence boundaries.

The architectural rule remains:

> **Parallel execution must not become parallel uncontrolled authority.**

Each worker remains subject to capability, scope, identity, and verification constraints.

---

## T7 — External API Actuation

T7 establishes controlled interaction with external software systems.

The system may act through external APIs while preserving:

`capability scope
authentication
expected-effect semantics
independent consequence observation
receipt generation`

External API access therefore becomes an extension of the verification model rather than an exception to it.

---

## T8 — Kernel-Level Observability and Governance

T8 extends observation below the application layer.

Kernel-level mechanisms such as eBPF-based instrumentation provide visibility into:

`process behavior
resource consumption
system activity
execution boundaries`

The architectural purpose is not surveillance for its own sake.

It is to reduce the distance between:

`declared execution`

and

`observable host consequence`

T8 baseline was sealed before T9.

---

## T9 — Adversarial Proving Ground

T9 establishes the mechanical proving-ground framework.

Its purpose is to create a controlled loop in which the system can encounter a failure, generate or apply a bounded repair, rerun the evaluator, and commit successful evidence.

Conceptually:

`candidate
    ↓
adversarial task
    ↓
execution
    ↓
failure
    ↓
repair candidate
    ↓
re-evaluation
    ↓
verified improvement
    ↓
evidence admission`

T9 establishes the machinery for adversarial improvement experiments.

It does **not**, by itself, prove unrestricted continuous self-improvement.

T9 baseline:

`bf7b7cf53613842f470b0538577283cfb7b12e0a`

---

# 4. T10 — Verified Software Authority

T10 is the first explicit infrastructure-authority boundary.

T10 introduces the `InfrastructureContract` and a bounded infrastructure provider.

The sealed implementation demonstrates:

`contract generation
→ cryptographic capability binding
→ AUTHORIZED
→ DISPATCHED
→ bounded provisioning
→ independent OS observation
→ EFFECT_OBSERVED
→ VERIFIED
→ COMMITTED
→ bounded reclamation
→ independent absence verification
→ second committed lifecycle`

The local crucible uses a subprocess-based disposable resource and an independent `/proc` attestor.

The provider's PID is not treated as sufficient evidence.

The attestor independently discovers the resource from the operating system.

Resource existence and reclamation are therefore verified through a second channel.

The T10 baseline is:

`26809d51546a92107fa8a0f38f10f7df7e0abe74`

T10 establishes the following narrower claim:

> **A bounded software authority can provision, independently verify, commit, and reclaim a resource under the F0 evidence model.**

It does not establish unrestricted real-world infrastructure autonomy.

---

# 5. T11 — Verified Economic Agency

T11 transitions from software authority to legally and economically bounded resource acquisition.

T11 must not assume that software can spontaneously create legal ownership or financial authority.

The architecture is:

`legal identity
    ↓
delegated authority
    ↓
treasury capability
    ↓
legitimate economic activity
    ↓
verified revenue
    ↓
bounded spending authority
    ↓
resource acquisition`

The treasury capability should resemble:

`TreasuryContract {
    legal_entity
    account_reference
    currency
    maximum_single_payment
    maximum_periodic_spend
    allowed_counterparties
    allowed_categories
    expiry
    nonce
    policy_hash
}`

The system must never receive unrestricted financial authority when bounded authority is sufficient.

### T11 verification

A financial event requires multiple truths:

`authorization truth
+
transaction truth
+
counterparty truth
+
resource/billing truth`

For cloud acquisition:

`capability
→ provider request
→ provider inventory observation
→ independent billing observation
→ resource verification
→ spend verification
→ F0 commitment`

The ultimate T11 property is not possession of money.

It is:

\[
Revenue_{t+1} - OperatingCost_{t+1} > 0
\]

over repeated independently verified cycles, subject to legal and contractual constraints.

T11 is therefore **verified economic agency**, not autonomous money creation.

---

# 6. T12 — Verified Physical Agency

T12 extends the two-truth model from APIs and operating systems into the physical world.

A procurement event must not be trusted solely because a supplier reports completion.

Example:

`purchase order
    ↓
supplier execution
    ↓
shipment
    ↓
physical arrival
    ↓
independent inspection
    ↓
metrology / serial identity
    ↓
electrical / functional testing
    ↓
verified acceptance
    ↓
F0 asset admission`

For a hardware asset:

`supplier database:
    SERIAL = X
    STATUS = PASS`

is insufficient.

Independent evidence may include:

`physical presence
serial-number identity
weight
dimensions
electrical behavior
functional behavior
environmental telemetry`

The acceptance condition becomes:

\[
ProviderClaim \land IndependentObservation \land ExpectedEffect
\]

Only then may ownership or operational state become committed.

### T12 expansion

T12 should progress through:

`T12a — hardware procurement
T12b — logistics orchestration
T12c — assembly
T12d — manufacturing
T12e — maintenance
T12f — productive-capacity expansion`

The key question is:

> Can the system cause and verify physical state transitions without replacing human verification with its own assertion?

---

# 7. T13 — Verified Open-World Generalization

T13 addresses the central intelligence question.

Passing predefined tasks is insufficient.

Tasks must be independently sampled from previously unseen domains.

Define:

\[
A_n(D)
\]

as independently measured task achievement for system state \(n\) over distribution \(D\).

T13 requires meaningful performance on:

`unseen tasks
unseen compositions
unseen domains
novel constraints
adversarial tasks`

without human engineers inserting domain-specific solutions between task issuance and execution.

The evaluation system must remain outside candidate control.

The core requirement is:

\[
A_n(D_{novel}) \gg A_{baseline}
\]

under a preregistered evaluation protocol.

T13 is where the project moves from sophisticated automation toward evidence of general intelligence.

---

# 8. T14 — Verified Recursive Capability Improvement

T14 asks whether ACE can improve the machinery that produces its own capabilities.

Let:

\[
A_n
\]

represent independently evaluated competence at generation \(n\).

T14 requires:

\[
A_{n+1} > A_n
\]

on held-out evaluations.

Additionally:

\[
R_{n+1} < R_n
\]

may be required where \(R_n\) represents the resource cost of producing equivalent capability.

The critical constraint is evaluator independence.

The system must not be permitted to:

`rewrite evaluator
rewrite acceptance criteria
delete failed examples
change benchmark distribution
manufacture its own ground truth`

T14 therefore distinguishes:

`self-modification`

from

`verified self-improvement`

Only the second counts.

---

# 9. T15 — Verified Economic and Material Self-Sustainability

T15 closes the resource loop.

A system is not operationally self-sustaining merely because it can perform tasks.

It must maintain the resources required for continued operation through verified autonomous activity.

Conceptually:

`capability
    ↓
economic output
    ↓
resource acquisition
    ↓
compute + infrastructure
    ↓
physical productivity
    ↓
new capability`

The sustainability condition is not infinite growth.

It is the existence of a stable positive closed loop under independently verified measurements.

At minimum:

\[
ResourceInflow_t \ge ResourceConsumption_t
\]

over sustained intervals relevant to the system's operational requirements.

For growth:

\[
Capability_{t+1} > Capability_t
\]

must continue without proportional manual engineering intervention.

T15 is therefore **verified self-sustainability**, not infinite expansion.

---

# 10. The Unified T1–T15 Hierarchy

The architecture can now be represented as:

`T1  Deterministic cognitive substrate
 ↓
T2  Hostile scientific evaluation
 ↓
T3  Capability-bounded action
 ↓
T4  Verification and evidence
 ↓
T5  Knowledge acquisition and composition
 ↓
T6  Distributed execution
 ↓
T7  External API actuation
 ↓
T8  Kernel observability / governance
 ↓
T9  Adversarial proving ground
 ↓
T10 Verified software authority
 ↓
T11 Verified economic agency
 ↓
T12 Verified physical agency
 ↓
T13 Verified open-world generalization
 ↓
T14 Verified recursive capability improvement
 ↓
T15 Verified economic/material self-sustainability`

No layer automatically proves the next.

Each transition requires independent evidence.

---

# 11. Cryptographic Laws

## Law 1 — No Unbounded Authority

No autonomous component may receive authority broader than necessary for the declared operation.

---

## Law 2 — No Provider-Only Truth

A provider's success response is never sufficient evidence of a consequential effect.

---

## Law 3 — No Unverified State Mutation

A consequential state change must produce independently verifiable evidence before entering trusted state.

---

## Law 4 — Canonicalize Before Hashing

All cryptographically committed structured data must pass through deterministic canonicalization.

---

## Law 5 — No Floating-Point Consensus State

Floating-point values must not enter the deterministic canonical evidence boundary unless and until a formally specified canonical floating-point representation is adopted.

Preferred representations are explicit integers and exact units.

---

## Law 6 — No Candidate-Controlled Acceptance

The candidate system must not control the acceptance criteria for claims about its own capability.

---

## Law 7 — Evidence Is Versioned

A capability claim must retain enough provenance to determine:

`what was tested
which version produced it
which evidence supported it
which evaluator accepted it
which dependencies were involved`

---

## Law 8 — Failure Is First-Class State

Failure, uncertainty and contradictory evidence must be represented explicitly.

They must not be silently coerced into success.

---

## Law 9 — Recovery Is a Barrier

Once execution becomes indeterminate, normal effect admission is blocked until reconciliation establishes authoritative state.

---

## Law 10 — Escalation Requires Proof

The existence of a lower-level capability does not automatically authorize a higher-level capability.

---

# 12. Epistemic Laws

ACE distinguishes the following categories:

`Observation
    ↓
Measurement
    ↓
Evidence
    ↓
Verification
    ↓
Knowledge
    ↓
Capability`

No arrow may be skipped merely because the result appears plausible.

The system therefore rejects:

`plausibility → truth
confidence → truth
simulation → reality
successful command → successful effect
historical performance → future capability
self-report → independent evidence`

---

# 13. The ASI Proof Obligation

ACE does not define ASI as a single benchmark score.

The target property is a conjunction.

A serious claim of open-world autonomous general intelligence requires evidence that:

\[
A_{n+1} > A_n
\]

while:

\[
R_{n+1} \not\propto A_{n+1}
\]

and:

\[
G_{n+1} > G_n
\]

where:

- \(A\) = independently measured competence
- \(R\) = resources required
- \(G\) = generalization across genuinely novel task distributions

and simultaneously:

\[
W_{n+1} > W_n
\]

where \(W\) measures independently verified real-world reachable capability.

The system must demonstrate:

`novel-domain competence
+
cross-domain composition
+
recursive improvement
+
economic agency
+
physical agency
+
resource sustainability`

without evaluator capture and without proportional human engineering intervention.

---

# 14. What Would Constitute the Strongest Evidence?

The decisive experiment is not:

`"ACE solved benchmark X."`

It is closer to:

`ACE receives a previously unseen objective.

ACE determines what knowledge is missing.

ACE acquires or generates the necessary knowledge.

ACE designs a solution.

ACE tests the solution.

ACE detects failures.

ACE improves its own capability.

ACE obtains the required economic resources.

ACE provisions the necessary compute.

ACE performs physical actions when required.

Independent observers verify the consequences.

ACE uses the verified experience to perform better on
future unseen objectives.

The cycle repeats.`

The evaluator remains outside the system's authority.

Every consequential transition is independently observed.

Every durable claim is cryptographically auditable.

That is the actual frontier.

---

# 15. Final Doctrine

ACE therefore rejects the statement:

> **"The system is omnipotent."**

and replaces it with a stronger scientific discipline:

> **"Every increase in claimed capability must correspond to independently verified evidence of increased capability."**

The system is not defined by what it claims to be.

It is defined by what survives adversarial verification.

The ultimate target is:

> **Sustained, independently verified, open-world autonomous general intelligence with recursive capability improvement and closed-loop economic and physical agency.**

Until T11–T15 are independently demonstrated, they remain roadmap stages.

Until T13–T15 are demonstrated, the architecture must not be described as having achieved general intelligence, self-improvement in the strong sense, or self-sustainability.

Until an independent evaluation establishes otherwise, the correct epistemic state remains:

`implemented
verified
unproven`

That distinction is not a limitation of ACE.

It is the core feature that makes ACE scientifically meaningful.

---

# 16. Current Sealed State

## Completed Baselines

### T8

Kernel-level observability and governance sealed.

### T9

Adversarial proving-ground framework sealed.

`bf7b7cf53613842f470b0538577283cfb7b12e0a`

### T10

Two-channel verified infrastructure authority and autonomous resource reclamation sealed.

`26809d51546a92107fa8a0f38f10f7df7e0abe74`

PR #22 was merged into:

`ace-full-system-20260911`

with the T10 merge commit as the current baseline.

---

# 17. Forward Boundary

The next phase is not another abstract architecture rewrite.

The next proof burden is:

`T11
Verified Economic Agency`

with particular attention to:

`legal identity
delegated authority
treasury boundaries
legitimate revenue
counterparty verification
billing verification
resource acquisition
financial reconciliation`

Only after those mechanisms have an independently verified implementation should T12 physical agency begin.

The project therefore proceeds by proof, not by narrative.

---

# 18. Closing Principle

> **ACE does not ask to be believed. ACE asks to be verified.**

Every higher claim must inherit the burden of proof from the layer beneath it.

No simulation is silently upgraded to reality.

No capability is upgraded to intelligence without generalization evidence.

No intelligence claim is upgraded to autonomy without real-world consequence.

No autonomy claim is upgraded to self-sustainability without an independently measured closed loop.

And no finite engineering system is described as omnipotent merely because its current boundary has not yet been found.

**The frontier is not where the system says it can go.**

**The frontier is where independently verified capability ends.**
