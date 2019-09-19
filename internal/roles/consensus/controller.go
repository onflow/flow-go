package consensus

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	consensusSvc "github.com/dapperlabs/flow-go/pkg/grpc/services/consensus"
)

type Controller struct {
	dal *DAL
}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) Ping(context.Context, *consensusSvc.PingRequest) (*consensusSvc.PingResponse, error) {
	return &consensusSvc.PingResponse{
		Address: []byte("pong!"),
	}, nil
}

func (c *Controller) SubmitCollection(context.Context, *consensusSvc.SubmitCollectionRequest) (*empty.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) ProposeBlock(context.Context, *consensusSvc.ProposeBlockRequest) (*consensusSvc.ProposeBlockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) UpdateProposedBlock(context.Context, *consensusSvc.UpdateProposedBlockRequest) (*consensusSvc.UpdateProposedBlockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetBlockByHash(context.Context, *consensusSvc.GetBlockByHashRequest) (*consensusSvc.GetBlockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetBlockByHeight(context.Context, *consensusSvc.GetBlockByHeightRequest) (*consensusSvc.GetBlockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) GetFinalizedStateTransitions(context.Context, *consensusSvc.GetFinalizedStateTransitionsRequest) (*consensusSvc.GetFinalizedStateTransitionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) ProcessStateTransitionProposal(context.Context, *consensusSvc.ProcessStateTransitionProposalRequest) (*empty.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) ProcessStateTransitionPrepareVote(context.Context, *consensusSvc.ProcessSignedStateTransitionPrepareVoteRequest) (*empty.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (c *Controller) ProcessStateTransitionCommitVote(context.Context, *consensusSvc.ProcessSignedStateTransitionCommitVoteRequest) (*empty.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
