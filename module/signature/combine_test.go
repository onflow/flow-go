package signature

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCombinerJoinSplitEven(t *testing.T) {

	length := uint(32)
	c := NewCombiner(length, length)

	sig1 := randomByteSliceT(t, length)
	sig2 := randomByteSliceT(t, length)

	// test valid format
	combined, err := c.Join(sig1, sig2)
	require.NoError(t, err)
	split1, split2, err := c.Split(combined)
	assert.NoError(t, err)
	assert.Equal(t, sig1, []byte(split1))
	assert.Equal(t, sig2, []byte(split2))

	// test invalid
	invalidSig1 := randomByteSliceT(t, length-1)
	invalidSig2 := randomByteSliceT(t, length+1)

	_, err = c.Join(invalidSig1, sig2) // sig1 invalid
	assert.ErrorIs(t, err, ErrInvalidFormat)
	_, err = c.Join(sig1, invalidSig2) // sig2 is invalid
	assert.ErrorIs(t, err, ErrInvalidFormat)
	_, err = c.Join(invalidSig1, invalidSig2) // both lengths are invalid but total length is valid
	assert.ErrorIs(t, err, ErrInvalidFormat)
	_, _, err = c.Split(combined[:len(combined)-1])
	assert.ErrorIs(t, err, ErrInvalidFormat)
}

func TestCombinerJoinSplitUneven(t *testing.T) {

	len1 := uint(18)
	len2 := uint(27)
	c := NewCombiner(uint(len1), uint(len2))

	sig1 := randomByteSliceT(t, len1)
	sig2 := randomByteSliceT(t, len2)

	// test valid format
	combined, err := c.Join(sig1, sig2)
	require.NoError(t, err)
	split1, split2, err := c.Split(combined)
	require.NoError(t, err)
	assert.Equal(t, sig1, []byte(split1))
	assert.Equal(t, sig2, []byte(split2))

	// test invalid format
	invalidSig1 := randomByteSliceT(t, len1-1)
	invalidSig2 := randomByteSliceT(t, len2+1)

	_, err = c.Join(invalidSig1, sig2) // sig1 invalid
	assert.ErrorIs(t, err, ErrInvalidFormat)
	_, err = c.Join(sig1, invalidSig2) // sig2 is invalid
	assert.ErrorIs(t, err, ErrInvalidFormat)
	_, err = c.Join(invalidSig1, invalidSig2) // both lengths are invalid but total length is valid
	assert.ErrorIs(t, err, ErrInvalidFormat)
	_, _, err = c.Split(combined[:len(combined)-1])
	assert.ErrorIs(t, err, ErrInvalidFormat)
}
