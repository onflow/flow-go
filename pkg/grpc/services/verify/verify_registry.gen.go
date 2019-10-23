package verify

import (
	"context"
	"fmt"

	gnode "github.com/dapperlabs/flow-go/pkg/network/gossip/v1"
	proto "github.com/golang/protobuf/proto"
)

type VerifyServiceServerRegistry struct {
	vss VerifyServiceServer
}

// To make sure the class complies with the gnode.Registry interface
var _ gnode.Registry = (*VerifyServiceServerRegistry)(nil)

func NewVerifyServiceServerRegistry(vss VerifyServiceServer) *VerifyServiceServerRegistry {
	return &VerifyServiceServerRegistry{
		vss: vss,
	}
}

func (vssr *VerifyServiceServerRegistry) Ping(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &PingRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := vssr.vss.Ping(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (vssr *VerifyServiceServerRegistry) SubmitExecutionReceipt(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &SubmitExecutionReceiptRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := vssr.vss.SubmitExecutionReceipt(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (vssr *VerifyServiceServerRegistry) MessageTypes() map[uint64]gnode.HandleFunc {
	return map[uint64]gnode.HandleFunc{
		0: vssr.Ping,
		1: vssr.SubmitExecutionReceipt,
	}
}

func (vssr *VerifyServiceServerRegistry) NameMapping() map[string]uint64 {
	return map[string]uint64{
		"Ping":                   0,
		"SubmitExecutionReceipt": 1,
	}
}
