# ACE-X V7 Scientific Protocol

## Constraint

V7 remains model-free in the narrow engineering sense used by this project:
no LLM, no neural network, no learned world model, no cloud service is required.

The agent stores only verified experience:
(state fingerprint, action, observed next fingerprint, reward, terminal).
It never predicts an unobserved transition as fact.

## Target

V7 attacks interactive intelligence:
- exploration of unknown action spaces,
- goal discovery from environmental feedback,
- persistent experience,
- verified macro induction,
- planning only through observed/verified transitions,
- surprise-driven replanning,
- transfer through action-effect signatures rather than raw names.

## Frozen capabilities

V7-A  Identifier-independent relational observation.

V7-B  Novelty-weighted action exploration in unknown environments.

V7-C  Goal acquisition from observed reward/terminal signals without a task-family label.

V7-D  Verified experience graph construction.

V7-E  Planning through verified empirical transitions only.

V7-F  Macro induction from repeated verified trajectories.

V7-G  Failure detection and replanning after an unexpected transition.

V7-H  Action-role transfer through structural effect signatures under identifier renaming.

V7-I  Persistent experience reuse after raw episode deletion.

V7-J  Resource-normalized improvement in successor environments.

## Terminal acceptance

Three evaluator-owned interactive environments must each demonstrate:
1. exploration before successful planning;
2. independently discovered goal;
3. verified planning;
4. unexpected-transition detection;
5. replanning;
6. reusable macro extraction;
7. structural action-role transfer;
8. replay after raw episode deletion.

No evaluator label may reveal the goal, action semantics, or winning sequence.

## Kill rule

Failure of an interactive capability kills V7. No threshold or environment is changed
after scientific failure.

## Claim boundary

V7 PASS is evidence for model-free interactive cognitive competence only.
It is not an ASI claim. A subsequent gate must demonstrate open-ended tool invention,
mathematical/scientific reasoning, cross-domain abstraction, and mechanism-level
self-improvement on independently implemented environments.
