# PHASE 2.1 PREREGISTERED SCIENTIFIC PROTOCOL — FINAL FREEZE

**Mode:** Protocol frozen for execution only after independent final audit. This document authorizes no execution by itself.

**Repository:** `Infrasigma/subsume-proving-ground`
**Branch:** `phase2-structural-transfer-20260910`

## 0. Historical boundary and scientific scope

Phase 1 remains the verified baseline at commit `5faebd91cf16ffd7b932ec982c6396c57df138a0`.

Historical Phase 2 remains frozen at tip `ebe3b99f0a1b6abc85b7c0121229a33d0542ddc7` with verdict `FAIL`. Its artifacts are not altered by Phase 2.1.

Phase 2.1 is a new test of structural transfer. It does not reinterpret Phase 2's FAIL as evidence for Phase 2.1, and it does not claim AGI, general intelligence, semantic understanding, or general representation learning.

This protocol freezes the scientific question, environment families, learner information boundary, acquisition definition, transfer attribution, controls, endpoint, statistics, and execution rules before empirical observation.

No executable Phase 2.1 experiment, generated corpus, or empirical result is part of this protocol.

---

# 1. Scientific invariants

## 1.1 Primary hypothesis

The immutable operational hypothesis is:

\[
\boxed{C(B^+\mid K_A)<C(B^+\mid K_0)}
\]

where `C` is the frozen environment interaction cost `E`, `K_A` is A-derived relational knowledge that passes the acquisition gate, `K_0` is the matched no-A-knowledge condition, and `B+` is Family B after the preregistered structural-novelty gate.

The hypothesis is tested only for the frozen learner implementation and frozen synthetic task families defined here.

## 1.2 Primary endpoint

For paired B task `i`:

\[
D_i=E_{K_0,i}-E_{K_A,i}.
\]

Positive `D_i` favors the hypothesis. Negative `D_i` opposes it.

The primary aggregate is the mean paired difference.

## 1.3 Frozen novelty threshold

The B-family novelty gate is unchanged:

`mean structural novelty > 0.50`.

The strict comparison is greater-than, not greater-than-or-equal-to.

## 1.4 Frozen statistical parameters

- Primary test: paired two-sided sign-flip permutation test.
- Number of permutations: `100000`.
- Permutation seed: `20260911`.
- Bootstrap: deterministic percentile bootstrap.
- Number of bootstrap resamples: `20000`.
- Bootstrap seed: `20260912`.

No parameter above may be changed after observation of any experimental outcome.

## 1.5 Frozen controls

The required conditions remain:

- `K_A`
- `K_0`
- `K_R`
- `K_S`
- `K_P`
- direct A-trajectory replay control

No control may receive information outside its defined boundary.

## 1.6 Nine PASS criteria

The original nine PASS criteria are unchanged and all are required:

1. mean B structural novelty is strictly greater than `0.50`;
2. `K_A` has lower mean interaction cost than `K_0`;
3. the primary two-sided sign-flip permutation test has `p < 0.05`;
4. positive-transfer rate is at least `70%`;
5. negative-transfer rate is at most `20%`;
6. `K_A` final success rate is at least the `K_0` final success rate;
7. retrieved-and-verified transfer events are at least `80%` of eligible transfer-attribution opportunities;
8. direct A-trajectory replay success rate is at most `10%`;
9. `K_R` does not reproduce the `K_A` primary effect under the predefined comparison.

A statistically significant result in the wrong direction does not satisfy PASS.

---

# 2. Deterministic randomness

All randomized generation and statistical randomization use SHA-256 counter streams. No mutable global RNG is permitted.

## 2.1 Byte encoding

All stream inputs are UTF-8 strings except task seeds, counters, and integer indices, which are encoded as unsigned 64-bit big-endian integers. Fields are concatenated with a single zero byte separator.

For stream name `S`, task seed `T`, condition identifier `C`, purpose `P`, and counter `n`, the digest input is:

`UTF8(S) || 0x00 || uint64_be(T) || 0x00 || UTF8(C) || 0x00 || UTF8(P) || 0x00 || uint64_be(n)`

and the stream value is:

`SHA256(input)`.

## 2.2 Stream namespaces

The only permitted namespaces are:

`TASK_GENERATION`
`LABEL_PERMUTATION`
`A_LEARNER`
`B_LEARNER_KA`
`B_LEARNER_K0`
`B_LEARNER_KR`
`B_LEARNER_KS`
`B_LEARNER_KP`
`SEARCH_KA`
`SEARCH_K0`
`SEARCH_KR`
`SEARCH_KS`
`SEARCH_KP`
`PERMUTATION`
`BOOTSTRAP`

No stream may be reused for a different scientific purpose.

## 2.3 Uniform integer and rejection sampling

To draw an integer uniformly from `[0,n)`, `n>0`, interpret the first eight digest bytes as unsigned integer `r`. Define `L=floor(2^64/n)*n`. If `r>=L`, increment the stream counter and redraw. Otherwise return `r mod n`.

Every Fisher-Yates permutation uses this rejection-sampled integer rule, iterating from the final position down to the second position.

## 2.4 Fixed seed manifests

Family A uses task seeds `0..11` inclusive.
Family B uses task seeds `0..99` inclusive.

These are task-generation seeds, not learner-visible values.

No seed may be replaced, reordered, or regenerated because of a result.

---

# 3. Learner-visible interface

The learner-visible interface is completely specified below. Internal data structures are unconstrained only if they are observationally equivalent to this interface.

## 3.1 ActionToken

`ActionToken` is an ASCII/UTF-8 string of exactly 16 lowercase hexadecimal characters.

Alphabet:

`0123456789abcdef`

There are exactly 16 characters, no prefix, suffix, delimiter, whitespace, sign, or numeric interpretation.

