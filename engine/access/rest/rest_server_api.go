package rest

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/status"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forwarder"
	"github.com/onflow/flow-go/module/metrics"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// RestRouter is a structure that represents the routing proxy algorithm.
// It splits requests between a local and a remote rest service.
type RestRouter struct {
	Logger   zerolog.Logger
	Metrics  *metrics.ObserverCollector
	Upstream RestServerApi
	Observer *RequestHandler
}

func (r *RestRouter) log(handler, rpc string, err error) {
	code := status.Code(err)
	r.Metrics.RecordRPC(handler, rpc, code)

	logger := r.Logger.With().
		Str("handler", handler).
		Str("rest_method", rpc).
		Str("rest_code", code.String()).
		Logger()

	if err != nil {
		logger.Error().Err(err).Msg("request failed")
		return
	}

	logger.Info().Msg("request succeeded")
}

func (r *RestRouter) GetTransactionByID(req request.GetTransaction, context context.Context, link models.LinkGenerator, chain flow.Chain) (models.Transaction, error) {
	res, err := r.Upstream.GetTransactionByID(req, context, link, chain)
	r.log("upstream", "GetNodeVersionInfo", err)
	return res, err
}

func (r *RestRouter) CreateTransaction(req request.CreateTransaction, context context.Context, link models.LinkGenerator) (models.Transaction, error) {
	res, err := r.Upstream.CreateTransaction(req, context, link)
	r.log("upstream", "CreateTransaction", err)
	return res, err
}

func (r *RestRouter) GetTransactionResultByID(req request.GetTransactionResult, context context.Context, link models.LinkGenerator) (models.TransactionResult, error) {
	res, err := r.Upstream.GetTransactionResultByID(req, context, link)
	r.log("upstream", "GetTransactionResultByID", err)
	return res, err
}

func (r *RestRouter) GetBlocksByIDs(req request.GetBlockByIDs, context context.Context, expandFields map[string]bool, link models.LinkGenerator) ([]*models.Block, error) {
	res, err := r.Observer.GetBlocksByIDs(req, context, expandFields, link)
	r.log("observer", "GetBlocksByIDs", err)
	return res, err
}

func (r *RestRouter) GetBlocksByHeight(req *request.Request, link models.LinkGenerator) ([]*models.Block, error) {
	res, err := r.Observer.GetBlocksByHeight(req, link)
	r.log("observer", "GetBlocksByHeight", err)
	return res, err
}

func (r *RestRouter) GetBlockPayloadByID(req request.GetBlockPayload, context context.Context, link models.LinkGenerator) (models.BlockPayload, error) {
	res, err := r.Observer.GetBlockPayloadByID(req, context, link)
	r.log("observer", "GetBlockPayloadByID", err)
	return res, err
}

func (r *RestRouter) GetExecutionResultByID(req request.GetExecutionResult, context context.Context, link models.LinkGenerator) (models.ExecutionResult, error) {
	res, err := r.Upstream.GetExecutionResultByID(req, context, link)
	r.log("upstream", "GetExecutionResultByID", err)
	return res, err
}

func (r *RestRouter) GetExecutionResultsByBlockIDs(req request.GetExecutionResultByBlockIDs, context context.Context, link models.LinkGenerator) ([]models.ExecutionResult, error) {
	res, err := r.Upstream.GetExecutionResultsByBlockIDs(req, context, link)
	r.log("upstream", "GetExecutionResultsByBlockIDs", err)
	return res, err
}

func (r *RestRouter) GetCollectionByID(req request.GetCollection, context context.Context, expandFields map[string]bool, link models.LinkGenerator, chain flow.Chain) (models.Collection, error) {
	res, err := r.Upstream.GetCollectionByID(req, context, expandFields, link, chain)
	r.log("upstream", "GetCollectionByID", err)
	return res, err
}

