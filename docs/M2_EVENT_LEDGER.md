# AACR M2 SQLite Event Ledger Specification

Status: **Protocol-locked M2 implementation specification**.

M2 adds a durable, append-only execution ledger. SQLite/WAL is the persistence substrate; the protocol is the event model, hash chain, lifecycle, and recovery semantics.

## 1. Non-negotiable invariants

1. **Genesis is explicit.** `previous_event_hash` is always a 64-character lowercase hexadecimal SHA-256 value. For `sequence = 1`, it MUST equal:

   `0000000000000000000000000000000000000000000000000000000000000000`

   This is the hex encoding of 32 zero bytes. No SQL `NULL`, omitted field, empty string, or textual `"null"` representation is permitted for genesis.

2. **The lifecycle is database-serialized.** Every state-changing append executes inside a SQLite transaction opened with `_txlock=immediate`. A database trigger validates sequence, previous hash, and the transition matrix before an event can be inserted. The application cannot bypass the matrix by racing another writer.

3. **Indeterminate is a reconciliation barrier.** Once the latest state becomes `INDETERMINATE` or `RECONCILIATION_REQUIRED`, normal lifecycle observations such as `EFFECT_OBSERVED` are rejected. Late provider responses MUST be routed to reconciliation. They are evidence for reconciliation, not ordinary lifecycle events.

4. **Capability time validity is anchored to dispatch.** Offline verification uses `DISPATCHED.created_at` as the relevant execution time. For every dispatched action:

   `capability.expires_at >= DISPATCHED.created_at`

   Receipt-generation time and terminal `COMMITTED.created_at` do not extend a capability's validity.

5. **Idempotency is durably bound before network dispatch.** The `DISPATCHED` event MUST contain a non-empty `provider_request_id` or `idempotency_key`. The event MUST be durably committed to the WAL before the provider network request is sent. The provider adapter uses that exact value during reconciliation. If no durable token exists, an indeterminate execution cannot be safely reconciled and MUST degrade to `QUARANTINED`.

6. **History is immutable.** Events are append-only. Recovery never rewrites or deletes history. State is derived from the latest accepted event.

## 2. SQLite layout

### `executions`

One materialized latest-state row per execution:

- `execution_id TEXT PRIMARY KEY`
- `latest_sequence INTEGER NOT NULL`
- `latest_state TEXT NOT NULL`
- `latest_event_hash TEXT NOT NULL`
- `updated_at TEXT NOT NULL`

This table is a projection maintained by an `AFTER INSERT` trigger. It is not the source of truth; `events` is.

### `events`

Immutable execution history:

- `event_id TEXT PRIMARY KEY` — UUID
- `execution_id TEXT NOT NULL`
- `sequence INTEGER NOT NULL`
- `event_type TEXT NOT NULL`
- `event_payload TEXT NOT NULL` — exact JSON artifact persisted for the event
- `event_hash TEXT NOT NULL` — lowercase SHA-256 hex
- `previous_event_hash TEXT NOT NULL`
- `created_at TEXT NOT NULL` — UTC RFC3339Nano

Constraints/indexes include:

- unique `(execution_id, sequence)`
- unique `(execution_id, event_id)` through the primary key plus execution lookup
- index `(execution_id, sequence)`

`events.previous_event_hash` is **never nullable**.

## 3. Event hash construction

The chain uses a domain-separated, length-prefixed binary encoding. Let:

- `D = "AACR/Event/v1"`
- `execution_id` = UTF-8 bytes
- `sequence` = unsigned 64-bit big-endian
- `event_type` = UTF-8 bytes
- `payload_hash = SHA-256(canonical event_payload)`
- `previous_hash` = raw 32-byte digest represented by `previous_event_hash`

The hash input is:

`u32be(len(D)) || D || u32be(len(execution_id)) || execution_id || u64be(sequence) || u32be(len(event_type)) || event_type || payload_hash || previous_hash`

`event_hash = SHA-256(hash_input)`.

For sequence 1, `previous_hash` is the 32 zero bytes represented by the genesis constant above.

The event payload is canonicalized with AACR's currently restricted v0.1 canonicalization profile before hashing. The profile rejects floats, bounds nesting, and fails closed for non-BMP object keys until full RFC 8785/JCS UTF-16 ordering is implemented.

## 4. State machine

Terminal states are `COMMITTED`, `ABORTED`, `COMPENSATED`, and `QUARANTINED`.

`INDETERMINATE` and `RECONCILIATION_REQUIRED` are **not** terminal; they are recovery barriers.

Allowed transitions:

