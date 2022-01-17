package request

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestID_InvalidParse(t *testing.T) {
	var id ID

	tests := map[string]string{
		"0x1": "invalid",
		"":    "",
		"foo": "invalid",
	}

	for in, outErr := range tests {
		err := id.Parse(in)
		assert.EqualError(t, err, outErr)
	}
}
