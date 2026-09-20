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
			MaxSearchDepth:         2,
			MinProcedureSteps:      2,
			RequireLatestAdmission: true,
			Budget:                 budget,
		},
	}
}

func DefaultT2ReactorVerifier() ReactorVerifier {
	return StaticReactorVerifier{
		HiddenByTask: map[string][]ReactorExample{
			"01-sequence-monotonic-normalization": {
				{Input: []string{"e", "d", "c", "b", "a"}, Expected: []string{"a", "b", "c", "d", "e"}},
				{Input: []string{"z", "y", "x"}, Expected: []string{"x", "y", "z"}},
			},
			"02-matrix-row-major-reversal": {
				{Input: []string{"r11", "r12", "r21", "r22"}, Expected: []string{"r22", "r21", "r12", "r11"}},
				{Input: []string{"q1", "q2", "q3", "q4"}, Expected: []string{"q4", "q3", "q2", "q1"}},
			},
		},
	}
}


func DefaultT3DomainEscapeTasks() []ReactorTask {
	budget := ResourceVector{
		Compute:          250,
		Memory:           128,
		Storage:          32,
		TimeMS:            5000,
		ExperimentBudget: 128,
	}
	return []ReactorTask{
		{
			ID:          "03-string-uppercase-vowels",
			Family:      "string-transform",
			InputKind:   "string",
			Description: "Uppercase every vowel in a UTF-8 text block while leaving every other rune unchanged. This task is outside the ArchitectureCandidate stream vocabulary.",
			Examples: []ReactorExample{
				{Input: []string{"hello world"}, Expected: []string{"hEllO wOrld"}},
				{Input: []string{"ace reactor"}, Expected: []string{"AcE rEActOr"}},
				{Input: []string{"strict verification"}, Expected: []string{"strIct vErIfIcAtIOn"}},
			},
			MaxSearchDepth:     1,
			MinProcedureSteps:  1,
			AdmitAsAbstraction: true,
			Budget:             budget,
		},
	}
}

func DefaultT3DomainEscapeVerifier() ReactorVerifier {
	hidden := DefaultT2ReactorVerifier().(StaticReactorVerifier).HiddenByTask
	hiddenCases := []ReactorExample{
		{Input: []string{"functional verification"}, Expected: []string{"fUnctIOnAl vErIfIcAtIOn"}},
		{Input: []string{"zero trust daemon"}, Expected: []string{"zErO trUst dAEmOn"}},
		{Input: []string{"cryptographic ledger"}, Expected: []string{"cryptOgrAphIc lEdgEr"}},
	}
	hidden["03-string-uppercase-vowels"] = hiddenCases
	hidden[MetaSearchHeuristicTaskID] = hiddenCases
	hidden[PostHotSwapTaskID] = hiddenCases
	return StaticReactorVerifier{HiddenByTask: hidden}
}
