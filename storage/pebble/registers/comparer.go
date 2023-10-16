package registers

import "github.com/cockroachdb/pebble"

const (
	// Size of the block height encoded in the key.
	HeightSuffixLen = 8
)

// NewMVCCComparer creates a new comparer with a
// custom Split function that separates the height from the rest of the key.
//
// This is needed for SeekPrefixGE to work.
func NewMVCCComparer() *pebble.Comparer {
	comparer := *pebble.DefaultComparer
	comparer.Split = func(a []byte) int {
		return len(a) - HeightSuffixLen
	}
	comparer.Name = "flow.MVCCComparer"

	return &comparer
}
