package state_stream

import (
	"context"
	"sync/atomic"

	access "github.com/onflow/flow/protobuf/go/flow/executiondata"
	executiondata "github.com/onflow/flow/protobuf/go/flow/executiondata"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/engine/common/state_stream"
	"github.com/onflow/flow-go/model/flow"
)

type Handler struct {
	api   state_stream.API
	chain flow.Chain

	eventFilterConfig state_stream.EventFilterConfig

	maxStreams  int32
	streamCount atomic.Int32
}

func NewHandler(api state_stream.API, chain flow.Chain, conf state_stream.EventFilterConfig, maxGlobalStreams uint32) *Handler {
	h := &Handler{
		api:               api,
		chain:             chain,
		eventFilterConfig: conf,
		maxStreams:        int32(maxGlobalStreams),
		streamCount:       atomic.Int32{},
	}
	return h
}

func (h *Handler) GetExecutionDataByBlockID(ctx context.Context, request *access.GetExecutionDataByBlockIDRequest) (*access.GetExecutionDataByBlockIDResponse, error) {
	blockID, err := convert.BlockID(request.GetBlockId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "could not convert block ID: %v", err)
	}

	execData, err := h.api.GetExecutionDataByBlockID(ctx, blockID)
	if err != nil {
		return nil, rpc.ConvertError(err, "could no get execution data", codes.Internal)
	}

	message, err := convert.BlockExecutionDataToMessage(execData)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not convert execution data to entity: %v", err)
	}

	return &access.GetExecutionDataByBlockIDResponse{BlockExecutionData: message}, nil
}

func (h *Handler) SubscribeExecutionData(request *access.SubscribeExecutionDataRequest, stream access.ExecutionDataAPI_SubscribeExecutionDataServer) error {
	// check if the maximum number of streams is reached
	if h.streamCount.Load() >= h.maxStreams {
		return status.Errorf(codes.ResourceExhausted, "maximum number of streams reached")
	}
	h.streamCount.Add(1)
	defer h.streamCount.Add(-1)

	startBlockID := flow.ZeroID
	if request.GetStartBlockId() != nil {
		blockID, err := convert.BlockID(request.GetStartBlockId())
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "could not convert start block ID: %v", err)
		}
		startBlockID = blockID
	}

	sub := h.api.SubscribeExecutionData(stream.Context(), startBlockID, request.GetStartBlockHeight())

	for {
		v, ok := <-sub.Channel()
		if !ok {
			if sub.Err() != nil {
				return rpc.ConvertError(sub.Err(), "stream encountered an error", codes.Internal)
			}
			return nil
		}

		resp, ok := v.(*state_stream.ExecutionDataResponse)
		if !ok {
			return status.Errorf(codes.Internal, "unexpected response type: %T", v)
		}

		execData, err := convert.BlockExecutionDataToMessage(resp.ExecutionData)
		if err != nil {
			return status.Errorf(codes.Internal, "could not convert execution data to entity: %v", err)
		}

		err = stream.Send(&executiondata.SubscribeExecutionDataResponse{
			BlockHeight:        resp.Height,
			BlockExecutionData: execData,
		})
		if err != nil {
			return rpc.ConvertError(err, "could not send response", codes.Internal)
		}
	}
}

func (h *Handler) SubscribeEvents(request *access.SubscribeEventsRequest, stream access.ExecutionDataAPI_SubscribeEventsServer) error {
	// check if the maximum number of streams is reached
	if h.streamCount.Load() >= h.maxStreams {
		return status.Errorf(codes.ResourceExhausted, "maximum number of streams reached")
	}
	h.streamCount.Add(1)
	defer h.streamCount.Add(-1)

	startBlockID := flow.ZeroID
	if request.GetStartBlockId() != nil {
		blockID, err := convert.BlockID(request.GetStartBlockId())
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "could not convert start block ID: %v", err)
		}
		startBlockID = blockID
	}

	filter := state_stream.EventFilter{}
	if request.GetFilter() != nil {
		var err error
		reqFilter := request.GetFilter()
		filter, err = state_stream.NewEventFilter(
			h.eventFilterConfig,
			h.chain,
			reqFilter.GetEventType(),
			reqFilter.GetAddress(),
			reqFilter.GetContract(),
		)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "invalid event filter: %v", err)
		}
	}
	sub := h.api.SubscribeEvents(stream.Context(), startBlockID, request.GetStartBlockHeight(), filter)

	for {
		v, ok := <-sub.Channel()
		if !ok {
			if sub.Err() != nil {
				return rpc.ConvertError(sub.Err(), "stream encountered an error", codes.Internal)
			}
			return nil
		}

		resp, ok := v.(*state_stream.EventsResponse)
		if !ok {
			return status.Errorf(codes.Internal, "unexpected response type: %T", v)
		}

		err := stream.Send(&executiondata.SubscribeEventsResponse{
			BlockHeight: resp.Height,
			BlockId:     convert.IdentifierToMessage(resp.BlockID),
			Events:      convert.EventsToMessages(resp.Events),
		})
		if err != nil {
			return rpc.ConvertError(err, "could not send response", codes.Internal)
		}
	}
}
