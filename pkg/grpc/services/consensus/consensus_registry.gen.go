package consensus

import (
	"context"
	"fmt"

	gnode "github.com/dapperlabs/flow-go/pkg/network/gossip/v1"
	proto "github.com/golang/protobuf/proto"
)

type ConsensusServiceServerRegistry struct {
	css ConsensusServiceServer
}

// To make sure the class complies with the gnode.Registry interface
var _ gnode.Registry = (*ConsensusServiceServerRegistry)(nil)

func NewConsensusServiceServerRegistry(css ConsensusServiceServer) *ConsensusServiceServerRegistry {
	return &ConsensusServiceServerRegistry{
		css: css,
	}
}

func (cssr *ConsensusServiceServerRegistry) Ping(ctx context.Context, payloadByte []byte) ([]byte, error) {
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

func (cssr *ConsensusServiceServerRegistry) SubmitCollection(ctx context.Context, payloadByte []byte) ([]byte, error) {
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

func (cssr *ConsensusServiceServerRegistry) ProposeBlock(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &ProposeBlockRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := cssr.css.ProposeBlock(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (cssr *ConsensusServiceServerRegistry) UpdateProposedBlock(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &UpdateProposedBlockRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := cssr.css.UpdateProposedBlock(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (cssr *ConsensusServiceServerRegistry) GetBlockByHash(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &GetBlockByHashRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := cssr.css.GetBlockByHash(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (cssr *ConsensusServiceServerRegistry) GetBlockByHeight(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &GetBlockByHeightRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := cssr.css.GetBlockByHeight(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (cssr *ConsensusServiceServerRegistry) GetFinalizedStateTransitions(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &GetFinalizedStateTransitionsRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := cssr.css.GetFinalizedStateTransitions(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (cssr *ConsensusServiceServerRegistry) ProcessStateTransitionProposal(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &ProcessStateTransitionProposalRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := cssr.css.ProcessStateTransitionProposal(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (cssr *ConsensusServiceServerRegistry) ProcessStateTransitionPrepareVote(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &ProcessSignedStateTransitionPrepareVoteRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := cssr.css.ProcessStateTransitionPrepareVote(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (cssr *ConsensusServiceServerRegistry) ProcessStateTransitionCommitVote(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &ProcessSignedStateTransitionCommitVoteRequest{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := cssr.css.ProcessStateTransitionCommitVote(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (cssr *ConsensusServiceServerRegistry) MessageTypes() map[uint64]gnode.HandleFunc {
	return map[uint64]gnode.HandleFunc{
		0: cssr.Ping,
		1: cssr.SubmitCollection,
		2: cssr.ProposeBlock,
		3: cssr.UpdateProposedBlock,
		4: cssr.GetBlockByHash,
		5: cssr.GetBlockByHeight,
		6: cssr.GetFinalizedStateTransitions,
		7: cssr.ProcessStateTransitionProposal,
		8: cssr.ProcessStateTransitionPrepareVote,
		9: cssr.ProcessStateTransitionCommitVote,
	}
}

func (cssr *ConsensusServiceServerRegistry) NameMapping() map[string]uint64 {
	return map[string]uint64{
		"Ping":                              0,
		"SubmitCollection":                  1,
		"ProposeBlock":                      2,
		"UpdateProposedBlock":               3,
		"GetBlockByHash":                    4,
		"GetBlockByHeight":                  5,
		"GetFinalizedStateTransitions":      6,
		"ProcessStateTransitionProposal":    7,
		"ProcessStateTransitionPrepareVote": 8,
		"ProcessStateTransitionCommitVote":  9,
	}
}
