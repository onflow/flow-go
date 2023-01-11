package seed

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getRandomSource(t *testing.T) []byte {
	r := time.Now().UnixNano()
	rand.Seed(r)
	t.Logf("math rand seed is %d", r)
	seed := make([]byte, RandomSourceLength)
	rand.Read(seed)
	return seed
}

// check PRGs created from the same source give the same outputs
func TestDeterministic(t *testing.T) {
	seed := getRandomSource(t)
	customizer := []byte("test")
	prg1, err := PRGFromRandomSource(seed, customizer)
	require.NoError(t, err)
	prg2, err := PRGFromRandomSource(seed, customizer)
	require.NoError(t, err)

	rand1 := make([]byte, 100)
	prg1.Read(rand1)
	rand2 := make([]byte, 100)
	prg2.Read(rand2)

	assert.Equal(t, rand1, rand2)
}

func TestCustomizer(t *testing.T) {
	seed := getRandomSource(t)
	customizer1 := []byte("test1")
	prg1, err := PRGFromRandomSource(seed, customizer1)
	require.NoError(t, err)
	customizer2 := []byte("test2")
	prg2, err := PRGFromRandomSource(seed, customizer2)
	require.NoError(t, err)

	rand1 := make([]byte, 100)
	prg1.Read(rand1)
	rand2 := make([]byte, 100)
	prg2.Read(rand2)

	assert.NotEqual(t, rand1, rand2)
}
