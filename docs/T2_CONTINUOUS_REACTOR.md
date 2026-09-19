# T2 Continuous Reactor

The T2 continuous reactor is the daemonized compounding layer built on the F0 cryptographic admission baseline.

## Contract

Each loop iteration:

1. Hydrate the acquired-abstraction library from its durable JSON body.
2. Replace any persisted signer map with the externally configured trusted KMS public key.
3. Resolve every persisted admission reference against the SQLite F0 admission ledger.
4. Recompute the artifact hash and verify the KMS Ed25519 admission receipt.
5. Pull the next task from the queue.
6. Run library-aware parameterized mechanism search over the existing acquisition-procedure substrate.
7. Verify candidates against evaluator-owned holdout fixtures, including comparison with the independent reference interpreter.
8. When a new abstraction is promoted, send it through the existing F0 admission choke point: KMS signature, durable SQLite admission, receipt verification, then installation.
9. Persist the expanded abstraction library.
10. Continue with the next task using the expanded in-memory library.

Task failures are isolated. A failed search or malformed task is moved to the failure path and the reactor continues with subsequent work. Queue, ledger, hydration, or acknowledgement failures are fatal infrastructure errors.

## Current frontier

The default queue contains two tasks:

- 01-sequence-monotonic-normalization: normalize descending candidate streams into ascending streams and admit the discovered procedure.
- 02-matrix-row-major-reversal: reverse flattened 2x2 row-major streams while requiring the latest admitted abstraction.

Holdout examples are owned by the verifier, not serialized into the task queue. The search process receives only training examples.

These are deliberate probes against the current ACE substrate. The universal execution/search representation is an ordered stream of ArchitectureCandidate values; it does not yet implement a typed sequence or matrix value system. Therefore these tasks demonstrate endogenous operator reuse and cross-task transfer in the existing substrate, not a general-purpose monotonicity classifier or a general matrix reasoning engine.

## Storage and trust boundary

The SQLite database stores durable F0 admission receipts and their hash chain. The executable abstraction body is stored in the persistent abstraction library JSON.

The daemon never treats the persisted signer map as a trust root. The trusted signer identity and public key must be supplied externally:

- T2_KMS_SIGNER_ID
- T2_TRUSTED_KMS_PUBLIC_KEY_B64

The remote signer is configured through the existing T2 KMS client:

- T2_KMS_URL
- T2_KMS_BEARER_TOKEN

The runtime additionally checks that the KMS response public key exactly matches the configured trust root.

## Daemon

Build/run the daemon with:

    go run ./cmd/t2-reactor --seed-defaults

Useful flags:

- --ledger: SQLite admission ledger path.
- --library: durable abstraction library path.
- --task-dir: directory-backed queue.
- --poll: queue polling interval.
- --max-tasks: finite run for CI/operations; zero means continuous.
- --seed-defaults: write the two default tasks if they do not already exist.

Queue files are atomically moved to processing/, then to done/ or failed/.

## CI evidence

The headless test TestContinuousReactorCompoundsAcrossTasks proves, using a real SQLite ledger, a real Ed25519 signing primitive, and evaluator-owned holdouts:

- Task 1 discovers and admits a new abstraction.
- The admission receipt is durably retrievable from SQLite.
- Task 2 uses the newly admitted abstraction rather than starting from the empty library.
- A fresh process-style hydration accepts the persisted body only when its ledger receipt and external trust root agree.
- A forged ledger admission reference is rejected during hydration.

This layer is an autonomous execution loop around the existing ACE substrate. It is not, by itself, evidence that ASI or AGI has been achieved.
