package t2

import (
	"os"
	"path/filepath"
	"testing"
)

func testPrereg(t *testing.T) Preregistration {
	t.Helper()
	p, _, err := LoadPreregistration(filepath.Join("testdata", "mock_preregistration.json"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSplitIsAuthenticated(t *testing.T) {
	p := testPrereg(t)
	if got := p.Generator.Novelty; got.ASTDepthMin != 3 || got.GraphNodeCountMin != 6 || got.CycleRankMin != 2 || got.DependencyPathMin != 5 {
		t.Fatalf("unexpected locked novelty rule: %+v", got)
	}
	tasks := make([]Task, 0, 20)
	for i := 0; i < 20; i++ {
		f := "g1"
		if i%2 == 1 {
			f = "g2"
		}
		tasks = append(tasks, Task{
			ID: string(rune('a' + i)),
			Family: f,
			Prompt: "unique prompt " + string(rune('a'+i)),
			Latent: map[string]string{"seed": string(rune('A' + i))},
			Structure: map[string]string{"depth": "d"},
		})
	}
	d := t.TempDir()
	m, key, err := SplitAndSeal(tasks, p, d)
	if err != nil {
		t.Fatal(err)
	}
	if m.CiphertextHash == m.ValidateHash {
		t.Fatal("ciphertext and plaintext hashes must differ")
	}
	ct, err := os.ReadFile(filepath.Join(d, "D_validate.enc"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(ct, key, p.StudyID+":D_validate"); err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(ct, make([]byte, 32), p.StudyID+":D_validate"); err == nil {
		t.Fatal("wrong key accepted")
	}
}

func TestMechanicalConjunction(t *testing.T) {
	p := testPrereg(t)
	a := map[string]bool{
		"null_simulation_calibrated": true,
		"power_evaluation_passed":    true,
		"F0_integrity_passed":        true,
		"generator_integrity_passed": true,
	}
	in := map[string]EstimandInput{
		"causality_established":           {Estimand: "ΔC_delete", Value: 1.1, PValue: 0.01},
		"structural_transfer_established": {Estimand: "Δ_X", Value: 0.3, PValue: 0.01},
		"capability_advantage_established": {Estimand: "ΔC_MA", Value: 1.1, PValue: 0.01},
		"cost_advantage_established":       {Estimand: "ΔK_MA", Value: 1.1, PValue: 0.01},
	}
	o, err := FinalizeFromPrereg(p, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if o["VERDICT"] != "T2_DEMONSTRATED" {
		t.Fatalf("unexpected verdict %v", o["VERDICT"])
	}
}
