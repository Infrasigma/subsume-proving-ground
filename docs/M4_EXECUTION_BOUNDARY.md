# M4 — Execution Boundary

M4 adds the first concrete AACR execution boundary using Bubblewrap on Linux.

## Security invariant

An agent launched through `BubblewrapBackend`:

- runs in a separate network namespace (`--unshare-net`);
- cannot inherit the host environment (`--clearenv`);
- receives only its executable and the broker socket directory as host filesystem mounts;
- has no provider credentials or PostgreSQL connection available inside the sandbox;
- can reach the outside broker only through the explicitly mounted Unix socket.

The broker remains outside the sandbox and is the only component that may later own provider connections. The M4 broker protocol is deliberately transport-only: authorization and ActionContract verification remain higher-layer responsibilities and must fail closed before provider execution.

## Mediated channel

`ListenUnix` creates a private `0600` Unix socket. The sandbox sees that socket at `/aacr/run/broker.sock` (or the same basename supplied by the caller). Requests are bounded newline-delimited JSON frames. The broker handler is the future integration point for authenticated ActionContract dispatch.

## Reference proof

`cmd/aacr-boundary-probe` is a static Go probe used only by tests. It must:

1. connect successfully to the broker Unix socket;
2. receive a broker response;
3. fail to establish a TCP connection to the host network.

The CI workflow installs Bubblewrap and runs this test on Ubuntu. The reference backend intentionally requires a self-contained executable; a future rootfs/backend layer can add explicit read-only runtime mounts without exposing the host filesystem wholesale.

M4 proves the physical mediation boundary. It does **not** yet claim that a request sent over the socket is authorized; that requires the existing AACR protocol verification and capability/policy layers to be wired into the broker in a later phase.
