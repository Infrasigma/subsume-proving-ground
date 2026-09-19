package ace

func DefaultT2ReactorTasks() []ReactorTask {
	budget := ResourceVector{
		Compute:          100,
		Memory:           100,
		Storage:          10,
		TimeMS:           5000,
		ExperimentBudget: 64,
	}
	return []ReactorTask{
		{
			ID:          "01-sequence-monotonic-normalization",
			Family:      "sequence-logic",
			Description: "Normalize a descending sequence into ascending order; this is the current candidate-stream proxy for monotonic sequence reasoning.",
			Examples: []ReactorExample{
				{Input: []string{"high", "mid", "low"}, Expected: []string{"low", "mid", "high"}},
				{Input: []string{"four", "three", "two", "one"}, Expected: []string{"one", "two", "three", "four"}},
			},
			Hidden: []ReactorExample{
				{Input: []string{"e", "d", "c", "b", "a"}, Expected: []string{"a", "b", "c", "d", "e"}},
				{Input: []string{"z", "y", "x"}, Expected: []string{"x", "y", "z"}},
			},
			MaxSearchDepth:     2,
			MinProcedureSteps:  2,
			AdmitAsAbstraction: true,
			Budget:             budget,
		},
		{
			ID:                      "02-matrix-row-major-reversal",
			Family:                  "spatial-matrix",
			Description:             "Reverse a flattened 2x2 row-major matrix; the learned sequence operator must transfer without retraining.",
			Examples: []ReactorExample{
				{Input: []string{"m11", "m12", "m21", "m22"}, Expected: []string{"m22", "m21", "m12", "m11"}},
				{Input: []string{"a", "b", "c", "d"}, Expected: []string{"d", "c", "b", "a"}},
			},
			Hidden: []ReactorExample{
				{Input: []string{"r11", "r12", "r21", "r22"}, Expected: []string{"r22", "r21", "r12", "r11"}},
				{Input: []string{"q1", "q2", "q3", "q4"}, Expected: []string{"q4", "q3", "q2", "q1"}},
			},
			MaxSearchDepth:         2,
			MinProcedureSteps:      2,
			RequireLatestAdmission: true,
			Budget:                 budget,
		},
	}
}
