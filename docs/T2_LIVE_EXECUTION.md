# T2 live execution infrastructure

Structural novelty is preregistered as X(s)=(ASTDepth,GraphNodeCount,CycleRank,DependencyPathLength) with:
N(s)=I[ASTDepth>=3] OR I[GraphNodeCount>=6] OR I[CycleRank>=2] OR I[DependencyPathLength>=5].

G1 arithmetic AST crosses AST depth 2 -> 3 in validation.
G2 finite-state crosses 4 -> 6 nodes and 1 -> 2 cycle rank.
G3 boolean AST crosses depth 2 -> 3.
G4 list-rewrite crosses dependency path 3 -> 5.

Each arm is a separately hashed executable run under Linux bubblewrap with network, PID, IPC, UTS, and cgroup namespace isolation, a cleared environment, read-only executable/task mounts, and a private writable workspace. The arm sees only public tasks; oracle scoring remains outside the sandbox.

Resource budget is enforced through token accounting reported by the arm ABI and measured wall/CPU limits. Composite K is preregistered:
K = TokenWeight*(tokens_in + tokens_out) + CPUTimeMSWeight*cpu_time_ms.

A0^fresh is a new process with no Phase-2 state mounted. The Phase-4 lock proof is Ed25519-signed and sent to a remote KMS. Production release requires T2_KMS_URL, T2_KMS_BEARER_TOKEN, and optionally a CA/client certificate/key.

The remote service must implement:
POST /v1/t2/keys
POST /v1/t2/release

The repository deliberately does not pretend that an external KMS has been provisioned. Without a real endpoint and credentials, validation release remains fail-closed.
