package ace

import "testing"

func TestRecursiveCapabilityProtocolV2(t *testing.T) {
	// This test is intentionally a scientific gate, not a claim of AGI. It
	// requires a real acquired primitive, a structurally different bottleneck,
	// a method upgrade, and a hidden composition win.
	m, err := RunRecursiveCapabilityProtocolV2()
	if err != nil { t.Fatal(err) }
	if m["verified"] != 1 { t.Fatalf("protocol not verified: %#v", m) }
	if m["R"] >= 1 { t.Fatalf("no capability compounding: %#v", m) }
}
