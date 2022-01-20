package request

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestHeight_InvalidParse(t *testing.T) {
	var height Height

	tests := map[string]string{
		"f":                            "invalid height format",
		"0xffffffffffffffff":           "invalid height value",
		"-1":                           "invalid height format",
		fmt.Sprintf("%d", EmptyHeight): "invalid height value",
	}

	for in, outErr := range tests {
		err := height.Parse(in)
		assert.EqualError(t, err, outErr, fmt.Sprintf("test: value %s", in))
	}
}

func TestHeight_ValidParse(t *testing.T) {
	tests := map[string]uint64{
		"1":      uint64(1),
		"":       EmptyHeight,
		"sealed": SealedHeight,
		"final":  FinalHeight,
	}

	var height Height
	for in, out := range tests {
		err := height.Parse(in)
		assert.NoError(t, err)
		assert.Equal(t, height.Flow(), out)
	}

	var heights Heights
	err := heights.Parse([]string{"", "1", "2", "2", "2", ""})
	assert.NoError(t, err)
	assert.Equal(t, len(heights.Flow()), 2)
	assert.Equal(t, heights.Flow()[0], uint64(1))
	assert.Equal(t, heights.Flow()[1], uint64(2))
}
