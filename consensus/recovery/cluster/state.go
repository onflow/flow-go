package cluster

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/cluster"
	"github.com/dapperlabs/flow-go/storage"
)

// FindLatest retrieves the latest finalized header and all of its pending
// children. These pending children have been verified by the compliance layer
// but are NOT guaranteed to have been verified by HotStuff. They MUST be
// validated by HotStuff during the recovery process.
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
