package rpc

import (
	"context"

	"github.com/rs/zerolog"

	obs "github.com/dapperlabs/flow-go/engine/observation"
	"github.com/dapperlabs/flow-go/engine/observation/rpc/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	"github.com/dapperlabs/flow-go/protocol"
)

// Handler implements a subset of the Observation API
type Handler struct {
	executionRPC  observation.ObserveServiceClient
	collectionRPC observation.ObserveServiceClient
	log           zerolog.Logger
	state         protocol.State
	blkState      *obs.BlockchainState
}

func NewHandler(log zerolog.Logger,
	s protocol.State,
	e observation.ObserveServiceClient,
	c observation.ObserveServiceClient,
	bcst *obs.BlockchainState) *Handler {
	return &Handler{
		executionRPC:  e,
		collectionRPC: c,
		blkState:      bcst,
		state:         s,
		log:           log,
	}
}

// Ping responds to requests when the server is up.
func (h *Handler) Ping(ctx context.Context, req *observation.PingRequest) (*observation.PingResponse, error) {
	return &observation.PingResponse{}, nil
}

func (h *Handler) ExecuteScript(ctx context.Context, req *observation.ExecuteScriptRequest) (*observation.ExecuteScriptResponse, error) {
	return h.executionRPC.ExecuteScript(ctx, req)
}

// SendTransaction forwards the transaction to the collection node
func (h *Handler) SendTransaction(ctx context.Context, req *observation.SendTransactionRequest) (*observation.SendTransactionResponse, error) {

	return h.collectionRPC.SendTransaction(ctx, req)
}

func (h *Handler) GetLatestBlock(ctx context.Context, req *observation.GetLatestBlockRequest) (*observation.GetLatestBlockResponse, error) {
	var header *flow.Header
	var seal flow.Seal
	var err error

	// If request if for the latest Sealed block, lookup the latest seal to get latest blockid
	// The follower engine should have updated the state
	if req.IsSealed {
		seal, err = h.state.Final().Seal()
		if err != nil {
			return nil, err
		}
		header, err = h.blkState.Block(seal.BlockID)
	} else {
		// Otherwise, for the latest finalized block, query the state
		header, err = h.state.Final().Head()
	}
	if err != nil {
		return nil, err
	}

	msg, err := convert.BlockHeaderToMessage(header)
	if err != nil {
		return nil, err
	}

	resp := &observation.GetLatestBlockResponse{
		Block: &msg,
	}
	return resp, nil
}

func (h *Handler) GetTransaction(context.Context, *observation.GetTransactionRequest) (*observation.GetTransactionResponse, error) {
	// TODO lookup transaction in local transaction storage
	return nil, nil
}

func (h *Handler) GetAccount(context.Context, *observation.GetAccountRequest) (*observation.GetAccountResponse, error) {
	return nil, nil
}

func (h *Handler) GetEvents(context.Context, *observation.GetEventsRequest) (*observation.GetEventsResponse, error) {
	return nil, nil
}