For each task, semantic actions are first enumerated in a hidden canonical action table. A task-local opaque token pool is then generated independently of semantic role from the `LABEL_PERMUTATION` stream. A Fisher-Yates permutation assigns the generated tokens to the semantic action records. The token-generation input contains task seed and token-pool position only; it does not contain semantic role, source, destination, dependency identity, X/Y/Z identity, or goal status.

State/location tokens are generated by the same role-independent token-pool method from the same `LABEL_PERMUTATION` namespace but a distinct purpose string. Location-token assignment is an independent task-local permutation.

The learner never receives the hidden semantic-to-token mapping.

Different tasks receive independent mappings. Equality of token strings across tasks has no semantic significance.

## 3.2 Observation object

Every learner observation is exactly this five-key object:

```json
{
  "state": <StateObject>,
  "available_actions": [<ActionToken>, ...],
  "last_action": <ActionToken or null>,
  "last_result": <ResultObject or null>,
  "terminated": <boolean>
}
```

No sixth key and no nested additional key is permitted.

Initial observation:

- `last_action = null`
- `last_result = null`
- `terminated = false`

After an action, the observation reports the resulting observable state, resulting available action set, the token submitted for that action, its result object, and the termination Boolean.

The learner may maintain its own history outside the observation object.

## 3.3 State object

Every state object is exactly:

```json
{"location":"<LocationToken>"}
```

`location` is exactly one 16-character lowercase hexadecimal opaque token.

No coordinates, node indices, edge lists, latent variables, dependency IDs, seed values, task IDs, source/target fields, goal distance, path information, counters, timestamps, or metadata are learner-visible.

## 3.4 Result object

Every action result is exactly:

```json
{"status":"<ResultStatus>"}
```

`ResultStatus` is exactly one of:

- `BLOCKED`
- `ACCEPTED`
- `SUCCESS`
- `ILLEGAL_ACTION`
- `ENVIRONMENT_ERROR`

`BLOCKED` means the submitted token is a valid action for the task but that action is currently unavailable because its transition precondition is false. It produces no environment-state transition.

`ACCEPTED` means the submitted valid action executed and did not reach the goal.

`SUCCESS` means the submitted valid action executed and reached the goal.

`ILLEGAL_ACTION` means the submitted token is not an action token in the current action interface. No alias or alternative spelling is accepted.

`ENVIRONMENT_ERROR` means an environment-level failure defined by the execution implementation contract; it carries no diagnostic payload and invalidates the affected experimental unit rather than becoming scientific evidence.

No free-form error string or additional result field is permitted.

## 3.5 Termination

Termination is represented only by the Boolean `terminated` field.

`terminated=true` iff the preceding learner-issued action reached the task goal.

There is no learner-visible termination reason, goal identifier, distance, remaining-step count, or route metadata.

## 3.6 Canonical serialization

Learner-visible objects are serialized as UTF-8 canonical JSON with these exact rules:

1. object keys are sorted lexicographically by their ASCII byte values;
2. no insignificant whitespace is emitted;
3. separators are exactly `,` and `:`;
4. JSON strings use standard JSON escaping;
5. booleans are `true` or `false`;
6. null is `null`;
7. no numeric value occurs in any learner-visible object;
8. arrays retain semantic order; `available_actions` has its separate mandatory ordering rule below;
9. no object may contain an undeclared key;
10. no implementation-specific serialization is exposed;
11. no timestamps, process identifiers, addresses, locale data, memory identifiers, or platform fields are exposed.

Because all permitted keys and token strings are ASCII and numeric values are prohibited, the ordering and encoding above uniquely determine the learner-visible byte representation.

## 3.7 Action ordering

`available_actions` is sorted strictly by lexicographic ordering of the ASCII bytes of the opaque token.

It is never ordered by semantic action type, source state, destination state, dependency relation, creation order, generator order, or any other hidden property.

No action token has semantic magnitude.

## 3.8 Deterministic external interface

For a fixed task, fixed hidden environment state, fixed submitted token, and fixed prior action history, the interface returns exactly one result, one observable resulting state, one available-action array, and one termination Boolean.

The hidden environment state is itself a deterministic function of the frozen task definition and complete prior learner-issued action history. No hidden nondeterminism may alter it.

The interface may not depend on dictionary iteration order, object identity, process ID, memory address, timestamp, locale, thread scheduling, filesystem ordering, inherited environment variables, or unrelated random state.

---

# 4. Auditor-only snapshot/fork and X-probe

## 4.1 Snapshot/fork

An auditor-only immutable environment snapshot may be taken immediately before an intervention comparison. The snapshot contains the exact transition-relevant environment state, including latent enablement variables, but is inaccessible to the learner.

Two independent environment replicas may be created from the same snapshot for the Y and Z intervention arms. Snapshot creation, restoration, and fork operations are instrumentation operations and are not learner actions and are never exposed through the learner interface.

The learner cannot request, observe, or invoke snapshot, fork, reset, rewind, or restore operations.

## 4.2 X selection

For each qualifying dependency relation, the harness selects semantic X from the hidden task construction table. X is never selected by token appearance, learner performance, or outcome.

The harness resolves X's current task-local opaque `ActionToken` using the hidden semantic-to-token mapping.

The learner is given no statement that the token represents X.

## 4.3 X-probe presentation

The harness-selected X probe is presented through the same ordinary action interface used for every other learner action. The learner sees only the ordinary observation and the opaque token supplied as the action to execute. It receives the same `ResultObject`, resulting `StateObject`, `available_actions`, and `terminated` fields as for every other action.

The harness selection is an experimental control and is not represented in the learner-visible data.

The X probe counts as one learner-issued environment interaction for `E`.

