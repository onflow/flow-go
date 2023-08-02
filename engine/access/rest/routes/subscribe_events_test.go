package routes

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/exp/slices"

	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/request"
	mockstatestream "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/engine/common/state_stream"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type testType struct {
	name         string
	startBlockID flow.Identifier
	startHeight  uint64

	eventTypes []string
	addresses  []string
	contracts  []string
}

var testEventTypes = []flow.EventType{
	"A.0123456789abcdef.flow.event",
	"B.0123456789abcdef.flow.event",
	"C.0123456789abcdef.flow.event",
}

type SubscribeEventsSuite struct {
	suite.Suite

	blocks      []*flow.Block
	blockEvents map[flow.Identifier]flow.EventsList
}

func TestSubscribeEventsSuite(t *testing.T) {
	suite.Run(t, new(SubscribeEventsSuite))
}

func (s *SubscribeEventsSuite) SetupTest() {
	rootBlock := unittest.BlockFixture()
	parent := rootBlock.Header

	blockCount := 5

	s.blocks = make([]*flow.Block, 0, blockCount)
	s.blockEvents = make(map[flow.Identifier]flow.EventsList, blockCount)

	for i := 0; i < blockCount; i++ {
		block := unittest.BlockWithParentFixture(parent)
		// update for next iteration
		parent = block.Header

		result := unittest.ExecutionResultFixture()
		blockEvents := unittest.BlockEventsFixture(block.Header, (i%len(testEventTypes))*3+1, testEventTypes...)

		s.blocks = append(s.blocks, block)
		s.blockEvents[block.ID()] = blockEvents.Events

		s.T().Logf("adding exec data for block %d %d %v => %v", i, block.Header.Height, block.ID(), result.ExecutionDataID)
	}
}

func (s *SubscribeEventsSuite) TestSubscribeEvents() {
	testVectors := []testType{
		{
			name:         "happy path - all events from root height",
			startBlockID: flow.ZeroID,
			startHeight:  request.EmptyHeight,
		},
		{
			name:         "happy path - all events from startHeight",
			startBlockID: flow.ZeroID,
			startHeight:  s.blocks[0].Header.Height,
		},
		{
			name:         "happy path - all events from startBlockID",
			startBlockID: s.blocks[0].ID(),
			startHeight:  request.EmptyHeight,
		},
	}
	chain := flow.MonotonicEmulator.Chain()

	// create variations for each of the base test
	tests := make([]testType, 0, len(testVectors)*2)
	for _, test := range testVectors {
		t1 := test
		t1.name = fmt.Sprintf("%s - all events", test.name)
		tests = append(tests, t1)

		t2 := test
		t2.name = fmt.Sprintf("%s - some events", test.name)
		t2.eventTypes = []string{string(testEventTypes[0])}
		tests = append(tests, t2)
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			stateStreamBackend := &mockstatestream.API{}
			backend := &mock.API{}

			subscription := &mockstatestream.Subscription{}

			expectedEvents := flow.EventsList{}
			for _, event := range s.blockEvents[s.blocks[0].ID()] {
				if slices.Contains(test.eventTypes, string(event.Type)) {
					expectedEvents = append(expectedEvents, event)
				}
			}

			// Create a channel to receive mock EventsResponse objects
			ch := make(chan interface{})
			var chReadOnly <-chan interface{}
			expectedEventsResponses := []*state_stream.EventsResponse{}

			for i, b := range s.blocks {
				s.T().Logf("checking block %d %v", i, b.ID())

				//simulate EventsResponse
				eventResponse := &state_stream.EventsResponse{
					Height:  b.Header.Height,
					BlockID: b.ID(),
					Events:  expectedEvents,
				}
				expectedEventsResponses = append(expectedEventsResponses, eventResponse)
			}

			// Simulate sending a mock EventsResponse
			go func() {
				for _, eventResponse := range expectedEventsResponses {
					// Send the mock EventsResponse through the channel
					ch <- eventResponse
				}
			}()

			chReadOnly = ch

			subscription.Mock.On("Channel").Return(chReadOnly)
			subscription.Mock.On("Err").Return(nil)

			filter, err := state_stream.NewEventFilter(state_stream.DefaultEventFilterConfig, chain, test.eventTypes, test.addresses, test.contracts)
			assert.NoError(s.T(), err)
			var startHeight uint64
			if test.startHeight == request.EmptyHeight {
				startHeight = 0
			} else {
				startHeight = test.startHeight
			}
			stateStreamBackend.Mock.On("SubscribeEvents", mocks.Anything, test.startBlockID, startHeight, filter).Return(subscription)

			req, err := getSubscribeEventsRequest(s.T(), test.startBlockID, test.startHeight, test.eventTypes, test.addresses, test.contracts)
			assert.NoError(s.T(), err)
			rr, err := executeRequest(req, backend, stateStreamBackend)
			assert.NoError(s.T(), err)
			assert.Equal(s.T(), http.StatusOK, rr.Code)
		})
	}
}

