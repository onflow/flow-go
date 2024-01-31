package backend

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rpc/connection"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

type ObserverLocalDataService struct {
	TransactionsLocalDataProvider
	backendEvents BackendEvents
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
	queryMode IndexQueryMode,
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
			queryMode:         queryMode,
		},
	}
}

func (o *ObserverLocalDataService) GetEventsForBlockIDsFromStorage(ctx context.Context,
	blockIDs []flow.Identifier,
	eventType string,
	requiredEventEncodingVersion entities.EventEncodingVersion) ([]flow.BlockEvents, error) {
	return o.backendEvents.GetEventsForBlockIDs(ctx, eventType, blockIDs, requiredEventEncodingVersion)
}

func (o *ObserverLocalDataService) GetEventsForHeightRangeFromStorage(
	ctx context.Context,
	eventType string,
	startHeight, endHeight uint64,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]flow.BlockEvents, error) {
	return o.backendEvents.GetEventsForHeightRange(ctx, eventType, startHeight, endHeight, requiredEventEncodingVersion)
}

//func (s *ObserverLocalDataService) GetEventsForBlockIDsFromStorage(_ context.Context, eventType string, blockIDs []flow.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) ([]flow.BlockEvents, error) {
//	// find the block headers for all the block IDs
//	blockHeaders := make([]*flow.Header, 0)
//	for _, blockID := range blockIDs {
//		header, err := s.backendEvents.headers.ByBlockID(blockID)
//		if err != nil {
//			return nil, rpc.ConvertStorageError(fmt.Errorf("failed to get events: %w", err))
//		}
//
//		blockHeaders = append(blockHeaders, header)
//	}
//
//	missing := make([]*flow.Header, 0)
//	resp := make([]flow.BlockEvents, 0)
//	for _, header := range blockHeaders {
//
//		events, err := s.events.ByBlockID(header.ID())
//		if err != nil {
//			// Note: if there are no events for a block, an empty slice is returned
//			if errors.Is(err, storage.ErrNotFound) {
//				missing = append(missing, header)
//				continue
//			}
//			err = fmt.Errorf("failed to get events for block %s: %w", header.ID(), err)
//			return nil, rpc.ConvertError(err, "failed to get events from storage", codes.Internal)
//		}
//
//		filteredEvents := make([]flow.Event, 0)
//		for _, e := range events {
//			if e.Type != flow.EventType(eventType) {
//				continue
//			}
//
//			// events are encoded in CCF format in storage. convert to JSON-CDC if requested
//			if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
//				payload, err := convert.CcfPayloadToJsonPayload(e.Payload)
//				if err != nil {
//					err = fmt.Errorf("failed to convert event payload for block %s: %w", header.ID(), err)
//					return nil, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
//				}
//				e.Payload = payload
//			}
//
//			filteredEvents = append(filteredEvents, e)
//		}
//
//		resp = append(resp, flow.BlockEvents{
//			BlockID:        header.ID(),
//			BlockHeight:    header.Height,
//			BlockTimestamp: header.Timestamp,
//			Events:         filteredEvents,
//		})
//
//	}
//
//	return resp, nil
//}
