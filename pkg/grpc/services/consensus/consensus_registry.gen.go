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

func (cssr *ConsensusServiceServerRegistry) MessageTypes() map[string]gnode.HandleFunc {
	return map[string]gnode.HandleFunc{
		"Ping":                              cssr.Ping,
		"SubmitCollection":                  cssr.SubmitCollection,
		"ProposeBlock":                      cssr.ProposeBlock,
		"UpdateProposedBlock":               cssr.UpdateProposedBlock,
		"GetBlockByHash":                    cssr.GetBlockByHash,
		"GetBlockByHeight":                  cssr.GetBlockByHeight,
		"GetFinalizedStateTransitions":      cssr.GetFinalizedStateTransitions,
		"ProcessStateTransitionProposal":    cssr.ProcessStateTransitionProposal,
		"ProcessStateTransitionPrepareVote": cssr.ProcessStateTransitionPrepareVote,
		"ProcessStateTransitionCommitVote":  cssr.ProcessStateTransitionCommitVote,
	}
}
