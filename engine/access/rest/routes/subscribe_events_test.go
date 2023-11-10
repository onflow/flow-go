package routes

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"golang.org/x/exp/slices"

	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	mockstatestream "github.com/onflow/flow-go/engine/access/state_stream/mock"
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

	heartbeatInterval uint64

	headers http.Header
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

// TestSubscribeEvents is a happy cases tests for the SubscribeEvents functionality.
// This test function covers various scenarios for subscribing to events via WebSocket.
//
// It tests scenarios:
//   - Subscribing to events from the root height.
//   - Subscribing to events from a specific start height.
//   - Subscribing to events from a specific start block ID.
//   - Subscribing to events from the root height with custom heartbeat interval.
//
// Every scenario covers the following aspects:
//   - Subscribing to all events.
//   - Subscribing to events of a specific type (some events).
//
// For each scenario, this test function creates WebSocket requests, simulates WebSocket responses with mock data,
// and validates that the received WebSocket response matches the expected EventsResponses.
func (s *SubscribeEventsSuite) TestSubscribeEvents() {
	testVectors := []testType{
		{
			name:              "happy path - all events from root height",
			startBlockID:      flow.ZeroID,
			startHeight:       request.EmptyHeight,
			heartbeatInterval: 1,
		},
		{
			name:              "happy path - all events from startHeight",
			startBlockID:      flow.ZeroID,
			startHeight:       s.blocks[0].Header.Height,
			heartbeatInterval: 1,
		},
		{
			name:              "happy path - all events from startBlockID",
			startBlockID:      s.blocks[0].ID(),
			startHeight:       request.EmptyHeight,
			heartbeatInterval: 1,
		},
		{
			name:              "happy path - events from root height with custom heartbeat",
			startBlockID:      flow.ZeroID,
			startHeight:       request.EmptyHeight,
			heartbeatInterval: 2,
		},
		{
			name:              "happy path - all origins allowed",
			startBlockID:      flow.ZeroID,
			startHeight:       request.EmptyHeight,
			heartbeatInterval: 1,
			headers: http.Header{
				"Origin": []string{"https://example.com"},
			},
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

		t3 := test
		t3.name = fmt.Sprintf("%s - non existing events", test.name)
		t3.eventTypes = []string{"A.0123456789abcdff.flow.event"}
		tests = append(tests, t3)
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			stateStreamBackend := mockstatestream.NewAPI(s.T())
			subscription := mockstatestream.NewSubscription(s.T())

			filter, err := state_stream.NewEventFilter(
				state_stream.DefaultEventFilterConfig,
				chain,
				test.eventTypes,
				test.addresses,
				test.contracts)
			require.NoError(s.T(), err)

			var expectedEventsResponses []*backend.EventsResponse
			var subscriptionEventsResponses []*backend.EventsResponse
			startBlockFound := test.startBlockID == flow.ZeroID

			// construct expected event responses based on the provided test configuration
			for i, block := range s.blocks {
				if startBlockFound || block.ID() == test.startBlockID {
					startBlockFound = true
					if test.startHeight == request.EmptyHeight || block.Header.Height >= test.startHeight {
						eventsForBlock := flow.EventsList{}
						for _, event := range s.blockEvents[block.ID()] {
							if slices.Contains(test.eventTypes, string(event.Type)) ||
								len(test.eventTypes) == 0 { //Include all events
								eventsForBlock = append(eventsForBlock, event)
							}
						}
						eventResponse := &backend.EventsResponse{
							Height:  block.Header.Height,
							BlockID: block.ID(),
							Events:  eventsForBlock,
						}

						if len(eventsForBlock) > 0 || (i+1)%int(test.heartbeatInterval) == 0 {
							expectedEventsResponses = append(expectedEventsResponses, eventResponse)
						}
						subscriptionEventsResponses = append(subscriptionEventsResponses, eventResponse)
					}
				}
			}

			// Create a channel to receive mock EventsResponse objects
			ch := make(chan interface{})
			var chReadOnly <-chan interface{}
			// Simulate sending a mock EventsResponse
			go func() {
				for _, eventResponse := range subscriptionEventsResponses {
					// Send the mock EventsResponse through the channel
					ch <- eventResponse
				}
			}()

			chReadOnly = ch
			subscription.Mock.On("Channel").Return(chReadOnly)

			var startHeight uint64
			if test.startHeight == request.EmptyHeight {
				startHeight = uint64(0)
			} else {
				startHeight = test.startHeight
			}
			stateStreamBackend.Mock.
				On("SubscribeEvents", mocks.Anything, test.startBlockID, startHeight, filter).
				Return(subscription)

			req, err := getSubscribeEventsRequest(s.T(), test.startBlockID, test.startHeight, test.eventTypes, test.addresses, test.contracts, test.heartbeatInterval, test.headers)
			require.NoError(s.T(), err)
			respRecorder := newTestHijackResponseRecorder()
			// closing the connection after 1 second
			go func() {
				time.Sleep(1 * time.Second)
				close(respRecorder.closed)
			}()
			executeWsRequest(req, stateStreamBackend, respRecorder)
			requireResponse(s.T(), respRecorder, expectedEventsResponses)
		})
	}
}

