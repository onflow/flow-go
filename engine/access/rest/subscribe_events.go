package rest

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/engine/common/state_stream"

	executiondata "github.com/onflow/flow/protobuf/go/flow/executiondata"
)

func SubscribeEvents(r *request.Request, w http.ResponseWriter, h *state_stream.SubscribeHandler) (interface{}, error) {
	req, err := r.SubscribeEventsRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	// Upgrade the HTTP connection to a WebSocket connection
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r.Request, nil)
	if err != nil {
		err = fmt.Errorf("webSocket upgrade error: %s", err)
		return nil, err
	}
	defer conn.Close()

	var filter state_stream.EventFilter
	// Retrieve the filter parameters from the request, if provided

	filter, err = state_stream.NewEventFilter(
		h.EventFilterConfig,
		r.Chain,
		req.EventTypes,
		req.Addresses,
		req.Contracts,
	)
	if err != nil {
		err = fmt.Errorf("invalid event filter: %s", err)
		return nil, err
	}

	sub, err := h.SubscribeEvents(r.Context(), req.StartBlockID, req.StartHeight, filter)

	// Write messages to the WebSocket connection
	writeToWebSocket := func(resp *state_stream.EventsResponse) error {
		// Prepare the response message
		response := &executiondata.SubscribeEventsResponse{
			BlockHeight: resp.Height,
			BlockId:     convert.IdentifierToMessage(resp.BlockID),
			Events:      convert.EventsToMessages(resp.Events),
		}

		// Send the response message over the WebSocket connection
		return conn.WriteJSON(response)
	}

	for {
		//select {
		//case
		v, ok := <-sub.Channel()
		if !ok {
			if sub.Err() != nil {
				err = fmt.Errorf("stream encountered an error: %w", sub.Err())
				return nil, err
			}
			return nil, err
		}

		resp, ok := v.(*state_stream.EventsResponse)
		if !ok {
			err = fmt.Errorf("unexpected response type: %s", v)
			return nil, err
		}

		// Write the response to the WebSocket connection
		err := writeToWebSocket(resp)
		if err != nil {
			err = fmt.Errorf("failed to send response: %w", err)
			return nil, err
		}

		//case <-conn.ReadyState():
		//	// WebSocket connection closed or in a non-writable state
		//	return
		//}
	}
}
