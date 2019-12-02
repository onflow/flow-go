package observation

import (
	"context"
	"fmt"

	registry "github.com/dapperlabs/flow-go/network/gossip/registry"
	proto "github.com/golang/protobuf/proto"
)

//go:generate stringer -type=registry.MessageType

const (
	Ping registry.MessageType = (iota + registry.DefaultTypes)
	SendTransaction
	GetLatestBlock
	GetTransaction
	GetAccount
	ExecuteScript
	GetEvents
)

type ObserveServiceServerRegistry struct {
	oss ObserveServiceServer
}

// To make sure the class complies with the registry.Registry interface
var _ registry.Registry = (*ObserveServiceServerRegistry)(nil)

func NewObserveServiceServerRegistry(oss ObserveServiceServer) *ObserveServiceServerRegistry {
	return &ObserveServiceServerRegistry{
		oss: oss,
	}
}

func (ossr *ObserveServiceServerRegistry) Ping(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &PingRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := ossr.oss.Ping(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (ossr *ObserveServiceServerRegistry) SendTransaction(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &SendTransactionRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := ossr.oss.SendTransaction(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (ossr *ObserveServiceServerRegistry) GetLatestBlock(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &GetLatestBlockRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := ossr.oss.GetLatestBlock(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (ossr *ObserveServiceServerRegistry) GetTransaction(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &GetTransactionRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := ossr.oss.GetTransaction(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (ossr *ObserveServiceServerRegistry) GetAccount(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &GetAccountRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := ossr.oss.GetAccount(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (ossr *ObserveServiceServerRegistry) ExecuteScript(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &ExecuteScriptRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := ossr.oss.ExecuteScript(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (ossr *ObserveServiceServerRegistry) GetEvents(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &GetEventsRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := ossr.oss.GetEvents(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (ossr *ObserveServiceServerRegistry) MessageTypes() map[registry.MessageType]registry.HandleFunc {
	return map[registry.MessageType]registry.HandleFunc{
		Ping:            ossr.Ping,
		SendTransaction: ossr.SendTransaction,
		GetLatestBlock:  ossr.GetLatestBlock,
		GetTransaction:  ossr.GetTransaction,
		GetAccount:      ossr.GetAccount,
		ExecuteScript:   ossr.ExecuteScript,
		GetEvents:       ossr.GetEvents,
	}
}
