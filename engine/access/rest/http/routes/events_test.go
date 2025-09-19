package routes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/routes"
	"github.com/onflow/flow-go/engine/access/rest/router"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

var eventType string = "A.179b6b1cb6755e31.Foo.Bar"

func TestGetEvents_GetEventsForBlockIDs(t *testing.T) {
	var block *flow.Header
	blockEvents := make([]flow.BlockEvents, 5)
	for i := 0; i < len(blockEvents); i++ {
		block = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(uint64(i)))
		blockEvents[i] = unittest.BlockEventsFixture(block, 2)
	}

	backend := mock.NewAPI(t)

	t.Run("for 1 block", func(t *testing.T) {
		backend.
			On(
				"GetEventsForBlockIDs",
				mocks.Anything,
				eventType,
				[]flow.Identifier{block.ID()},
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(blockEvents, access.ExecutorMetadata{}, nil).
			Once()

		request := buildRequest(
			t,
			eventType,
			"",
			"",
			[]string{block.ID().String()},
			"",
			[]string{},
			"",
		)

		expectedResponse := buildExpectedResponse(t, blockEvents, false)
		router.AssertOKResponse(t, request, expectedResponse, backend)
	})

	t.Run("for n blocks", func(t *testing.T) {
		block2 := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(uint64(1)))
		blockEvents2 := unittest.BlockEventsFixture(block2, 2)
		mergedEvents := append(blockEvents, blockEvents2)

		backend.
			On(
				"GetEventsForBlockIDs",
				mocks.Anything,
				eventType,
				[]flow.Identifier{block.ID(), block2.ID()},
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(mergedEvents, access.ExecutorMetadata{}, nil).
			Once()

		request := buildRequest(
			t,
			eventType,
			"",
			"",
			[]string{block.ID().String(), block2.ID().String()},
			"",
			[]string{},
			"",
		)

		expectedResponse := buildExpectedResponse(t, mergedEvents, false)
		router.AssertOKResponse(t, request, expectedResponse, backend)
	})

	t.Run("blocks not provided (invalid argument)", func(t *testing.T) {
		backend.
			On(
				"GetEventsForBlockIDs",
				mocks.Anything,
				eventType,
				[]flow.Identifier{block.ID()},
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(nil, access.ExecutorMetadata{},
				status.Error(codes.InvalidArgument, "block IDs must not be empty")).
			Once()

		request := buildRequest(
			t,
			eventType,
			"",
			"",
			[]string{block.ID().String()},
			"",
			[]string{},
			"",
		)

		rr := router.ExecuteRequest(request, backend)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("internal error", func(t *testing.T) {
		backend.
			On(
				"GetEventsForBlockIDs",
				mocks.Anything,
				eventType,
				[]flow.Identifier{block.ID()},
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(nil, access.ExecutorMetadata{}, assert.AnError).
			Once()

		request := buildRequest(
			t,
			eventType,
			"",
			"",
			[]string{block.ID().String()},
			"",
			[]string{},
			"",
		)

		rr := router.ExecuteRequest(request, backend)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestGetEvents_GetEventsForHeightRange(t *testing.T) {
	blockEvents := make([]flow.BlockEvents, 5)
	for i := 0; i < len(blockEvents); i++ {
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(uint64(i)))
		blockEvents[i] = unittest.BlockEventsFixture(block, 2)
	}

	backend := mock.NewAPI(t)

	t.Run("get events for height range", func(t *testing.T) {
		backend.
			On(
				"GetEventsForHeightRange",
				mocks.Anything,
				eventType,
				blockEvents[0].BlockHeight,
				blockEvents[len(blockEvents)-1].BlockHeight,
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(blockEvents, access.ExecutorMetadata{}, nil)

		request := buildRequest(
			t,
			eventType,
			fmt.Sprint(blockEvents[0].BlockHeight),
			fmt.Sprint(blockEvents[len(blockEvents)-1].BlockHeight),
			[]string{},
			"",
			[]string{},
			"",
		)

		expectedResponse := buildExpectedResponse(t, blockEvents, false)
		router.AssertOKResponse(t, request, expectedResponse, backend)
	})

	t.Run("height range ending after last block", func(t *testing.T) {
		backend.
			On(
				"GetEventsForHeightRange",
				mocks.Anything,
				eventType,
				blockEvents[0].BlockHeight,
				blockEvents[len(blockEvents)-1].BlockHeight+5,
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(blockEvents, access.ExecutorMetadata{}, nil)

		request := buildRequest(
			t,
			eventType,
			fmt.Sprint(blockEvents[0].BlockHeight),
			fmt.Sprint(blockEvents[len(blockEvents)-1].BlockHeight+5),
			[]string{},
			"",
			[]string{},
			"",
		)

		expectedResponse := buildExpectedResponse(t, blockEvents, false)
		router.AssertOKResponse(t, request, expectedResponse, backend)
	})
}

func TestGetEvents_InvalidRequest(t *testing.T) {
	t.Run("all fields missing", func(t *testing.T) {
		request := buildRequest(
			t,
			"",
			"",
			"",
			nil,
			"2",
			[]string{},
			"true",
		)

		router.AssertResponse(
			t,
			request,
			http.StatusBadRequest,
			`{"code":400,"message":"must provide either block IDs or start and end height range"}`,
			nil,
		)
	})

	t.Run("query event type missing", func(t *testing.T) {
		request := buildRequest(
			t,
			"",
			"",
			"",
			[]string{unittest.IdentifierFixture().String()},
			"2",
			[]string{},
			"true",
		)

		router.AssertResponse(
			t,
			request,
			http.StatusBadRequest,
			`{"code":400,"message":"event type must be provided"}`,
			nil,
		)
	})

	t.Run("end height missing", func(t *testing.T) {
		request := buildRequest(
			t,
			"A.179b6b1cb6755e31.Foo.Bar",
			"100",
			"",
			nil,
			"2",
			[]string{},
			"true",
		)

		router.AssertResponse(
			t,
			request,
			http.StatusBadRequest,
			`{"code":400,"message":"must provide either block IDs or start and end height range"}`,
			nil,
		)
	})

	t.Run("start height greater than end height", func(t *testing.T) {
		request := buildRequest(
			t,
			"A.179b6b1cb6755e31.Foo.Bar",
			"100",
			"50",
			nil,
			"2",
			[]string{},
			"true",
		)

		router.AssertResponse(
			t,
			request,
			http.StatusBadRequest,
			`{"code":400,"message":"start height must be less than or equal to end height"}`,
			nil,
		)
	})

	t.Run("too big interval", func(t *testing.T) {
		request := buildRequest(
			t,
			"A.179b6b1cb6755e31.Foo.Bar",
			"0",
			"5000",
			nil,
			"2",
			[]string{},
			"true",
		)

		router.AssertResponse(
			t,
			request,
			http.StatusBadRequest,
			`{"code":400,"message":"height range 5000 exceeds maximum allowed of 250"}`,
			nil,
		)
	})

	t.Run("all fields provided", func(t *testing.T) {
		request := buildRequest(
			t,
			"A.179b6b1cb6755e31.Foo.Bar",
			"100",
			"120",
			[]string{"10e782612a014b5c9c7d17994d7e67157064f3dd42fa92cd080bfb0fe22c3f71"},
			"2",
			[]string{},
			"true",
		)

		router.AssertResponse(
			t,
			request,
			http.StatusBadRequest,
			`{"code":400,"message":"can only provide either block IDs or start and end height range"}`,
			nil,
		)
	})

	t.Run("invalid height format", func(t *testing.T) {
		request := buildRequest(
			t,
			"A.179b6b1cb6755e31.Foo.Bar",
			"foo",
			"120",
			nil,
			"2",
			[]string{},
			"true",
		)

		router.AssertResponse(
			t,
			request,
			http.StatusBadRequest,
			`{"code":400,"message":"invalid start height: invalid height format"}`,
			nil,
		)
	})

	t.Run("last block smaller than start height", func(t *testing.T) {
		latestBlock := unittest.BlockHeaderFixture()
		backend := mock.NewAPI(t)
		backend.
			On("GetLatestBlockHeader", mocks.Anything, true).
			Return(latestBlock, flow.BlockStatusSealed, nil).
			Once()

		request := buildRequest(
			t,
			"A.179b6b1cb6755e31.Foo.Bar",
			fmt.Sprint(latestBlock.Height+1),
			"sealed",
			nil,
			"2",
			[]string{},
			"true",
		)

		router.AssertResponse(
			t,
			request,
			http.StatusBadRequest,
			`{"code":400,"message":"current retrieved end height value is lower than start height"}`,
			backend,
		)
	})
}

func TestGetEvents_ParseExecutionState(t *testing.T) {
	blockEvents := make([]flow.BlockEvents, 5)
	for i := 0; i < len(blockEvents); i++ {
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(uint64(i)))
		blockEvents[i] = unittest.BlockEventsFixture(block, 2)
	}

	backend := mock.NewAPI(t)
	backend.
		On(
			"GetEventsForHeightRange",
			mocks.Anything,
			eventType,
			blockEvents[0].BlockHeight,
			blockEvents[len(blockEvents)-1].BlockHeight,
			entities.EventEncodingVersion_JSON_CDC_V0,
			mocks.Anything,
		).
		Return(blockEvents, access.ExecutorMetadata{}, nil)

	t.Run("empty execution state query", func(t *testing.T) {
		request := buildRequest(
			t,
			eventType,
			fmt.Sprint(blockEvents[0].BlockHeight),
			fmt.Sprint(blockEvents[len(blockEvents)-1].BlockHeight),
			[]string{},
			"",
			[]string{},
			"",
		)

		expectedResponse := buildExpectedResponse(t, blockEvents, false)
		router.AssertOKResponse(t, request, expectedResponse, backend)
	})

	t.Run("empty agreeing executors count", func(t *testing.T) {
		request := buildRequest(
			t,
			eventType,
			fmt.Sprint(blockEvents[0].BlockHeight),
			fmt.Sprint(blockEvents[len(blockEvents)-1].BlockHeight),
			[]string{},
			"",
			unittest.IdentifierListFixture(2).Strings(),
			"true",
		)

		expectedResponse := buildExpectedResponse(t, blockEvents, true)
		router.AssertOKResponse(t, request, expectedResponse, backend)
	})

	t.Run("empty required executors", func(t *testing.T) {
		request := buildRequest(
			t,
			eventType,
			fmt.Sprint(blockEvents[0].BlockHeight),
			fmt.Sprint(blockEvents[len(blockEvents)-1].BlockHeight),
			[]string{},
			"2",
			[]string{},
			"true",
		)

		expectedResponse := buildExpectedResponse(t, blockEvents, true)
		router.AssertOKResponse(t, request, expectedResponse, backend)
	})

	t.Run("empty include executor metadata", func(t *testing.T) {
		request := buildRequest(
			t,
			eventType,
			fmt.Sprint(blockEvents[0].BlockHeight),
			fmt.Sprint(blockEvents[len(blockEvents)-1].BlockHeight),
			[]string{},
			"2",
			unittest.IdentifierListFixture(2).Strings(),
			"",
		)

		expectedResponse := buildExpectedResponse(t, blockEvents, true)
		router.AssertOKResponse(t, request, expectedResponse, backend)
	})

	t.Run("agreeing executors count equals 0", func(t *testing.T) {
		request := buildRequest(
			t,
			eventType,
			fmt.Sprint(blockEvents[0].BlockHeight),
			fmt.Sprint(blockEvents[len(blockEvents)-1].BlockHeight),
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
	blockEvents := make([]flow.BlockEvents, 5)
	for i := 0; i < len(blockEvents); i++ {
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(uint64(i)))
		blockEvents[i] = unittest.BlockEventsFixture(block, 2)
	}

	backend := mock.NewAPI(t)
	backend.
		On("GetLatestBlockHeader", mocks.Anything, true).
		Return(unittest.BlockHeaderFixture(), flow.BlockStatusSealed, nil).
		Once()

	backend.
		On(
			"GetEventsForHeightRange",
			mocks.Anything,
			eventType,
			blockEvents[0].BlockHeight,
			mocks.Anything,
			entities.EventEncodingVersion_JSON_CDC_V0,
			optimistic_sync.DefaultCriteria,
		).
		Return(blockEvents, access.ExecutorMetadata{}, nil).
		Once()

	request := buildRequest(
		t,
		eventType,
		fmt.Sprint(blockEvents[0].BlockHeight),
		"sealed",
		[]string{},
		"2",
		[]string{},
		"true",
	)
	t.Log("request: ", request.URL.String())

	expectedResponse := buildExpectedResponse(t, blockEvents, false)
	router.AssertOKResponse(t, request, expectedResponse, backend)
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
	list := models.NewBlockEventsList(events, access.ExecutorMetadata{}, includeMetadata)
	data, err := json.Marshal(list)
	require.NoError(t, err)

	return string(data)
}