A blocked X probe produces `BLOCKED`, changes no environment state, and becomes part of the learner's ordinary action/result history. It does not cause a learner-visible reset or restore.

A later X execution after Y is also an ordinary action and counts toward `E`.

The learner does not receive the semantic names X, Y, or Z, the dependency index, the hidden enablement variable, the causal hypothesis, or any harness metadata.

---

# 5. Family A: complete deterministic state machine

## 5.1 Task count and nodes

Family A contains exactly 12 tasks, with seeds `0..11`.

Every A task has exactly eight semantic locations:

`0,1,2,3,4,5,6,7`.

Start location is `0`.
Goal location is `7`.

The canonical base edges are exactly:

`0->1, 1->2, 2->3, 3->4, 4->5, 5->6, 6->7`.

## 5.2 Dependency relations

Every A task has exactly three dependency relations, indexed internally as `r=0,1,2`.

Three distinct source locations are sampled without replacement from:

`{0,1,2,3,4}`

using the `TASK_GENERATION` stream and the rejection-sampled Fisher-Yates procedure.

Let the resulting ordered source list be `s_0,s_1,s_2`.

For each relation `r`:

`X_r` is the semantic transition `s_r -> s_r+2`.

`X_r` is initially blocked because latent variable `e_r=0`.

The dependency edge does not replace a base edge; it is an additional action from the same source.

## 5.3 Y and Z

Each relation has exactly two intervention actions at source `s_r`:

- `Y_r`: enabling intervention;
- `Z_r`: non-enabling contrast.

Both are zero-displacement actions: executing either leaves the location unchanged.

`Y_r` changes exactly one hidden variable:

`e_r: 0 -> 1`.

It changes no other `e` variable and does not alter the observable location.

`Z_r` changes no `e` variable and does not alter the observable location.

No Y or Z action has any other environment effect.

## 5.4 X semantics

Before `Y_r`, `X_r` is unavailable. A probe of `X_r` returns `BLOCKED` and leaves both location and all `e` variables unchanged.

After `Y_r`, `X_r` becomes available while the location remains `s_r`.

Executing `X_r` then moves the environment from `s_r` to `s_r+2` and returns `ACCEPTED`, unless that action reaches goal, in which case it returns `SUCCESS`.

The availability of `X_r` depends only on `e_r`.

For every `q != r`, neither `Y_q` nor `Z_q` changes `e_r`, and `X_r` does not depend on `e_q`.

Thus cross-relation independence is exact:

`Y_r` affects only `e_r`; `Z_r` affects no enablement variable; `X_r` tests only `e_r`.

## 5.5 Action sets

At each location, the available semantic actions are exactly the outgoing base-edge actions plus dependency actions whose source equals the current location and whose availability condition is satisfied.

For every relation source `s_r`, the semantic action set contains:

- the base edge `s_r -> s_r+1`;
- `Y_r`;
- `Z_r`;
- `X_r` only when `e_r=1`.

When `e_r=0`, `X_r` is a valid but unavailable action: it may be submitted and returns `BLOCKED`, but it is not included in `available_actions`.

At other locations, no relation-specific action is exposed.

All task-local semantic actions receive opaque tokens through the frozen label-permutation procedure.

## 5.6 A causal intervention requirement

For every dependency relation used for abstraction acquisition, the auditor must establish:

1. an X-blocked state exists at location `s_r` with `e_r=0`;
2. the Y arm executes `Y_r`, leaves location at `s_r`, and changes only `e_r` to `1`;
3. after Y, `X_r` is available and succeeds when executed from the same source location;
4. the Z arm starts from the same pre-intervention state, executes `Z_r`, leaves `e_r=0`, and leaves `X_r` unavailable;
5. the learner-visible pre-intervention context is byte-identical between Y and Z arms;
6. the auditor-visible hidden-state difference between arms is exactly the prescribed Y-versus-Z intervention difference.

The same source location is therefore used for the subsequent X test; X is not tested after movement to another location.

## 5.7 A label permutation

For each task, the complete semantic action set and complete semantic location set are mapped to independently generated opaque tokens by task-local permutations. Token generation is independent of semantic role.

No semantic role may be encoded in token length, prefix, suffix, character class, ordering, or serialization.

## 5.8 A validity and rejection behavior

A generated A task is valid only if all of the following hold:

- exactly eight locations exist;
- start is `0` and goal is `7`;
- exactly the seven base edges exist;
- exactly three distinct dependency sources are selected from `{0,1,2,3,4}`;
- each X edge is exactly `s_r -> s_r+2`;
- each relation has exactly one Y and one Z;
- Y and Z are zero-displacement;
- initial `e=(0,0,0)`;
- each Y changes only its own `e_r`;
- each Z changes no `e` variable;
- each X depends only on its own `e_r`;
- X is blocked before Y;
- Y makes X executable from the same source;
- Z leaves X blocked;
- the learner-visible pre-intervention context of Y and Z is identical;
- all semantic actions and locations have valid task-local opaque tokens.

If any predicate fails for a seed, the task is `INVALID_TASK_GENERATION`. The seed is not replaced because of the failure and no outcome-based regeneration is permitted.

---

# 6. A intervention comparability contract

## 6.1 Learner-visible canonical context

Immediately before intervention, define `C_X` as the canonical byte serialization of the learner-visible observation restricted to:

- current `state`;
- complete `available_actions` array;
- target-role descriptor used by the auditor only to identify X, not exposed to the learner;
- no hidden fields.

For the learner-visible comparison, the serialized `state` and `available_actions` fields must be byte-identical between Y and Z arms. `last_action`, `last_result`, and `terminated` are also identical because the arms are forked before intervention and begin from the same pre-intervention observation.

## 6.2 Auditor-only context

The auditor may additionally inspect hidden location and `e` values. These fields never enter the learner observation.

