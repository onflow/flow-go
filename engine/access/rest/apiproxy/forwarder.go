package apiproxy

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/api"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/rest/routes"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forwarder"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// RestForwarder - structure which handles requests to an upstream access node using gRPC API.
type RestForwarder struct {
	log zerolog.Logger
	*forwarder.Forwarder
}

var _ api.RestServerApi = (*RestForwarder)(nil)

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
		block, err := getBlockFromGrpc(routes.ForID(&id), context, expandFields, upstream, link)
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
		return nil, models.NewBadRequestError(err)
	}

	upstream, err := f.FaultTolerantClient()
	if err != nil {
		return nil, err
	}

	if req.FinalHeight || req.SealedHeight {
		block, err := getBlockFromGrpc(routes.ForFinalized(req.Heights[0]), r.Context(), r.ExpandFields, upstream, link)
		if err != nil {
			return nil, err
		}

		return []*models.Block{block}, nil
	}

	// if the query is /blocks/height=1000,1008,1049...
	if req.HasHeights() {
		blocks := make([]*models.Block, len(req.Heights))
		for i, height := range req.Heights {
			block, err := getBlockFromGrpc(routes.ForHeight(height), r.Context(), r.ExpandFields, upstream, link)
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
			return nil, models.NewBadRequestError(fmt.Errorf("start height must be less than or equal to end height"))
		}
	}

	blocks := make([]*models.Block, 0)
	// start and end height inclusive
	for i := req.StartHeight; i <= req.EndHeight; i++ {
		block, err := getBlockFromGrpc(routes.ForHeight(i), r.Context(), r.ExpandFields, upstream, link)
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

	blkProvider := routes.NewBlockFromGrpcProvider(upstream, routes.ForID(&r.ID))
	block, _, statusErr := blkProvider.GetBlock(context)
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
		return response, models.NewNotFoundError(err.Error(), err)
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
		return response, models.NewNotFoundError("not found account at block height", err)
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
			return nil, models.NewBadRequestError(fmt.Errorf("current retrieved end height value is lower than start height"))
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

func getBlockFromGrpc(option routes.BlockRequestOption, context context.Context, expandFields map[string]bool, upstream accessproto.AccessAPIClient, link models.LinkGenerator) (*models.Block, error) {
	// lookup block
	blkProvider := routes.NewBlockFromGrpcProvider(upstream, option)
	blk, blockStatus, err := blkProvider.GetBlock(context)
	if err != nil {
		return nil, err
	}

	// lookup execution result
	// (even if not specified as expandable, since we need the execution result ID to generate its expandable link)
	var block models.Block
	getExecutionResultForBlockIDRequest := &accessproto.GetExecutionResultForBlockIDRequest{
		BlockId: blk.Id,
	}

	executionResultForBlockIDResponse, err := upstream.GetExecutionResultForBlockID(context, getExecutionResultForBlockIDRequest)
	if err != nil {
		return nil, err
	}

	flowExecResult, err := convert.MessageToExecutionResult(executionResultForBlockIDResponse.ExecutionResult)
	if err != nil {
		return nil, err
	}

	flowBlock, err := convert.MessageToBlock(blk)
	if err != nil {
		return nil, err
	}

	flowBlockStatus, err := convert.MessagesToBlockStatus(blockStatus)
	if err != nil {
		return nil, err
	}

	if err != nil {
		// handle case where execution result is not yet available
		if se, ok := status.FromError(err); ok {
			if se.Code() == codes.NotFound {
				err := block.Build(flowBlock, nil, link, flowBlockStatus, expandFields)
				if err != nil {
					return nil, err
				}
				return &block, nil
			}
		}
		return nil, err
	}

	err = block.Build(flowBlock, flowExecResult, link, flowBlockStatus, expandFields)
	if err != nil {
		return nil, err
	}
	return &block, nil
}
