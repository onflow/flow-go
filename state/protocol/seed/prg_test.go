package seed

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getRandomSource(t *testing.T) []byte {
	seed := make([]byte, RandomSourceLength)
	rand.Read(seed)
	t.Logf("seed is %#x", seed)
	return seed
}

func getRandoms(t *testing.T, seed, customizer, diversifier []byte, N int) []byte {
	prg, err := PRGFromRandomSource(seed, customizer, diversifier)
	require.NoError(t, err)
	rand := make([]byte, N)
	prg.Read(rand)
	return rand
}

// check PRGs created from the same source give the same outputs
func TestDeterministic(t *testing.T) {
	seed := getRandomSource(t)
	customizer := []byte("cust test")
	diversifer := []byte("div test")

	rand1 := getRandoms(t, seed, customizer, diversifer, 100)
	rand2 := getRandoms(t, seed, customizer, diversifer, 100)
	assert.Equal(t, rand1, rand2)
}

// check different cutomizers lead to different randoms
func TestDifferentInstances(t *testing.T) {
	seed := getRandomSource(t)
	customizer1 := []byte("cust test1")
	customizer2 := []byte("cust test2")
	diversifer1 := []byte("div test1")
	diversifer2 := []byte("div test2")
	// different custmizers
	rand1 := getRandoms(t, seed, customizer1, diversifer1, 2)
	rand2 := getRandoms(t, seed, customizer2, diversifer1, 2)
	assert.NotEqual(t, rand1, rand2)
	// different custmizers
	rand1 = getRandoms(t, seed, customizer1, diversifer1, 2)
	rand2 = getRandoms(t, seed, customizer1, diversifer2, 2)
	assert.NotEqual(t, rand1, rand2)
	// test no error is returned with empty custmizer and diversifier, should not error
	_ = getRandoms(t, seed, nil, nil, 2)
}

// Sanity check that all customizers used by the Flow protocol
// are different and are not prefixes of each other
func TestProtocolConstants(t *testing.T) {
	// include all sub-protocol customizers
	customizers := [][]byte{
		ConsensusLeaderSelection,
		VerificationChunkAssignment,
		ExecutionEnvironment,
		customizerFromIndices(clusterLeaderSelectionPrefix...),
	}

	// go through all couples
	for i, c := range customizers {
		for j, other := range customizers {
			if i == j {
				continue
			}
			assert.False(t, bytes.HasPrefix(c, other))
		}
	}
}
