package gnode

import (
	"github.com/twinj/uuid"

	"github.com/dapperlabs/bamboo-node/pkg/grpc/shared"
)

func generateGossipMessage(payloadBytes []byte, recipients []string, method string) (*shared.GossipMessage, error) {
	return &shared.GossipMessage{
		Uuid:       uuid.NewV4().String(),
		Payload:    payloadBytes,
		Method:     method,
		Recipients: recipients,
	}, nil
}