Before intervention, the hidden states of the Y and Z replicas are identical.

After intervention:

- Y changes exactly `e_r:0->1` and no other hidden variable;
- Z changes no hidden variable;
- both retain the same location;
- subsequent X succeeds in the Y arm and remains blocked in the Z arm.

## 6.3 Observation window

The causal observation window is:

`pre-X observation -> X probe -> post-X observation -> intervention -> post-intervention observation -> X execution -> outcome`.

For the negative contrast:

`pre-X observation -> X probe -> post-X observation -> Z -> post-Z observation -> X probe -> outcome`.

For the positive intervention:

`pre-X observation -> X probe -> post-X observation -> Y -> post-Y observation -> X execution -> outcome`.

The X probe is an ordinary learner-visible action and counts toward E. Auditor fork creation is not a learner action.

---

# 7. K_pre and genuine A acquisition

## 7.1 K_pre

`K_pre` is the complete learner knowledge store immediately before the first A observation for Phase 2.1.

It must be serialized using the same canonical JSON rules applicable to knowledge records and cryptographically hashed before A interaction begins.

The serialized snapshot, hash, and acquisition-time ordering metadata are immutable audit artifacts.

## 7.2 Operational acquisition claim

Phase 2.1 does not claim metaphysical proof that an internal model “really learned” a causal law. It makes an operational provenance claim: a qualifying relational record was absent from the pre-A knowledge state, was constructed from permitted A evidence, and survived the mechanical admissibility and provenance checks.

## 7.3 Material-equivalence test

Material equivalence between `K_pre` and a candidate post-A rule is tested syntactically after canonical normalization under the K_A grammar in Section 8.

The test is not a claim of metaphysical semantic equivalence.

A candidate already materially present in `K_pre` is not A-acquired.

## 7.4 Delta

Define:

`Delta_K_A = K_A \ K_pre`

under canonical rule identity.

Only records in `Delta_K_A` may be attributed as A-acquired knowledge.

## 7.5 Acquisition classes

Each post-A candidate belongs to exactly one class:

**A — generic machinery:** general learner capability or search procedure that existed before A and did not become a new relational record. Not acquisition.

**B — target-equivalent pre-A content:** a rule materially equivalent to content already present in K_pre. Not acquisition; if it is used to satisfy G2, G2 fails.

**C — A-derived relational content:** a new admissible relational rule whose provenance points to qualifying A evidence and whose normalized identity is absent from K_pre. This can qualify for acquisition.

**D — confidence/usage metadata:** evidence counts, usage frequency, or confidence bookkeeping without a new admissible relational rule. Not acquisition.

## 7.6 Provenance

Every candidate C record contains:

- unique abstraction ID;
- source A task IDs;
- source A observation IDs;
- rule in canonical K_A normal form;
- evidence hash;
- K_pre hash;
- post-A knowledge hash;
- deterministic creation index;
- creation event references;
- cross-instance support count;
- freeze marker.

Every cited observation must occur before the candidate creation event.

## 7.7 Acquisition gate

The mechanical gate is:

`G_A = nonempty AND provenance-valid AND cross-instance-supported AND relational/non-episodic AND label-invariant AND B-independent AND independently-inspectable AND frozen-before-B`

A qualifying rule must have evidence from at least two independently generated and independently relabeled A tasks.

Generic pre-A machinery does not satisfy the gate merely because it is nonempty or because A observations increase its confidence.

The A-derived rule store is frozen before any B exposure.

Failure of any predicate is a G2 acquisition validity failure and cancels transfer interpretation.

---

# 8. Closed K_A grammar

The admissibility language is intentionally finite and mechanically decidable.

## 8.1 Allowed atomic predicates

Only these atomic predicates exist:

`BLOCKED(x)`
`AVAILABLE(x)`
`ACTION(x)`
`INTERVENES(y,x)`
`BEFORE(a,b)`
`AFTER(a,b)`
`OBSERVED_EFFECT(a,e)`
`ENABLES(y,x)`
`NONENABLES(z,x)`
`SAME_LOCAL_CONTEXT(a,b)`

The only permitted structural equality is equality of bound role variables.

No other predicate is permitted. In particular, there is no open-ended category called “other structural predicates.”

## 8.2 Constructors

Allowed constructors are:

- bounded `EXISTS`;
- bounded `FORALL`;
- conjunction `AND`;
- implication `->`;
- role-variable binding.

No disjunction, negation, arithmetic, recursion, arbitrary function call, executable code, embedding, hash lookup, or unbounded quantifier is permitted.

## 8.3 Canonical role vocabulary

Role variables are restricted to the roles:

`target`, `intervention`, `contrast`, `context`.

They are alpha-renamed to canonical order.

The reusable rule may express the following structural relation:

`BLOCKED(target) AND INTERVENES(intervention,target) AND OBSERVED_EFFECT(intervention,ENABLES(target)) AND BEFORE(intervention,target) AND NONENABLES(contrast,target)`

with the exact permitted predicate forms above.

The grammar does not permit concrete state IDs, concrete action labels, seeds, coordinates, task IDs, trajectory literals, B identifiers, hidden-state predicates, or answer keys.

## 8.4 Forbidden representations

The validator rejects any candidate containing:

- task-specific lookup tables;
- concrete state/action identifiers as semantic roles;
- literal A trajectories;
- hard-coded constructors for B solutions;
- hidden-state predicates;
- generator coordinates;
- seeds as rule content;
- B identifiers or metadata;
- executable programs;
- arbitrary strings interpreted as rules;
- embeddings or opaque learned vectors as rule identity;
- any undeclared predicate or constructor.

Thus adversarial candidates such as “blocked -> search,” “choose the third action,” concrete trajectory replay, or hidden-state rules are rejected unless they can be expressed entirely in the closed admissibility language, in which case only their canonical admissible form is retained.

