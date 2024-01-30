package backend

import (
	"context"
	"errors"
	"fmt"
	"github.com/onflow/flow-go/module/irrecoverable"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"google.golang.org/grpc/codes"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type ObserverLocalDataService struct {
	txLocalProvider TransactionsLocalDataProvider
	headers         storage.Headers
}

func (s *ObserverLocalDataService) GetEventsForBlockIDsFromStorage(_ context.Context, eventType string, blockIDs []flow.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) ([]flow.BlockEvents, error) {
	// find the block headers for all the block IDs
	blockHeaders := make([]*flow.Header, 0)
	for _, blockID := range blockIDs {
		header, err := s.headers.ByBlockID(blockID)
		if err != nil {
			return nil, rpc.ConvertStorageError(fmt.Errorf("failed to get events: %w", err))
		}

		blockHeaders = append(blockHeaders, header)
	}

	missing := make([]*flow.Header, 0)
	resp := make([]flow.BlockEvents, 0)
	for _, header := range blockHeaders {

		events, err := s.txLocalProvider.events.ByBlockID(header.ID())
		if err != nil {
			// Note: if there are no events for a block, an empty slice is returned
			if errors.Is(err, storage.ErrNotFound) {
				missing = append(missing, header)
				continue
			}
			err = fmt.Errorf("failed to get events for block %s: %w", header.ID(), err)
			return nil, rpc.ConvertError(err, "failed to get events from storage", codes.Internal)
		}

		filteredEvents := make([]flow.Event, 0)
		for _, e := range events {
			if e.Type != flow.EventType(eventType) {
				continue
			}

			// events are encoded in CCF format in storage. convert to JSON-CDC if requested
			if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
				payload, err := convert.CcfPayloadToJsonPayload(e.Payload)
				if err != nil {
					err = fmt.Errorf("failed to convert event payload for block %s: %w", header.ID(), err)
					return nil, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
				}
				e.Payload = payload
			}

			filteredEvents = append(filteredEvents, e)
		}

		resp = append(resp, flow.BlockEvents{
			BlockID:        header.ID(),
			BlockHeight:    header.Height,
			BlockTimestamp: header.Timestamp,
			Events:         filteredEvents,
		})

	}

	return resp, nil
}

func (b *backendEvents) GetEventsForHeightRangeFromStorage(
	ctx context.Context,
	eventType string,
	startHeight, endHeight uint64,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]flow.BlockEvents, error) {

	if endHeight < startHeight {
		return nil, status.Error(codes.InvalidArgument, "start height must not be larger than end height")
	}

	rangeSize := endHeight - startHeight + 1 // range is inclusive on both ends
	if rangeSize > uint64(b.maxHeightRange) {
		return nil, status.Errorf(codes.InvalidArgument,
			"requested block range (%d) exceeded maximum (%d)", rangeSize, b.maxHeightRange)
	}

	// get the latest sealed block header
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		// sealed block must be in the store, so throw an exception for any error
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	// start height should not be beyond the last sealed height
	if startHeight > sealed.Height {
		return nil, status.Errorf(codes.OutOfRange,
			"start height %d is greater than the last sealed block height %d", startHeight, sealed.Height)
	}

	// limit max height to last sealed block in the chain
	//
	// Note: this causes unintuitive behavior for clients making requests through a proxy that
	// fronts multiple nodes. With that setup, clients may receive responses for a smaller range
	// than requested because the node serving the request has a slightly delayed view of the chain.
	//
	// An alternative option is to return an error here, but that's likely to cause more pain for
	// these clients since the requests would intermittently fail. it's recommended instead to
	// check the block height of the last message in the response. this will be the last block
	// height searched, and can be used to determine the start height for the next range.
	if endHeight > sealed.Height {
		endHeight = sealed.Height
	}

	// find the block headers for all the blocks between min and max height (inclusive)
	blockHeaders := make([]blockMetadata, 0, endHeight-startHeight+1)

	for i := startHeight; i <= endHeight; i++ {
		// this looks inefficient, but is actually what's done under the covers by `headers.ByHeight`
		// and avoids calculating header.ID() for each block.
		blockID, err := b.headers.BlockIDByHeight(i)
		if err != nil {
			return nil, rpc.ConvertStorageError(fmt.Errorf("failed to get blockID for %d: %w", i, err))
		}
		header, err := b.headers.ByBlockID(blockID)
		if err != nil {
			return nil, rpc.ConvertStorageError(fmt.Errorf("failed to get block header for %d: %w", i, err))
		}

		blockHeaders = append(blockHeaders, blockMetadata{
			ID:        blockID,
			Height:    header.Height,
			Timestamp: header.Timestamp,
		})
	}

	// ask if we should call GetBlockHeaderByIDFromStorage or copy code here?

}

func (s *ObserverLocalDataService) GetBlockHeaderByIDFromStorage(_ context.Context, id flow.Identifier) (*flow.Header, flow.BlockStatus, error) {
	header, err := s.headers.ByBlockID(id)
	if err != nil {
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(err)
	}

	//stat, err := s.getBlockStatus(ctx, header) //ask
	//if err != nil {
	//	return nil, stat, err
	//}
	return header, flow.BlockStatusUnknown, nil
}

func (s *ObserverLocalDataService) GetBlockHeaderByHeightFromStorage(_ context.Context, height uint64) (*flow.Header, flow.BlockStatus, error) {
	header, err := s.headers.ByHeight(height)
	if err != nil {
		return nil, flow.BlockStatusUnknown, rpc.ConvertStorageError(err)
	}

	//stat, err := b.getBlockStatus(ctx, header)
	//if err != nil {
	//	return nil, stat, err
	//}
	return header, flow.BlockStatusUnknown, nil
}
