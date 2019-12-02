package collection

import (
	"context"
	"fmt"

	registry "github.com/dapperlabs/flow-go/network/gossip/registry"
	proto "github.com/golang/protobuf/proto"
)

//go:generate stringer -type=registry.MessageType

const (
	Ping registry.MessageType = (iota + registry.DefaultTypes)
	SubmitTransaction
	SubmitCollection
	GetTransaction
	GetCollection
)

type CollectServiceServerRegistry struct {
	css CollectServiceServer
}

// To make sure the class complies with the registry.Registry interface
var _ registry.Registry = (*CollectServiceServerRegistry)(nil)

func NewCollectServiceServerRegistry(css CollectServiceServer) *CollectServiceServerRegistry {
	return &CollectServiceServerRegistry{
		css: css,
	}
}

func (cssr *CollectServiceServerRegistry) Ping(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &PingRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := cssr.css.Ping(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (cssr *CollectServiceServerRegistry) SubmitTransaction(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &SubmitTransactionRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := cssr.css.SubmitTransaction(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (cssr *CollectServiceServerRegistry) SubmitCollection(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &SubmitCollectionRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := cssr.css.SubmitCollection(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (cssr *CollectServiceServerRegistry) GetTransaction(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &GetTransactionRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := cssr.css.GetTransaction(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (cssr *CollectServiceServerRegistry) GetCollection(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &GetCollectionRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := cssr.css.GetCollection(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (cssr *CollectServiceServerRegistry) MessageTypes() map[registry.MessageType]registry.HandleFunc {
	return map[registry.MessageType]registry.HandleFunc{
		Ping:              cssr.Ping,
		SubmitTransaction: cssr.SubmitTransaction,
		SubmitCollection:  cssr.SubmitCollection,
		GetTransaction:    cssr.GetTransaction,
		GetCollection:     cssr.GetCollection,
	}
}
