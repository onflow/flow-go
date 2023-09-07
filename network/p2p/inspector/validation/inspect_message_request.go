package validation

import (
	"fmt"

	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p/inspector/internal"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
)

// InspectMsgRequest represents a short digest of an RPC control message. It is used for further message inspection by component workers.
type InspectMsgRequest struct {
	// Nonce adds random value so that when msg req is stored on hero store a unique ID can be created from the struct fields.
	Nonce []byte
	// Peer sender of the message.
	Peer peer.ID
	// CtrlMsg the control message that will be inspected.
	ctrlMsg          *pubsub_pb.ControlMessage
	validationConfig *p2pconf.CtrlMsgValidationConfig
}

// NewInspectMsgRequest returns a new *InspectMsgRequest.
func NewInspectMsgRequest(from peer.ID, validationConfig *p2pconf.CtrlMsgValidationConfig, ctrlMsg *pubsub_pb.ControlMessage) (*InspectMsgRequest, error) {
	nonce, err := internal.Nonce()
	if err != nil {
		return nil, fmt.Errorf("failed to get inspect message request nonce: %w", err)
	}
	return &InspectMsgRequest{Nonce: nonce, Peer: from, validationConfig: validationConfig, ctrlMsg: ctrlMsg}, nil
}
