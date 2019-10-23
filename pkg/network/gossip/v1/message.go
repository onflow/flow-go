package gnode

import (
	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
)

// generateGossipMessage initializes a new gossip message made from the given inputs
func generateGossipMessage(payloadBytes []byte, recipients []string, msgType uint64) (*shared.GossipMessage, error) {
	return &shared.GossipMessage{
		Payload:     payloadBytes,
		MessageType: msgType,
		Recipients:  recipients,
	}, nil
}

// generateHashMessage inititializes a new hash message
func generateHashMessage(hashBytes []byte, senderAddr *shared.Socket) (*shared.HashMessage, error) {

	return &shared.HashMessage{
		HashBytes:    hashBytes,
		SenderSocket: senderAddr,
	}, nil
}
