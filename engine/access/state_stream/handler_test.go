package state_stream

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	jsoncdc "github.com/onflow/cadence/encoding/json"
	access "github.com/onflow/flow/protobuf/go/flow/executiondata"
	pb "google.golang.org/genproto/googleapis/bytestream"

	//"github.com/onflow/flow-go/engine/access/state_stream"
	ssmock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/generator"

	"github.com/stretchr/testify/suite"
)

func TestHeartbeatResponseSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

type HandlerTestSuite struct {
	BackendExecutionDataSuite
	handler *Handler
}

// fakeReadServerImpl is an utility structure for receiving response from grpc handler without building a complete pipeline with client and server.
// It allows to receive streamed events pushed by server in buffered channel that can be later used to assert correctness of responses
type fakeReadServerImpl struct {
	pb.ByteStream_ReadServer
	ctx      context.Context
	received chan *access.SubscribeEventsResponse
}

var _ access.ExecutionDataAPI_SubscribeEventsServer = (*fakeReadServerImpl)(nil)

func (fake *fakeReadServerImpl) Context() context.Context {
	return fake.ctx
}

func (fake *fakeReadServerImpl) Send(response *access.SubscribeEventsResponse) error {
	fake.received <- response
	return nil
}

func (s *HandlerTestSuite) SetupTest() {
	s.BackendExecutionDataSuite.SetupTest()
	conf := DefaultEventFilterConfig
	chain := flow.MonotonicEmulator.Chain()
	s.handler = NewHandler(s.backend, chain, conf, 5)
}

// TestHeartbeatResponse tests the periodic heartbeat response.
//
// Test Steps:
// - Generate different events in blocks.
// - Create different filters for generated events.
// - Wait for either responses with filtered events or heartbeat responses.
// - Verify that the responses are being sent with proper heartbeat interval.
func (s *HandlerTestSuite) TestHeartbeatResponse() {
	reader := &fakeReadServerImpl{
		ctx:      context.Background(),
		received: make(chan *access.SubscribeEventsResponse, 100),
	}

	// notify backend block is available
	s.backend.setHighestHeight(s.blocks[len(s.blocks)-1].Header.Height)

	s.Run("All events filter", func() {
		// create empty event filter
		filter := &access.EventFilter{}
		// create subscribe events request, set the created filter and heartbeatInterval
		req := &access.SubscribeEventsRequest{
			StartBlockHeight:  0,
			Filter:            filter,
			HeartbeatInterval: 1,
		}

		// subscribe for events
		go func() {
			err := s.handler.SubscribeEvents(req, reader)
			require.NoError(s.T(), err)
		}()

		for _, b := range s.blocks {
			// consume execution data from subscription
			unittest.RequireReturnsBefore(s.T(), func() {
				resp, ok := <-reader.received
				require.True(s.T(), ok, "channel closed while waiting for exec data for block %d %v", b.Header.Height, b.ID())

				blockID, err := convert.BlockID(resp.BlockId)
				require.NoError(s.T(), err)
				require.Equal(s.T(), b.Header.ID(), blockID)
				require.Equal(s.T(), b.Header.Height, resp.BlockHeight)
			}, time.Second, fmt.Sprintf("timed out waiting for exec data for block %d %v", b.Header.Height, b.ID()))
		}
	})

	s.Run("Event A.0x1.Foo.Bar filter with heartbeat interval 1", func() {
		// create A.0x1.Foo.Bar event filter
		pbFilter := &access.EventFilter{
			EventType: []string{string(testEventTypes[0])},
			Contract:  nil,
			Address:   nil,
		}
		// create subscribe events request, set the created filter and heartbeatInterval
		req := &access.SubscribeEventsRequest{
			StartBlockHeight:  0,
			Filter:            pbFilter,
			HeartbeatInterval: 1,
		}

		// subscribe for events
		go func() {
			err := s.handler.SubscribeEvents(req, reader)
			require.NoError(s.T(), err)
		}()

		for _, b := range s.blocks {

			// consume execution data from subscription
			unittest.RequireReturnsBefore(s.T(), func() {
				resp, ok := <-reader.received
				require.True(s.T(), ok, "channel closed while waiting for exec data for block %d %v", b.Header.Height, b.ID())

				blockID, err := convert.BlockID(resp.BlockId)
				require.NoError(s.T(), err)
				require.Equal(s.T(), b.Header.ID(), blockID)
				require.Equal(s.T(), b.Header.Height, resp.BlockHeight)
			}, time.Second, fmt.Sprintf("timed out waiting for exec data for block %d %v", b.Header.Height, b.ID()))
		}
	})

	s.Run("Non existent filter with heartbeat interval 2", func() {
		// create non existent filter
		pbFilter := &access.EventFilter{
			EventType: []string{"A.0x1.NonExistent.Event"},
			Contract:  nil,
			Address:   nil,
		}

		// create subscribe events request, set the created filter and heartbeatInterval
		req := &access.SubscribeEventsRequest{
			StartBlockHeight:  0,
			Filter:            pbFilter,
			HeartbeatInterval: 2,
		}

		// subscribe for events
		go func() {
			err := s.handler.SubscribeEvents(req, reader)
			require.NoError(s.T(), err)
		}()

		// expect a response for every other block
		expectedBlocks := make([]*flow.Block, 0)
		for i, block := range s.blocks {
			if (i+1)%int(req.HeartbeatInterval) == 0 {
				expectedBlocks = append(expectedBlocks, block)
			}
		}

		require.Len(s.T(), expectedBlocks, len(s.blocks)/int(req.HeartbeatInterval))

		for _, b := range expectedBlocks {
			// consume execution data from subscription
			unittest.RequireReturnsBefore(s.T(), func() {
				resp, ok := <-reader.received
				require.True(s.T(), ok, "channel closed while waiting for exec data for block %d %v", b.Header.Height, b.ID())

				blockID, err := convert.BlockID(resp.BlockId)
				require.NoError(s.T(), err)
				require.Equal(s.T(), b.Header.Height, resp.BlockHeight)
				require.Equal(s.T(), b.Header.ID(), blockID)
				require.Empty(s.T(), resp.Events)
			}, time.Second, fmt.Sprintf("timed out waiting for exec data for block %d %v", b.Header.Height, b.ID()))
		}
	})
}

