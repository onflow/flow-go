package routes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/routes"
	"github.com/onflow/flow-go/engine/access/rest/router"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/utils/unittest"
)

const eventType = "A.179b6b1cb6755e31.Foo.Bar"

func blockEventsFixture() []flow.BlockEvents {
	events := make([]flow.BlockEvents, 5)
	for i := 0; i < len(events); i++ {
		// we don't want a block height to be equal to 0 in this test
		// to distinguish it from a default value
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(uint64(i + 1)))
		events[i] = unittest.BlockEventsFixture(block, 2)
	}
	return events
}

func TestGetEvents_GetEventsForBlockIDs(t *testing.T) {
	t.Run("for 3 blocks", func(t *testing.T) {
		backend := mock.NewAPI(t)
		events := blockEventsFixture()
		expectedEvents := events[0:2]

		args := requestArgs{
			eventType: eventType,
			blockIDs:  flow.IdentifierList{events[0].BlockID, events[1].BlockID},
		}

		backend.
			On(
				"GetEventsForBlockIDs",
				mocks.Anything,
				args.eventType,
				args.blockIDs,
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(expectedEvents, nil, nil).
			Once()

		request := buildRequest(t, args)

		responseRecorder := router.ExecuteRequest(request, backend)
		require.Equal(t, http.StatusOK, responseRecorder.Code)

		expectedResponse := buildExpectedResponse(t, expectedEvents, false)
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	t.Run("invalid argument", func(t *testing.T) {
		backend := mock.NewAPI(t)
		events := blockEventsFixture()
		args := requestArgs{
			eventType: eventType,
			blockIDs:  flow.IdentifierList{events[0].BlockID}, // pass any block
		}

		backend.
			On(
				"GetEventsForBlockIDs",
				mocks.Anything,
				args.eventType,
				args.blockIDs,
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(nil, nil,
				status.Error(codes.InvalidArgument, "block IDs must not be empty")).
			Once()

		request := buildRequest(t, args)

		responseRecorder := router.ExecuteRequest(request, backend)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("internal error", func(t *testing.T) {
		backend := mock.NewAPI(t)
		events := blockEventsFixture()
		args := requestArgs{
			eventType: eventType,
			blockIDs:  flow.IdentifierList{events[0].BlockID},
		}

		backend.
			On(
				"GetEventsForBlockIDs",
				mocks.Anything,
				args.eventType,
				args.blockIDs,
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(nil, nil, assert.AnError).
			Once()

		request := buildRequest(t, args)

		responseRecorder := router.ExecuteRequest(request, backend)
		require.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	})
}

func TestGetEvents_GetEventsForHeightRange(t *testing.T) {
	t.Run("happy path for height range", func(t *testing.T) {
		backend := mock.NewAPI(t)
		events := blockEventsFixture()
		args := requestArgs{
			eventType: eventType,
			start:     events[0].BlockHeight,
			end:       events[len(events)-1].BlockHeight,
		}
		backend.
			On(
				"GetEventsForHeightRange",
				mocks.Anything,
				args.eventType,
				args.start,
				args.end,
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(events, nil, nil).
			Once()

		request := buildRequest(t, args)

		responseRecorder := router.ExecuteRequest(request, backend)
		require.Equal(t, http.StatusOK, responseRecorder.Code)

		expectedResponse := buildExpectedResponse(t, events, false)
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	t.Run("height range that exceeds existing events", func(t *testing.T) {
		backend := mock.NewAPI(t)
		events := blockEventsFixture()
		args := requestArgs{
			eventType: eventType,
			start:     events[0].BlockHeight,
			end:       events[len(events)-1].BlockHeight + 5,
		}
		backend.
			On(
				"GetEventsForHeightRange",
				mocks.Anything,
				args.eventType,
				args.start,
				args.end,
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(events, nil, nil).
			Once()

		request := buildRequest(t, args)

		responseRecorder := router.ExecuteRequest(request, backend)
		require.Equal(t, http.StatusOK, responseRecorder.Code)

		expectedResponse := buildExpectedResponse(t, events, false)
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})
}

func TestGetEvents_InvalidRequest(t *testing.T) {
	type test struct {
		name             string
		args             rawRequestArgs
		expectedResponse string
	}

	tests := []test{
		{
			name: "all fields missing",
			args: rawRequestArgs{
				agreeingExecutorsCount:  "2",
				includeExecutorMetadata: "true",
			},
			expectedResponse: `{"code":400,"message":"must provide either block IDs or start and end height range"}`,
		},
		{
			name: "event type missing",
			args: rawRequestArgs{
				blockIDs:                []string{unittest.IdentifierFixture().String()},
				agreeingExecutorsCount:  "2",
				includeExecutorMetadata: "true",
			},
			expectedResponse: `{"code":400,"message":"event type must be provided"}`,
		},
		{
			name: "end height missing",
			args: rawRequestArgs{
				eventType:               eventType,
				start:                   "100",
				agreeingExecutorsCount:  "2",
				includeExecutorMetadata: "true",
			},
			expectedResponse: `{"code":400,"message":"must provide either block IDs or start and end height range"}`,
		},
		{
			name: "start height greater than end height",
			args: rawRequestArgs{
				eventType:               eventType,
				start:                   "100",
				end:                     "50",
				agreeingExecutorsCount:  "2",
				includeExecutorMetadata: "true",
			},
			expectedResponse: `{"code":400,"message":"start height must be less than or equal to end height"}`,
		},
		{
			name: "too big interval",
			args: rawRequestArgs{
				eventType: eventType,
				start:     "0",
				end:       "5000",
			},
			expectedResponse: `{"code":400,"message":"height range 5000 exceeds maximum allowed of 250"}`,
		},
		{
			name: "all fields provided",
			args: rawRequestArgs{
				eventType:               eventType,
				start:                   "100",
				end:                     "120",
				blockIDs:                []string{"10e782612a014b5c9c7d17994d7e67157064f3dd42fa92cd080bfb0fe22c3f71"},
				agreeingExecutorsCount:  "2",
				includeExecutorMetadata: "true",
			},
			expectedResponse: `{"code":400,"message":"can only provide either block IDs or start and end height range"}`,
		},
		{
			name: "invalid height format",
			args: rawRequestArgs{
				eventType: eventType,
				start:     "foo",
				end:       "120",
			},
			expectedResponse: `{"code":400,"message":"invalid start height: invalid height format"}`,
		},
		{
			name: "agreeing executors count equal 0",
			args: rawRequestArgs{
				eventType:               eventType,
				start:                   "0",
				end:                     "5",
				agreeingExecutorsCount:  "0",
				requiredExecutors:       unittest.IdentifierListFixture(2).Strings(),
				includeExecutorMetadata: "true",
			},
			expectedResponse: `{"code":400,"message":"agreeingExecutorCount cannot be equal 0"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := buildRequestFromRawArgs(t, test.args)

			responseRecorder := router.ExecuteRequest(request, nil)
			require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

			actualResponse := responseRecorder.Body.String()
			require.JSONEq(t, test.expectedResponse, actualResponse)
		})
	}
}

func TestGetEvents_LastBlockSmallerThanStartHeight(t *testing.T) {
	latestBlock := unittest.BlockHeaderFixture()
	backend := mock.NewAPI(t)
	backend.
		On("GetLatestBlockHeader", mocks.Anything, true).
		Return(latestBlock, flow.BlockStatusSealed, nil).
		Once()

	request := buildRequestFromRawArgs(
		t,
		rawRequestArgs{
			eventType:               eventType,
			start:                   fmt.Sprint(latestBlock.Height + 1),
			end:                     "sealed",
			agreeingExecutorsCount:  "2",
			includeExecutorMetadata: "true",
		},
	)

	responseRecorder := router.ExecuteRequest(request, backend)
	require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

	expectedResponse := `{"code":400,"message":"current retrieved end height value is lower than start height"}`
	actualResponse := responseRecorder.Body.String()
	require.JSONEq(t, expectedResponse, actualResponse)
}

func TestGetEvents_ParseExecutionState(t *testing.T) {
	backend := mock.NewAPI(t)
	events := blockEventsFixture()
	args := requestArgs{
		eventType: eventType,
		start:     events[0].BlockHeight,
		end:       events[len(events)-1].BlockHeight,
	}

	backend.
		On(
			"GetEventsForHeightRange",
			mocks.Anything,
			args.eventType,
			args.start,
			args.end,
			entities.EventEncodingVersion_JSON_CDC_V0,
			mocks.Anything,
		).
		Return(events, nil, nil)

	t.Run("empty execution state query", func(t *testing.T) {
		request := buildRequest(t, args)

		responseRecorder := router.ExecuteRequest(request, backend)
		require.Equal(t, http.StatusOK, responseRecorder.Code)

		expectedResponse := buildExpectedResponse(t, events, false)
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	t.Run("empty agreeing executors count", func(t *testing.T) {
		request := buildRequestFromRawArgs(
			t,
			rawRequestArgs{
				eventType:               eventType,
				start:                   fmt.Sprint(events[0].BlockHeight),
				end:                     fmt.Sprint(events[len(events)-1].BlockHeight),
				requiredExecutors:       unittest.IdentifierListFixture(2).Strings(),
				includeExecutorMetadata: "true",
			},
		)

		responseRecorder := router.ExecuteRequest(request, backend)
		require.Equal(t, http.StatusOK, responseRecorder.Code)

		expectedResponse := buildExpectedResponse(t, events, true)
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	t.Run("empty required executors", func(t *testing.T) {
		request := buildRequestFromRawArgs(
			t,
			rawRequestArgs{
				eventType:               eventType,
				start:                   fmt.Sprint(events[0].BlockHeight),
				end:                     fmt.Sprint(events[len(events)-1].BlockHeight),
				agreeingExecutorsCount:  "2",
				includeExecutorMetadata: "true",
			},
		)

		responseRecorder := router.ExecuteRequest(request, backend)
		require.Equal(t, http.StatusOK, responseRecorder.Code)

		expectedResponse := buildExpectedResponse(t, events, true)
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	t.Run("empty include executor metadata", func(t *testing.T) {
		request := buildRequestFromRawArgs(
			t,
			rawRequestArgs{
				eventType:              eventType,
				start:                  fmt.Sprint(events[0].BlockHeight),
				end:                    fmt.Sprint(events[len(events)-1].BlockHeight),
				agreeingExecutorsCount: "2",
				requiredExecutors:      unittest.IdentifierListFixture(2).Strings(),
			},
		)

		responseRecorder := router.ExecuteRequest(request, backend)
		require.Equal(t, http.StatusOK, responseRecorder.Code)

		expectedResponse := buildExpectedResponse(t, events, false)
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})
}

func TestGetEvents_GetAtSealedBlock(t *testing.T) {
	backend := mock.NewAPI(t)
	events := blockEventsFixture()
	backend.
		On("GetLatestBlockHeader", mocks.Anything, true).
		Return(unittest.BlockHeaderFixture(), flow.BlockStatusSealed, nil).
		Once()

	backend.
		On(
			"GetEventsForHeightRange",
			mocks.Anything,
			eventType,
			events[0].BlockHeight,
			mocks.Anything,
			entities.EventEncodingVersion_JSON_CDC_V0,
			optimistic_sync.DefaultCriteria,
		).
		Return(events, nil, nil).
		Once()

	request := buildRequestFromRawArgs(
		t,
		rawRequestArgs{
			eventType:               eventType,
			start:                   fmt.Sprint(events[0].BlockHeight),
			end:                     "sealed",
			agreeingExecutorsCount:  "2",
			includeExecutorMetadata: "true",
		},
	)

	responseRecorder := router.ExecuteRequest(request, backend)
	require.Equal(t, http.StatusOK, responseRecorder.Code)

	expectedResponse := buildExpectedResponse(t, events, false)
	actualResponse := responseRecorder.Body.String()
	require.JSONEq(t, expectedResponse, actualResponse)
}

type rawRequestArgs struct {
	eventType               string
	start                   string
	end                     string
	blockIDs                []string
	agreeingExecutorsCount  string
	requiredExecutors       []string
	includeExecutorMetadata string
}

func buildRequestFromRawArgs(t *testing.T, args rawRequestArgs) *http.Request {
	t.Helper()

	u, _ := url.Parse("/v1/events")
	q := u.Query()

	if len(args.blockIDs) > 0 {
		q.Add(routes.BlockQueryParam, strings.Join(args.blockIDs, ","))
	}

	if args.start != "" && args.end != "" {
		q.Add(router.StartHeightQueryParam, args.start)
		q.Add(router.EndHeightQueryParam, args.end)
	}

	q.Add(router.AgreeingExecutorsCountQueryParam, args.agreeingExecutorsCount)

	if len(args.requiredExecutors) > 0 {
		q.Add(router.RequiredExecutorIdsQueryParam, strings.Join(args.requiredExecutors, ","))
	}

	if len(args.includeExecutorMetadata) > 0 {
		q.Add(router.IncludeExecutorMetadataQueryParam, args.includeExecutorMetadata)
	}

	q.Add(routes.EventTypeQuery, args.eventType)

	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	require.NoError(t, err)

	return req
}

type requestArgs struct {
	eventType               string
	start                   uint64
	end                     uint64
	blockIDs                flow.IdentifierList
	agreeingExecutorsCount  uint64
	requiredExecutors       flow.IdentifierList
	includeExecutorMetadata bool
}

func buildRequest(t *testing.T, args requestArgs) *http.Request {
	t.Helper()

	u, _ := url.Parse("/v1/events")
	q := u.Query()

	if len(args.blockIDs) > 0 {
		q.Add(routes.BlockQueryParam, strings.Join(args.blockIDs.Strings(), ","))
	}

	if args.start != 0 && args.end != 0 {
		q.Add(router.StartHeightQueryParam, fmt.Sprint(args.start))
		q.Add(router.EndHeightQueryParam, fmt.Sprint(args.end))
	}

	if args.agreeingExecutorsCount != 0 {
		q.Add(router.AgreeingExecutorsCountQueryParam, fmt.Sprint(args.agreeingExecutorsCount))
	}

	if args.requiredExecutors.Len() > 0 {
		q.Add(router.RequiredExecutorIdsQueryParam, strings.Join(args.requiredExecutors.Strings(), ","))
	}

	if args.includeExecutorMetadata {
		q.Add(router.IncludeExecutorMetadataQueryParam, fmt.Sprint(args.includeExecutorMetadata))
	}

	q.Add(routes.EventTypeQuery, args.eventType)

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
