package acex

import "testing"

func TestV12EffectIRLinearBoundary(t *testing.T) {
	r := NewV11DirectedExecutableRepresentation()
	r.Root = 0
	r.Nodes = 3
	r.Edges = []V11DirectedPatternEdge{{From: 0, To: 1}, {From: 1, To: 2}}
	r.Valid = true

	ir, err := BuildV12EffectIR(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := ir.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(ir.Tokens) != 3 || ir.Tokens[0].Kind != "state-before" ||
		ir.Tokens[1].Kind != "action" || ir.Tokens[2].Kind != "state-after" {
		t.Fatalf("unexpected token boundary: %+v", ir.Tokens)
	}

	bad := ir
	bad.EffectOps = append([]V12EffectOp(nil), ir.EffectOps...)
	bad.EffectOps[0].Consumes = []uint32{1, 1}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected duplicate linear token consumption to be rejected")
	}
}

func TestV12EffectIRRejectsAliasedStateOutput(t *testing.T) {
	r := NewV11DirectedExecutableRepresentation()
	r.Root = 0
	r.Nodes = 2
	r.Edges = []V11DirectedPatternEdge{{From: 0, To: 1}}
	r.Valid = true
	ir, err := BuildV12EffectIR(r)
	if err != nil {
		t.Fatal(err)
	}
	bad := ir
	bad.EffectOps = append([]V12EffectOp(nil), ir.EffectOps...)
	bad.EffectOps[0].Produces = []uint32{1}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected effect output alias to be rejected")
	}
}
