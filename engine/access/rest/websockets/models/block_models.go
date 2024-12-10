package models

import (
	"github.com/onflow/flow-go/engine/access/rest/common/models"
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
	Header *models.BlockHeader `json:"header"`
}

// BlockDigestMessageResponse is the response message for 'block_digests' topic.
type BlockDigestMessageResponse struct {
	// The sealed or finalized block digest according to the block status
	// in the request.
	Block *BlockDigest `json:"block_digest"`
}
