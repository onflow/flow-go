package execute

import (
	"context"
	"fmt"

	gnode "github.com/dapperlabs/flow-go/pkg/network/gossip/v1"
	proto "github.com/golang/protobuf/proto"
)

type ExecuteServiceServerRegistry struct {
	ess ExecuteServiceServer
}

// To make sure the class complies with the gnode.Registry interface
var _ gnode.Registry = (*ExecuteServiceServerRegistry)(nil)

func NewExecuteServiceServerRegistry(ess ExecuteServiceServer) *ExecuteServiceServerRegistry {
	return &ExecuteServiceServerRegistry{
		ess: ess,
	}
}

func (essr *ExecuteServiceServerRegistry) Ping(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &PingRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := essr.ess.Ping(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (essr *ExecuteServiceServerRegistry) ExecuteBlock(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &ExecuteBlockRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := essr.ess.ExecuteBlock(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (essr *ExecuteServiceServerRegistry) NotifyBlockExecuted(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &NotifyBlockExecutedRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := essr.ess.NotifyBlockExecuted(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (essr *ExecuteServiceServerRegistry) GetRegisters(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &GetRegistersRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := essr.ess.GetRegisters(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (essr *ExecuteServiceServerRegistry) GetRegistersAtBlockHeight(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &GetRegistersAtBlockHeightRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := essr.ess.GetRegistersAtBlockHeight(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (essr *ExecuteServiceServerRegistry) MessageTypes() map[string]gnode.HandleFunc {
	return map[string]gnode.HandleFunc{
		"Ping":                      essr.Ping,
		"ExecuteBlock":              essr.ExecuteBlock,
		"NotifyBlockExecuted":       essr.NotifyBlockExecuted,
		"GetRegisters":              essr.GetRegisters,
		"GetRegistersAtBlockHeight": essr.GetRegistersAtBlockHeight,
	}
}
