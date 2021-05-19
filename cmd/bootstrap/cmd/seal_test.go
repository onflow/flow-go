package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestRandomSource_Deterministic(t *testing.T) {
	blockID := unittest.IdentifierFixture()

	randomSource1 := getRandomSource(blockID)
	randomSource2 := getRandomSource(blockID)
	assert.Equal(t, randomSource1, randomSource2)
}