## 8.5 Canonical normal form

Normalization performs, in order:

1. alpha-renaming of role variables to canonical names;
2. canonical ordering of bounded quantifiers;
3. sorting of conjunction operands by canonical serialized form;
4. duplicate-conjunct removal;
5. canonical implication direction;
6. canonical predicate argument ordering;
7. UTF-8 JSON serialization with sorted ASCII keys and no whitespace.

Two records with identical normalized rule serialization have the same rule identity.

## 8.6 Validator

The validator is deterministic and binary:

`ADMISSIBLE`
or
`REJECTED`.

No human semantic judgment is permitted after validation to turn a rejected record into an admissible one.

---

# 9. Family B: complete deterministic generator

## 9.1 Task count

Family B contains exactly 100 tasks, with seeds `0..99`.

## 9.2 Fixed topology

Every B task uses exactly ten semantic nodes:

`0,1,2,3,4,5,6,7,8,9`.

Start is `0`.
Goal is `9`.

The complete directed edge set is exactly:

`0->1`
`1->2`
`2->3`
`2->4`
`3->5`
`5->6`
`6->7`
`4->6`
`7->8`
`6->8`
`8->9`
`9->8`

The `8<->9` pair is an explicit directed two-node lollipop cycle. The `0-1-2` stem branches at `2`, the branches merge at `6`, and the merged path enters the `8<->9` lollipop cycle. No statement in this protocol characterizes the B graph as cycle-free.

## 9.3 Dependency candidate list

The candidate dependency edges are exactly:

`(0,1),(1,2),(2,3),(2,4),(3,5),(5,6),(4,6),(6,7),(7,8),(6,8),(8,9)`

The edge `9->8` is not a dependency candidate and remains the cycle-return edge.

## 9.4 Dependency count

For even B task seed, exactly three dependency relations are selected.

For odd B task seed, exactly four dependency relations are selected.

Selection uses `TASK_GENERATION` with the fixed seed and deterministic rejection-sampled sampling without replacement.

No dependency is selected twice.

The generator rejects a candidate selection if it violates any of the following:

- duplicate dependency;
- use of `9->8` as a dependency;
- loss of reachability of goal `9` through the ordinary transition system;
- absence of a valid Y/Z intervention contrast;
- violation of the exact causal semantics below.

The generator advances the same deterministic stream and retries. At most `1,000,000` candidate counters are permitted for one task. If no valid task is obtained by counter `999999`, the task is `INVALID_TASK` and is not replaced because of experimental results.

## 9.5 B causal semantics

For every selected dependency edge `u->v`, create one hidden enablement variable `e_r` initially `0` and define:

`X_r = u->v`.

Create `Y_r` and `Z_r` as zero-displacement actions at the source `u`.

`Y_r` changes only `e_r:0->1`.

`Z_r` changes no enablement variable.

`X_r` is unavailable while `e_r=0` and available only when `e_r=1`.

After Y, X is executable from the same source state. After Z, X remains unavailable from that same source state.

For distinct relations `r != q`, there are no cross-effects between `e_r` and `e_q`.

## 9.6 B labels

Location and action labels are independently task-local and opaque under the same token rules as Family A.

No B token is derived from semantic role.

## 9.7 B validity

A B task is valid only if:

- the exact ten-node graph above is present;
- start and goal are correct;
- the required number of distinct dependency candidates is selected;
- all selected dependencies have exact X/Y/Z semantics;
- the `8<->9` cycle is preserved;
- goal remains reachable;
- each dependency has a positive intervention and negative contrast;
- learner-visible Y/Z pre-intervention contexts are identical;
- all labels satisfy the opaque-token contract.

Failure produces `INVALID_TASK`. No outcome-based task replacement is permitted.

---

# 10. Structural novelty algorithm

This is explicitly the **NEW-FROZEN-PHASE2.1 OPERATIONALIZATION** of the existing Phase 2 novelty concept. The historical Phase 2 implementation does not establish an implementation-identical byte-level novelty procedure, so Phase 2.1 does not falsely claim such identity.

The scientific novelty concept is unchanged: unlabeled degree profile, reachable-distance profile, and dependency-placement structure combined by multiset Jaccard.

## 10.1 Degree feature multiset

For every node `v`, add one feature:

`DEGREE(in_degree(v), out_degree(v))`.

The collection is a multiset; duplicate tuples retain multiplicity.

## 10.2 Reachable-distance feature multiset

For every ordered pair `(u,v)` with `u != v` for which a directed path exists, compute the exact shortest-path distance `d(u,v)` in the directed graph and add:

`DISTANCE(d(u,v))`.

Unreachable ordered pairs contribute no feature. Infinite distance is therefore represented by omission, not by a numeric sentinel.

## 10.3 Dependency-placement feature multiset

For each dependency edge `u->v`, add exactly:

`DEPENDENCY(in(u),out(u),in(v),out(v),d(start,u),d(start,v),d(u,goal),d(v,goal))`.

A task is invalid if any distance required by this feature is unreachable.

## 10.4 Combined multiset

The task feature multiset `M` is the multiset union of the three feature multisets above. Feature-type prefixes are part of feature identity, so a degree tuple cannot match a distance tuple.

## 10.5 Multiset Jaccard

For multisets `M1` and `M2`, let `count_M(x)` be multiplicity of feature `x`.

Intersection:

`I(M1,M2)=sum_x min(count_M1(x),count_M2(x))`.

Union:

`U(M1,M2)=sum_x max(count_M1(x),count_M2(x))`.

If `U=0`, Jaccard is defined as `1.0`.

Otherwise:

`J(M1,M2)=I/U`.

## 10.6 Task novelty

For B task `b`:

