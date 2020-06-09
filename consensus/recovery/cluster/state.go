package cluster

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/cluster"
	"github.com/dapperlabs/flow-go/storage"
)

// FindLatest retrieves the latest finalized header and all of its pending
// children. These are child blocks that have been verified by both the
// compliance layer and HotStuff and thus are safe to inject directly into
// HotStuff with no further validation.
func FindLatest(state cluster.State, headers storage.Headers) (*flow.Header, []*flow.Header, error) {

	finalized, err := state.Final().Head()
	if err != nil {
		return nil, nil, fmt.Errorf("could not get finalized header: %w", err)
	}

	pendingIDs, err := state.Final().Pending()
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

	return finalized, pending, nil
}