func (s *SubscribeEventsSuite) TestSubscribeEventsHandlesErrors() {
	s.Run("returns error for block id and height", func() {
		stateStreamBackend := &mockstatestream.API{}
		backend := &mock.API{}

		req, err := getSubscribeEventsRequest(s.T(), s.blocks[0].ID(), s.blocks[0].Header.Height, nil, nil, nil)
		assert.NoError(s.T(), err)
		assertResponse(s.T(), req, http.StatusBadRequest, `{"code":400,"message":"can only provide either block ID or start height"}`, backend, stateStreamBackend)
	})

	s.Run("returns error for invalid block id", func() {
		stateStreamBackend := &mockstatestream.API{}
		backend := &mock.API{}

		invalidBlock := unittest.BlockFixture()
		subscription := &mockstatestream.Subscription{}

		ch := make(chan interface{})
		var chReadOnly <-chan interface{}
		go func() {
			close(ch)
		}()
		chReadOnly = ch

		subscription.Mock.On("Channel").Return(chReadOnly)
		subscription.Mock.On("Err").Return(fmt.Errorf("subscription error"))
		stateStreamBackend.Mock.On("SubscribeEvents", mocks.Anything, invalidBlock.ID(), mocks.Anything, mocks.Anything).Return(subscription)

		req, err := getSubscribeEventsRequest(s.T(), invalidBlock.ID(), request.EmptyHeight, nil, nil, nil)
		assert.NoError(s.T(), err)
		assertResponse(s.T(), req, http.StatusBadRequest, `{"code":400,"message":"stream encountered an error: subscription error"}`, backend, stateStreamBackend)
	})

	s.Run("returns error when channel closed", func() {
		stateStreamBackend := &mockstatestream.API{}
		backend := &mock.API{}
		subscription := &mockstatestream.Subscription{}

		ch := make(chan interface{})
		var chReadOnly <-chan interface{}

		go func() {
			close(ch)
		}()
		chReadOnly = ch

		subscription.Mock.On("Channel").Return(chReadOnly)
		subscription.Mock.On("Err").Return(nil)
		stateStreamBackend.Mock.On("SubscribeEvents", mocks.Anything, s.blocks[0].ID(), mocks.Anything, mocks.Anything).Return(subscription)

		req, err := getSubscribeEventsRequest(s.T(), s.blocks[0].ID(), request.EmptyHeight, nil, nil, nil)
		assert.NoError(s.T(), err)
		assertResponse(s.T(), req, http.StatusRequestTimeout, `{"code":408,"message":"subscription channel closed"}`, backend, stateStreamBackend)
	})

	s.Run("returns error for unexpected response type", func() {
		stateStreamBackend := &mockstatestream.API{}
		backend := &mock.API{}
		subscription := &mockstatestream.Subscription{}

		ch := make(chan interface{})
		var chReadOnly <-chan interface{}
		go func() {
			executionDataResponse := &state_stream.ExecutionDataResponse{
				Height: s.blocks[0].Header.Height,
			}
			ch <- executionDataResponse
		}()
		chReadOnly = ch

		subscription.Mock.On("Channel").Return(chReadOnly)
		subscription.Mock.On("Err").Return(nil)
		stateStreamBackend.Mock.On("SubscribeEvents", mocks.Anything, s.blocks[0].ID(), 0, mocks.Anything).Return(subscription)

		req, err := getSubscribeEventsRequest(s.T(), s.blocks[0].ID(), request.EmptyHeight, nil, nil, nil)
		assert.NoError(s.T(), err)
		assertResponse(s.T(), req, http.StatusInternalServerError, `{"code":500,"message":"internal server error"}`, backend, stateStreamBackend)
	})
}

func getSubscribeEventsRequest(t *testing.T, startBlockId flow.Identifier, startHeight uint64, eventTypes []string, addresses []string, contracts []string) (*http.Request, error) {
	u, _ := url.Parse("/v1/subscribe_events")
	q := u.Query()

	if startBlockId != flow.ZeroID {
		q.Add(startBlockIdQueryParam, startBlockId.String())
	}

	if startHeight != request.EmptyHeight {
		q.Add(startHeightQueryParam, fmt.Sprintf("%d", startHeight))
	}

	if len(eventTypes) > 0 {
		q.Add(eventTypesQueryParams, strings.Join(eventTypes, ","))
	}
	if len(addresses) > 0 {
		q.Add(addressesQueryParams, strings.Join(addresses, ","))
	}
	if len(contracts) > 0 {
		q.Add(contractsQueryParams, strings.Join(contracts, ","))
	}

	u.RawQuery = q.Encode()
	key, err := generateWebSocketKey()
	if err != nil {
		err := fmt.Errorf("error generating websocket key: %v", err)
		return nil, err
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-Websocket-Version", "13")
	req.Header.Set("Sec-Websocket-Key", key)
	require.NoError(t, err)
	return req, nil
}

func generateWebSocketKey() (string, error) {
	// Generate 16 random bytes.
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", err
	}

	// Encode the bytes to base64 and return the key as a string.
	return base64.StdEncoding.EncodeToString(keyBytes), nil
}
