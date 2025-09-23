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
	"github.com/stretchr/testify/suite"
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
)

type GetEventsSuite struct {
	suite.Suite

	backend *mock.API

	events    []flow.BlockEvents
	eventType string
}

func TestGetEventsSuite(t *testing.T) {
	suite.Run(t, new(GetEventsSuite))
}

func (s *GetEventsSuite) SetupTest() {
	s.backend = mock.NewAPI(s.T())
	s.eventType = "A.179b6b1cb6755e31.Foo.Bar"

	s.events = make([]flow.BlockEvents, 5)
	for i := 0; i < len(s.events); i++ {
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(uint64(i)))
		s.events[i] = unittest.BlockEventsFixture(block, 2)
	}
}

func (s *GetEventsSuite) SetupSubTest() {
	s.backend = mock.NewAPI(s.T())
}

func (s *GetEventsSuite) TestGetEvents_GetEventsForBlockIDs() {
	s.Run("for n blocks", func() {
		expectedEvents := s.events[0:2]

		s.backend.
			On(
				"GetEventsForBlockIDs",
				mocks.Anything,
				s.eventType,
				[]flow.Identifier{s.events[0].BlockID, s.events[1].BlockID},
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(expectedEvents, access.ExecutorMetadata{}, nil).
			Once()

		request := buildRequest(
			s.T(),
			s.eventType,
			"",
			"",
			[]string{s.events[0].BlockID.String(), s.events[1].BlockID.String()},
			"",
			[]string{},
			"",
		)

		responseRecorder := router.ExecuteRequest(request, s.backend)
		require.Equal(s.T(), http.StatusOK, responseRecorder.Code)

		expectedResponse := buildExpectedResponse(s.T(), expectedEvents, false)
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(s.T(), expectedResponse, actualResponse)
	})

	s.Run("invalid argument", func() {
		s.backend.
			On(
				"GetEventsForBlockIDs",
				mocks.Anything,
				s.eventType,
				[]flow.Identifier{s.events[0].BlockID},
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(nil, access.ExecutorMetadata{},
				status.Error(codes.InvalidArgument, "block IDs must not be empty")).
			Once()

		request := buildRequest(
			s.T(),
			s.eventType,
			"",
			"",
			[]string{s.events[0].BlockID.String()}, // pass any block
			"",
			[]string{},
			"",
		)

		responseRecorder := router.ExecuteRequest(request, s.backend)
		require.Equal(s.T(), http.StatusBadRequest, responseRecorder.Code)
	})

	s.Run("internal error", func() {
		s.backend.
			On(
				"GetEventsForBlockIDs",
				mocks.Anything,
				s.eventType,
				[]flow.Identifier{s.events[0].BlockID},
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(nil, access.ExecutorMetadata{}, assert.AnError).
			Once()

		request := buildRequest(
			s.T(),
			s.eventType,
			"",
			"",
			[]string{s.events[0].BlockID.String()},
			"",
			[]string{},
			"",
		)

		responseRecorder := router.ExecuteRequest(request, s.backend)
		require.Equal(s.T(), http.StatusInternalServerError, responseRecorder.Code)
	})
}

func (s *GetEventsSuite) TestGetEvents_GetEventsForHeightRange() {
	s.Run("happy path for height range", func() {
		s.backend.
			On(
				"GetEventsForHeightRange",
				mocks.Anything,
				s.eventType,
				s.events[0].BlockHeight,
				s.events[len(s.events)-1].BlockHeight,
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(s.events, access.ExecutorMetadata{}, nil).
			Once()

		request := buildRequest(
			s.T(),
			s.eventType,
			fmt.Sprint(s.events[0].BlockHeight),
			fmt.Sprint(s.events[len(s.events)-1].BlockHeight),
			[]string{},
			"",
			[]string{},
			"",
		)

		responseRecorder := router.ExecuteRequest(request, s.backend)
		require.Equal(s.T(), http.StatusOK, responseRecorder.Code)

		expectedResponse := buildExpectedResponse(s.T(), s.events, false)
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(s.T(), expectedResponse, actualResponse)
	})

	s.Run("height range that exceeds existing events", func() {
		s.backend.
			On(
				"GetEventsForHeightRange",
				mocks.Anything,
				s.eventType,
				s.events[0].BlockHeight,
				s.events[len(s.events)-1].BlockHeight+5,
				entities.EventEncodingVersion_JSON_CDC_V0,
				mocks.Anything,
			).
			Return(s.events, access.ExecutorMetadata{}, nil).
			Once()

		request := buildRequest(
			s.T(),
			s.eventType,
			fmt.Sprint(s.events[0].BlockHeight),
			fmt.Sprint(s.events[len(s.events)-1].BlockHeight+5),
			[]string{},
			"",
			[]string{},
			"",
		)

		responseRecorder := router.ExecuteRequest(request, s.backend)
		require.Equal(s.T(), http.StatusOK, responseRecorder.Code)

		expectedResponse := buildExpectedResponse(s.T(), s.events, false)
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(s.T(), expectedResponse, actualResponse)
	})
}

func (s *GetEventsSuite) TestGetEvents_InvalidRequest() {
	s.T().Run("all fields missing", func(t *testing.T) {
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

		responseRecorder := router.ExecuteRequest(request, nil)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		expectedResponse := `{"code":400,"message":"must provide either block IDs or start and end height range"}`
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	s.T().Run("query event type missing", func(t *testing.T) {
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

		responseRecorder := router.ExecuteRequest(request, nil)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		expectedResponse := `{"code":400,"message":"event type must be provided"}`
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	s.T().Run("end height missing", func(t *testing.T) {
		request := buildRequest(
			t,
			s.eventType,
			"100",
			"",
			nil,
			"2",
			[]string{},
			"true",
		)

		responseRecorder := router.ExecuteRequest(request, nil)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		expectedResponse := `{"code":400,"message":"must provide either block IDs or start and end height range"}`
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	s.T().Run("start height greater than end height", func(t *testing.T) {
		request := buildRequest(
			t,
			s.eventType,
			"100",
			"50",
			nil,
			"2",
			[]string{},
			"true",
		)

		responseRecorder := router.ExecuteRequest(request, nil)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		expectedResponse := `{"code":400,"message":"start height must be less than or equal to end height"}`
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	s.T().Run("too big interval", func(t *testing.T) {
		request := buildRequest(
			t,
			s.eventType,
			"0",
			"5000",
			nil,
			"2",
			[]string{},
			"true",
		)

		responseRecorder := router.ExecuteRequest(request, nil)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		expectedResponse := `{"code":400,"message":"height range 5000 exceeds maximum allowed of 250"}`
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	s.T().Run("all fields provided", func(t *testing.T) {
		request := buildRequest(
			t,
			s.eventType,
			"100",
			"120",
			[]string{"10e782612a014b5c9c7d17994d7e67157064f3dd42fa92cd080bfb0fe22c3f71"},
			"2",
			[]string{},
			"true",
		)

		responseRecorder := router.ExecuteRequest(request, nil)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		expectedResponse := `{"code":400,"message":"can only provide either block IDs or start and end height range"}`
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	s.T().Run("invalid height format", func(t *testing.T) {
		request := buildRequest(
			t,
			s.eventType,
			"foo",
			"120",
			nil,
			"2",
			[]string{},
			"true",
		)

		responseRecorder := router.ExecuteRequest(request, nil)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		expectedResponse := `{"code":400,"message":"invalid start height: invalid height format"}`
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	s.T().Run("last block smaller than start height", func(t *testing.T) {
		latestBlock := unittest.BlockHeaderFixture()
		backend := mock.NewAPI(t)
		backend.
			On("GetLatestBlockHeader", mocks.Anything, true).
			Return(latestBlock, flow.BlockStatusSealed, nil).
			Once()

		request := buildRequest(
			t,
			s.eventType,
			fmt.Sprint(latestBlock.Height+1),
			"sealed",
			nil,
			"2",
			[]string{},
			"true",
		)

		responseRecorder := router.ExecuteRequest(request, backend)
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		expectedResponse := `{"code":400,"message":"current retrieved end height value is lower than start height"}`
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})
}

func (s *GetEventsSuite) TestGetEvents_ParseExecutionState() {
	s.backend.
		On(
			"GetEventsForHeightRange",
			mocks.Anything,
			s.eventType,
			s.events[0].BlockHeight,
			s.events[len(s.events)-1].BlockHeight,
			entities.EventEncodingVersion_JSON_CDC_V0,
			mocks.Anything,
		).
		Return(s.events, access.ExecutorMetadata{}, nil)

	s.T().Run("empty execution state query", func(t *testing.T) {
		request := buildRequest(
			t,
			s.eventType,
			fmt.Sprint(s.events[0].BlockHeight),
			fmt.Sprint(s.events[len(s.events)-1].BlockHeight),
			[]string{},
			"",
			[]string{},
			"",
		)

		responseRecorder := router.ExecuteRequest(request, s.backend)
		require.Equal(t, http.StatusOK, responseRecorder.Code)

		expectedResponse := buildExpectedResponse(t, s.events, false)
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	s.T().Run("empty agreeing executors count", func(t *testing.T) {
		request := buildRequest(
			t,
			s.eventType,
			fmt.Sprint(s.events[0].BlockHeight),
			fmt.Sprint(s.events[len(s.events)-1].BlockHeight),
			[]string{},
			"",
			unittest.IdentifierListFixture(2).Strings(),
			"true",
		)

		responseRecorder := router.ExecuteRequest(request, s.backend)
		require.Equal(t, http.StatusOK, responseRecorder.Code)

		expectedResponse := buildExpectedResponse(t, s.events, false)
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	s.T().Run("empty required executors", func(t *testing.T) {
		request := buildRequest(
			t,
			s.eventType,
			fmt.Sprint(s.events[0].BlockHeight),
			fmt.Sprint(s.events[len(s.events)-1].BlockHeight),
			[]string{},
			"2",
			[]string{},
			"true",
		)

		responseRecorder := router.ExecuteRequest(request, s.backend)
		require.Equal(t, http.StatusOK, responseRecorder.Code)

		expectedResponse := buildExpectedResponse(t, s.events, false)
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	s.T().Run("empty include executor metadata", func(t *testing.T) {
		request := buildRequest(
			t,
			s.eventType,
			fmt.Sprint(s.events[0].BlockHeight),
			fmt.Sprint(s.events[len(s.events)-1].BlockHeight),
			[]string{},
			"2",
			unittest.IdentifierListFixture(2).Strings(),
			"",
		)

		responseRecorder := router.ExecuteRequest(request, s.backend)
		require.Equal(t, http.StatusOK, responseRecorder.Code)

		expectedResponse := buildExpectedResponse(t, s.events, false)
		actualResponse := responseRecorder.Body.String()
		require.JSONEq(t, expectedResponse, actualResponse)
	})

	s.T().Run("agreeing executors count equals 0", func(t *testing.T) {
		request := buildRequest(
			t,
			s.eventType,
			fmt.Sprint(s.events[0].BlockHeight),
			fmt.Sprint(s.events[len(s.events)-1].BlockHeight),
			[]string{},
			"0",
			unittest.IdentifierListFixture(2).Strings(),
			"true",
		)

		responseRecorder := router.ExecuteRequest(request, s.backend)
		// agreeing executors count should be either omitted or greater than 0
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})
}

func (s *GetEventsSuite) TestGetEvents_GetAtSealedBlock() {
	s.backend.
		On("GetLatestBlockHeader", mocks.Anything, true).
		Return(unittest.BlockHeaderFixture(), flow.BlockStatusSealed, nil).
		Once()

	s.backend.
		On(
			"GetEventsForHeightRange",
			mocks.Anything,
			s.eventType,
			s.events[0].BlockHeight,
			mocks.Anything,
			entities.EventEncodingVersion_JSON_CDC_V0,
			optimistic_sync.DefaultCriteria,
		).
		Return(s.events, access.ExecutorMetadata{}, nil).
		Once()

	request := buildRequest(
		s.T(),
		s.eventType,
		fmt.Sprint(s.events[0].BlockHeight),
		"sealed",
		[]string{},
		"2",
		[]string{},
		"true",
	)

	responseRecorder := router.ExecuteRequest(request, s.backend)
	require.Equal(s.T(), http.StatusOK, responseRecorder.Code)

	expectedResponse := buildExpectedResponse(s.T(), s.events, false)
	actualResponse := responseRecorder.Body.String()
	require.JSONEq(s.T(), expectedResponse, actualResponse)
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
