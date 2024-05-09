package unittest_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestAreNumericallyClose(t *testing.T) {
	tests := []struct {
		name     string
		a        float64
		b        float64
		epsilon  float64
		expected bool
	}{
		{"close enough under epsilon", 1.0, 1.1, 0.1, true},
		{"not close under epsilon", 1.0, 1.1, 0.01, false},
		{"equal values", 2.0, 2.0, 0.1, true},
		{"zero epsilon with equal values", 2.0, 2.0, 0.0, true},
		{"zero epsilon with different values", 2.0, 2.1, 0.0, false},
		{"first value zero", 0, 0.1, 0.1, true},
		{"both values zero", 0, 0, 0.1, true},
		{"negative values close enough", -1.0, -1.1, 0.1, true},
		{"negative values not close enough", -1.0, -1.2, 0.1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := unittest.AreNumericallyClose(tt.a, tt.b, tt.epsilon)
			require.Equal(t, tt.expected, actual, "test Failed: %s", tt.name)
		})
	}
}
