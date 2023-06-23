package rest

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/api"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/rest/routes"
	"github.com/onflow/flow-go/model/flow"
)

// ServerRequestHandler is a structure that represents local requests
type ServerRequestHandler struct {
	log     zerolog.Logger
	backend access.API
}

var _ api.RestServerApi = (*ServerRequestHandler)(nil)

// NewServerRequestHandler returns new ServerRequestHandler.
func NewServerRequestHandler(log zerolog.Logger, backend access.API) *ServerRequestHandler {
	return &ServerRequestHandler{
		log:     log,
		backend: backend,
	}
}

// GetTransactionByID gets a transaction by requested ID.
func (h *ServerRequestHandler) GetTransactionByID(r request.GetTransaction, context context.Context, link models.LinkGenerator, _ flow.Chain) (models.Transaction, error) {
	var response models.Transaction

	tx, err := h.backend.GetTransaction(context, r.ID)
	if err != nil {
		return response, err
	}

	var txr *access.TransactionResult
	// only lookup result if transaction result is to be expanded
	if r.ExpandsResult {
		txr, err = h.backend.GetTransactionResult(context, r.ID, r.BlockID, r.CollectionID)
		if err != nil {
			return response, err
		}
	}

	response.Build(tx, txr, link)
	return response, nil
}

// CreateTransaction creates a new transaction from provided payload.
func (h *ServerRequestHandler) CreateTransaction(r request.CreateTransaction, context context.Context, link models.LinkGenerator) (models.Transaction, error) {
	var response models.Transaction

	err := h.backend.SendTransaction(context, &r.Transaction)
	if err != nil {
		return response, err
	}

	response.Build(&r.Transaction, nil, link)
	return response, nil
}

// GetTransactionResultByID retrieves transaction result by the transaction ID.
func (h *ServerRequestHandler) GetTransactionResultByID(r request.GetTransactionResult, context context.Context, link models.LinkGenerator) (models.TransactionResult, error) {
	var response models.TransactionResult

	txr, err := h.backend.GetTransactionResult(context, r.ID, r.BlockID, r.CollectionID)
	if err != nil {
		return response, err
	}

	response.Build(txr, r.ID, link)
	return response, nil
}

// GetBlocksByIDs gets blocks by provided ID or list of IDs.
func (h *ServerRequestHandler) GetBlocksByIDs(r request.GetBlockByIDs, context context.Context, expandFields map[string]bool, link models.LinkGenerator) ([]*models.Block, error) {
	blocks := make([]*models.Block, len(r.IDs))

	for i, id := range r.IDs {
		block, err := routes.GetBlock(routes.ForID(&id), context, expandFields, h.backend, link)
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}

	return blocks, nil
}

