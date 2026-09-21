# ACE-X V9 Scientific Protocol — Counterexample-Guided Open Search

## Constraint

No LLM, neural network, learned world model, cloud service, or remote inference is required.

## Hypothesis

A model-free symbolic entity becomes substantially more general when synthesis is not
"find one program that fits current examples", but an iterative closed loop:

candidate -> independent verifier -> counterexample -> hypothesis refinement -> candidate.

The verifier is not allowed to rank candidates; it only returns a falsifying counterexample
or certifies the candidate within its declared bounded domain.

## Frozen capabilities

V9-A  Counterexample-guided inductive synthesis (CEGIS).

V9-B  Independent verifier produces concrete counterexamples from a bounded semantic domain.

V9-C  Candidate refinement preserves prior verified examples while incorporating new counterexamples.

V9-D  Verification certificate records the complete probe domain searched, all counterexamples,
       and the final no-counterexample result.

V9-E  CEGIS can bootstrap abstractions/tool candidates rather than only solve one task.

V9-F  Resource accounting includes synthesis, verification, refinement, and certificate work.

V9-G  Failed candidates become explicit failure memory and can change the next synthesis round.

## Kill rule

A candidate that passes current examples but fails an independent counterexample is NOT learned.

A scientific failure after mechanical defects are excluded kills V9.

## Claim boundary

V9 PASS establishes counterexample-guided symbolic generalization under the frozen language
and bounded verifier. It does not establish AGI/ASI.
