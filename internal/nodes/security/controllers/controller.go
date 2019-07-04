package controllers

import (
	"context"

	empty "github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	bambooProto "github.com/dapperlabs/bamboo-node/grpc/shared"
	"github.com/dapperlabs/bamboo-node/internal/nodes/security/data"
)

// Controller implements SecurityNodeServer interface
type Controller struct {
	dal *data.DAL
}

// NewController ..
func NewController(dal *data.DAL) *Controller {
	return &Controller{dal: dal}
}

func (c *Controller) Ping(context.Context, *bambooProto.PingRequest) (*bambooProto.PingResponse, error) {
	return &bambooProto.PingResponse{
		Address: []byte("ping pong!"),
	}, nil
}

// Receive a signed collection from an access node.
func (c *Controller) AccessSubmitCollection(context.Context, *bambooProto.AccessCollectionRequest) (*bambooProto.AccessCollectionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// Notify the security node that another has proposed a block.
func (c *Controller) ProposeBlock(context.Context, *bambooProto.ProposeBlockRequest) (*bambooProto.ProposeBlockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// Update a block proposal to add new signatures.
func (c *Controller) UpdateProposedBlock(context.Context, *bambooProto.ProposeBlockUpdateRequest) (*bambooProto.ProposeBlockUpdateResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// Returns a block by hash
func (c *Controller) GetBlockByHash(context.Context, *bambooProto.GetBlockByHashRequest) (*bambooProto.GetBlockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// Returns a block by height
func (c *Controller) GetBlockByHeight(context.Context, *bambooProto.GetBlockByHeightRequest) (*bambooProto.GetBlockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// Process result approval from access nodes.
func (c *Controller) ProcessResultApproval(context.Context, *bambooProto.ProcessResultApprovalRequest) (*bambooProto.ProcessResultApprovalResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// Returns the finalized state transitions at the requested heights.
func (c *Controller) GetFinalizedStateTransitions(context.Context, *bambooProto.FinalizedStateTransitionsRequest) (*bambooProto.FinalizedStateTransitionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// Process state transition proposal from other security node.
func (c *Controller) ProcessStateTransitionProposal(context.Context, *bambooProto.SignedStateTransition) (*empty.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// Process state transition prepare vote from other security node.
func (c *Controller) ProcessStateTransitionPrepareVote(context.Context, *bambooProto.SignedStateTransitionPrepareVote) (*empty.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// Process state transition commit vote from other security node.
func (c *Controller) ProcessStateTransitionCommitVote(context.Context, *bambooProto.SignedStateTransitionCommitVote) (*empty.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// Process execution result from execute nodes to propose block seals
func (c *Controller) ProcessExecutionReceipt(context.Context, *bambooProto.ProcessExecutionReceiptRequest) (*bambooProto.ProcessExecutionReceiptResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// Receive an execution receipt challenge.
func (c *Controller) SubmitInvalidExecutionReceiptChallenge(context.Context, *bambooProto.InvalidExecutionReceiptChallengeRequest) (*bambooProto.InvalidExecutionReceiptChallengeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
