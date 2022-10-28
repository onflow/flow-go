package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsStructureInvalidError(t *testing.T) {
	err := NewStructureInvalidError("")
	assert.True(t, IsStructureInvalidError(err))
}
