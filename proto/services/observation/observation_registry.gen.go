package observation

import (
	"context"
	"fmt"

	gossip "github.com/dapperlabs/flow-go/network/gossip"
	proto "github.com/golang/protobuf/proto"
)

type ObserveServiceServerRegistry struct {
	oss ObserveServiceServer
}

// To make sure the class complies with the gossip.Registry interface
var _ gossip.Registry = (*ObserveServiceServerRegistry)(nil)

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

func (ossr *ObserveServiceServerRegistry) CallScript(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &CallScriptRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := ossr.oss.CallScript(ctx, payload)

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

func (ossr *ObserveServiceServerRegistry) MessageTypes() map[uint64]gossip.HandleFunc {
	return map[uint64]gossip.HandleFunc{
		0: ossr.Ping,
		1: ossr.SendTransaction,
		2: ossr.GetLatestBlock,
		3: ossr.GetTransaction,
		4: ossr.GetAccount,
		5: ossr.CallScript,
		6: ossr.GetEvents,
	}
}

func (ossr *ObserveServiceServerRegistry) NameMapping() map[string]uint64 {
	return map[string]uint64{
		"Ping":            0,
		"SendTransaction": 1,
		"GetLatestBlock":  2,
		"GetTransaction":  3,
		"GetAccount":      4,
		"CallScript":      5,
		"GetEvents":       6,
	}
}