func (s *SubscribeEventsSuite) TestSubscribeEventsHandlesErrors() {
	s.Run("returns error for block id and height", func() {
		stateStreamBackend := mockstatestream.NewAPI(s.T())
		req, err := getSubscribeEventsRequest(s.T(), s.blocks[0].ID(), s.blocks[0].Header.Height, nil, nil, nil, 1, nil)
		require.NoError(s.T(), err)
		respRecorder := newTestHijackResponseRecorder()
		executeWsRequest(req, stateStreamBackend, respRecorder)
		requireError(s.T(), respRecorder, "can only provide either block ID or start height")
	})

	s.Run("returns error for invalid block id", func() {
		stateStreamBackend := mockstatestream.NewAPI(s.T())
		invalidBlock := unittest.BlockFixture()
		subscription := mockstatestream.NewSubscription(s.T())

		ch := make(chan interface{})
		var chReadOnly <-chan interface{}
		go func() {
			close(ch)
		}()
		chReadOnly = ch

		subscription.Mock.On("Channel").Return(chReadOnly)
		subscription.Mock.On("Err").Return(fmt.Errorf("subscription error"))
		stateStreamBackend.Mock.
			On("SubscribeEvents", mocks.Anything, invalidBlock.ID(), uint64(0), mocks.Anything).
			Return(subscription)

		req, err := getSubscribeEventsRequest(s.T(), invalidBlock.ID(), request.EmptyHeight, nil, nil, nil, 1, nil)
		require.NoError(s.T(), err)
		respRecorder := newTestHijackResponseRecorder()
		executeWsRequest(req, stateStreamBackend, respRecorder)
		requireError(s.T(), respRecorder, "stream encountered an error: subscription error")
	})

	s.Run("returns error for invalid event filter", func() {
		stateStreamBackend := mockstatestream.NewAPI(s.T())
		req, err := getSubscribeEventsRequest(s.T(), s.blocks[0].ID(), request.EmptyHeight, []string{"foo"}, nil, nil, 1, nil)
		require.NoError(s.T(), err)
		respRecorder := newTestHijackResponseRecorder()
		executeWsRequest(req, stateStreamBackend, respRecorder)
		requireError(s.T(), respRecorder, "invalid event type format")
	})

	s.Run("returns error when channel closed", func() {
		stateStreamBackend := mockstatestream.NewAPI(s.T())
		subscription := mockstatestream.NewSubscription(s.T())

		ch := make(chan interface{})
		var chReadOnly <-chan interface{}

		go func() {
			close(ch)
		}()
		chReadOnly = ch

		subscription.Mock.On("Channel").Return(chReadOnly)
		subscription.Mock.On("Err").Return(nil)
		stateStreamBackend.Mock.
			On("SubscribeEvents", mocks.Anything, s.blocks[0].ID(), uint64(0), mocks.Anything).
			Return(subscription)

		req, err := getSubscribeEventsRequest(s.T(), s.blocks[0].ID(), request.EmptyHeight, nil, nil, nil, 1, nil)
		require.NoError(s.T(), err)
		respRecorder := newTestHijackResponseRecorder()
		executeWsRequest(req, stateStreamBackend, respRecorder)
		requireError(s.T(), respRecorder, "subscription channel closed")
	})
}

func getSubscribeEventsRequest(t *testing.T,
	startBlockId flow.Identifier,
	startHeight uint64,
	eventTypes []string,
	addresses []string,
	contracts []string,
	heartbeatInterval uint64,
	header http.Header,
) (*http.Request, error) {
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

	q.Add(heartbeatIntervalQueryParam, fmt.Sprintf("%d", heartbeatInterval))

	u.RawQuery = q.Encode()
	key, err := generateWebSocketKey()
	if err != nil {
		err := fmt.Errorf("error generating websocket key: %v", err)
		return nil, err
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	require.NoError(t, err)

	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-Websocket-Version", "13")
	req.Header.Set("Sec-Websocket-Key", key)

	for k, v := range header {
		req.Header.Set(k, v[0])
	}

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

func requireError(t *testing.T, recorder *testHijackResponseRecorder, expected string) {
	<-recorder.closed
	require.Contains(t, recorder.responseBuff.String(), expected)
}

// requireResponse validates that the response received from WebSocket communication matches the expected EventsResponses.
// This function compares the BlockID, Events count, and individual event properties for each expected and actual
// EventsResponse. It ensures that the response received from WebSocket matches the expected structure and content.
func requireResponse(t *testing.T, recorder *testHijackResponseRecorder, expected []*backend.EventsResponse) {
	<-recorder.closed
	// Convert the actual response from respRecorder to JSON bytes
	actualJSON := recorder.responseBuff.Bytes()
	// Define a regular expression pattern to match JSON objects
	pattern := `\{"BlockID":".*?","Height":\d+,"Events":\[(\{.*?})*\]\}`
	matches := regexp.MustCompile(pattern).FindAll(actualJSON, -1)

	// Unmarshal each matched JSON into []state_stream.EventsResponse
	var actual []backend.EventsResponse
	for _, match := range matches {
		var response backend.EventsResponse
		if err := json.Unmarshal(match, &response); err == nil {
			actual = append(actual, response)
		}
	}

	// Compare the count of expected and actual responses
	require.Equal(t, len(expected), len(actual))

	// Compare the BlockID and Events count for each response
	for responseIndex := range expected {
		expectedEventsResponse := expected[responseIndex]
		actualEventsResponse := actual[responseIndex]

		require.Equal(t, expectedEventsResponse.BlockID, actualEventsResponse.BlockID)
		require.Equal(t, len(expectedEventsResponse.Events), len(actualEventsResponse.Events))

		for eventIndex, expectedEvent := range expectedEventsResponse.Events {
			actualEvent := actualEventsResponse.Events[eventIndex]
			require.Equal(t, expectedEvent.Type, actualEvent.Type)
			require.Equal(t, expectedEvent.TransactionID, actualEvent.TransactionID)
			require.Equal(t, expectedEvent.TransactionIndex, actualEvent.TransactionIndex)
			require.Equal(t, expectedEvent.EventIndex, actualEvent.EventIndex)
			require.Equal(t, expectedEvent.Payload, actualEvent.Payload)
		}
	}
}
