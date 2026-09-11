package ace

import (
	"reflect"
	"testing"
)

func TestFrontierTaskGeneratorDeterministicAndLatentSeparated(t *testing.T) {
	g := FrontierTaskGenerator{}
	for _, regime := range SortedRegimes() {
		a, err := g.Generate(regime, 41)
		if err != nil {
			t.Fatal(err)
		}
		b, err := g.Generate(regime, 41)
		if err != nil {
			t.Fatal(err)
		}
		if a.ID != b.ID || a.Regime != b.Regime || !reflect.DeepEqual(a.Examples, b.Examples) || !reflect.DeepEqual(a.HeldOut, b.HeldOut) {
			t.Fatalf("non-deterministic generation for %s", regime)
		}
		if a.Public.ID != a.ID || a.Public.Goal == a.Latent {
			t.Fatalf("latent/public identity leak for %s", regime)
		}
		if len(a.HeldOut) == 0 || len(a.Examples) == 0 {
			t.Fatalf("missing train/held-out split for %s", regime)
		}
	}
}

func TestFrontierTaskGeneratorSeedChangesInstance(t *testing.T) {
	g := FrontierTaskGenerator{}
	a, err := g.Generate(RegimeSymbolic, 1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := g.Generate(RegimeSymbolic, 2)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID {
		t.Fatal("independent seeds produced identical task identity")
	}
	if reflect.DeepEqual(a.Examples, b.Examples) && reflect.DeepEqual(a.HeldOut, b.HeldOut) {
		t.Fatal("independent seeds produced identical evidence")
	}
}

func TestFrontierRegimesAreDistinct(t *testing.T) {
	g := FrontierTaskGenerator{}
	seen := map[string]bool{}
	for _, regime := range SortedRegimes() {
		task, err := g.Generate(regime, 9)
		if err != nil {
			t.Fatal(err)
		}
		if seen[task.Latent] {
			t.Fatalf("latent family reused across regimes: %q", task.Latent)
		}
		seen[task.Latent] = true
	}
	if len(seen) != 4 {
		t.Fatalf("expected four latent families, got %d", len(seen))
	}
}
