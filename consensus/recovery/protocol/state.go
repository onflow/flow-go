package protocol

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// FindLatest retrieves the latest finalized header and all of its pending
// children. These pending children have been verified by the compliance layer
// but are NOT guaranteed to have been verified by HotStuff. They MUST be
// validated by HotStuff during the recovery process.
func FindLatest(state protocol.State, headers storage.Headers, rootHeader *flow.Header) (*flow.Header, []*flow.Header, error) {
	// find finalized block
	finalized, err := state.Final().Head()

	if err != nil {
		return nil, nil, fmt.Errorf("could not find finalized block")
	}

	// find all pending blockIDs
	pendingIDs, err := state.Final().Pending()
	if err != nil {
		return nil, nil, fmt.Errorf("could not find pending block")
	}

	// find all pending header by ID
	pending := make([]*flow.Header, 0, len(pendingIDs))
	for _, pendingID := range pendingIDs {
		pendingHeader, err := headers.ByBlockID(pendingID)
		if err != nil {
			return nil, nil, fmt.Errorf("could not find pending block by ID: %w", err)
		}
		pending = append(pending, pendingHeader)
	}

	return finalized, pending, nil
}