// GetBlocksByHeight gets blocks by provided height.
func (h *ServerRequestHandler) GetBlocksByHeight(r *request.Request, link models.LinkGenerator) ([]*models.Block, error) {
	req, err := r.GetBlockRequest()
	if err != nil {
		return nil, models.NewBadRequestError(err)
	}

	if req.FinalHeight || req.SealedHeight {
		block, err := routes.GetBlock(routes.ForFinalized(req.Heights[0]), r.Context(), r.ExpandFields, h.backend, link)
		if err != nil {
			return nil, err
		}

		return []*models.Block{block}, nil
	}

	// if the query is /blocks/height=1000,1008,1049...
	if req.HasHeights() {
		blocks := make([]*models.Block, len(req.Heights))
		for i, height := range req.Heights {
			block, err := routes.GetBlock(routes.ForHeight(height), r.Context(), r.ExpandFields, h.backend, link)
			if err != nil {
				return nil, err
			}
			blocks[i] = block
		}

		return blocks, nil
	}

	// support providing end height as "sealed" or "final"
	if req.EndHeight == request.FinalHeight || req.EndHeight == request.SealedHeight {
		latest, _, err := h.backend.GetLatestBlock(r.Context(), req.EndHeight == request.SealedHeight)
		if err != nil {
			return nil, err
		}

		req.EndHeight = latest.Header.Height // overwrite special value height with fetched

		if req.StartHeight > req.EndHeight {
			return nil, models.NewBadRequestError(fmt.Errorf("start height must be less than or equal to end height"))
		}
	}

	blocks := make([]*models.Block, 0)
	// start and end height inclusive
	for i := req.StartHeight; i <= req.EndHeight; i++ {
		block, err := routes.GetBlock(routes.ForHeight(i), r.Context(), r.ExpandFields, h.backend, link)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

// GetBlockPayloadByID gets block payload by ID
func (h *ServerRequestHandler) GetBlockPayloadByID(r request.GetBlockPayload, context context.Context, _ models.LinkGenerator) (models.BlockPayload, error) {
	var payload models.BlockPayload

	blkProvider := routes.NewBlockRequestProvider(h.backend, routes.ForID(&r.ID))
	blk, _, statusErr := blkProvider.GetBlock(context)
	if statusErr != nil {
		return payload, statusErr
	}

	err := payload.Build(blk.Payload)
	if err != nil {
		return payload, err
	}

	return payload, nil
}

// GetExecutionResultByID gets execution result by the ID.
func (h *ServerRequestHandler) GetExecutionResultByID(r request.GetExecutionResult, context context.Context, link models.LinkGenerator) (models.ExecutionResult, error) {
	var response models.ExecutionResult

	res, err := h.backend.GetExecutionResultByID(context, r.ID)
	if err != nil {
		return response, err
	}

	if res == nil {
		err := fmt.Errorf("execution result with ID: %s not found", r.ID.String())
		return response, models.NewNotFoundError(err.Error(), err)
	}

	err = response.Build(res, link)
	if err != nil {
		return response, err
	}

	return response, nil
}

// GetExecutionResultsByBlockIDs gets Execution Result payload by block IDs.
func (h *ServerRequestHandler) GetExecutionResultsByBlockIDs(r request.GetExecutionResultByBlockIDs, context context.Context, link models.LinkGenerator) ([]models.ExecutionResult, error) {
	// for each block ID we retrieve execution result
	results := make([]models.ExecutionResult, len(r.BlockIDs))
	for i, id := range r.BlockIDs {
		res, err := h.backend.GetExecutionResultForBlockID(context, id)
		if err != nil {
			return nil, err
		}

		var response models.ExecutionResult
		err = response.Build(res, link)
		if err != nil {
			return nil, err
		}
		results[i] = response
	}

	return results, nil
}

// GetCollectionByID retrieves a collection by ID and builds a response
func (h *ServerRequestHandler) GetCollectionByID(r request.GetCollection, context context.Context, expandFields map[string]bool, link models.LinkGenerator, _ flow.Chain) (models.Collection, error) {
	var response models.Collection

	collection, err := h.backend.GetCollectionByID(context, r.ID)
	if err != nil {
		return response, err
	}

	// if we expand transactions in the query retrieve each transaction data
	transactions := make([]*flow.TransactionBody, 0)
	if r.ExpandsTransactions {
		for _, tid := range collection.Transactions {
			tx, err := h.backend.GetTransaction(context, tid)
			if err != nil {
				return response, err
			}

			transactions = append(transactions, tx)
		}
	}

	err = response.Build(collection, transactions, link, expandFields)
	if err != nil {
		return response, err
	}

	return response, nil
}

// ExecuteScript handler sends the script from the request to be executed.
func (h *ServerRequestHandler) ExecuteScript(r request.GetScript, context context.Context, _ models.LinkGenerator) ([]byte, error) {
	if r.BlockID != flow.ZeroID {
		return h.backend.ExecuteScriptAtBlockID(context, r.BlockID, r.Script.Source, r.Script.Args)
	}

	// default to sealed height
	if r.BlockHeight == request.SealedHeight || r.BlockHeight == request.EmptyHeight {
		return h.backend.ExecuteScriptAtLatestBlock(context, r.Script.Source, r.Script.Args)
	}

	if r.BlockHeight == request.FinalHeight {
		finalBlock, _, err := h.backend.GetLatestBlockHeader(context, false)
		if err != nil {
			return nil, err
		}
		r.BlockHeight = finalBlock.Height
	}

	return h.backend.ExecuteScriptAtBlockHeight(context, r.BlockHeight, r.Script.Source, r.Script.Args)
}

// GetAccount handler retrieves account by address and returns the response.
func (h *ServerRequestHandler) GetAccount(r request.GetAccount, context context.Context, expandFields map[string]bool, link models.LinkGenerator) (models.Account, error) {
	var response models.Account

	// in case we receive special height values 'final' and 'sealed', fetch that height and overwrite request with it
	if r.Height == request.FinalHeight || r.Height == request.SealedHeight {
		header, _, err := h.backend.GetLatestBlockHeader(context, r.Height == request.SealedHeight)
		if err != nil {
			return response, err
		}
		r.Height = header.Height
	}

	account, err := h.backend.GetAccountAtBlockHeight(context, r.Address, r.Height)
	if err != nil {
		return response, models.NewNotFoundError("not found account at block height", err)
	}

	err = response.Build(account, link, expandFields)
	return response, err
}

// GetEvents for the provided block range or list of block IDs filtered by type.
func (h *ServerRequestHandler) GetEvents(r request.GetEvents, context context.Context) (models.BlocksEvents, error) {
	// if the request has block IDs provided then return events for block IDs
	var blocksEvents models.BlocksEvents
	if len(r.BlockIDs) > 0 {
		events, err := h.backend.GetEventsForBlockIDs(context, r.Type, r.BlockIDs)
		if err != nil {
			return nil, err
		}

		blocksEvents.Build(events)
		return blocksEvents, nil
	}

	// if end height is provided with special values then load the height
	if r.EndHeight == request.FinalHeight || r.EndHeight == request.SealedHeight {
		latest, _, err := h.backend.GetLatestBlockHeader(context, r.EndHeight == request.SealedHeight)
		if err != nil {
			return nil, err
		}

		r.EndHeight = latest.Height
		// special check after we resolve special height value
		if r.StartHeight > r.EndHeight {
			return nil, models.NewBadRequestError(fmt.Errorf("current retrieved end height value is lower than start height"))
		}
	}

	// if request provided block height range then return events for that range
	events, err := h.backend.GetEventsForHeightRange(context, r.Type, r.StartHeight, r.EndHeight)
	if err != nil {
		return nil, err
	}

	blocksEvents.Build(events)
	return blocksEvents, nil
}

// GetNetworkParameters returns network-wide parameters of the blockchain
func (h *ServerRequestHandler) GetNetworkParameters(r *request.Request) (models.NetworkParameters, error) {
	params := h.backend.GetNetworkParameters(r.Context())

	var response models.NetworkParameters
	response.Build(&params)
	return response, nil
}

// GetNodeVersionInfo returns node version information
func (h *ServerRequestHandler) GetNodeVersionInfo(r *request.Request) (models.NodeVersionInfo, error) {
	var response models.NodeVersionInfo

	params, err := h.backend.GetNodeVersionInfo(r.Context())
	if err != nil {
		return response, err
	}

	response.Build(params)
	return response, nil
}
