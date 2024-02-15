package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGetEvents(t *testing.T) {
	backend := &mock.API{}
	events := generateEventsMocks(backend, 5)

	allBlockIDs := make([]string, len(events))
	for i, e := range events {
		allBlockIDs[i] = e.BlockID.String()
	}
	startHeight := fmt.Sprint(events[0].BlockHeight)
	endHeight := fmt.Sprint(events[len(events)-1].BlockHeight)

	// remove events from the last block to test that an empty BlockEvents is returned when the last
	// block contains no events
	truncatedEvents := append(events[:len(events)-1], flow.BlockEvents{
		BlockHeight:    events[len(events)-1].BlockHeight,
		BlockID:        events[len(events)-1].BlockID,
		BlockTimestamp: events[len(events)-1].BlockTimestamp,
	})

	testVectors := []testVector{
		// valid
		{
			description:      "Get events for a single block by ID",
			request:          getEventReq(t, "A.179b6b1cb6755e31.Foo.Bar", "", "", []string{events[0].BlockID.String()}),
			expectedStatus:   http.StatusOK,
			expectedResponse: testBlockEventResponse(t, []flow.BlockEvents{events[0]}),
		},
		{
			description:      "Get events by all block IDs",
			request:          getEventReq(t, "A.179b6b1cb6755e31.Foo.Bar", "", "", allBlockIDs),
			expectedStatus:   http.StatusOK,
			expectedResponse: testBlockEventResponse(t, events),
		},
		{
			description:      "Get events for height range",
			request:          getEventReq(t, "A.179b6b1cb6755e31.Foo.Bar", startHeight, endHeight, nil),
			expectedStatus:   http.StatusOK,
			expectedResponse: testBlockEventResponse(t, events),
		},
		{
			description:      "Get events range ending at sealed block",
			request:          getEventReq(t, "A.179b6b1cb6755e31.Foo.Bar", "0", "sealed", nil),
			expectedStatus:   http.StatusOK,
			expectedResponse: testBlockEventResponse(t, events),
		},
		{
			description:      "Get events range ending after last block",
			request:          getEventReq(t, "A.179b6b1cb6755e31.Foo.Bar", "0", fmt.Sprint(events[len(events)-1].BlockHeight+5), nil),
			expectedStatus:   http.StatusOK,
			expectedResponse: testBlockEventResponse(t, truncatedEvents),
		},
		// invalid
		{
			description:      "Get invalid - missing all fields",
			request:          getEventReq(t, "", "", "", nil),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"must provide either block IDs or start and end height range"}`,
		},
		{
			description:      "Get invalid - missing query event type",
			request:          getEventReq(t, "", "", "", []string{events[0].BlockID.String()}),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"event type must be provided"}`,
		},
		{
			description:      "Get invalid - missing end height",
			request:          getEventReq(t, "A.179b6b1cb6755e31.Foo.Bar", "100", "", nil),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"must provide either block IDs or start and end height range"}`,
		},
		{
			description:      "Get invalid - start height bigger than end height",
			request:          getEventReq(t, "A.179b6b1cb6755e31.Foo.Bar", "100", "50", nil),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"start height must be less than or equal to end height"}`,
		},
		{
			description:      "Get invalid - too big interval",
			request:          getEventReq(t, "A.179b6b1cb6755e31.Foo.Bar", "0", "5000", nil),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"height range 5000 exceeds maximum allowed of 250"}`,
		},
		{
			description:      "Get invalid - can not provide all params",
			request:          getEventReq(t, "A.179b6b1cb6755e31.Foo.Bar", "100", "120", []string{"10e782612a014b5c9c7d17994d7e67157064f3dd42fa92cd080bfb0fe22c3f71"}),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"can only provide either block IDs or start and end height range"}`,
		},
		{
			description:      "Get invalid - invalid height format",
			request:          getEventReq(t, "A.179b6b1cb6755e31.Foo.Bar", "foo", "120", nil),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"invalid start height: invalid height format"}`,
		},
		{
			description:      "Get invalid - latest block smaller than start",
			request:          getEventReq(t, "A.179b6b1cb6755e31.Foo.Bar", "100000", "sealed", nil),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"current retrieved end height value is lower than start height"}`,
		},
	}

	for _, test := range testVectors {
		t.Run(test.description, func(t *testing.T) {
			assertResponse(t, test.request, test.expectedStatus, test.expectedResponse, backend)
		})
	}

}

