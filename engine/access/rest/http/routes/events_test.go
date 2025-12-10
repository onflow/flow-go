package routes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/routes"
	"github.com/onflow/flow-go/engine/access/rest/router"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow/protobuf/go/flow/entities"
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
		//valid
		{
			description: "Get events for a single block by ID",
			request: buildRequest(
				t,
				"A.179b6b1cb6755e31.Foo.Bar",
				"",
				"",
				[]string{events[0].BlockID.String()},
				"2",
				[]string{},
				"true",
			),
			expectedStatus:   http.StatusOK,
			expectedResponse: buildExpectedResponse(t, []flow.BlockEvents{events[0]}, true),
		},
		{
			description: "Get events by all block IDs",
			request: buildRequest(
				t,
				"A.179b6b1cb6755e31.Foo.Bar",
				"",
				"",
				allBlockIDs,
				"2",
				[]string{},
				"true",
			),
			expectedStatus:   http.StatusOK,
			expectedResponse: buildExpectedResponse(t, events, true),
		},
		{
			description: "Get events for height range",
			request: buildRequest(
				t,
				"A.179b6b1cb6755e31.Foo.Bar",
				startHeight,
				endHeight,
				nil,
				"2",
				[]string{},
				"true",
			),
			expectedStatus:   http.StatusOK,
			expectedResponse: buildExpectedResponse(t, events, true),
		},
		{
			description: "Get events range ending after last block",
			request: buildRequest(
				t,
				"A.179b6b1cb6755e31.Foo.Bar",
				"0",
				fmt.Sprint(events[len(events)-1].BlockHeight+5),
				nil,
				"2",
				[]string{},
				"true",
			),
			expectedStatus:   http.StatusOK,
			expectedResponse: buildExpectedResponse(t, truncatedEvents, true),
		},
		// invalid
		{
			description: "Get invalid - missing all fields",
			request: buildRequest(
				t,
				"",
				"",
				"",
				nil,
				"2",
				[]string{},
				"true",
			),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"must provide either block IDs or start and end height range"}`,
		},
		{
			description: "Get invalid - missing query event type",
			request: buildRequest(
				t,
				"",
				"",
				"",
				[]string{events[0].BlockID.String()},
				"2",
				[]string{},
				"true",
			),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"event type must be provided"}`,
		},
		{
			description: "Get invalid - missing end height",
			request: buildRequest(
				t,
				"A.179b6b1cb6755e31.Foo.Bar",
				"100",
				"", nil,
				"2",
				[]string{},
				"true",
			),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"must provide either block IDs or start and end height range"}`,
		},
		{
			description: "Get invalid - start height bigger than end height",
			request: buildRequest(
				t,
				"A.179b6b1cb6755e31.Foo.Bar",
				"100",
				"50",
				nil,
				"2",
				[]string{},
				"true",
			),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"start height must be less than or equal to end height"}`,
		},
		{
			description: "Get invalid - too big interval",
			request: buildRequest(
				t,
				"A.179b6b1cb6755e31.Foo.Bar",
				"0",
				"5000",
				nil,
				"2",
				[]string{},
				"true",
			),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"height range 5000 exceeds maximum allowed of 250"}`,
		},
		{
			description: "Get invalid - can not provide all params",
			request: buildRequest(
				t,
				"A.179b6b1cb6755e31.Foo.Bar",
				"100",
				"120",
				[]string{"10e782612a014b5c9c7d17994d7e67157064f3dd42fa92cd080bfb0fe22c3f71"},
				"2",
				[]string{},
				"true",
			),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"can only provide either block IDs or start and end height range"}`,
		},
		{
			description: "Get invalid - invalid height format",
			request: buildRequest(
				t,
				"A.179b6b1cb6755e31.Foo.Bar",
				"foo",
				"120",
				nil,
				"2",
				[]string{},
				"true",
			),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"invalid start height: invalid height format"}`,
		},
		{
			description: "Get invalid - latest block smaller than start",
			request: buildRequest(
				t,
				"A.179b6b1cb6755e31.Foo.Bar",
				"100000",
				"sealed",
				nil,
				"2",
				[]string{},
				"true",
			),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: `{"code":400,"message":"current retrieved end height value is lower than start height"}`,
		},
	}

	for _, test := range testVectors {
		t.Run(test.description, func(t *testing.T) {
			t.Log("request: ", test.request.URL.String())
			router.AssertResponse(t, test.request, test.expectedStatus, test.expectedResponse, backend)
		})
	}

}

func TestGetEvents_ParseExecutionState(t *testing.T) {
	eventType := "A.179b6b1cb6755e31.Foo.Bar"
	startHeight := 0
	endHeight := 5
	expectedBlockEvents := make([]flow.BlockEvents, endHeight)

	for i := 0; i < len(expectedBlockEvents); i++ {
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(uint64(i)))
		expectedBlockEvents[i] = unittest.BlockEventsFixture(header, 2)
	}

	backend := mock.NewAPI(t)
	backend.
		On(
			"GetEventsForHeightRange",
			mocks.Anything,
			eventType,
			uint64(startHeight),
			uint64(endHeight),
			entities.EventEncodingVersion_JSON_CDC_V0,
			mocks.Anything,
		).
		Return(expectedBlockEvents, nil, nil)

	t.Run("empty execution state query", func(t *testing.T) {
		request := buildRequest(
			t,
			eventType,
			"0",
			fmt.Sprint(endHeight),
			[]string{},
			"",
			[]string{},
			"",
		)

		expectedResponseBody := buildExpectedResponse(t, expectedBlockEvents, false)
		router.AssertOKResponse(t, request, expectedResponseBody, backend)
	})

	t.Run("empty agreeing executors count", func(t *testing.T) {
		request := buildRequest(
			t,
			eventType,
			"0",
			fmt.Sprint(endHeight),
			[]string{},
			"",
			unittest.IdentifierListFixture(2).Strings(),
			"true",
		)

		expectedResponseBody := buildExpectedResponse(t, expectedBlockEvents, true)
		router.AssertOKResponse(t, request, expectedResponseBody, backend)
	})

	t.Run("empty required executors", func(t *testing.T) {
		request := buildRequest(
			t,
			eventType,
			"0",
			fmt.Sprint(endHeight),
			[]string{},
			"2",
			[]string{},
			"true",
		)

		expectedResponseBody := buildExpectedResponse(t, expectedBlockEvents, true)
		router.AssertOKResponse(t, request, expectedResponseBody, backend)
	})

	t.Run("empty include executor metadata", func(t *testing.T) {
		request := buildRequest(
			t,
			eventType,
			"0",
			fmt.Sprint(endHeight),
			[]string{},
			"2",
			unittest.IdentifierListFixture(2).Strings(),
			"",
		)

		expectedResponseBody := buildExpectedResponse(t, expectedBlockEvents, true)
		router.AssertOKResponse(t, request, expectedResponseBody, backend)
	})

	t.Run("agreeing executors count equals 0", func(t *testing.T) {
		request := buildRequest(
			t,
			eventType,
			"0",
			fmt.Sprint(endHeight),
			[]string{},
			"0",
			unittest.IdentifierListFixture(2).Strings(),
			"true",
		)

		rr := router.ExecuteRequest(request, backend)
		// agreeing executors count should be either omitted or greater than 0
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestGetEvents_GetAtSealedBlock(t *testing.T) {
	expectedBlockEvents := make([]flow.BlockEvents, 5)
	for i := 0; i < len(expectedBlockEvents); i++ {
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(uint64(i)))
		expectedBlockEvents[i] = unittest.BlockEventsFixture(header, 2)
	}

	eventType := "A.179b6b1cb6755e31.Foo.Bar"
	latestBlockHeader := unittest.BlockHeaderFixture()

	backend := mock.NewAPI(t)
	backend.Mock.
		On("GetLatestBlockHeader", mocks.Anything, true).
		Return(latestBlockHeader, flow.BlockStatusSealed, nil).
		Once()

	backend.
		On(
			"GetEventsForHeightRange",
			mocks.Anything,
			eventType,
			uint64(0),
			mocks.Anything,
			entities.EventEncodingVersion_JSON_CDC_V0,
			mocks.Anything,
		).
		Return(expectedBlockEvents, nil, nil).
		Once()

	request := buildRequest(
		t,
		eventType,
		"0",
		"sealed",
		[]string{},
		"2",
		[]string{},
		"true",
	)
	t.Log("request: ", request.URL.String())

	expectedResponseBody := buildExpectedResponse(t, expectedBlockEvents, false)
	router.AssertOKResponse(t, request, expectedResponseBody, backend)
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
			On(
				"GetEventsForBlockIDs",
				mocks.Anything,
				mocks.Anything,
				[]flow.Identifier{header.ID()},
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return([]flow.BlockEvents{events[i]}, nil, nil).
			Once()

		lastHeader = header
	}

	backend.Mock.
		On(
			"GetEventsForBlockIDs",
			mocks.Anything,
			mocks.Anything, ids,
			entities.EventEncodingVersion_JSON_CDC_V0,
			mocks.Anything,
		).
		Return(events, nil, nil).
		Once()

	// range from first to last block
	backend.Mock.
		On(
			"GetEventsForHeightRange",
			mocks.Anything,
			mocks.Anything,
			events[0].BlockHeight,
			events[len(events)-1].BlockHeight,
			entities.EventEncodingVersion_JSON_CDC_V0,
			mocks.Anything,
		).
		Return(events, nil, nil).
		Once()

	// range from first to last block + 5
	backend.Mock.
		On(
			"GetEventsForHeightRange",
			mocks.Anything,
			mocks.Anything,
			events[0].BlockHeight,
			events[len(events)-1].BlockHeight+5,
			entities.EventEncodingVersion_JSON_CDC_V0,
			mocks.Anything,
		).
		Return(append(events[:len(events)-1], unittest.BlockEventsFixture(lastHeader, 0)), nil, nil).
		Once()

	latestBlock := unittest.BlockHeaderFixture()
	latestBlock.Height = uint64(n - 1)

	// default not found
	backend.Mock.
		On(
			"GetEventsForBlockIDs",
			mocks.Anything,
			mocks.Anything,
			mocks.Anything,
			entities.EventEncodingVersion_JSON_CDC_V0,
			mocks.Anything,
		).
		Return(nil, nil, status.Error(codes.NotFound, "not found")).
		Once()

	backend.Mock.
		On(
			"GetEventsForHeightRange",
			mocks.Anything,
			mocks.Anything,
			mocks.Anything,
			mocks.Anything,
			mocks.Anything,
			mocks.Anything,
		).
		Return(events[0:3], nil, nil).
		Once()

	backend.Mock.
		On("GetLatestBlockHeader", mocks.Anything, true).
		Return(latestBlock, flow.BlockStatusSealed, nil)

	return events
}

func buildRequest(
	t *testing.T,
	eventType string,
	start string,
	end string,
	blockIDs []string,
	agreeingExecutorsCount string,
	requiredExecutors []string,
	includeExecutorMetadata string,
) *http.Request {
	u, _ := url.Parse("/v1/events")
	q := u.Query()

	if len(blockIDs) > 0 {
		q.Add(routes.BlockQueryParam, strings.Join(blockIDs, ","))
	}

	if start != "" && end != "" {
		q.Add(router.StartHeightQueryParam, start)
		q.Add(router.EndHeightQueryParam, end)
	}

	q.Add(router.AgreeingExecutorsCountQueryParam, agreeingExecutorsCount)

	if len(requiredExecutors) > 0 {
		q.Add(router.RequiredExecutorIdsQueryParam, strings.Join(requiredExecutors, ","))
	}

	if len(includeExecutorMetadata) > 0 {
		q.Add(router.IncludeExecutorMetadataQueryParam, includeExecutorMetadata)
	}

	q.Add(routes.EventTypeQuery, eventType)

	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	require.NoError(t, err)

	return req
}

func buildExpectedResponse(t *testing.T, events []flow.BlockEvents, includeMetadata bool) string {
	list := models.NewBlockEventsList(events, nil, includeMetadata)
	data, err := json.Marshal(list)
	require.NoError(t, err)

	return string(data)
}
