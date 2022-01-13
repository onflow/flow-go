package request

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestArguments_InvalidParse(t *testing.T) {
	var arguments Arguments

	args := [][]string{
		{"foo"},
		{"-1"},
		{"dGVzdA==", "foo"},
	}

	for _, a := range args {
		err := arguments.Parse(a)
		assert.EqualError(t, err, "invalid argument encoding: illegal base64 data at input byte 0", a)
	}

	tooLong := make([]string, maxAllowedScriptArguments+1)
	for i := range tooLong {
		tooLong[i] = "dGVzdA=="
	}

	err := arguments.Parse(tooLong)
	assert.EqualError(t, err, fmt.Sprintf("too many arguments. Maximum arguments allowed: %d", maxAllowedScriptArguments))
}

func TestArguments_ValidParse(t *testing.T) {
	var arguments Arguments

	args := [][]string{
		{"dGVzdA=="},
		{},
		{""},
		{"dGVzdA==", "dGVzdCB0ZXN0IHRlc3QgMgo="},
	}

	for _, a := range args {
		err := arguments.Parse(a)
		assert.NoError(t, err)
	}
}