func getEventReq(t *testing.T, eventType string, start string, end string, blockIDs []string) *http.Request {
	u, _ := url.Parse("/v1/events")
	q := u.Query()

	if len(blockIDs) > 0 {
		q.Add(BlockQueryParam, strings.Join(blockIDs, ","))
	}

	if start != "" && end != "" {
		q.Add(startHeightQueryParam, start)
		q.Add(endHeightQueryParam, end)
	}

	q.Add(EventTypeQuery, eventType)

	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	require.NoError(t, err)

	return req
}

func generateEventsMocks(backend *mock.API, n int) []flow.BlockEvents {
	events := make([]flow.BlockEvents, n)
	ids := make([]flow.Identifier, n)

	var lastHeader *flow.Header
	for i := 0; i < n; i++ {
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(uint64(i)))
		ids[i] = header.ID()

		events[i] = unittest.BlockEventsFixture(header, 2)

		backend.Mock.
			On("GetEventsForBlockIDs", mocks.Anything, mocks.Anything, []flow.Identifier{header.ID()}, entities.EventEncodingVersion_JSON_CDC_V0).
			Return([]flow.BlockEvents{events[i]}, nil)

		lastHeader = header
	}

	backend.Mock.
		On("GetEventsForBlockIDs", mocks.Anything, mocks.Anything, ids, entities.EventEncodingVersion_JSON_CDC_V0).
		Return(events, nil)

	// range from first to last block
	backend.Mock.On(
		"GetEventsForHeightRange",
		mocks.Anything,
		mocks.Anything,
		events[0].BlockHeight,
		events[len(events)-1].BlockHeight,
		entities.EventEncodingVersion_JSON_CDC_V0,
	).Return(events, nil)

	// range from first to last block + 5
	backend.Mock.On(
		"GetEventsForHeightRange",
		mocks.Anything,
		mocks.Anything,
		events[0].BlockHeight,
		events[len(events)-1].BlockHeight+5,
		entities.EventEncodingVersion_JSON_CDC_V0,
	).Return(append(events[:len(events)-1], unittest.BlockEventsFixture(lastHeader, 0)), nil)

	latestBlock := unittest.BlockHeaderFixture()
	latestBlock.Height = uint64(n - 1)

	// default not found
	backend.Mock.
		On("GetEventsForBlockIDs", mocks.Anything, mocks.Anything, mocks.Anything, entities.EventEncodingVersion_JSON_CDC_V0).
		Return(nil, status.Error(codes.NotFound, "not found"))

	backend.Mock.
		On("GetEventsForHeightRange", mocks.Anything, mocks.Anything).
		Return(nil, status.Error(codes.NotFound, "not found"))

	backend.Mock.
		On("GetLatestBlockHeader", mocks.Anything, true).
		Return(latestBlock, flow.BlockStatusSealed, nil)

	return events
}

func testBlockEventResponse(t *testing.T, events []flow.BlockEvents) string {

	type eventResponse struct {
		Type             flow.EventType  `json:"type"`
		TransactionID    flow.Identifier `json:"transaction_id"`
		TransactionIndex string          `json:"transaction_index"`
		EventIndex       string          `json:"event_index"`
		Payload          string          `json:"payload"`
	}

	type blockEventsResponse struct {
		BlockID        flow.Identifier `json:"block_id"`
		BlockHeight    string          `json:"block_height"`
		BlockTimestamp string          `json:"block_timestamp"`
		Events         []eventResponse `json:"events,omitempty"`
	}

	res := make([]blockEventsResponse, len(events))

	for i, e := range events {
		events := make([]eventResponse, len(e.Events))

		for i, ev := range e.Events {
			events[i] = eventResponse{
				Type:             ev.Type,
				TransactionID:    ev.TransactionID,
				TransactionIndex: fmt.Sprint(ev.TransactionIndex),
				EventIndex:       fmt.Sprint(ev.EventIndex),
				Payload:          util.ToBase64(ev.Payload),
			}
		}

		res[i] = blockEventsResponse{
			BlockID:        e.BlockID,
			BlockHeight:    fmt.Sprint(e.BlockHeight),
			BlockTimestamp: e.BlockTimestamp.Format(time.RFC3339Nano),
			Events:         events,
		}
	}

	data, err := json.Marshal(res)
	require.NoError(t, err)

	return string(data)
}
