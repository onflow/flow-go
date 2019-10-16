package gnode

import (
	"github.com/google/uuid"

	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
)

// generateGossipMessage initializes a new gossip message made from the given inputs
func generateGossipMessage(payloadBytes []byte, recipients []string, msgType string) (*shared.GossipMessage, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	return &shared.GossipMessage{
		Uuid:        id.String(),
		Payload:     payloadBytes,
		MessageType: msgType,
		Recipients:  recipients,
	}, nil
}
