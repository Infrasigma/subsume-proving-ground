package ace

import (
	"context"
	"testing"
	"time"
)

func TestBlindAcquisitionGateShowsFutureAcquisitionGain(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := RunBlindAcquisitionGate(ctx)
	if err != nil {
		t.Fatalf("blind acquisition gate failed: %v", err)
	}

	if result.BootstrapCapabilitiesVerified != 3 {
		t.Fatalf("unexpected verified bootstrap count: got %d want 3", result.BootstrapCapabilitiesVerified)
	}
	if result.FutureTasks != 8 {
		t.Fatalf("unexpected future-task count: got %d want 8", result.FutureTasks)
	}
	if result.DirectTasksSolved != 0 {
		t.Fatalf("direct frontier solved blind future tasks: got %d want 0", result.DirectTasksSolved)
	}
	if result.ComposedTasksSolved != result.FutureTasks {
		t.Fatalf(
			"retained composition failed blind future tasks: solved %d/%d",
			result.ComposedTasksSolved,
			result.FutureTasks,
		)
	}
	if result.ComposedStrategyEvaluations >= result.DirectStrategyEvaluations {
		t.Fatalf(
			"no acquisition-frontier reduction: composed=%d direct=%d",
			result.ComposedStrategyEvaluations,
			result.DirectStrategyEvaluations,
		)
	}
	if !(result.AcquisitionCostRatio < 1) {
		t.Fatalf("expected acquisition strategy ratio < 1, got %.4f", result.AcquisitionCostRatio)
	}
	if result.FullCostAccounting {
		t.Fatal("blind gate incorrectly claims full compute-inclusive cost accounting")
	}
}