| Current state | Allowed next event | Next state |
|---|---|---|
| none | `AUTHORIZED` | `AUTHORIZED` |
| `AUTHORIZED` | `DISPATCHED` | `DISPATCHED` |
| `DISPATCHED` | `RECOVERY_DETECTED` | `DISPATCHED` |
| `DISPATCHED` | `EFFECT_OBSERVED` | `EFFECT_OBSERVED` |
| `DISPATCHED` | `ABORTED` | `ABORTED` |
| `DISPATCHED` | `INDETERMINATE` | `INDETERMINATE` |
| `EFFECT_OBSERVED` | `VERIFIED` | `VERIFIED` |
| `EFFECT_OBSERVED` | `INDETERMINATE` | `INDETERMINATE` |
| `EFFECT_OBSERVED` | `QUARANTINED` | `QUARANTINED` |
| `VERIFIED` | `COMMITTED` | `COMMITTED` |
| `VERIFIED` | `ABORTED` | `ABORTED` |
| `VERIFIED` | `COMPENSATED` | `COMPENSATED` |
| `INDETERMINATE` | `RECONCILIATION_REQUIRED` | `RECONCILIATION_REQUIRED` |
| `RECONCILIATION_REQUIRED` | `VERIFIED` | `VERIFIED` |
| `RECONCILIATION_REQUIRED` | `ABORTED` | `ABORTED` |
| `RECONCILIATION_REQUIRED` | `COMPENSATED` | `COMPENSATED` |
| `RECONCILIATION_REQUIRED` | `QUARANTINED` | `QUARANTINED` |

No event may append after a terminal state.

Most importantly, there is **no** `INDETERMINATE -> EFFECT_OBSERVED` or `RECONCILIATION_REQUIRED -> EFFECT_OBSERVED` path.

## 5. Database enforcement

The write path is:

1. Open SQLite in WAL mode with foreign keys enabled, synchronous durability selected explicitly, a bounded busy timeout, and `_txlock=immediate`.
2. Begin a transaction using the immediate lock mode.
3. Read the materialized current execution state.
4. Let a `BEFORE INSERT` trigger enforce:
   - sequence starts at 1 and increments exactly by 1;
   - genesis hash for sequence 1;
   - `previous_event_hash` equals the current latest hash for later events;
   - event transition is in the frozen matrix;
   - `DISPATCHED` carries exactly a usable provider idempotency token;
   - no append is accepted after a terminal state;
   - no normal observation is accepted from an indeterminate/reconciliation barrier.
5. Insert the event.
6. Let an `AFTER INSERT` trigger update `executions` atomically.
7. Commit.

The caller treats the successful commit as the durable point. The provider network request MUST occur only after the `DISPATCHED` transaction commits.

## 6. Crash recovery and SIGKILL semantics

The critical ordering is:

`AUTHORIZED commit -> DISPATCHED commit -> network request`

Never:

`AUTHORIZED -> network request -> DISPATCHED commit`

On process restart, the recovery scanner finds an execution whose latest state is `DISPATCHED` and has no following lifecycle event. It appends:

`RECOVERY_DETECTED -> INDETERMINATE -> RECONCILIATION_REQUIRED`

The provider adapter then reconciles using the **exact durable `provider_request_id` / `idempotency_key`** stored in `DISPATCHED`.

A provider response arriving after restart cannot mutate the normal lifecycle directly. It is submitted to the reconciliation subsystem, which creates a reconciliation event/evidence record and then drives the execution through one of the explicitly allowed resolution paths.

A broker crash cannot be interpreted as proof that the external provider failed. Before reconciliation, the external effect is unknown.

## 7. Capability expiry rule

The `DISPATCHED` event payload MUST identify the capability validity used for the dispatch, including its expiration value in a machine-comparable representation.

For M2 the ledger records `created_at` as UTC RFC3339Nano text and the append path validates the capability expiration using parsed timestamps before accepting `DISPATCHED`.

The offline verifier reconstructs the event history and checks:

`capability.expires_at >= timestamp(DISPATCHED)`

It MUST NOT compare the capability expiration against `COMMITTED` time or receipt creation time.

Timestamps remain claims about local observation time. They do not independently prove when an external provider performed an effect.

## 8. Idempotency contract

Before dispatch, the broker obtains the provider's exact idempotency value, or deterministically derives one when the provider contract explicitly permits that derivation:

`provider_request_id = stable(provider, execution_id, attempt)`

The chosen value is persisted in the `DISPATCHED` payload and can never be silently substituted during reconciliation.

If the provider does not support idempotency lookup and no durable request identifier is available, reconciliation cannot establish whether an effect occurred. The safe resolution is `QUARANTINED`, not a fabricated success or failure.

## 9. SQLite durability profile

M2 uses:

- journal mode: `WAL`
- foreign keys: `ON`
- `synchronous=FULL`
- bounded `busy_timeout`
- immediate write transactions
- a single writer connection pool for the ledger handle

WAL improves reader/writer concurrency, but SQLite still serializes writers. M2 therefore optimizes for correctness and deterministic recovery rather than horizontal write scaling.

## 10. Verification requirements

`aac ledger verify <execution-id>` will reconstruct the chain from `events`, verify:

- contiguous sequences;
- genesis correctness;
- previous-hash linkage;
- event-hash recomputation;
- transition validity;
- immutability assumptions;
- terminal-state integrity.

M2 does not claim independent verification of the external provider's state. That remains M3 provider/evidence functionality.