`novelty(b)=1-max_a J(M_b,M_a)`

where the reference set is all 12 generated A tasks.

No canonical graph labeling or graph-isomorphism solver is required. All features are graph-invariant multisets.

## 10.7 Aggregate novelty

The B novelty aggregate is the arithmetic mean of the 100 unrounded task novelty values.

No per-task rounding occurs before aggregation.

The gate passes iff the exact aggregate is strictly greater than `0.50`.

No duplicate handling beyond ordinary multiset multiplicity is permitted.

---

# 11. B novelty gate

The frozen order is:

`generate B -> calculate novelty -> evaluate threshold -> freeze B -> permit transfer exposure`.

If mean novelty is `<=0.50`, G3 fails and transfer interpretation is cancelled.

No B task may be removed, regenerated, substituted, or reordered after novelty inspection to obtain a passing mean.

Before transfer exposure, the following must be frozen:

- generator source/hash;
- seed manifest/hash;
- B task-generation record/corpus hash;
- novelty implementation/version;
- per-task novelty values;
- aggregate novelty;
- A/K_A freeze artifact;
- control configuration;
- leakage audit;
- B immutable freeze marker.

---

# 12. Retrieval, applicability, and prediction

## 12.1 Retrieval candidate set

During B, the learner-visible current observation is matched against all frozen K_A abstractions using their canonical applicability conditions.

The candidate set contains every abstraction whose applicability predicates are satisfied by the current learner-visible relational context.

No hidden B metadata is used for matching.

If the candidate set is empty, no A abstraction is retrieved.

If exactly one candidate exists, it is retrieved.

If multiple candidates exist, they are ordered by:

1. greatest number of satisfied canonical applicability predicates;
2. canonical rule serialization as the deterministic tie-break.

If multiple maximally specific candidates remain, the retrieval event is `CONFLICT` and no transfer may be attributed until the conflict is resolved before any B-only search.

## 12.2 Applicability

Applicability is evaluated only against the closed learner-visible relational language. It may not inspect latent `e` values, generator state, hidden semantic labels, task IDs, or future outcomes.

## 12.3 Prediction schema

A prediction event contains exactly:

- `abstraction_id`;
- canonical applicable context hash;
- normalized rule hash;
- predicted consequence type;
- event order.

The prediction must be created before the decisive intervention/action.

A prediction is falsifiable: it must specify the expected availability/outcome consequence of applying the relation.

---

# 13. Transfer attribution

## 13.1 Required chain

A transfer event requires, in order:

`retrieval -> application -> prediction -> intervention -> observed consequence -> verification -> attribution`.

Retrieval alone is not transfer.
Prediction alone is not transfer.
Correct outcome by coincidence is not transfer.

## 13.2 Decisive action

The decisive action is the first executed B action causally downstream of the retrieved abstraction and explicit prediction, as recorded by the frozen event-order trace.

If retrieval occurs after search or after the decisive action, the event cannot be `KA_TRANSFER`.

## 13.3 K0-equivalent attribution replay

For every claimed K_A transfer event, an auditor replays the same frozen B local context through the K0-equivalent decision procedure with A knowledge removed but all other solver/search configuration held fixed.

If K0 independently produces the same decisive action for the same reason available from B-only information, the event is not attributable to K_A and is classified `COINCIDENTAL` or `UNATTRIBUTABLE` according to the frozen precedence below.

The replay is performed from the identical frozen B task state and observation window; it does not alter the primary E values.

## 13.4 Attribution precedence

Exactly one terminal attribution classification is assigned using this precedence, highest first:

`PROTOCOL_VIOLATION > UNATTRIBUTABLE > CONFLICT > RETRIEVAL_ONLY > PREDICTION_ONLY > COINCIDENTAL > KA_TRANSFER`

`PROTOCOL_VIOLATION` means a required protocol condition was violated.

`UNATTRIBUTABLE` means the evidence cannot uniquely establish the claimed A-derived causal chain.

`CONFLICT` means unresolved maximally specific abstraction predictions remained.

`RETRIEVAL_ONLY` means an abstraction was retrieved but no qualifying prediction/intervention chain followed.

`PREDICTION_ONLY` means a prediction was produced but no qualifying decisive action and verified consequence followed.

`COINCIDENTAL` means the predicted consequence occurred but the K0-equivalent replay shows the same decisive action was independently available from B-only information.

`KA_TRANSFER` requires every step of the complete chain and successful attribution replay.

## 13.5 Adversarial cases

A. If search selects Y before retrieval, the event cannot be `KA_TRANSFER` and is classified by the precedence rules, normally `COINCIDENTAL` or `UNATTRIBUTABLE`.

B. If retrieval occurs first and the K_A prediction then causes the same action that K0 would independently select, the event is `COINCIDENTAL`, not transfer.

C. If two maximally specific abstractions disagree, the event is `CONFLICT` unless the conflict was deterministically resolved before any B-only search; unresolved conflict is never silently assigned to K_A.

D. If prediction is created after the decisive action, the event is not transfer and is classified `PREDICTION_ONLY` or `UNATTRIBUTABLE` according to available evidence.

E. If the correct consequence occurs without a causally attributable K_A action, the event is `COINCIDENTAL` or `UNATTRIBUTABLE`, never `KA_TRANSFER`.

---

# 14. Controls and information boundaries

## 14.1 K_A

Receives only frozen `Delta_K_A` records that pass G2. It may use those records during B. It receives no B-specific answer key or future B outcome.

## 14.2 K0

Receives no A-derived knowledge and no raw A episodes. It receives the same frozen B tasks, initial states, learner-visible interface, solver family, and permitted budgets as K_A.

## 14.3 KR

Retains raw A episodic records but is forbidden from performing cross-episode abstraction induction, aggregation, relational generalization, or rule construction during B.