func TestExecutionDataStream(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api := ssmock.NewAPI(t)
	stream := makeStreamMock[access.SubscribeExecutionDataRequest, access.SubscribeExecutionDataResponse](ctx)
	sub := NewSubscription(1)

	// generate some events with a payload to include
	// generators will produce identical event payloads (before encoding)
	ccfEventGenerator := generator.EventGenerator(generator.WithEncoding(generator.EncodingCCF))
	jsonEventsGenerator := generator.EventGenerator(generator.WithEncoding(generator.EncodingJSON))
	inputEvents := make([]flow.Event, 0, 3)
	expectedEvents := make([]flow.Event, 0, 3)
	for i := 0; i < 3; i++ {
		inputEvents = append(inputEvents, ccfEventGenerator.New())
		expectedEvents = append(expectedEvents, jsonEventsGenerator.New())
	}

	api.On("SubscribeExecutionData", mock.Anything, flow.ZeroID, uint64(0), mock.Anything).Return(sub)

	h := NewHandler(api, flow.Localnet.Chain(), EventFilterConfig{}, 1)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		wg.Done()
		err := h.SubscribeExecutionData(&access.SubscribeExecutionDataRequest{}, stream)
		require.NoError(t, err)
		t.Log("subscription closed")
	}()
	wg.Wait()

	// send a single response
	blockHeight := uint64(1)
	executionData := unittest.BlockExecutionDataFixture(
		unittest.WithChunkExecutionDatas(
			unittest.ChunkExecutionDataFixture(t, 1024, unittest.WithChunkEvents(inputEvents)),
			unittest.ChunkExecutionDataFixture(t, 1024, unittest.WithChunkEvents(inputEvents)),
		),
	)

	err := sub.Send(ctx, &ExecutionDataResponse{
		Height:        blockHeight,
		ExecutionData: executionData,
	}, 100*time.Millisecond)
	require.NoError(t, err)

	// notify end of data
	sub.Close()

	receivedCount := 0
	for {
		t.Log(receivedCount)
		resp, err := stream.RecvToClient()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		convertedExecData, err := convert.MessageToBlockExecutionData(resp.GetBlockExecutionData(), flow.Testnet.Chain())
		require.NoError(t, err)

		assert.Equal(t, blockHeight, resp.GetBlockHeight())

		// make sure the payload is valid JSON-CDC
		for _, chunk := range convertedExecData.ChunkExecutionDatas {
			for i, e := range chunk.Events {
				assert.Equal(t, expectedEvents[i], e)

				_, err := jsoncdc.Decode(nil, e.Payload)
				require.NoError(t, err)
			}
		}

		receivedCount++

		// shutdown the stream after one response
		close(stream.sentFromServer)
	}

	// only expect a single response
	assert.Equal(t, 1, receivedCount)
}

