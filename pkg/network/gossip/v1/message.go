package gnode

import (
	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
)

// generateGossipMessage initializes a new gossip message made from the given inputs
func generateGossipMessage(payloadBytes []byte, recipients []string, msgType string) (*shared.GossipMessage, error) {
	return &shared.GossipMessage{
		Payload:     payloadBytes,
		MessageType: msgType,
		Recipients:  recipients,
	}, nil
}

// generateHashMessage inititializes a new hash message
func generateHashMessage(hashBytes []byte, senderAddr string) (*shared.HashMessage, error) {
	return &shared.HashMessage{
		HashBytes:  hashBytes,
		SenderAddr: senderAddr,
	}, nil
}
