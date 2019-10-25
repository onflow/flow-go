package collect

import (
	"context"
	"fmt"

	gnode "github.com/dapperlabs/flow-go/pkg/network/gossip/v1"
	proto "github.com/golang/protobuf/proto"
)

type CollectServiceServerRegistry struct {
	css CollectServiceServer
}

// To make sure the class complies with the gnode.Registry interface
var _ gnode.Registry = (*CollectServiceServerRegistry)(nil)

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

func (cssr *CollectServiceServerRegistry) MessageTypes() map[uint64]gnode.HandleFunc {
	return map[uint64]gnode.HandleFunc{
		0: cssr.Ping,
		1: cssr.SubmitTransaction,
		2: cssr.SubmitCollection,
		3: cssr.GetTransaction,
		4: cssr.GetCollection,
	}
}

func (cssr *CollectServiceServerRegistry) NameMapping() map[string]uint64 {
	return map[string]uint64{
		"Ping":              0,
		"SubmitTransaction": 1,
		"SubmitCollection":  2,
		"GetTransaction":    3,
		"GetCollection":     4,
	}
}
