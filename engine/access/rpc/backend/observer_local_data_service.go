package backend

import (
	"context"
	"fmt"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

type ObserverLocalDataService struct {
	TransactionsLocalDataProvider
	backendEvents BackendEvents
	me            module.Local
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
	me module.Local,
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
			events:            events,
			executionReceipts: executionReceipts,
			state:             state,
			chain:             chain,
			connFactory:       connFactory,
			log:               log,
			maxHeightRange:    maxHeightRange,
			nodeCommunicator:  nodeCommunicator,
			queryMode:         IndexQueryModeLocalOnly,
		},
		me: me,
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

	lastFinalizedHeader, err := o.state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized, %w", err)
	}
	blockId := lastFinalizedHeader.ID()
	nodeId := o.me.NodeID()

	metadata := &entities.Metadata{
		LatestFinalizedBlockId: blockId[:],
		LatestFinalizedHeight:  lastFinalizedHeader.Height,
		NodeId:                 nodeId[:],
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

	lastFinalizedHeader, err := o.state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized, %w", err)
	}
	blockId := lastFinalizedHeader.ID()
	nodeId := o.me.NodeID()

	metadata := &entities.Metadata{
		LatestFinalizedBlockId: blockId[:],
		LatestFinalizedHeight:  lastFinalizedHeader.Height,
		NodeId:                 nodeId[:],
	}

	return &access.EventsResponse{
		Results:  resultEvents,
		Metadata: metadata,
	}, nil
}

func (o *ObserverLocalDataService) GetTransactionResultFromStorageData(
	ctx context.Context,
	txID flow.Identifier,
	blockID flow.Identifier,
	collectionID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	errorMessageGetter TransactionErrorMessageGetter,
) (*access.TransactionResultResponse, error) {

	result, err := o.GetTransactionResultFromStorage(ctx, block, txID, requiredEventEncodingVersion, errorMessageGetter)
	if err != nil {
		return nil, err
	}

	lastFinalizedHeader, err := o.state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized, %w", err)
	}
	blockId := lastFinalizedHeader.ID()
	nodeId := o.me.NodeID()

	metadata := &entities.Metadata{
		LatestFinalizedBlockId: blockId[:],
		LatestFinalizedHeight:  lastFinalizedHeader.Height,
		NodeId:                 nodeId[:],
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
	block *flow.Block,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	errorMessageByBlockIDGetter TransactionErrorMessagesByBlockIDGetter,
) (*access.TransactionResultsResponse, error) {
	results, err := o.GetTransactionResultsByBlockIDFromStorage(ctx, block, requiredEventEncodingVersion, errorMessageByBlockIDGetter)
	if err != nil {
		return nil, err
	}

	lastFinalizedHeader, err := o.state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized, %w", err)
	}
	blockId := lastFinalizedHeader.ID()
	nodeId := o.me.NodeID()

	metadata := &entities.Metadata{
		LatestFinalizedBlockId: blockId[:],
		LatestFinalizedHeight:  lastFinalizedHeader.Height,
		NodeId:                 nodeId[:],
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
			Metadata:      metadata,
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
	block *flow.Block,
	index uint32,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	errorMessageByIndexGetter TransactionErrorMessageByIndexGetter,
) (*access.TransactionResultResponse, error) {
	result, err := o.GetTransactionResultByIndexFromStorage(ctx, block, index, requiredEventEncodingVersion, errorMessageByIndexGetter)
	if err != nil {
		return nil, err
	}

	lastFinalizedHeader, err := o.state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized, %w", err)
	}
	blockId := lastFinalizedHeader.ID()
	nodeId := o.me.NodeID()

	metadata := &entities.Metadata{
		LatestFinalizedBlockId: blockId[:],
		LatestFinalizedHeight:  lastFinalizedHeader.Height,
		NodeId:                 nodeId[:],
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
