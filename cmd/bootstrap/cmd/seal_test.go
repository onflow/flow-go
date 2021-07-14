package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomSource_Deterministic(t *testing.T) {
	seed := generateRandomSeed()

	randomSource1 := getRandomSource(seed)
	randomSource2 := getRandomSource(seed)
	assert.Equal(t, randomSource1, randomSource2)
}
