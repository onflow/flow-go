package main

import (
	"context"
	"fmt"

	gnode "github.com/dapperlabs/flow-go/pkg/network/gossip/v1"
	proto "github.com/golang/protobuf/proto"
)

type ReceiverServerRegistry struct {
	rs ReceiverServer
}

// To make sure the class complies with the gnode.Registry interface
var _ gnode.Registry = (*ReceiverServerRegistry)(nil)

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

func (rsr *ReceiverServerRegistry) MessageTypes() map[string]gnode.HandleFunc {
	return map[string]gnode.HandleFunc{
		"DisplayMessage": rsr.DisplayMessage,
	}
}
