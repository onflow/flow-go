package sema

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestOptionalSubtyping(t *testing.T) {

	assert.True(t,
		IsSubType(
			&OptionalType{Type: &IntType{}},
			&OptionalType{Type: &IntType{}},
		),
	)

	assert.False(t,
		IsSubType(
			&OptionalType{Type: &IntType{}},
			&OptionalType{Type: &BoolType{}},
		),
	)

	assert.True(t,
		IsSubType(
			&OptionalType{Type: &Int8Type{}},
			&OptionalType{Type: &IntegerType{}},
		),
	)
}
