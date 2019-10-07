package parser

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseIncomplete(t *testing.T) {

	program, inputIsComplete, err := ParseProgram("struct X")

	assert.Nil(t, program)
	assert.False(t, inputIsComplete)
	assert.NotNil(t, err)
}
