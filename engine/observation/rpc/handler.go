package rpc

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine/common/observerclient"
	"github.com/dapperlabs/flow-go/engine/observation/rpc/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// Handler implements a subset of the Observation API
type Handler struct {
	observerclient.NullObserverClient
	executionRPC  observation.ObserveServiceClient
	collectionRPC observation.ObserveServiceClient
	log           zerolog.Logger
	state         protocol.State

	// storage
	headers storage.Headers
}

func NewHandler(log zerolog.Logger,
	s protocol.State,
	e observation.ObserveServiceClient,
	c observation.ObserveServiceClient,
	headers storage.Headers) *Handler {
	return &Handler{
		executionRPC:       e,
		collectionRPC:      c,
		headers:            headers,
		state:              s,
		log:                log,
		NullObserverClient: observerclient.NullObserverClient{},
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

func (h *Handler) GetLatestBlockHeader(ctx context.Context, req *observation.GetLatestBlockHeaderRequest) (*observation.BlockHeaderResponse, error) {

	header, err := h.getLatestBlockHeader(req.IsSealed)

	if err != nil {
		return nil, err
	}

	msg, err := convert.BlockHeaderToMessage(header)
	if err != nil {
		return nil, err
	}

	resp := &observation.BlockHeaderResponse{
		Block: &msg,
	}
	return resp, nil
}

func (h *Handler) getLatestBlockHeader(isSealed bool) (*flow.Header, error) {
	// If latest Sealed block is needed, lookup the latest seal to get latest blockid
	// then query storage for that blockid
	if isSealed {
		seal, err := h.state.Final().Seal()
		if err != nil {
			return nil, err
		}
		return h.headers.ByBlockID(seal.BlockID)
	}
	// Otherwise, for the latest finalized block, just query the state
	return h.state.Final().Head()

}
