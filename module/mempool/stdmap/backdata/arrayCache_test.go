package backdata

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBucketIndex(t *testing.T) {
	// global index of 0 belongs to element 0 of bucket 0
	bIndex, eIndex := bucketIndex(uint(1))
	require.Equal(t, bIndex, uint(0))
	require.Equal(t, eIndex, uint(0))
}
