package main

import (
	"context"
	"fmt"

	registry "github.com/dapperlabs/flow-go/network/gossip/registry"
	proto "github.com/golang/protobuf/proto"
)

//go:generate stringer -type=registry.MessageType

const (
	DisplayMessage registry.MessageType = (iota + registry.DefaultTypes)
)

type ReceiverServerRegistry struct {
	rs ReceiverServer
}

// To make sure the class complies with the registry.Registry interface
var _ registry.Registry = (*ReceiverServerRegistry)(nil)

func NewReceiverServerRegistry(rs ReceiverServer) *ReceiverServerRegistry {
	return &ReceiverServerRegistry{
		rs: rs,
	}
}

func (rsr *ReceiverServerRegistry) DisplayMessage(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &Message{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := rsr.rs.DisplayMessage(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (rsr *ReceiverServerRegistry) MessageTypes() map[registry.MessageType]registry.HandleFunc {
	return map[registry.MessageType]registry.HandleFunc{
		DisplayMessage: rsr.DisplayMessage,
	}
}