func (r *RestRouter) ExecuteScript(req request.GetScript, context context.Context, link models.LinkGenerator) ([]byte, error) {
	res, err := r.Upstream.ExecuteScript(req, context, link)
	r.log("upstream", "ExecuteScript", err)
	return res, err
}

func (r *RestRouter) GetAccount(req request.GetAccount, context context.Context, expandFields map[string]bool, link models.LinkGenerator) (models.Account, error) {
	res, err := r.Upstream.GetAccount(req, context, expandFields, link)
	r.log("upstream", "GetAccount", err)
	return res, err
}

func (r *RestRouter) GetEvents(req request.GetEvents, context context.Context) (models.BlocksEvents, error) {
	res, err := r.Upstream.GetEvents(req, context)
	r.log("upstream", "GetEvents", err)
	return res, err
}

func (r *RestRouter) GetNetworkParameters(req *request.Request) (models.NetworkParameters, error) {
	res, err := r.Observer.GetNetworkParameters(req)
	r.log("observer", "GetNetworkParameters", err)
	return res, err
}

func (r *RestRouter) GetNodeVersionInfo(req *request.Request) (models.NodeVersionInfo, error) {
	res, err := r.Observer.GetNodeVersionInfo(req)
	r.log("observer", "GetNodeVersionInfo", err)
	return res, err
}

// RestServerApi is the server API for REST service.
type RestServerApi interface {
	// GetTransactionByID gets a transaction by requested ID.
	GetTransactionByID(r request.GetTransaction, context context.Context, link models.LinkGenerator, chain flow.Chain) (models.Transaction, error)
	// CreateTransaction creates a new transaction from provided payload.
	CreateTransaction(r request.CreateTransaction, context context.Context, link models.LinkGenerator) (models.Transaction, error)
	// GetTransactionResultByID retrieves transaction result by the transaction ID.
	GetTransactionResultByID(r request.GetTransactionResult, context context.Context, link models.LinkGenerator) (models.TransactionResult, error)
	// GetBlocksByIDs gets blocks by provided ID or list of IDs.
	GetBlocksByIDs(r request.GetBlockByIDs, context context.Context, expandFields map[string]bool, link models.LinkGenerator) ([]*models.Block, error)
	// GetBlocksByHeight gets blocks by provided height.
	GetBlocksByHeight(r *request.Request, link models.LinkGenerator) ([]*models.Block, error)
	// GetBlockPayloadByID gets block payload by ID
	GetBlockPayloadByID(r request.GetBlockPayload, context context.Context, link models.LinkGenerator) (models.BlockPayload, error)
	// GetExecutionResultByID gets execution result by the ID.
	GetExecutionResultByID(r request.GetExecutionResult, context context.Context, link models.LinkGenerator) (models.ExecutionResult, error)
	// GetExecutionResultsByBlockIDs gets Execution Result payload by block IDs.
	GetExecutionResultsByBlockIDs(r request.GetExecutionResultByBlockIDs, context context.Context, link models.LinkGenerator) ([]models.ExecutionResult, error)
	// GetCollectionByID retrieves a collection by ID and builds a response
	GetCollectionByID(r request.GetCollection, context context.Context, expandFields map[string]bool, link models.LinkGenerator, chain flow.Chain) (models.Collection, error)
	// ExecuteScript handler sends the script from the request to be executed.
	ExecuteScript(r request.GetScript, context context.Context, link models.LinkGenerator) ([]byte, error)
	// GetAccount handler retrieves account by address and returns the response.
	GetAccount(r request.GetAccount, context context.Context, expandFields map[string]bool, link models.LinkGenerator) (models.Account, error)
	// GetEvents for the provided block range or list of block IDs filtered by type.
	GetEvents(r request.GetEvents, context context.Context) (models.BlocksEvents, error)
	// GetNetworkParameters returns network-wide parameters of the blockchain
	GetNetworkParameters(r *request.Request) (models.NetworkParameters, error)
	// GetNodeVersionInfo returns node version information
	GetNodeVersionInfo(r *request.Request) (models.NodeVersionInfo, error)
}

