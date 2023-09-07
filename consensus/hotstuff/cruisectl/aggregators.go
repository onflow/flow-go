package cruisectl

import (
	"fmt"
)

// Ewma implements the exponentially weighted moving average with smoothing factor α.
// The Ewma is a filter commonly applied to time-discrete signals. Mathematically,
// it is represented by the recursive update formula
//
//	value ← α·v + (1-α)·value
//
// where `v` the next observation. Intuitively, the loss factor `α` relates to the
// time window of N observations that we average over. For example, let
// α ≡ 1/N and consider an input that suddenly changes from x to y as a step
// function. Then N is _roughly_ the number of samples required to move the output
// average about 2/3 of the way from x to y.
// For numeric stability, we require α to satisfy 0 < a < 1.
// Not concurrency safe.
type Ewma struct {
	alpha float64
	value float64
}

// NewEwma instantiates a new exponentially weighted moving average.
// The smoothing factor `alpha` relates to the averaging time window. Let `alpha` ≡ 1/N and
// consider an input that suddenly changes from x to y as a step function. Then N is roughly
// the number of samples required to move the output average about 2/3 of the way from x to y.
// For numeric stability, we require `alpha` to satisfy 0 < `alpha` < 1.
func NewEwma(alpha, initialValue float64) (Ewma, error) {
	if (alpha <= 0) || (1 <= alpha) {
		return Ewma{}, fmt.Errorf("for numeric stability, we require the smoothing factor to satisfy 0 < alpha < 1")
	}
	return Ewma{
		alpha: alpha,
		value: initialValue,
	}, nil
}

// AddRepeatedObservation adds k consecutive observations with the same value v. Returns the updated value.
func (e *Ewma) AddRepeatedObservation(v float64, k int) float64 {
	// closed from for k consecutive updates with the same observation v:
	//   value ← r·value + v·(1-r)   with r := (1-α)^k
	r := powWithIntegerExponent(1.0-e.alpha, k)
	e.value = r*e.value + v*(1.0-r)
	return e.value
}

// AddObservation adds the value `v` to the EWMA. Returns the updated value.
func (e *Ewma) AddObservation(v float64) float64 {
	// Update formula: value ← α·v + (1-α)·value = value + α·(v - value)
	e.value = e.value + e.alpha*(v-e.value)
	return e.value
}

func (e *Ewma) Value() float64 {
	return e.value
}

// LeakyIntegrator is a filter commonly applied to time-discrete signals.
// Intuitively, it sums values over a limited time window. This implementation is
// parameterized by the loss factor `ß`:
//
//	value ← v + (1-ß)·value
//
// where `v` the next observation. Intuitively, the loss factor `ß` relates to the
// time window of N observations that we integrate over. For example, let ß ≡ 1/N
// and consider a constant input x:
//   - the integrator value will saturate at x·N
//   - an integrator initialized at 0 reaches 2/3 of the saturation value after N samples
//
// For numeric stability, we require ß to satisfy 0 < ß < 1.
// Further details on Leaky Integrator: https://www.music.mcgill.ca/~gary/307/week2/node4.html
// Not concurrency safe.
type LeakyIntegrator struct {
	feedbackCoef float64 // feedback coefficient := (1-ß)
	value        float64
}

// NewLeakyIntegrator instantiates a new leaky integrator with loss factor `beta`, where
// `beta relates to window of N observations that we integrate over. For example, let
// `beta` ≡ 1/N and consider a constant input x. The integrator value will saturate at x·N.
// An integrator initialized at 0 reaches 2/3 of the saturation value after N samples.
// For numeric stability, we require `beta` to satisfy 0 < `beta` < 1.
func NewLeakyIntegrator(beta, initialValue float64) (LeakyIntegrator, error) {
	if (beta <= 0) || (1 <= beta) {
		return LeakyIntegrator{}, fmt.Errorf("for numeric stability, we require the loss factor to satisfy 0 < beta < 1")
	}
	return LeakyIntegrator{
		feedbackCoef: 1.0 - beta,
		value:        initialValue,
	}, nil
}

// AddRepeatedObservation adds k consecutive observations with the same value v.  Returns the updated value.
func (e *LeakyIntegrator) AddRepeatedObservation(v float64, k int) float64 {
	// closed from for k consecutive updates with the same observation v:
	//   value ← r·value + v·(1-r)   with r := α^k
	r := powWithIntegerExponent(e.feedbackCoef, k)
	e.value = r*e.value + v*(1.0-r)/(1.0-e.feedbackCoef)
	return e.value
}

// AddObservation adds the value `v` to the LeakyIntegrator. Returns the updated value.
func (e *LeakyIntegrator) AddObservation(v float64) float64 {
	// Update formula: value ← v + feedbackCoef·value
	// where feedbackCoef = (1-beta)
	e.value = v + e.feedbackCoef*e.value
	return e.value
}

func (e *LeakyIntegrator) Value() float64 {
	return e.value
}

// powWithIntegerExponent implements exponentiation b^k optimized for integer k >=1
func powWithIntegerExponent(b float64, k int) float64 {
	r := 1.0
	for {
		if k&1 == 1 {
			r *= b
		}
		k >>= 1
		if k == 0 {
			break
		}
		b *= b
	}
	return r
}
