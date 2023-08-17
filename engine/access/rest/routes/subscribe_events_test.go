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

	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/state_stream"
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

			filter, err := state_stream.NewEventFilter(state_stream.DefaultEventFilterConfig, chain, test.eventTypes, test.addresses, test.contracts)
			assert.NoError(s.T(), err)

			var expectedEventsResponses []*state_stream.EventsResponse
			startBlockFound := test.startBlockID == flow.ZeroID

			// Helper function to check if a string is present in a slice
			addExpectedEvent := func(slice []string, item string) bool {
				if slice == nil {
					return true // Include all events when test.eventTypes is nil
				}
				for _, s := range slice {
					if s == item {
						return true
					}
				}
				return false
			}

			// construct expected event responses based on the provided test configuration
			for _, block := range s.blocks {
				if startBlockFound || block.ID() == test.startBlockID {
					startBlockFound = true
					if test.startHeight == request.EmptyHeight || block.Header.Height >= test.startHeight {
						eventsForBlock := flow.EventsList{}
						for _, event := range s.blockEvents[block.ID()] {
							if addExpectedEvent(test.eventTypes, string(event.Type)) {
								eventsForBlock = append(eventsForBlock, event)
							}
						}
						eventResponse := &state_stream.EventsResponse{
							Height:  block.Header.Height,
							BlockID: block.ID(),
							Events:  eventsForBlock,
						}
						expectedEventsResponses = append(expectedEventsResponses, eventResponse)
					}
				}
			}

			// Create a channel to receive mock EventsResponse objects
			ch := make(chan interface{})
			var chReadOnly <-chan interface{}
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

			var startHeight uint64
			if test.startHeight == request.EmptyHeight {
				startHeight = uint64(0)
			} else {
				startHeight = test.startHeight
			}
			stateStreamBackend.Mock.On("SubscribeEvents", mocks.Anything, test.startBlockID, startHeight, filter).Return(subscription)

			req, err := getSubscribeEventsRequest(s.T(), test.startBlockID, test.startHeight, test.eventTypes, test.addresses, test.contracts)
			assert.NoError(s.T(), err)
			respRecorder, err := executeRequest(req, backend, stateStreamBackend)
			assert.NoError(s.T(), err)
			requireResponse(s.T(), respRecorder, expectedEventsResponses)
		})
	}
}

func (s *SubscribeEventsSuite) TestSubscribeEventsHandlesErrors() {
	s.Run("returns error for block id and height", func() {
		stateStreamBackend := &mockstatestream.API{}
		backend := &mock.API{}

		req, err := getSubscribeEventsRequest(s.T(), s.blocks[0].ID(), s.blocks[0].Header.Height, nil, nil, nil)
		assert.NoError(s.T(), err)
		respRecorder, err := executeRequest(req, backend, stateStreamBackend)
		assert.NoError(s.T(), err)
		requireError(s.T(), respRecorder, "can only provide either block ID or start height")
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
		stateStreamBackend.Mock.On("SubscribeEvents", mocks.Anything, invalidBlock.ID(), uint64(0), mocks.Anything).Return(subscription)

		req, err := getSubscribeEventsRequest(s.T(), invalidBlock.ID(), request.EmptyHeight, nil, nil, nil)
		assert.NoError(s.T(), err)
		respRecorder, err := executeRequest(req, backend, stateStreamBackend)
		assert.NoError(s.T(), err)
		requireError(s.T(), respRecorder, "stream encountered an error: subscription error")
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
		stateStreamBackend.Mock.On("SubscribeEvents", mocks.Anything, s.blocks[0].ID(), uint64(0), mocks.Anything).Return(subscription)

		req, err := getSubscribeEventsRequest(s.T(), s.blocks[0].ID(), request.EmptyHeight, nil, nil, nil)
		assert.NoError(s.T(), err)
		respRecorder, err := executeRequest(req, backend, stateStreamBackend)
		assert.NoError(s.T(), err)
		requireError(s.T(), respRecorder, "subscription channel closed")
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
		stateStreamBackend.Mock.On("SubscribeEvents", mocks.Anything, s.blocks[0].ID(), uint64(0), mocks.Anything).Return(subscription)

		req, err := getSubscribeEventsRequest(s.T(), s.blocks[0].ID(), request.EmptyHeight, nil, nil, nil)
		assert.NoError(s.T(), err)

		respRecorder, err := executeRequest(req, backend, stateStreamBackend)
		assert.NoError(s.T(), err)
		requireError(s.T(), respRecorder, "unexpected response type: *state_stream.ExecutionDataResponse")
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

func requireError(t *testing.T, recorder *HijackResponseRecorder, expected string) {
	<-recorder.closed
	require.Contains(t, recorder.responseBuff.String(), expected)
}

func requireResponse(t *testing.T, recorder *HijackResponseRecorder, expected []*state_stream.EventsResponse) {
	time.Sleep(1 * time.Second)
	// Convert the actual response from respRecorder to JSON bytes
	actualJSON := recorder.responseBuff.Bytes()
	// Define a regular expression pattern to match JSON objects
	pattern := `\{"BlockID":".*?","Height":\d+,"Events":\[\{.*?\}\]\}`
	matches := regexp.MustCompile(pattern).FindAll(actualJSON, -1)

	// Unmarshal each matched JSON into []state_stream.EventsResponse
	var actual []state_stream.EventsResponse
	for _, match := range matches {
		var response state_stream.EventsResponse
		if err := json.Unmarshal(match, &response); err == nil {
			actual = append(actual, response)
		}
	}

	// Compare the count of expected and actual responses
	assert.Equal(t, len(expected), len(actual))

	// Compare the BlockID and Events count for each response
	for i := 0; i < len(expected); i++ {
		expected := expected[i]
		actual := actual[i]

		assert.Equal(t, expected.BlockID, actual.BlockID)
		assert.Equal(t, len(expected.Events), len(actual.Events))
	}
}
