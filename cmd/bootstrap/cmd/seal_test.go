package cmd

import (
	"testing"

	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
)

func TestRandomSource_Deterministic(t *testing.T) {
	blockID := unittest.IdentifierFixture()

	randomSource1 := getRandomSource(blockID)
	randomSource2 := getRandomSource(blockID)
	assert.Equal(t, randomSource1, randomSource2)
}
