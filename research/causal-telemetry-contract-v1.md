# ACSIE causal telemetry contract v1

Telemetry is observational only. It MUST NOT affect candidate generation,
candidate ordering, RNG state, pruning, admission, retention, evaluator
thresholds, or checkpoint state.

Required generation-level cardinalities:
- generated_expressions
- unique_structures
- unique_states
- unique_behaviors
- lineage_variants
- target_candidates
- target_matches
- validated_candidates
- retained_capabilities
- frontier_size
- frontier_max_size
- dominance_comparisons
- composition_attempts
- composition_successes
- capability_depth

Required stage wall-time fields:
- synthesis
- matching
- semantic
- validation
- admission
- lineage
- dominance
- extraction

Required ratios:
R1 = generated_expressions / max(validated_candidates, 1)
R2 = target_matches / max(generated_expressions, 1)
R3 = unique_behaviors / max(generated_expressions, 1)
R4 = lineage_variants / max(unique_behaviors, 1)
R5 = dominance_comparisons / max(frontier_size^2, 1)
R6 = retained_capabilities / max(validated_candidates, 1)

The causal diagnosis remains:
A generation -> B matching -> C representation -> D retention,
with E relocation when a repair moves the dominant cost elsewhere.

No ratio is itself evidence of inefficiency without stage timing and cardinality
definitions. Compression is not part of the acceptance gate in this version.
