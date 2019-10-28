package gnode

import (
	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
)

// generateGossipMessage initializes a new gossip message made from the given inputs
// payloadBytes: payloads of gossip message
// recipients:   list of recipients of gossip message
// msgType:      message type of gossip message
func generateGossipMessage(payloadBytes []byte, recipients []string, msgType uint64) *shared.GossipMessage {
	return &shared.GossipMessage{
		Payload:     payloadBytes,
		MessageType: msgType,
		Recipients:  recipients,
	}
}

// generateHashMessage inititializes a new hash message
// hashBytes:  main content of the hash message
// senderAddr: a socket representing the address of the sender of the generated hash message
func generateHashMessage(hashBytes []byte, senderAddr *shared.Socket) *shared.HashMessage {

	return &shared.HashMessage{
		HashBytes:    hashBytes,
		SenderSocket: senderAddr,
	}
}
