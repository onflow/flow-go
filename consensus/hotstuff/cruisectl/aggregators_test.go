package cruisectl

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// Test_Instantiation verifies successful instantiation of Ewma
func Test_EWMA_Instantiation(t *testing.T) {
	w, err := NewEwma(0.5, 17.2)
	require.NoError(t, err)
	require.Equal(t, 17.2, w.Value())
}

// Test_EnforceNumericalBounds verifies that constructor only accepts
// alpha values that satisfy 0 < alpha  < 1
func Test_EWMA_EnforceNumericalBounds(t *testing.T) {
	for _, alpha := range []float64{-1, 0, 1, 2} {
		_, err := NewEwma(alpha, 17.2)
		require.Error(t, err)
	}
}

// Test_AddingObservations verifies correct numerics when adding a single value.
// Reference values were generated via python
func Test_EWMA_AddingObservations(t *testing.T) {
	alpha := math.Pi / 7.0
	initialValue := 17.0
	w, err := NewEwma(alpha, initialValue)
	require.NoError(t, err)

	v := w.AddObservation(6.0)
	require.InEpsilon(t, 12.063211544358897, v, 1e-12)
	require.InEpsilon(t, 12.063211544358897, w.Value(), 1e-12)
	v = w.AddObservation(-1.16)
	require.InEpsilon(t, 6.128648080841518, v, 1e-12)
	require.InEpsilon(t, 6.128648080841518, w.Value(), 1e-12)
	v = w.AddObservation(1.23)
	require.InEpsilon(t, 3.9301399632281675, v, 1e-12)
	require.InEpsilon(t, 3.9301399632281675, w.Value(), 1e-12)
}

// Test_AddingRepeatedObservations verifies correct numerics when repeated observations.
// Reference values were generated via python
func Test_EWMA_AddingRepeatedObservations(t *testing.T) {
	alpha := math.Pi / 7.0
	initialValue := 17.0
	w, err := NewEwma(alpha, initialValue)
	require.NoError(t, err)

	v := w.AddRepeatedObservation(6.0, 11)
	require.InEpsilon(t, 6.015696509200239, v, 1e-12)
	require.InEpsilon(t, 6.015696509200239, w.Value(), 1e-12)
	v = w.AddRepeatedObservation(-1.16, 4)
	require.InEpsilon(t, -0.49762458373978324, v, 1e-12)
	require.InEpsilon(t, -0.49762458373978324, w.Value(), 1e-12)
	v = w.AddRepeatedObservation(1.23, 1)
	require.InEpsilon(t, 0.27773151632279214, v, 1e-12)
	require.InEpsilon(t, 0.27773151632279214, w.Value(), 1e-12)
}

// Test_AddingRepeatedObservations_selfConsistency applies a self-consistency check
// for  repeated observations.
func Test_EWMA_AddingRepeatedObservations_selfConsistency(t *testing.T) {
	alpha := math.Pi / 7.0
	initialValue := 17.0
	w1, err := NewEwma(alpha, initialValue)
	require.NoError(t, err)
	w2, err := NewEwma(alpha, initialValue)
	require.NoError(t, err)

	for i := 7; i > 0; i-- {
		w1.AddObservation(6.0)
	}
	v := w2.AddRepeatedObservation(6.0, 7)
	require.InEpsilon(t, w1.Value(), v, 1e-12)
	require.InEpsilon(t, w1.Value(), w2.Value(), 1e-12)

	for i := 4; i > 0; i-- {
		w2.AddObservation(6.0)
	}
	v = w1.AddRepeatedObservation(6.0, 4)
	require.InEpsilon(t, w2.Value(), v, 1e-12)
	require.InEpsilon(t, w2.Value(), w1.Value(), 1e-12)
}

// Test_LI_Instantiation verifies successful instantiation of LeakyIntegrator
func Test_LI_Instantiation(t *testing.T) {
	li, err := NewLeakyIntegrator(0.5, 17.2)
	require.NoError(t, err)
	require.Equal(t, 17.2, li.Value())
}

// Test_EnforceNumericalBounds verifies that constructor only accepts
// alpha values that satisfy 0 < alpha  < 1
func Test_LI_EnforceNumericalBounds(t *testing.T) {
	for _, beta := range []float64{-1, 0, 1, 2} {
		_, err := NewLeakyIntegrator(beta, 17.2)
		require.Error(t, err)
	}
}

// Test_AddingObservations verifies correct numerics when adding a single value.
// Reference values were generated via python
func Test_LI_AddingObservations(t *testing.T) {
	beta := math.Pi / 7.0
	initialValue := 17.0
	li, err := NewLeakyIntegrator(beta, initialValue)
	require.NoError(t, err)

	v := li.AddObservation(6.0)
	require.InEpsilon(t, 15.370417841281931, v, 1e-12)
	require.InEpsilon(t, 15.370417841281931, li.Value(), 1e-12)
	v = li.AddObservation(-1.16)
	require.InEpsilon(t, 7.312190445170959, v, 1e-12)
	require.InEpsilon(t, 7.312190445170959, li.Value(), 1e-12)
	v = li.AddObservation(1.23)
	require.InEpsilon(t, 5.260487047428308, v, 1e-12)
	require.InEpsilon(t, 5.260487047428308, li.Value(), 1e-12)
}

// Test_AddingRepeatedObservations verifies correct numerics when repeated observations.
// Reference values were generated via python
func Test_LI_AddingRepeatedObservations(t *testing.T) {
	beta := math.Pi / 7.0
	initialValue := 17.0
	li, err := NewLeakyIntegrator(beta, initialValue)
	require.NoError(t, err)

	v := li.AddRepeatedObservation(6.0, 11)
	require.InEpsilon(t, 13.374196472992809, v, 1e-12)
	require.InEpsilon(t, 13.374196472992809, li.Value(), 1e-12)
	v = li.AddRepeatedObservation(-1.16, 4)
	require.InEpsilon(t, -1.1115419303895382, v, 1e-12)
	require.InEpsilon(t, -1.1115419303895382, li.Value(), 1e-12)
	v = li.AddRepeatedObservation(1.23, 1)
	require.InEpsilon(t, 0.617316921420289, v, 1e-12)
	require.InEpsilon(t, 0.617316921420289, li.Value(), 1e-12)

}

// Test_AddingRepeatedObservations_selfConsistency applies a self-consistency check
// for repeated observations.
func Test_LI_AddingRepeatedObservations_selfConsistency(t *testing.T) {
	beta := math.Pi / 7.0
	initialValue := 17.0
	li1, err := NewLeakyIntegrator(beta, initialValue)
	require.NoError(t, err)
	li2, err := NewLeakyIntegrator(beta, initialValue)
	require.NoError(t, err)

	for i := 7; i > 0; i-- {
		li1.AddObservation(6.0)
	}
	v := li2.AddRepeatedObservation(6.0, 7)
	require.InEpsilon(t, li1.Value(), v, 1e-12)
	require.InEpsilon(t, li1.Value(), li2.Value(), 1e-12)

	for i := 4; i > 0; i-- {
		li2.AddObservation(6.0)
	}
	v = li1.AddRepeatedObservation(6.0, 4)
	require.InEpsilon(t, li2.Value(), v, 1e-12)
	require.InEpsilon(t, li2.Value(), li1.Value(), 1e-12)
}