func TestEventStream(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api := ssmock.NewAPI(t)
	stream := makeStreamMock[access.SubscribeEventsRequest, access.SubscribeEventsResponse](ctx)
	sub := NewSubscription(1)

	// generate some events with a payload to include
	// generators will produce identical event payloads (before encoding)
	ccfEventGenerator := generator.EventGenerator(generator.WithEncoding(generator.EncodingCCF))
	jsonEventsGenerator := generator.EventGenerator(generator.WithEncoding(generator.EncodingJSON))
	inputEvents := make([]flow.Event, 0, 3)
	expectedEvents := make([]flow.Event, 0, 3)
	for i := 0; i < 3; i++ {
		inputEvents = append(inputEvents, ccfEventGenerator.New())
		expectedEvents = append(expectedEvents, jsonEventsGenerator.New())
	}

	api.On("SubscribeEvents", mock.Anything, flow.ZeroID, uint64(0), mock.Anything).Return(sub)

	h := NewHandler(api, flow.Localnet.Chain(), EventFilterConfig{}, 1)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		wg.Done()
		err := h.SubscribeEvents(&access.SubscribeEventsRequest{}, stream)
		require.NoError(t, err)
		t.Log("subscription closed")
	}()
	wg.Wait()

	// send a single response
	blockHeight := uint64(1)
	blockID := unittest.IdentifierFixture()
	err := sub.Send(ctx, &EventsResponse{
		BlockID: blockID,
		Height:  blockHeight,
		Events:  inputEvents,
	}, 100*time.Millisecond)
	require.NoError(t, err)

	// notify end of data
	sub.Close()

	receivedCount := 0
	for {
		t.Log(receivedCount)
		resp, err := stream.RecvToClient()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		convertedEvents := convert.MessagesToEvents(resp.GetEvents())

		assert.Equal(t, blockHeight, resp.GetBlockHeight())
		assert.Equal(t, blockID, convert.MessageToIdentifier(resp.GetBlockId()))
		assert.Equal(t, expectedEvents, convertedEvents)

		// make sure the payload is valid JSON-CDC
		for _, e := range convertedEvents {
			_, err := jsoncdc.Decode(nil, e.Payload)
			require.NoError(t, err)
		}

		receivedCount++

		// shutdown the stream after one response
		close(stream.sentFromServer)
	}

	// only expect a single response
	assert.Equal(t, 1, receivedCount)
}

func makeStreamMock[R, T any](ctx context.Context) *StreamMock[R, T] {
	return &StreamMock[R, T]{
		ctx:            ctx,
		recvToServer:   make(chan *R, 10),
		sentFromServer: make(chan *T, 10),
	}
}

type StreamMock[R, T any] struct {
	grpc.ServerStream
	ctx            context.Context
	recvToServer   chan *R
	sentFromServer chan *T
}

func (m *StreamMock[R, T]) Context() context.Context {
	return m.ctx
}
func (m *StreamMock[R, T]) Send(resp *T) error {
	m.sentFromServer <- resp
	return nil
}

func (m *StreamMock[R, T]) Recv() (*R, error) {
	req, more := <-m.recvToServer
	if !more {
		return nil, io.EOF
	}
	return req, nil
}

func (m *StreamMock[R, T]) SendFromClient(req *R) error {
	m.recvToServer <- req
	return nil
}

func (m *StreamMock[R, T]) RecvToClient() (*T, error) {
	response, more := <-m.sentFromServer
	if !more {
		return nil, io.EOF
	}
	return response, nil
}
