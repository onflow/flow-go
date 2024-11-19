package models

import (
	"github.com/onflow/flow-go/model/flow"
)

// BlockMessageResponse is the response message for 'blocks' topic.
type BlockMessageResponse struct {
	// The sealed or finalized blocks according to the block status
	// in the request.
	Block *flow.Block `json:"block"`
}

// BlockHeaderMessageResponse is the response message for 'block_headers' topic.
type BlockHeaderMessageResponse struct {
	// The sealed or finalized block headers according to the block status
	// in the request.
	Header *flow.Header `json:"header"`
}
