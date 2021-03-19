package unittest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// RequireContainBlock is a test helper that fails if specified blocks list does not contain the
// specified blocks.
func RequireContainBlock(t *testing.T, block *flow.Block, blocks []*flow.Block) {
	blockID := block.ID()
	for _, b := range blocks {
		if b.ID() == blockID {
			return
		}
	}

	require.Fail(t, fmt.Sprintf("block %x is not in the list %v", blockID, blocks))
}

// RequireContainBlock is a test helper that fails two blocks lists are not equal in their set of blocks.
// It ignores the order of blocks in the lists.
func RequireBlockListsMatchElements(t *testing.T, actual []*flow.Block, expected []*flow.Block) {
	require.Equal(t, len(actual), len(expected), fmt.Sprintf("block lists are not of same length actual: %d, expected: %d", len(actual), len(expected)))
	for _, e := range actual {
		RequireContainBlock(t, e, expected)
	}
}