// RequestHandler is a structure that represents local requests
type RequestHandler struct {
	RestServerApi
	log     zerolog.Logger
	backend access.API
}

// NewRequestHandler returns new RequestHandler.
func NewRequestHandler(log zerolog.Logger, backend access.API) *RequestHandler {
	return &RequestHandler{
		log:     log,
		backend: backend,
	}
}

// GetTransactionByID gets a transaction by requested ID.
func (h *RequestHandler) GetTransactionByID(r request.GetTransaction, context context.Context, link models.LinkGenerator, _ flow.Chain) (models.Transaction, error) {
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
func (h *RequestHandler) CreateTransaction(r request.CreateTransaction, context context.Context, link models.LinkGenerator) (models.Transaction, error) {
	var response models.Transaction

	err := h.backend.SendTransaction(context, &r.Transaction)
	if err != nil {
		return response, err
	}

	response.Build(&r.Transaction, nil, link)
	return response, nil
}

// GetTransactionResultByID retrieves transaction result by the transaction ID.
func (h *RequestHandler) GetTransactionResultByID(r request.GetTransactionResult, context context.Context, link models.LinkGenerator) (models.TransactionResult, error) {
	var response models.TransactionResult

	txr, err := h.backend.GetTransactionResult(context, r.ID, r.BlockID, r.CollectionID)
	if err != nil {
		return response, err
	}

	response.Build(txr, r.ID, link)
	return response, nil
}

// GetBlocksByIDs gets blocks by provided ID or list of IDs.
func (h *RequestHandler) GetBlocksByIDs(r request.GetBlockByIDs, context context.Context, expandFields map[string]bool, link models.LinkGenerator) ([]*models.Block, error) {
	blocks := make([]*models.Block, len(r.IDs))

	for i, id := range r.IDs {
		block, err := getBlock(forID(&id), context, expandFields, h.backend, link)
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}

	return blocks, nil
}

// GetBlocksByHeight gets blocks by provided height.
func (h *RequestHandler) GetBlocksByHeight(r *request.Request, link models.LinkGenerator) ([]*models.Block, error) {
	req, err := r.GetBlockRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	if req.FinalHeight || req.SealedHeight {
		block, err := getBlock(forFinalized(req.Heights[0]), r.Context(), r.ExpandFields, h.backend, link)
		if err != nil {
			return nil, err
		}

		return []*models.Block{block}, nil
	}

	// if the query is /blocks/height=1000,1008,1049...
	if req.HasHeights() {
		blocks := make([]*models.Block, len(req.Heights))
		for i, height := range req.Heights {
			block, err := getBlock(forHeight(height), r.Context(), r.ExpandFields, h.backend, link)
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
			return nil, NewBadRequestError(fmt.Errorf("start height must be less than or equal to end height"))
		}
	}

	blocks := make([]*models.Block, 0)
	// start and end height inclusive
	for i := req.StartHeight; i <= req.EndHeight; i++ {
		block, err := getBlock(forHeight(i), r.Context(), r.ExpandFields, h.backend, link)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

// GetBlockPayloadByID gets block payload by ID
func (h *RequestHandler) GetBlockPayloadByID(r request.GetBlockPayload, context context.Context, _ models.LinkGenerator) (models.BlockPayload, error) {
	var payload models.BlockPayload

	blkProvider := NewBlockRequestProvider(h.backend, forID(&r.ID))
	blk, _, statusErr := blkProvider.getBlock(context)
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
func (h *RequestHandler) GetExecutionResultByID(r request.GetExecutionResult, context context.Context, link models.LinkGenerator) (models.ExecutionResult, error) {
	var response models.ExecutionResult

	res, err := h.backend.GetExecutionResultByID(context, r.ID)
	if err != nil {
		return response, err
	}

	if res == nil {
		err := fmt.Errorf("execution result with ID: %s not found", r.ID.String())
		return response, NewNotFoundError(err.Error(), err)
	}

	err = response.Build(res, link)
	if err != nil {
		return response, err
	}

	return response, nil
}

// GetExecutionResultsByBlockIDs gets Execution Result payload by block IDs.
func (h *RequestHandler) GetExecutionResultsByBlockIDs(r request.GetExecutionResultByBlockIDs, context context.Context, link models.LinkGenerator) ([]models.ExecutionResult, error) {
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
func (h *RequestHandler) GetCollectionByID(r request.GetCollection, context context.Context, expandFields map[string]bool, link models.LinkGenerator, _ flow.Chain) (models.Collection, error) {
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
func (h *RequestHandler) ExecuteScript(r request.GetScript, context context.Context, _ models.LinkGenerator) ([]byte, error) {
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
func (h *RequestHandler) GetAccount(r request.GetAccount, context context.Context, expandFields map[string]bool, link models.LinkGenerator) (models.Account, error) {
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
		return response, err
	}

	err = response.Build(account, link, expandFields)
	return response, err
}

// GetEvents for the provided block range or list of block IDs filtered by type.
func (h *RequestHandler) GetEvents(r request.GetEvents, context context.Context) (models.BlocksEvents, error) {
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
			return nil, NewBadRequestError(fmt.Errorf("current retrieved end height value is lower than start height"))
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
func (h *RequestHandler) GetNetworkParameters(r *request.Request) (models.NetworkParameters, error) {
	params := h.backend.GetNetworkParameters(r.Context())

	var response models.NetworkParameters
	response.Build(&params)
	return response, nil
}

// GetNodeVersionInfo returns node version information
func (h *RequestHandler) GetNodeVersionInfo(r *request.Request) (models.NodeVersionInfo, error) {
	var response models.NodeVersionInfo

	params, err := h.backend.GetNodeVersionInfo(r.Context())
	if err != nil {
		return response, err
	}

	response.Build(params)
	return response, nil
}

// RestForwarder handles the request forwarding to upstream
type RestForwarder struct {
	log zerolog.Logger
	*forwarder.Forwarder
}

// NewRestForwarder returns new RestForwarder.
func NewRestForwarder(log zerolog.Logger, identities flow.IdentityList, timeout time.Duration, maxMsgSize uint) (*RestForwarder, error) {
	f, err := forwarder.NewForwarder(identities, timeout, maxMsgSize)

	restForwarder := &RestForwarder{
		log:       log,
		Forwarder: f,
	}
	return restForwarder, err
}

// GetTransactionByID gets a transaction by requested ID.
func (f *RestForwarder) GetTransactionByID(r request.GetTransaction, context context.Context, link models.LinkGenerator, chain flow.Chain) (models.Transaction, error) {
	var response models.Transaction

	upstream, err := f.FaultTolerantClient()
	if err != nil {
		return response, err
	}

	getTransactionRequest := &accessproto.GetTransactionRequest{
		Id: r.ID[:],
	}
	transactionResponse, err := upstream.GetTransaction(context, getTransactionRequest)
	if err != nil {
		return response, err
	}

	var transactionResultResponse *accessproto.TransactionResultResponse
	// only lookup result if transaction result is to be expanded
	if r.ExpandsResult {
		getTransactionResultRequest := &accessproto.GetTransactionRequest{
			Id:           r.ID[:],
			BlockId:      r.BlockID[:],
			CollectionId: r.CollectionID[:],
		}
		transactionResultResponse, err = upstream.GetTransactionResult(context, getTransactionResultRequest)
		if err != nil {
			return response, err
		}
	}
	flowTransaction, err := convert.MessageToTransaction(transactionResponse.Transaction, chain)
	if err != nil {
		return response, err
	}

	flowTransactionResult := access.MessageToTransactionResult(transactionResultResponse)

	response.Build(&flowTransaction, flowTransactionResult, link)
	return response, nil
}

// CreateTransaction creates a new transaction from provided payload.
func (f *RestForwarder) CreateTransaction(r request.CreateTransaction, context context.Context, link models.LinkGenerator) (models.Transaction, error) {
	var response models.Transaction

	upstream, err := f.FaultTolerantClient()
	if err != nil {
		return response, err
	}

	entitiesTransaction := convert.TransactionToMessage(r.Transaction)
	sendTransactionRequest := &accessproto.SendTransactionRequest{
		Transaction: entitiesTransaction,
	}

	_, err = upstream.SendTransaction(context, sendTransactionRequest)
	if err != nil {
		return response, err
	}

	response.Build(&r.Transaction, nil, link)
	return response, nil
}

// GetTransactionResultByID retrieves transaction result by the transaction ID.
func (f *RestForwarder) GetTransactionResultByID(r request.GetTransactionResult, context context.Context, link models.LinkGenerator) (models.TransactionResult, error) {
	var response models.TransactionResult

	upstream, err := f.FaultTolerantClient()
	if err != nil {
		return response, err
	}

	getTransactionResult := &accessproto.GetTransactionRequest{
		Id:           r.ID[:],
		BlockId:      r.BlockID[:],
		CollectionId: r.CollectionID[:],
	}
	transactionResultResponse, err := upstream.GetTransactionResult(context, getTransactionResult)
	if err != nil {
		return response, err
	}

	flowTransactionResult := access.MessageToTransactionResult(transactionResultResponse)
	response.Build(flowTransactionResult, r.ID, link)
	return response, nil
}

// GetBlocksByIDs gets blocks by provided ID or list of IDs.
func (f *RestForwarder) GetBlocksByIDs(r request.GetBlockByIDs, context context.Context, expandFields map[string]bool, link models.LinkGenerator) ([]*models.Block, error) {
	blocks := make([]*models.Block, len(r.IDs))

	upstream, err := f.FaultTolerantClient()
	if err != nil {
		return blocks, err
	}

	for i, id := range r.IDs {
		block, err := getBlockFromGrpc(forID(&id), context, expandFields, upstream, link)
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}

	return blocks, nil
}

// GetBlocksByHeight gets blocks by provided height.
func (f *RestForwarder) GetBlocksByHeight(r *request.Request, link models.LinkGenerator) ([]*models.Block, error) {
	req, err := r.GetBlockRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	upstream, err := f.FaultTolerantClient()
	if err != nil {
		return nil, err
	}

	if req.FinalHeight || req.SealedHeight {
		block, err := getBlockFromGrpc(forFinalized(req.Heights[0]), r.Context(), r.ExpandFields, upstream, link)
		if err != nil {
			return nil, err
		}

		return []*models.Block{block}, nil
	}

	// if the query is /blocks/height=1000,1008,1049...
	if req.HasHeights() {
		blocks := make([]*models.Block, len(req.Heights))
		for i, height := range req.Heights {
			block, err := getBlockFromGrpc(forHeight(height), r.Context(), r.ExpandFields, upstream, link)
			if err != nil {
				return nil, err
			}
			blocks[i] = block
		}

		return blocks, nil
	}

	// support providing end height as "sealed" or "final"
	if req.EndHeight == request.FinalHeight || req.EndHeight == request.SealedHeight {
		getLatestBlockRequest := &accessproto.GetLatestBlockRequest{
			IsSealed: req.EndHeight == request.SealedHeight,
		}
		blockResponse, err := upstream.GetLatestBlock(r.Context(), getLatestBlockRequest)
		if err != nil {
			return nil, err
		}

		req.EndHeight = blockResponse.Block.BlockHeader.Height // overwrite special value height with fetched

		if req.StartHeight > req.EndHeight {
			return nil, NewBadRequestError(fmt.Errorf("start height must be less than or equal to end height"))
		}
	}

	blocks := make([]*models.Block, 0)
	// start and end height inclusive
	for i := req.StartHeight; i <= req.EndHeight; i++ {
		block, err := getBlockFromGrpc(forHeight(i), r.Context(), r.ExpandFields, upstream, link)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

// GetBlockPayloadByID gets block payload by ID
func (f *RestForwarder) GetBlockPayloadByID(r request.GetBlockPayload, context context.Context, _ models.LinkGenerator) (models.BlockPayload, error) {
	var payload models.BlockPayload

	upstream, err := f.FaultTolerantClient()
	if err != nil {
		return payload, err
	}

	blkProvider := NewBlockFromGrpcProvider(upstream, forID(&r.ID))
	block, _, statusErr := blkProvider.getBlock(context)
	if statusErr != nil {
		return payload, statusErr
	}

	flowPayload, err := convert.PayloadFromMessage(block)
	if err != nil {
		return payload, err
	}

	err = payload.Build(flowPayload)
	if err != nil {
		return payload, err
	}

	return payload, nil
}

// GetExecutionResultByID gets execution result by the ID.
func (f *RestForwarder) GetExecutionResultByID(r request.GetExecutionResult, context context.Context, link models.LinkGenerator) (models.ExecutionResult, error) {
	var response models.ExecutionResult

	upstream, err := f.FaultTolerantClient()
	if err != nil {
		return response, err
	}

	executionResultByIDRequest := &accessproto.GetExecutionResultByIDRequest{
		Id: r.ID[:],
	}

	executionResultByIDResponse, err := upstream.GetExecutionResultByID(context, executionResultByIDRequest)
	if err != nil {
		return response, err
	}

	if executionResultByIDResponse == nil {
		err := fmt.Errorf("execution result with ID: %s not found", r.ID.String())
		return response, NewNotFoundError(err.Error(), err)
	}

	flowExecResult, err := convert.MessageToExecutionResult(executionResultByIDResponse.ExecutionResult)
	if err != nil {
		return response, err
	}
	err = response.Build(flowExecResult, link)
	if err != nil {
		return response, err
	}

	return response, nil
}

// GetExecutionResultsByBlockIDs gets Execution Result payload by block IDs.
func (f *RestForwarder) GetExecutionResultsByBlockIDs(r request.GetExecutionResultByBlockIDs, context context.Context, link models.LinkGenerator) ([]models.ExecutionResult, error) {
	// for each block ID we retrieve execution result
	results := make([]models.ExecutionResult, len(r.BlockIDs))

	upstream, err := f.FaultTolerantClient()
	if err != nil {
		return results, err
	}

	for i, id := range r.BlockIDs {
		getExecutionResultForBlockID := &accessproto.GetExecutionResultForBlockIDRequest{
			BlockId: id[:],
		}
		executionResultForBlockIDResponse, err := upstream.GetExecutionResultForBlockID(context, getExecutionResultForBlockID)
		if err != nil {
			return nil, err
		}

		var response models.ExecutionResult
		flowExecResult, err := convert.MessageToExecutionResult(executionResultForBlockIDResponse.ExecutionResult)
		if err != nil {
			return nil, err
		}
		err = response.Build(flowExecResult, link)
		if err != nil {
			return nil, err
		}
		results[i] = response
	}

	return results, nil
}

// GetCollectionByID retrieves a collection by ID and builds a response
func (f *RestForwarder) GetCollectionByID(r request.GetCollection, context context.Context, expandFields map[string]bool, link models.LinkGenerator, chain flow.Chain) (models.Collection, error) {
	var response models.Collection

	upstream, err := f.FaultTolerantClient()
	if err != nil {
		return response, err
	}

	getCollectionByIDRequest := &accessproto.GetCollectionByIDRequest{
		Id: r.ID[:],
	}

	collectionResponse, err := upstream.GetCollectionByID(context, getCollectionByIDRequest)
	if err != nil {
		return response, err
	}

	// if we expand transactions in the query retrieve each transaction data
	transactions := make([]*entities.Transaction, 0)
	if r.ExpandsTransactions {
		for _, tid := range collectionResponse.Collection.TransactionIds {
			getTransactionRequest := &accessproto.GetTransactionRequest{
				Id: tid,
			}
			transactionResponse, err := upstream.GetTransaction(context, getTransactionRequest)
			if err != nil {
				return response, err
			}

			transactions = append(transactions, transactionResponse.Transaction)
		}
	}

	err = response.BuildFromGrpc(collectionResponse.Collection, transactions, link, expandFields, chain)
	if err != nil {
		return response, err
	}

	return response, nil
}

// ExecuteScript handler sends the script from the request to be executed.
func (f *RestForwarder) ExecuteScript(r request.GetScript, context context.Context, _ models.LinkGenerator) ([]byte, error) {
	upstream, err := f.FaultTolerantClient()
	if err != nil {
		return nil, err
	}

	if r.BlockID != flow.ZeroID {
		executeScriptAtBlockIDRequest := &accessproto.ExecuteScriptAtBlockIDRequest{
			BlockId:   r.BlockID[:],
			Script:    r.Script.Source,
			Arguments: r.Script.Args,
		}
		executeScriptAtBlockIDResponse, err := upstream.ExecuteScriptAtBlockID(context, executeScriptAtBlockIDRequest)
		if err != nil {
			return nil, err
		}
		return executeScriptAtBlockIDResponse.Value, nil
	}

	// default to sealed height
	if r.BlockHeight == request.SealedHeight || r.BlockHeight == request.EmptyHeight {
		executeScriptAtLatestBlockRequest := &accessproto.ExecuteScriptAtLatestBlockRequest{
			Script:    r.Script.Source,
			Arguments: r.Script.Args,
		}
		executeScriptAtLatestBlockResponse, err := upstream.ExecuteScriptAtLatestBlock(context, executeScriptAtLatestBlockRequest)
		if err != nil {
			return nil, err
		}
		return executeScriptAtLatestBlockResponse.Value, nil
	}

	if r.BlockHeight == request.FinalHeight {
		getLatestBlockHeaderRequest := &accessproto.GetLatestBlockHeaderRequest{
			IsSealed: false,
		}
		getLatestBlockHeaderResponse, err := upstream.GetLatestBlockHeader(context, getLatestBlockHeaderRequest)
		if err != nil {
			return nil, err
		}
		r.BlockHeight = getLatestBlockHeaderResponse.Block.Height
	}

	executeScriptAtBlockHeightRequest := &accessproto.ExecuteScriptAtBlockHeightRequest{
		BlockHeight: r.BlockHeight,
		Script:      r.Script.Source,
		Arguments:   r.Script.Args,
	}
	executeScriptAtBlockHeightResponse, err := upstream.ExecuteScriptAtBlockHeight(context, executeScriptAtBlockHeightRequest)
	if err != nil {
		return nil, err
	}
	return executeScriptAtBlockHeightResponse.Value, nil
}

// GetAccount handler retrieves account by address and returns the response.
func (f *RestForwarder) GetAccount(r request.GetAccount, context context.Context, expandFields map[string]bool, link models.LinkGenerator) (models.Account, error) {
	var response models.Account

	upstream, err := f.FaultTolerantClient()
	if err != nil {
		return response, err
	}

	// in case we receive special height values 'final' and 'sealed', fetch that height and overwrite request with it
	if r.Height == request.FinalHeight || r.Height == request.SealedHeight {
		getLatestBlockHeaderRequest := &accessproto.GetLatestBlockHeaderRequest{
			IsSealed: r.Height == request.SealedHeight,
		}
		blockHeaderResponse, err := upstream.GetLatestBlockHeader(context, getLatestBlockHeaderRequest)
		if err != nil {
			return response, err
		}
		r.Height = blockHeaderResponse.Block.Height
	}
	getAccountAtBlockHeightRequest := &accessproto.GetAccountAtBlockHeightRequest{
		Address:     r.Address.Bytes(),
		BlockHeight: r.Height,
	}

	accountResponse, err := upstream.GetAccountAtBlockHeight(context, getAccountAtBlockHeightRequest)
	if err != nil {
		return response, err
	}

	flowAccount, err := convert.MessageToAccount(accountResponse.Account)
	if err != nil {
		return response, err
	}

	err = response.Build(flowAccount, link, expandFields)
	return response, err
}

// GetEvents for the provided block range or list of block IDs filtered by type.
func (f *RestForwarder) GetEvents(r request.GetEvents, context context.Context) (models.BlocksEvents, error) {
	// if the request has block IDs provided then return events for block IDs
	var blocksEvents models.BlocksEvents

	upstream, err := f.FaultTolerantClient()
	if err != nil {
		return blocksEvents, err
	}

	if len(r.BlockIDs) > 0 {
		var blockIds [][]byte
		for _, id := range r.BlockIDs {
			blockIds = append(blockIds, id[:])
		}
		getEventsForBlockIDsRequest := &accessproto.GetEventsForBlockIDsRequest{
			Type:     r.Type,
			BlockIds: blockIds,
		}
		eventsResponse, err := upstream.GetEventsForBlockIDs(context, getEventsForBlockIDsRequest)
		if err != nil {
			return nil, err
		}

		blocksEvents.BuildFromGrpc(eventsResponse.Results)

		return blocksEvents, nil
	}

	// if end height is provided with special values then load the height
	if r.EndHeight == request.FinalHeight || r.EndHeight == request.SealedHeight {
		getLatestBlockHeaderRequest := &accessproto.GetLatestBlockHeaderRequest{
			IsSealed: r.EndHeight == request.SealedHeight,
		}
		latestBlockHeaderResponse, err := upstream.GetLatestBlockHeader(context, getLatestBlockHeaderRequest)
		if err != nil {
			return nil, err
		}

		r.EndHeight = latestBlockHeaderResponse.Block.Height
		// special check after we resolve special height value
		if r.StartHeight > r.EndHeight {
			return nil, NewBadRequestError(fmt.Errorf("current retrieved end height value is lower than start height"))
		}
	}

	// if request provided block height range then return events for that range
	getEventsForHeightRangeRequest := &accessproto.GetEventsForHeightRangeRequest{
		Type:        r.Type,
		StartHeight: r.StartHeight,
		EndHeight:   r.EndHeight,
	}
	eventsResponse, err := upstream.GetEventsForHeightRange(context, getEventsForHeightRangeRequest)
	if err != nil {
		return nil, err
	}

	blocksEvents.BuildFromGrpc(eventsResponse.Results)
	return blocksEvents, nil
}

// GetNetworkParameters returns network-wide parameters of the blockchain
func (f *RestForwarder) GetNetworkParameters(r *request.Request) (models.NetworkParameters, error) {
	var response models.NetworkParameters

	upstream, err := f.FaultTolerantClient()
	if err != nil {
		return response, err
	}

	getNetworkParametersRequest := &accessproto.GetNetworkParametersRequest{}
	getNetworkParametersResponse, err := upstream.GetNetworkParameters(r.Context(), getNetworkParametersRequest)
	if err != nil {
		return response, err
	}
	response.BuildFromGrpc(getNetworkParametersResponse)
	return response, nil
}

// GetNodeVersionInfo returns node version information
func (f *RestForwarder) GetNodeVersionInfo(r *request.Request) (models.NodeVersionInfo, error) {
	var response models.NodeVersionInfo

	upstream, err := f.FaultTolerantClient()
	if err != nil {
		return response, err
	}

	getNodeVersionInfoRequest := &accessproto.GetNodeVersionInfoRequest{}
	getNodeVersionInfoResponse, err := upstream.GetNodeVersionInfo(r.Context(), getNodeVersionInfoRequest)
	if err != nil {
		return response, err
	}

	response.BuildFromGrpc(getNodeVersionInfoResponse.Info)
	return response, nil
}
