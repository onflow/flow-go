package cruisectl

import (
	"github.com/stretchr/testify/require"
	"math"
	"testing"
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
	require.Equal(t, 12.063211544358897, v)
	require.Equal(t, 12.063211544358897, w.Value())
	v = w.AddObservation(-1.16)
	require.Equal(t, 6.128648080841518, v)
	require.Equal(t, 6.128648080841518, w.Value())
	v = w.AddObservation(1.23)
	require.Equal(t, 3.9301399632281675, v)
	require.Equal(t, 3.9301399632281675, w.Value())
}

// Test_AddingRepeatedObservations verifies correct numerics when repeated observations.
// Reference values were generated via python
func Test_EWMA_AddingRepeatedObservations(t *testing.T) {
	alpha := math.Pi / 7.0
	initialValue := 17.0
	w, err := NewEwma(alpha, initialValue)
	require.NoError(t, err)

	v := w.AddRepeatedObservation(6.0, 11)
	require.Equal(t, 6.015696509200239, v)
	require.Equal(t, 6.015696509200239, w.Value())
	v = w.AddRepeatedObservation(-1.16, 4)
	require.Equal(t, -0.49762458373978324, v)
	require.Equal(t, -0.49762458373978324, w.Value())
	v = w.AddRepeatedObservation(1.23, 1)
	require.Equal(t, 0.27773151632279214, v)
	require.Equal(t, 0.27773151632279214, w.Value())
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
	require.Equal(t, w1.Value(), v)
	require.Equal(t, w1.Value(), w2.Value())

	for i := 4; i > 0; i-- {
		w2.AddObservation(6.0)
	}
	v = w1.AddRepeatedObservation(6.0, 4)
	require.Equal(t, w2.Value(), v)
	require.Equal(t, w2.Value(), w1.Value())
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
	require.Equal(t, 15.370417841281931, v)
	require.Equal(t, 15.370417841281931, li.Value())
	v = li.AddObservation(-1.16)
	require.Equal(t, 7.312190445170959, v)
	require.Equal(t, 7.312190445170959, li.Value())
	v = li.AddObservation(1.23)
	require.Equal(t, 5.260487047428308, v)
	require.Equal(t, 5.260487047428308, li.Value())
}

// Test_AddingRepeatedObservations verifies correct numerics when repeated observations.
// Reference values were generated via python
func Test_LI_AddingRepeatedObservations(t *testing.T) {
	beta := math.Pi / 7.0
	initialValue := 17.0
	li, err := NewLeakyIntegrator(beta, initialValue)
	require.NoError(t, err)

	v := li.AddRepeatedObservation(6.0, 11)
	require.Equal(t, 13.374196472992809, v)
	require.Equal(t, 13.374196472992809, li.Value())
	v = li.AddRepeatedObservation(-1.16, 4)
	require.Equal(t, -1.1115419303895382, v)
	require.Equal(t, -1.1115419303895382, li.Value())
	v = li.AddRepeatedObservation(1.23, 1)
	require.Equal(t, 0.617316921420289, v)
	require.Equal(t, 0.617316921420289, li.Value())

}

// Test_AddingRepeatedObservations_selfConsistency applies a self-consistency check
// for  repeated observations.
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
	require.Equal(t, li1.Value(), v)
	require.Equal(t, li1.Value(), li2.Value())

	for i := 4; i > 0; i-- {
		li2.AddObservation(6.0)
	}
	v = li1.AddRepeatedObservation(6.0, 4)
	require.Equal(t, li2.Value(), v)
	require.Equal(t, li2.Value(), li1.Value())
}
