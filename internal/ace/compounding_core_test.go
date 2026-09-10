package ace

import "testing"

func TestRecursiveCapabilityCompoundingCore(t *testing.T) {
	m, e := RunRecursiveCapabilityProtocolV3(); if e != nil { t.Fatal(e) }
	if m["verified"] != 1 || m["method_installed"] != 1 || m["future_search_changed"] != 1 || m["K2_future_solved"] != 1 { t.Fatalf("autonomous method compounding failure: %#v", m) }
}

func TestReplicatedCapabilityCompoundingCore(t *testing.T) {
	m, e := RunReplicatedCompoundingV2(12); if e != nil { t.Fatal(e) }
	if m["valid"] != 0 || m["baseline_degenerate"] != 1 || m["wins"] != 0 { t.Fatalf("replication guard did not preserve invalidation: %#v", m) }
}
