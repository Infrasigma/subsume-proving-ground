package acex

import "testing"

func TestV2ModelIndependentInterfaceAblation(t *testing.T) {
	core:=NewPureCore()
	if err:=core.Validate(); err!=nil { t.Fatal(err) }

	// The core is usable without any external model implementation. Replace
	// the predictor with nil and validation must fail closed rather than
	// silently invent a model-dependent capability.
	core.Predictor=nil
	if err:=core.Validate(); err==nil {
		t.Fatal("core accepted a missing predictor interface")
	}

	// Rehydrate pure predictor; no learned external weights, API, or provider
	// are needed.
	core=NewPureCore()
	p:=core.Predictor.Predict(NumericState{"x":0},"inc")
	if p.Known {
		t.Fatal("fresh pure predictor claimed prior knowledge")
	}

	// A hypothetical external model occupies only the Predictor slot; the
	// acceptance verifier remains independent and separate.
	if !core.Verifier.VerifyPrediction(
		NumericState{"x":0},"inc",
		NumericState{"x":1},
		NumericState{"x":1},
	) {
		t.Fatal("independent verifier rejected correct effect")
	}
}
