package validation

import (
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p/inspector/internal"
)

// InspectRPCRequest represents a short digest of an RPC control message. It is used for further message inspection by component workers.
type InspectRPCRequest struct {
	// Nonce adds random value so that when msg req is stored on hero store a unique ID can be created from the struct fields.
	Nonce []byte
	// Peer sender of the message.
	Peer peer.ID
	rpc  *pubsub.RPC
}

// NewInspectRPCRequest returns a new *InspectRPCRequest.
func NewInspectRPCRequest(from peer.ID, rpc *pubsub.RPC) (*InspectRPCRequest, error) {
	nonce, err := internal.Nonce()
	if err != nil {
		return nil, fmt.Errorf("failed to get inspect message request nonce: %w", err)
	}
	return &InspectRPCRequest{Nonce: nonce, Peer: from, rpc: rpc}, nil
}