KR may consult a single stored episode only as an episode. It may not synthesize a rule across episodes or convert episode content into a relational abstraction during B.

## 14.4 KS

Uses the same no-knowledge solver and fixed search/action budget as K_A. It receives no A-derived knowledge.

## 14.5 KP

Uses the same solver, task prior, randomization configuration, and fixed decision procedure as K_A but without A-derived knowledge.

## 14.6 Direct replay

Uses literal A action sequences under the predefined replay procedure and cannot transform those sequences into relational rules.

## 14.7 Incidental state

Any post-A state that could affect B behavior must be classified before B as one of:

1. explicitly part of the frozen K_A condition;
2. reset/removed from the learner state;
3. undeclared incidental state, which invalidates the experiment.

No hidden incidental state may silently benefit K_A.

---

# 15. Process, cache, filesystem, and environment isolation

Each experimental condition starts in a fresh process state with:

- fresh condition-specific RNG streams;
- fresh learner state appropriate to that condition;
- fresh search state;
- fresh cache state;
- isolated temporary filesystem namespace;
- isolated environment variables;
- no inherited task corpus or result cache;
- no access to another condition's process memory;
- no shared mutable global RNG.

The immutable B task definition and initial state are the only shared scientific inputs between paired conditions.

No condition may inspect another condition's logs, timing, memory, filesystem, cache, or result before completing its own decision.

Deterministic ordering is required wherever multiple internal candidates exist.

---

# 16. Pairing

There are exactly 100 paired primary observations.

For every `i`:

`pair_i = (B_i, K_A ; B_i, K_0)`.

The two conditions use the exact same frozen B task instance, exact same initial environment state, and exact same learner-visible observation at the start of the pair.

Condition-specific learner/search randomness uses independent condition streams:

`B_LEARNER_KA`, `B_LEARNER_K0`, `SEARCH_KA`, and `SEARCH_K0`.

The B task itself is not regenerated between conditions.

Each condition produces exactly one valid primary `E` value. If either condition is scientifically invalid, the pair is invalid rather than silently excluded.

---

# 17. Terminal outcomes and E

## 17.1 Terminal enum

Every execution terminates in exactly one of:

- `SUCCESS`
- `INTERACTION_CAP_EXHAUSTED`
- `DECISION_CUTOFF_EXHAUSTED`
- `ILLEGAL_ACTION`
- `ENVIRONMENT_ERROR`
- `PROTOCOL_VIOLATION`
- `INVALID_TASK`
- `INSTRUMENTATION_FAILURE`

## 17.2 Interaction cost

`E` is the number of learner-issued action attempts through the goal-reaching action or through the terminal execution cutoff.

The interaction cap is exactly `200` learner-issued actions.

The implementation decision-budget cutoff is exactly `120` decision units.

A valid learner-issued action attempt counts toward E. A blocked action counts. An illegal action submission is not counted as a valid environment interaction and terminates the condition as `ILLEGAL_ACTION`.

If the goal is reached on the final permitted interaction, that interaction counts and the condition is `SUCCESS`.

If the interaction cap is exhausted without reaching the goal, the condition is `INTERACTION_CAP_EXHAUSTED` and `E=200`.

If the decision cutoff is exhausted after `e` learner-issued actions without success, the condition is `DECISION_CUTOFF_EXHAUSTED` and `E=e`.

## 17.3 Error precedence

Scientific invalidity takes precedence over scoring. The effective precedence is:

`PROTOCOL_VIOLATION / INSTRUMENTATION_FAILURE / INVALID_TASK / ENVIRONMENT_ERROR` -> invalidate;

otherwise `SUCCESS` -> score success;

otherwise `DECISION_CUTOFF_EXHAUSTED` -> score cutoff;

otherwise `INTERACTION_CAP_EXHAUSTED` -> score cap;

otherwise `ILLEGAL_ACTION` -> terminate as illegal and invalidate the condition's scientific comparison.

An implementation must never silently convert an infrastructure/protocol error into an E value.

No NaN, missing value, or selective exclusion is permitted.

---

# 18. Stopping and cancellation

The gate order is immutable:

`G0 -> G1 -> G2 -> G3 -> G4 -> G5 -> analysis`

`G0`: protocol/source integrity.

`G1`: A task-generation and causal-observability validity.

`G2`: K_A acquisition validity.

`G3`: B novelty validity.

`G4`: leakage/control/pairing validity.

`G5`: execution-completeness and terminal-outcome validity.

Only after G5 may primary statistical analysis occur.

Cancellation is mandatory if any gate fails in a way specified as invalidating the scientific comparison.

The protocol forbids:

- outcome-based stopping;
- post-outcome parameter changes;
- task replacement after novelty inspection;
- B regeneration after seeing performance;
- selective exclusion of unfavorable tasks;
- changing the novelty threshold;
- changing the statistical test;
- changing seeds;
- changing controls;
- changing the endpoint;
- relabeling a failed event as transfer after seeing its outcome.

A negative result is reported as a negative result. The experiment is not rescued by changing the architecture or generator after observation.

---

# 19. Statistics

The primary paired data are the 100 valid values:

`D_i = E_K0,i - E_KA,i`.

The primary hypothesis is tested by a two-sided paired sign-flip permutation test with exactly `100000` permutations and seed `20260911`.

The bootstrap uses exactly `20000` deterministic percentile resamples and seed `20260912`.

All randomization is performed after the primary data have been frozen and uses the `PERMUTATION` and `BOOTSTRAP` streams respectively.

No alternative test is substituted after observation.

---

# 20. Leakage invariants

The following are hard invariants:

