package signature

import (
	rand "math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCombinerJoinSplitEven(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	c := NewCombiner(32, 32)

	sig1 := make([]byte, 32)
	sig2 := make([]byte, 32)
	n, err := rand.Read(sig1)
	require.Equal(t, n, 32)
	require.NoError(t, err)
	n, err = rand.Read(sig2)
	require.Equal(t, n, 32)
	require.NoError(t, err)

	combined := c.Join(sig1, sig2)
	split1, split2, err := c.Split(combined)
	assert.NoError(t, err)
	assert.Equal(t, sig1, []byte(split1))
	assert.Equal(t, sig2, []byte(split2))
}

func TestCombinerJoinSplitUneven(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	len1 := rand.Intn(128) + 1
	len2 := rand.Intn(128) + 1
	c := NewCombiner(uint(len1), uint(len2))

	sig1 := make([]byte, len1)
	sig2 := make([]byte, len2)
	n, err := rand.Read(sig1)
	require.Equal(t, n, len1)
	require.NoError(t, err)
	n, err = rand.Read(sig2)
	require.Equal(t, n, len2)
	require.NoError(t, err)

	combined := c.Join(sig1, sig2)
	split1, split2, err := c.Split(combined)
	assert.Equal(t, sig1, []byte(split1))
	assert.Equal(t, sig2, []byte(split2))
}
