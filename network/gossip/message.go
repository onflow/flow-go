package gossip

import "github.com/dapperlabs/flow-go/proto/gossip/messages"

// generateGossipMessage initializes a new gossip message made from the given inputs
func generateGossipMessage(payloadBytes []byte, recipients []string, msgType uint64) (*messages.GossipMessage, error) {
	return &messages.GossipMessage{
		Payload:     payloadBytes,
		MessageType: msgType,
		Recipients:  recipients,
	}, nil
}

// generateHashMessage inititializes a new hash message
func generateHashMessage(hashBytes []byte, senderAddr *messages.Socket) (*messages.HashMessage, error) {

	return &messages.HashMessage{
		HashBytes:    hashBytes,
		SenderSocket: senderAddr,
	}, nil
}
