package gnode

import (
	"github.com/google/uuid"

	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
)

func generateGossipMessage(payloadBytes []byte, recipients []string, method string) (*shared.GossipMessage, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	return &shared.GossipMessage{
		Uuid:       id.String(),
		Payload:    payloadBytes,
		Method:     method,
		Recipients: recipients,
	}, nil
}
