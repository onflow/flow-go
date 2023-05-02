package cluster

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/storage"
)

// FindLatest returns:
//   - [first value] latest finalized header
//   - [second value] all known descendants (i.e. pending blocks)
//   - No errors expected during normal operations.
//
// All returned blocks have been verified by the compliance layer, i.e. they are guaranteed to be valid.
// The descendants are listed in ancestor-first order, i.e. for any block B = descendants[i], B's parent
// must be included at an index _smaller_ than i, unless B's parent is the latest finalized block.
//
// Note: this is an expensive method, which is intended to help recover from a crash, e.g. help to
// re-built the in-memory consensus state.
func FindLatest(state cluster.State, headers storage.Headers) (*flow.Header, []*flow.Header, error) {
	finalizedSnapshot := state.Final()              // state snapshot at latest finalized block
	finalizedBlock, err := finalizedSnapshot.Head() // header of latest finalized block
	if err != nil {
		return nil, nil, fmt.Errorf("could not get finalized header: %w", err)
	}
	pendingIDs, err := finalizedSnapshot.Pending() // find IDs of all blocks descending from the finalized block
	if err != nil {
		return nil, nil, fmt.Errorf("could not get pending children: %w", err)
	}

	pending := make([]*flow.Header, 0, len(pendingIDs))
	for _, pendingID := range pendingIDs {
		header, err := headers.ByBlockID(pendingID)
		if err != nil {
			return nil, nil, fmt.Errorf("could not find pending child: %w", err)
		}
		pending = append(pending, header)
	}

	return finalizedBlock, pending, nil
}
