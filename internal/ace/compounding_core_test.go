package ace

import "testing"

func TestRecursiveCapabilityCompoundingCore(t *testing.T) {
	m, e := RunRecursiveCapabilityProtocolV3()
	if e != nil { t.Fatal(e) }
	if m["verified"] != 1 || m["method_installed"] != 1 || m["future_search_changed"] != 1 || m["K2_future_solved"] != 1 {
		t.Fatalf("autonomous method compounding failure: %#v", m)
	}
	// This protocol deliberately does not expose a full compute-inclusive R_n;
	// candidate-count ratios from earlier experiments are not promoted here.
}

func TestReplicatedCapabilityCompoundingCore(t *testing.T) {
	m, e := RunReplicatedCompoundingV2(12)
	if e != nil { t.Fatal(e) }
	if m["wins"] != 12 || m["all_verified"] != 1 { t.Fatalf("replication failure: %#v", m) }
	if m["mean_R"] >= 1 { t.Fatalf("mean R not below one: %#v", m) }
}
