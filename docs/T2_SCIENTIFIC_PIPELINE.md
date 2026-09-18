# T2 hostile-adjudication pipeline

This layer is deliberately separate from the ACE runtime. It is a scientific adjudication kernel, not an agent.

## Hard gates

- The preregistration must be LOCKED, contain concrete thresholds/effect sizes, and carry a SHA-256 criteria hash over the canonical JSON.
- Phase 0 runs distinct Monte Carlo Type-I calibration and power evaluation with exactly 10,000 outer simulations.
- F0 accepts only the independently supplied D_audit score file.
- Generator auditing rejects unexpected/missing families, duplicate identifiers, excessive n-gram overlap, and over-constant latent variables.
- D_validate is authenticated AES-256-GCM ciphertext and is never written as plaintext by the seal operation.
- Production decryption must be remote-KMS mediated by a Phase-4 fresh-search lock proof. LocalKMS is test-only.
- Final verdict construction is mechanical and has no manual override.
- The acquisition-cost rule is fixed to preregistered stepwise-linear interpolation with no extrapolation.

## Adapter boundary

The branch does not expose a single immutable ABI for M+, M-, M^reimpl, and A0^fresh. This kernel therefore does not invent scientific assumptions about those executors. The arms must be adapted to emit canonical measurement records with identical D_validate task IDs.

The production adapter must additionally enforce the fresh-baseline lock, immutable workspaces, resource accounting, hashed mechanism specification before clean-room implementation, and remote KMS release after Phase-4 attestation.

The repository must not silently substitute the local test KMS for that production boundary.
