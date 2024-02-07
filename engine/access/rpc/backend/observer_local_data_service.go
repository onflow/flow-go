package backend

import (
	"context"
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type ObserverLocalDataService struct {
	TransactionsLocalDataProvider
	backendEvents BackendEvents
	nodeId        flow.Identifier
}

func NewObserverLocalDataService(state protocol.State,
	collections storage.Collections,
	blocks storage.Blocks,
	results storage.LightTransactionResults,
	events storage.Events,
	headers storage.Headers,
	executionReceipts storage.ExecutionReceipts,
	chain flow.Chain,
	connFactory connection.ConnectionFactory,
	log zerolog.Logger,
	maxHeightRange uint,
	nodeCommunicator Communicator,
	nodeId flow.Identifier,
) *ObserverLocalDataService {

	return &ObserverLocalDataService{
		TransactionsLocalDataProvider: TransactionsLocalDataProvider{
			state:       state,
			collections: collections,
			blocks:      blocks,
			results:     results,
			events:      events,
		},
		backendEvents: BackendEvents{
			headers:           headers,
			executionReceipts: executionReceipts,
			state:             state,
			chain:             chain,
			connFactory:       connFactory,
			log:               log,
			maxHeightRange:    maxHeightRange,
			nodeCommunicator:  nodeCommunicator,
			queryMode:         IndexQueryModeLocalOnly,
			eventsIndex:       NewEventsIndex(events),
		},
		nodeId: nodeId,
	}
}

func (o *ObserverLocalDataService) GetEventsForBlockIDsFromStorage(ctx context.Context,
	blockIDs []flow.Identifier,
	eventType string,
	requiredEventEncodingVersion entities.EventEncodingVersion) (*access.EventsResponse, error) {
	events, err := o.backendEvents.GetEventsForBlockIDs(ctx, eventType, blockIDs, requiredEventEncodingVersion)
	if err != nil {
		return nil, err
	}

	resultEvents, err := convert.BlockEventsToMessages(events)
	if err != nil {
		return nil, err
	}

	metadata, err := o.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	return &access.EventsResponse{
		Results:  resultEvents,
		Metadata: metadata,
	}, nil
}

func (o *ObserverLocalDataService) GetEventsForHeightRangeFromStorageData(
	ctx context.Context,
	eventType string,
	startHeight, endHeight uint64,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*access.EventsResponse, error) {
	events, err := o.backendEvents.GetEventsForHeightRange(ctx, eventType, startHeight, endHeight, requiredEventEncodingVersion)
	if err != nil {
		return nil, err
	}

	resultEvents, err := convert.BlockEventsToMessages(events)
	if err != nil {
		return nil, err
	}

	metadata, err := o.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	return &access.EventsResponse{
		Results:  resultEvents,
		Metadata: metadata,
	}, nil
}

func (o *ObserverLocalDataService) GetTransactionResultFromStorageData(
	ctx context.Context,
	txID []byte,
	blockID []byte,
	collectionID []byte,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	errorMessageGetter TransactionErrorMessageGetter,
) (*access.TransactionResultResponse, error) {

	transactionID, err := convert.TransactionID(txID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid transaction id: %v", err)
	}

	blockId := flow.ZeroID
	requestBlockId := blockID
	if requestBlockId != nil {
		blockId, err = convert.BlockID(requestBlockId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
		}
	}

	collectionId := flow.ZeroID
	requestCollectionId := collectionID
	if requestCollectionId != nil {
		collectionId, err = convert.CollectionID(requestCollectionId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid collection id: %v", err)
		}
	}

	block, err := o.retrieveBlock(blockId, collectionId, transactionID)
	// an error occurred looking up the block or the requested block or collection was not found.
	// If looking up the block based solely on the txID returns not found, then no error is
	// returned since the block may not be finalized yet.
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	result, err := o.GetTransactionResultFromStorage(ctx, block, transactionID, requiredEventEncodingVersion, errorMessageGetter)
	if err != nil {
		return nil, err
	}

	metadata, err := o.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	return &access.TransactionResultResponse{
		Status:        entities.TransactionStatus(result.Status),
		StatusCode:    uint32(result.StatusCode),
		ErrorMessage:  result.ErrorMessage,
		Events:        convert.EventsToMessages(result.Events),
		BlockId:       result.BlockID[:],
		TransactionId: result.TransactionID[:],
		CollectionId:  result.CollectionID[:],
		BlockHeight:   result.BlockHeight,
		Metadata:      metadata,
	}, nil
}

func (o *ObserverLocalDataService) GetTransactionResultsByBlockIDFromStorageData(
	ctx context.Context,
	blockID []byte,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	errorMessageByBlockIDGetter TransactionErrorMessagesByBlockIDGetter,
) (*access.TransactionResultsResponse, error) {

	blockId, err := convert.BlockID(blockID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
	}

	block, err := o.blocks.ByID(blockId)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	results, err := o.GetTransactionResultsByBlockIDFromStorage(ctx, block, requiredEventEncodingVersion, errorMessageByBlockIDGetter)
	if err != nil {
		return nil, err
	}

	metadata, err := o.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	var txResultsResponse []*access.TransactionResultResponse
	for _, result := range results {
		resultResponse := &access.TransactionResultResponse{
			Status:        entities.TransactionStatus(result.Status),
			StatusCode:    uint32(result.StatusCode),
			ErrorMessage:  result.ErrorMessage,
			Events:        convert.EventsToMessages(result.Events),
			BlockId:       result.BlockID[:],
			TransactionId: result.TransactionID[:],
			CollectionId:  result.CollectionID[:],
			BlockHeight:   result.BlockHeight,
		}

		txResultsResponse = append(txResultsResponse, resultResponse)
	}

	return &access.TransactionResultsResponse{
		TransactionResults: txResultsResponse,
		Metadata:           metadata,
	}, nil
}

func (o *ObserverLocalDataService) GetTransactionResultByIndexFromStorageData(
	ctx context.Context,
	blockID []byte,
	index uint32,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	errorMessageByIndexGetter TransactionErrorMessageByIndexGetter,
) (*access.TransactionResultResponse, error) {
	blockId, err := convert.BlockID(blockID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
	}

	block, err := o.blocks.ByID(blockId)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	result, err := o.GetTransactionResultByIndexFromStorage(ctx, block, index, requiredEventEncodingVersion, errorMessageByIndexGetter)
	if err != nil {
		return nil, err
	}

	metadata, err := o.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	return &access.TransactionResultResponse{
		Status:        entities.TransactionStatus(result.Status),
		StatusCode:    uint32(result.StatusCode),
		ErrorMessage:  result.ErrorMessage,
		Events:        convert.EventsToMessages(result.Events),
		BlockId:       result.BlockID[:],
		TransactionId: result.TransactionID[:],
		CollectionId:  result.CollectionID[:],
		BlockHeight:   result.BlockHeight,
		Metadata:      metadata,
	}, nil
}

// buildMetadataResponse builds and returns the metadata response object.
func (o *ObserverLocalDataService) buildMetadataResponse() (*entities.Metadata, error) {
	lastFinalizedHeader, err := o.state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized, %w", err)
	}
	finalizedBlockId := lastFinalizedHeader.ID()
	nodeId := o.nodeId

	return &entities.Metadata{
		LatestFinalizedBlockId: finalizedBlockId[:],
		LatestFinalizedHeight:  lastFinalizedHeader.Height,
		NodeId:                 nodeId[:],
	}, nil
}
