package unittest

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// RequireNumericallyClose is a wrapper around require.Equal that allows for a small epsilon difference between
// two floats. This is useful when comparing floats that are the result of a computation. For example, when comparing
// the result of a computation with a constant.
// The epsilon is calculated as:
//
//	epsilon = max(|a|, |b|) * epsilon
//
// Example:
//
//	RequireNumericallyClose(t, 1.0, 1.1, 0.1) // passes since 1.0 * 0.1 = 0.1 < 0.1
//	RequireNumericallyClose(t, 1.0, 1.1, 0.01) // fails since 1.0 * 0.01 = 0.01 < 0.1
//	RequireNumericallyClose(t, 1.0, 1.1, 0.11) // fails since 1.1 * 0.11 = 0.121 > 0.1
//
// Args:
//
//		t: the testing.TB instance
//	 a: the first float
//	 b: the second float
func RequireNumericallyClose(t testing.TB, a, b float64, epsilon float64, msgAndArgs ...interface{}) {
	require.True(t, AreNumericallyClose(a, b, epsilon), msgAndArgs...)
}

// AreNumericallyClose returns true if the two floats are within epsilon of each other.
// The epsilon is calculated as:
//
//	epsilon = max(|a|, |b|) * epsilon
//
// Example:
//
//	AreNumericallyClose(1.0, 1.1, 0.1) // true since 1.0 * 0.1 = 0.1 < 0.1
//	AreNumericallyClose(1.0, 1.1, 0.01) // false since 1.0 * 0.01 = 0.01 < 0.1
//	AreNumericallyClose(1.0, 1.1, 0.11) // false since 1.1 * 0.11 = 0.121 > 0.1
//
// Args:
// a: the first float
// b: the second float
// epsilon: the epsilon value
// Returns:
// true if the two floats are within epsilon of each other
// false otherwise
func AreNumericallyClose(a, b float64, epsilon float64) bool {
	if a == float64(0) {
		return math.Abs(b) <= epsilon
	}
	if b == float64(0) {
		return math.Abs(a) <= epsilon
	}

	nominator := math.Abs(a - b)
	denominator := math.Max(math.Abs(a), math.Abs(b))
	return nominator/denominator <= epsilon
}
