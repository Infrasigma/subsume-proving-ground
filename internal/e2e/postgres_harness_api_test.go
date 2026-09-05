package e2e

// HostPort returns the loopback TCP address of the ephemeral PostgreSQL
// instance. Callers do not need access to the underlying pgx pool.
func (h postgresHarness) HostPort() string {
	return h.address
}
