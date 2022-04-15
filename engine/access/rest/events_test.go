package rest

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/onflow/flow-go/engine/access/rest/util"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetEvents(t *testing.T) {
	backend := &mock.API{}
	events := generateEventsMocks(backend, 5)

	allBlockIDs := make([]string, len(events))
	for i, e := range events {
		allBlockIDs[i] = e.BlockID.String()
	}
	startHeight := fmt.Sprintf("%d", events[0].BlockHeight)
	endHeight := fmt.Sprintf("%d", events[len(events)-1].BlockHeight)

	testVectors := []testVector{
		// valid
		{
			description:      "Get events for a single block by ID",
			request:          getEventReq(t, "A.179b6b1cb6755e31.Foo.Bar", "", "", []string{events[0].BlockID.String()}),
			expectedStatus:   http.StatusOK,
			expectedResponse: testBlockEventResponse([]flow.BlockEvents{events[0]}),
		},
		{
			description:      "Get events by all block IDs",
			request:          getEventReq(t, "A.179b6b1cb6755e31.Foo.Bar", "", "", allBlockIDs),
			expectedStatus:   http.StatusOK,
			expectedResponse: testBlockEventResponse(events),
		},
		{
			description:      "Get events for height range",
			request:          getEventReq(t, "A.179b6b1cb6755e31.Foo.Bar", startHeight, endHeight, nil),
			expectedStatus:   http.StatusOK,
			expectedResponse: testBlockEventResponse(events),
		},
		{
			description:      "Get invalid - invalid height format",
			request:          getEventReq(t, "A.179b6b1cb6755e31.Foo.Bar", "0", "sealed", nil),
			expectedStatus:   http.StatusOK,
			expectedResponse: testBlockEventResponse(events),
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
			expectedResponse: `{"code":400,"message":"height range 5000 exceeds maximum allowed of 50"}`,
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
		q.Add(blockQueryParam, strings.Join(blockIDs, ","))
	}

	if start != "" && end != "" {
		q.Add(startHeightQueryParam, start)
		q.Add(endHeightQueryParam, end)
	}

	q.Add(eventTypeQuery, eventType)

	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	require.NoError(t, err)

	return req
}

func generateEventsMocks(backend *mock.API, n int) []flow.BlockEvents {
	events := make([]flow.BlockEvents, n)
	ids := make([]flow.Identifier, n)

	for i := 0; i < n; i++ {
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(uint64(i)))
		ids[i] = header.ID()

		events[i] = unittest.BlockEventsFixture(header, 2)

		backend.Mock.
			On("GetEventsForBlockIDs", mocks.Anything, mocks.Anything, []flow.Identifier{header.ID()}).
			Return([]flow.BlockEvents{events[i]}, nil)
	}

	backend.Mock.
		On("GetEventsForBlockIDs", mocks.Anything, mocks.Anything, ids).
		Return(events, nil)

	backend.Mock.On(
		"GetEventsForHeightRange",
		mocks.Anything,
		mocks.Anything,
		events[0].BlockHeight,
		events[len(events)-1].BlockHeight,
	).Return(events, nil)

	latestBlock := unittest.BlockHeaderFixture()
	latestBlock.Height = uint64(n - 1)

	// default not found
	backend.Mock.
		On("GetEventsForBlockIDs", mocks.Anything, mocks.Anything, mocks.Anything).
		Return(nil, status.Error(codes.NotFound, "not found"))

	backend.Mock.
		On("GetEventsForHeightRange", mocks.Anything, mocks.Anything).
		Return(nil, status.Error(codes.NotFound, "not found"))

	backend.Mock.
		On("GetLatestBlockHeader", mocks.Anything, true).
		Return(&latestBlock, nil)

	return events
}

func testBlockEventResponse(events []flow.BlockEvents) string {
	res := make([]string, len(events))

	for i, e := range events {
		events := make([]string, len(e.Events))

		for i, ev := range e.Events {
			events[i] = fmt.Sprintf(`{
				"type": "%s",
				"transaction_id": "%s",
				"transaction_index": "%d",
				"event_index": "%d",
				"payload": "%s"
			}`, ev.Type, ev.TransactionID, ev.TransactionIndex, ev.EventIndex, util.ToBase64(ev.Payload))
		}

		res[i] = fmt.Sprintf(`{
			"block_id": "%s",
			"block_height": "%d",
			"block_timestamp": "%s",
			"events": [%s]
		}`,
			e.BlockID.String(),
			e.BlockHeight,
			e.BlockTimestamp.Format(time.RFC3339Nano),
			strings.Join(events, ","),
		)
	}

	return fmt.Sprintf(`[%s]`, strings.Join(res, ","))
}
