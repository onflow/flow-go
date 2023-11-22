package backend

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
	"github.com/stretchr/testify/suite"
	pb "google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/executiondata"

	"github.com/onflow/flow-go/engine/access/state_stream"
	ssmock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/generator"
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
	received chan *executiondata.SubscribeEventsResponse
}

var _ executiondata.ExecutionDataAPI_SubscribeEventsServer = (*fakeReadServerImpl)(nil)

func (fake *fakeReadServerImpl) Context() context.Context {
	return fake.ctx
}

func (fake *fakeReadServerImpl) Send(response *executiondata.SubscribeEventsResponse) error {
	fake.received <- response
	return nil
}

func (s *HandlerTestSuite) SetupTest() {
	s.BackendExecutionDataSuite.SetupTest()
	chain := flow.MonotonicEmulator.Chain()
	s.handler = NewHandler(s.backend, chain, makeConfig(5))
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
		received: make(chan *executiondata.SubscribeEventsResponse, 100),
	}

	// notify backend block is available
	s.backend.setHighestHeight(s.blocks[len(s.blocks)-1].Header.Height)

	s.Run("All events filter", func() {
		// create empty event filter
		filter := &executiondata.EventFilter{}
		// create subscribe events request, set the created filter and heartbeatInterval
		req := &executiondata.SubscribeEventsRequest{
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
		pbFilter := &executiondata.EventFilter{
			EventType: []string{string(testEventTypes[0])},
			Contract:  nil,
			Address:   nil,
		}
		// create subscribe events request, set the created filter and heartbeatInterval
		req := &executiondata.SubscribeEventsRequest{
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
		pbFilter := &executiondata.EventFilter{
			EventType: []string{"A.0x1.NonExistent.Event"},
			Contract:  nil,
			Address:   nil,
		}

		// create subscribe events request, set the created filter and heartbeatInterval
		req := &executiondata.SubscribeEventsRequest{
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

// TestGetExecutionDataByBlockID tests the execution data by block id with different event encoding versions.
func TestGetExecutionDataByBlockID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ccfEvents := generator.GetEventsWithEncoding(3, entities.EventEncodingVersion_CCF_V0)
	jsonEvents := generator.GetEventsWithEncoding(3, entities.EventEncodingVersion_JSON_CDC_V0)

	tests := []struct {
		eventVersion entities.EventEncodingVersion
		expected     []flow.Event
	}{
		{
			entities.EventEncodingVersion_JSON_CDC_V0,
			jsonEvents,
		},
		{
			entities.EventEncodingVersion_CCF_V0,
			ccfEvents,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("test %s event encoding version", test.eventVersion.String()), func(t *testing.T) {
			result := unittest.BlockExecutionDataFixture(
				unittest.WithChunkExecutionDatas(
					unittest.ChunkExecutionDataFixture(t, 1024, unittest.WithChunkEvents(ccfEvents)),
					unittest.ChunkExecutionDataFixture(t, 1024, unittest.WithChunkEvents(ccfEvents)),
				),
			)
			blockID := result.BlockID

			api := ssmock.NewAPI(t)
			api.On("GetExecutionDataByBlockID", mock.Anything, blockID).Return(result, nil)

			h := NewHandler(api, flow.Localnet.Chain(), makeConfig(1))

			response, err := h.GetExecutionDataByBlockID(ctx, &executiondata.GetExecutionDataByBlockIDRequest{
				BlockId:              blockID[:],
				EventEncodingVersion: test.eventVersion,
			})
			require.NoError(t, err)
			require.NotNil(t, response)

			blockExecutionData := response.GetBlockExecutionData()
			require.Equal(t, blockID[:], blockExecutionData.GetBlockId())

			convertedExecData, err := convert.MessageToBlockExecutionData(blockExecutionData, flow.Testnet.Chain())
			require.NoError(t, err)

			// Verify that the payload is valid
			for _, chunk := range convertedExecData.ChunkExecutionDatas {
				for i, e := range chunk.Events {
					assert.Equal(t, test.expected[i], e)

					var err error
					if test.eventVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
						_, err = jsoncdc.Decode(nil, e.Payload)
					} else {
						_, err = ccf.Decode(nil, e.Payload)
					}
					require.NoError(t, err)
				}
			}
		})
	}
}

// TestExecutionDataStream tests the execution data stream with different event encoding versions.
func TestExecutionDataStream(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Send a single response.
	blockHeight := uint64(1)

	// Helper function to perform a stream request and handle responses.
	makeStreamRequest := func(
		stream *StreamMock[executiondata.SubscribeExecutionDataRequest, executiondata.SubscribeExecutionDataResponse],
		api *ssmock.API,
		request *executiondata.SubscribeExecutionDataRequest,
		response *ExecutionDataResponse,
	) {
		sub := NewSubscription(1)

		api.On("SubscribeExecutionData", mock.Anything, flow.ZeroID, uint64(0), mock.Anything).Return(sub)

		h := NewHandler(api, flow.Localnet.Chain(), makeConfig(1))

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			wg.Done()
			err := h.SubscribeExecutionData(request, stream)
			require.NoError(t, err)
			t.Log("subscription closed")
		}()
		wg.Wait()

		err := sub.Send(ctx, response, 100*time.Millisecond)
		require.NoError(t, err)

		// Notify end of data.
		sub.Close()
	}

	// handleExecutionDataStreamResponses handles responses from the execution data stream.
	handleExecutionDataStreamResponses := func(
		stream *StreamMock[executiondata.SubscribeExecutionDataRequest, executiondata.SubscribeExecutionDataResponse],
		version entities.EventEncodingVersion,
		expectedEvents []flow.Event,
	) {
		var responses []*executiondata.SubscribeExecutionDataResponse
		for {
			t.Log(len(responses))
			resp, err := stream.RecvToClient()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			responses = append(responses, resp)
			close(stream.sentFromServer)
		}

		for _, resp := range responses {
			convertedExecData, err := convert.MessageToBlockExecutionData(resp.GetBlockExecutionData(), flow.Testnet.Chain())
			require.NoError(t, err)

			assert.Equal(t, blockHeight, resp.GetBlockHeight())

			// only expect a single response
			assert.Equal(t, 1, len(responses))

			// Verify that the payload is valid
			for _, chunk := range convertedExecData.ChunkExecutionDatas {
				for i, e := range chunk.Events {
					assert.Equal(t, expectedEvents[i], e)

					var err error
					if version == entities.EventEncodingVersion_JSON_CDC_V0 {
						_, err = jsoncdc.Decode(nil, e.Payload)
					} else {
						_, err = ccf.Decode(nil, e.Payload)
					}
					require.NoError(t, err)
				}
			}
		}
	}

	ccfEvents := generator.GetEventsWithEncoding(3, entities.EventEncodingVersion_CCF_V0)
	jsonEvents := generator.GetEventsWithEncoding(3, entities.EventEncodingVersion_JSON_CDC_V0)

	tests := []struct {
		eventVersion entities.EventEncodingVersion
		expected     []flow.Event
	}{
		{
			entities.EventEncodingVersion_JSON_CDC_V0,
			jsonEvents,
		},
		{
			entities.EventEncodingVersion_CCF_V0,
			ccfEvents,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("test %s event encoding version", test.eventVersion.String()), func(t *testing.T) {
			api := ssmock.NewAPI(t)
			stream := makeStreamMock[executiondata.SubscribeExecutionDataRequest, executiondata.SubscribeExecutionDataResponse](ctx)

			makeStreamRequest(
				stream,
				api,
				&executiondata.SubscribeExecutionDataRequest{
					EventEncodingVersion: test.eventVersion,
				},
				&ExecutionDataResponse{
					Height: blockHeight,
					ExecutionData: unittest.BlockExecutionDataFixture(
						unittest.WithChunkExecutionDatas(
							unittest.ChunkExecutionDataFixture(t, 1024, unittest.WithChunkEvents(ccfEvents)),
							unittest.ChunkExecutionDataFixture(t, 1024, unittest.WithChunkEvents(ccfEvents)),
						),
					),
				},
			)
			handleExecutionDataStreamResponses(stream, test.eventVersion, test.expected)
		})
	}
}

// TestEventStream tests the event stream with different event encoding versions.
func TestEventStream(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	blockHeight := uint64(1)
	blockID := unittest.IdentifierFixture()

	// Helper function to perform a stream request and handle responses.
	makeStreamRequest := func(
		stream *StreamMock[executiondata.SubscribeEventsRequest, executiondata.SubscribeEventsResponse],
		api *ssmock.API,
		request *executiondata.SubscribeEventsRequest,
		response *EventsResponse,
	) {
		sub := NewSubscription(1)

		api.On("SubscribeEvents", mock.Anything, flow.ZeroID, uint64(0), mock.Anything).Return(sub)

		h := NewHandler(api, flow.Localnet.Chain(), makeConfig(1))

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			wg.Done()
			err := h.SubscribeEvents(request, stream)
			require.NoError(t, err)
			t.Log("subscription closed")
		}()
		wg.Wait()

		// send a single response
		err := sub.Send(ctx, response, 100*time.Millisecond)
		require.NoError(t, err)

		// notify end of data
		sub.Close()
	}

	// handleExecutionDataStreamResponses handles responses from the execution data stream.
	handleExecutionDataStreamResponses := func(
		stream *StreamMock[executiondata.SubscribeEventsRequest, executiondata.SubscribeEventsResponse],
		version entities.EventEncodingVersion,
		expectedEvents []flow.Event,
	) {
		var responses []*executiondata.SubscribeEventsResponse
		for {
			t.Log(len(responses))
			resp, err := stream.RecvToClient()
			if err == io.EOF {
				break
			}
			// make sure the payload is valid
			require.NoError(t, err)
			responses = append(responses, resp)

			// shutdown the stream after one response
			close(stream.sentFromServer)
		}

		for _, resp := range responses {
			convertedEvents := convert.MessagesToEvents(resp.GetEvents())

			assert.Equal(t, blockHeight, resp.GetBlockHeight())
			assert.Equal(t, blockID, convert.MessageToIdentifier(resp.GetBlockId()))
			assert.Equal(t, expectedEvents, convertedEvents)
			// only expect a single response
			assert.Equal(t, 1, len(responses))

			for _, e := range convertedEvents {
				var err error
				if version == entities.EventEncodingVersion_JSON_CDC_V0 {
					_, err = jsoncdc.Decode(nil, e.Payload)
				} else {
					_, err = ccf.Decode(nil, e.Payload)
				}
				require.NoError(t, err)
			}
		}
	}

	// generate events with a payload to include
	// generators will produce identical event payloads (before encoding)
	ccfEvents := generator.GetEventsWithEncoding(3, entities.EventEncodingVersion_CCF_V0)
	jsonEvents := generator.GetEventsWithEncoding(3, entities.EventEncodingVersion_JSON_CDC_V0)

	tests := []struct {
		eventVersion entities.EventEncodingVersion
		expected     []flow.Event
	}{
		{
			entities.EventEncodingVersion_JSON_CDC_V0,
			jsonEvents,
		},
		{
			entities.EventEncodingVersion_CCF_V0,
			ccfEvents,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("test %s event encoding version", test.eventVersion.String()), func(t *testing.T) {
			stream := makeStreamMock[executiondata.SubscribeEventsRequest, executiondata.SubscribeEventsResponse](ctx)

			makeStreamRequest(
				stream,
				ssmock.NewAPI(t),
				&executiondata.SubscribeEventsRequest{
					EventEncodingVersion: test.eventVersion,
				},
				&EventsResponse{
					BlockID: blockID,
					Height:  blockHeight,
					Events:  ccfEvents,
				},
			)
			handleExecutionDataStreamResponses(stream, test.eventVersion, test.expected)
		})
	}
}

// TestGetRegisterValues tests the register values.
func TestGetRegisterValues(t *testing.T) {
	t.Parallel()
	testHeight := uint64(1)

	// test register IDs + values
	testIds := flow.RegisterIDs{
		flow.UUIDRegisterID(0),
		flow.AccountStatusRegisterID(unittest.AddressFixture()),
		unittest.RegisterIDFixture(),
	}
	testValues := []flow.RegisterValue{
		[]byte("uno"),
		[]byte("dos"),
		[]byte("tres"),
	}
	invalidIDs := append(testIds, flow.RegisterID{}) // valid + invalid IDs

	t.Run("invalid message", func(t *testing.T) {
		api := ssmock.NewAPI(t)
		h := NewHandler(api, flow.Localnet.Chain(), makeConfig(1))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		invalidMessage := &executiondata.GetRegisterValuesRequest{
			RegisterIds: nil,
		}
		_, err := h.GetRegisterValues(ctx, invalidMessage)
		require.Equal(t, status.Code(err), codes.InvalidArgument)
	})

	t.Run("valid registers", func(t *testing.T) {
		api := ssmock.NewAPI(t)
		api.On("GetRegisterValues", testIds, testHeight).Return(testValues, nil)
		h := NewHandler(api, flow.Localnet.Chain(), makeConfig(1))
		validRegisters := make([]*entities.RegisterID, len(testIds))
		for i, id := range testIds {
			validRegisters[i] = convert.RegisterIDToMessage(id)
		}
		req := &executiondata.GetRegisterValuesRequest{
			RegisterIds: validRegisters,
			BlockHeight: testHeight,
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		resp, err := h.GetRegisterValues(ctx, req)
		require.NoError(t, err)
		require.Equal(t, resp.GetValues(), testValues)

	})

	t.Run("unavailable registers", func(t *testing.T) {
		api := ssmock.NewAPI(t)
		expectedErr := status.Errorf(codes.NotFound, "could not get register values: %v", storage.ErrNotFound)
		api.On("GetRegisterValues", invalidIDs, testHeight).Return(nil, expectedErr)
		h := NewHandler(api, flow.Localnet.Chain(), makeConfig(1))
		unavailableRegisters := make([]*entities.RegisterID, len(invalidIDs))
		for i, id := range invalidIDs {
			unavailableRegisters[i] = convert.RegisterIDToMessage(id)
		}
		req := &executiondata.GetRegisterValuesRequest{
			RegisterIds: unavailableRegisters,
			BlockHeight: testHeight,
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_, err := h.GetRegisterValues(ctx, req)
		require.Equal(t, status.Code(err), codes.NotFound)

	})

	t.Run("wrong height", func(t *testing.T) {
		api := ssmock.NewAPI(t)
		expectedErr := status.Errorf(codes.OutOfRange, "could not get register values: %v", storage.ErrHeightNotIndexed)
		api.On("GetRegisterValues", testIds, testHeight+1).Return(nil, expectedErr)
		h := NewHandler(api, flow.Localnet.Chain(), makeConfig(1))
		validRegisters := make([]*entities.RegisterID, len(testIds))
		for i, id := range testIds {
			validRegisters[i] = convert.RegisterIDToMessage(id)
		}
		req := &executiondata.GetRegisterValuesRequest{
			RegisterIds: validRegisters,
			BlockHeight: testHeight + 1,
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_, err := h.GetRegisterValues(ctx, req)
		require.Equal(t, status.Code(err), codes.OutOfRange)
	})

}

func makeConfig(maxGlobalStreams uint32) Config {
	return Config{
		EventFilterConfig:    state_stream.DefaultEventFilterConfig,
		ClientSendTimeout:    state_stream.DefaultSendTimeout,
		ClientSendBufferSize: state_stream.DefaultSendBufferSize,
		MaxGlobalStreams:     maxGlobalStreams,
		HeartbeatInterval:    state_stream.DefaultHeartbeatInterval,
	}
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
