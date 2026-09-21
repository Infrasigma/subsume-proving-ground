package acex

import "testing"

func TestV12EntityExportLoadAfterRawForgetting(t *testing.T) {
	e := NewV8CognitiveEntity()
	r := NewV11DirectedExecutableRepresentation()
	r.Root = 0
	r.Nodes = 3
	r.Edges = []V11DirectedPatternEdge{{From: 0, To: 1}, {From: 1, To: 2}, {From: 0, To: 2}}
	r.Valid = true
	e.DirectedRepresentation = r

	e.Remember(V5MemoryTrace{
		ID: "raw-source",
		Context: []string{"source-only"},
		Verified: true,
	})
	if len(e.Memory.Items) == 0 {
		t.Fatal("expected source memory before destructive boundary")
	}

	e.ForgetRawExperiences()
	if len(e.Memory.Items) != 0 {
		t.Fatalf("raw memory survived: %d", len(e.Memory.Items))
	}
	if !e.DirectedRepresentation.Retained {
		t.Fatal("directed representation was not finalized")
	}
	artifact, err := e.ExportRetainedRepresentation()
	if err != nil {
		t.Fatal(err)
	}
	module, err := DecodeV12WasmBase64(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if len(module) == 0 || e.HermeticArtifactHash == "" {
		t.Fatal("missing hermetic artifact")
	}

	fresh := NewV8CognitiveEntity()
	if err := fresh.LoadRetainedRepresentation(artifact); err != nil {
		t.Fatal(err)
	}
	if len(fresh.Memory.Items) != 0 {
		t.Fatalf("fresh entity received raw memory: %d", len(fresh.Memory.Items))
	}
	if !fresh.DirectedRepresentation.Retained {
		t.Fatal("fresh entity did not restore retained representation")
	}
	if fresh.HermeticArtifactHash == "" {
		t.Fatal("fresh entity did not restore artifact identity")
	}
}
