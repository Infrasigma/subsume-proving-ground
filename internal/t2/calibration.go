package t2

import (
	"errors"
	"math"
	"math/rand"
)

type CalibrationResult struct {
	OuterSimulations      int     `json:"outer_simulations"`
	AlphaTarget           float64 `json:"alpha_target"`
	EmpiricalNullAlpha    float64 `json:"empirical_null_alpha"`
	PowerTarget           float64 `json:"power_target"`
	MinimumEffect         float64 `json:"minimum_effect"`
	EstimatedPower        float64 `json:"estimated_power"`
	NullTolerance         float64 `json:"null_tolerance"`
	PowerPass             bool    `json:"power_pass"`
	Seed                  int64   `json:"seed"`
}

func standardNormalCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

func standardNormalQuantile(p float64) float64 {
	lo, hi := -9.0, 9.0
	for i := 0; i < 100; i++ {
		mid := (lo + hi) / 2
		if standardNormalCDF(mid) < p {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

func SimulateCalibration(p Preregistration) (CalibrationResult, error) {
	if err := p.Validate(); err != nil {
		return CalibrationResult{}, err
	}
	n := p.Statistics.SampleSize
	if n < 2 || p.Statistics.NullStd <= 0 || p.Statistics.AltStd <= 0 {
		return CalibrationResult{}, errors.New("invalid calibration sample parameters")
	}
	m := p.Statistics.OuterSimulations
	if m != 10000 {
		return CalibrationResult{}, errors.New("calibration requires exactly 10000 outer simulations")
	}

	// This is a deterministic Monte Carlo calibration of the one-sided normal
	// reference used only for protocol preflight. It is not experimental data.
	r := rand.New(rand.NewSource(p.Statistics.Seed))
	seNull := math.Sqrt(2) * p.Statistics.NullStd / math.Sqrt(float64(n))
	seAlt := math.Sqrt(2) * p.Statistics.AltStd / math.Sqrt(float64(n))
	critical := standardNormalQuantile(1 - p.Thresholds.Alpha) * seNull

	nullRejects := 0
	altRejects := 0
	for i := 0; i < m; i++ {
		nullDifference := r.NormFloat64() * seNull
		if nullDifference >= critical {
			nullRejects++
		}
		altDifference := p.Thresholds.DeltaC + r.NormFloat64()*seAlt
		if altDifference >= critical {
			altRejects++
		}
	}
	nullAlpha := float64(nullRejects) / float64(m)
	power := float64(altRejects) / float64(m)

	// Conservative Monte Carlo tolerance: three standard errors around alpha.
	tol := 3 * math.Sqrt(p.Thresholds.Alpha*(1-p.Thresholds.Alpha)/float64(m))
	return CalibrationResult{
		OuterSimulations: m,
		AlphaTarget: p.Thresholds.Alpha,
		EmpiricalNullAlpha: nullAlpha,
		PowerTarget: p.Thresholds.PowerTarget,
		MinimumEffect: p.Thresholds.DeltaC,
		EstimatedPower: power,
		NullTolerance: tol,
		PowerPass: power >= p.Thresholds.PowerTarget && math.Abs(nullAlpha-p.Thresholds.Alpha) <= tol,
		Seed: p.Statistics.Seed,
	}, nil
}