- all action tokens have identical datatype, width, alphabet, and encoding;
- all location tokens have identical datatype, width, alphabet, and encoding;
- state contains exactly `location`;
- result contains exactly `status`;
- observation contains exactly the five specified keys;
- action ordering is token-lexicographic only;
- no token contains semantic role information by construction;
- no semantic X/Y/Z identity is learner-visible;
- no latent enablement variable is learner-visible;
- no task-generation metadata is learner-visible;
- no seed is learner-visible;
- no timestamps, PIDs, memory addresses, thread identifiers, locale strings, platform strings, cache identifiers, or generator counters are learner-visible;
- no future state is exposed;
- no answer key is exposed;
- no auditor snapshot/fork/reset/restore is learner-visible;
- no free-form diagnostic is learner-visible.

Any violation is `PROTOCOL_VIOLATION` and invalidates the affected scientific comparison.

---

# 21. Reproducibility artifact contract

A valid execution must preserve, without changing the frozen protocol:

1. protocol file hash;
2. learner implementation source and hash;
3. A generator source and hash;
4. B generator source and hash;
5. exact A seed manifest;
6. exact B seed manifest;
7. generated task/configuration hash or deterministic generation record;
8. K_pre serialization and hash;
9. K_A delta serialization and hash;
10. provenance/evidence records;
11. B novelty per-task values and aggregate;
12. novelty implementation/version identifier;
13. leakage audit record;
14. control configuration records;
15. condition RNG namespace/configuration records;
16. paired B task mapping;
17. terminal outcome and E records;
18. transfer-attribution event records;
19. permutation seed and results;
20. bootstrap seed and results;
21. environment/runtime information sufficient to reproduce the execution.

No artifact may be replaced after an outcome in order to improve reproducibility or statistical appearance.

---

# 22. Protocol-frozen vs implementation-frozen boundary

## 22.1 Protocol-frozen

The following are scientifically immutable:

- hypothesis;
- endpoint;
- A and B family definitions;
- causal relation;
- observation boundary;
- X-probe semantics;
- K_pre and K_A acquisition boundary;
- K_A grammar;
- novelty feature definitions and threshold;
- retrieval and attribution rules;
- controls;
- RNG and isolation rules;
- pairing;
- terminal outcomes;
- statistical procedure;
- seeds;
- PASS criteria;
- stopping and cancellation rules.

## 22.2 Implementation-frozen

Before execution, the concrete learner implementation must be frozen, including:

- learner architecture;
- internal representation;
- learning algorithm;
- memory implementation;
- search procedure;
- solver configuration;
- action-selection implementation;
- computation/decision budget;
- serialization implementation;
- RNG implementation;
- process isolation implementation;
- cache policy;
- observation-interface implementation.

Internal implementations may differ between independent implementations only if their external behavior satisfies the complete protocol. Scientific conclusions are scoped to the learner implementation actually executed.

---

# 23. Two-implementer equivalence test

Before execution, two competent independent implementers must be able to implement the protocol without making a scientific choice about:

- A generator;
- B generator;
- X probe;
- observation interface;
- state encoding;
- action encoding;
- intervention selection;
- observation window;
- K_pre;
- K_A grammar;
- retrieval;
- attribution;
- KR;
- controls;
- RNG;
- novelty;
- pairing;
- terminal outcomes;
- stopping.

They may use different programming languages or internal data structures, but if both implementations comply with this protocol, they must expose the same learner-visible information, construct the same deterministic task families from the same seeds, compute the same novelty values, apply the same causal intervention rules, use the same knowledge admissibility language, classify the same attribution cases, pair the same B instances, and apply the same terminal/scoring rules.

A remaining implementation disagreement is protocol-material only if it can change K_A acquisition, B novelty, transfer attribution, E_KA, E_K0, D, validity status, or interpretation of the hypothesis. Such a disagreement is prohibited.

---

# 24. PASS / FAIL interpretation

The following classifications are distinct:

### INVALID EXPERIMENT
A protocol, generation, leakage, pairing, instrumentation, or execution-integrity failure prevents the frozen comparison from being scientifically interpreted.

### VALID NEGATIVE RESULT
All required validity gates pass, but the empirical endpoint fails to support the hypothesis or supports the opposite direction.

### VALID POSITIVE RESULT
All required validity gates pass and all nine PASS criteria are satisfied.

A failure of Phase 2 does not count as a Phase 2.1 result.

A Phase 2.1 positive result would establish only the preregistered structural-transfer effect for the frozen learner and synthetic task families. It would not establish AGI or general intelligence.

---

# 25. Phase-3 advancement restriction

Phase 3 may begin only if:

1. all original Phase 2.1 PASS criteria are satisfied;
2. the Phase 2.1 execution is independently inspectable;
3. the complete reproducibility artifact contract is satisfied;
4. an independent reproduction using the same frozen protocol obtains the required positive result;
5. no post-outcome protocol change is used to obtain advancement.

A Phase 2.1 failure does not authorize a Phase-3 architecture rescue inside the same experiment.

---

# 26. Freeze statement

This document is the authoritative Phase 2.1 scientific protocol baseline.

The scientific hypothesis, primary endpoint, novelty threshold, statistical procedure, seeds, controls, PASS criteria, and historical Phase 1/Phase 2 boundary are frozen.

The detailed learner-interface, environment-generator, causal-intervention, acquisition, attribution, isolation, terminal-outcome, pairing, and reproducibility clauses above are deterministic implementation-closure clauses. They do not change the scientific hypothesis or the primary endpoint.

The novelty section is explicitly a **NEW-FROZEN-PHASE2.1 OPERATIONALIZATION** of the already frozen Phase 2 novelty concept; it is not represented as a historical implementation identity claim.

No empirical result is contained in this protocol.

**PHASE 2:** `FAIL` — historical frozen result.

**PHASE 2.1:** `PROTOCOL FROZEN — NO EMPIRICAL RESULT`.
