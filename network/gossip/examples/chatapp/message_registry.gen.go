package main

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"

	"github.com/dapperlabs/flow-go/network/gossip"
)

type ReceiverServerRegistry struct {
	rs ReceiverServer
}

// To make sure the class complies with the gossip.Registry interface
var _ gossip.Registry = (*ReceiverServerRegistry)(nil)

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

func (rsr *ReceiverServerRegistry) MessageTypes() map[uint64]gossip.HandleFunc {
	return map[uint64]gossip.HandleFunc{
		0: rsr.DisplayMessage,
	}
}

func (rsr *ReceiverServerRegistry) NameMapping() map[string]uint64 {
	return map[string]uint64{
		"DisplayMessage": 0,
	}
}
