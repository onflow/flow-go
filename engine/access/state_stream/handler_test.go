package state_stream_test

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	access "github.com/onflow/flow/protobuf/go/flow/executiondata"

	"github.com/onflow/flow-go/engine/access/state_stream"
	ssmock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/generator"
)

func TestExecutionDataStream(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api := ssmock.NewAPI(t)
	stream := makeStreamMock[access.SubscribeExecutionDataRequest, access.SubscribeExecutionDataResponse](ctx)
	sub := state_stream.NewSubscription(1)

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

	h := state_stream.NewHandler(api, flow.Localnet.Chain(), state_stream.EventFilterConfig{}, 1)

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

	err := sub.Send(ctx, &state_stream.ExecutionDataResponse{
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

func TestExecutionDataStreamEventEncoding(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// generate some events with a payload to include
	// generators will produce identical event payloads (before encoding)
	ccfEventGenerator := generator.EventGenerator(generator.WithEncoding(generator.EncodingCCF))
	inputEvents := make([]flow.Event, 0, 1)
	inputEvents = append(inputEvents, ccfEventGenerator.New())

	jsonEventsGenerator := generator.EventGenerator(generator.WithEncoding(generator.EncodingJSON))
	expectedEvents := make([]flow.Event, 0, 1)
	expectedEvents = append(expectedEvents, jsonEventsGenerator.New())

	api := ssmock.NewAPI(t)
	stream := makeStreamMock[access.SubscribeExecutionDataRequest, access.SubscribeExecutionDataResponse](ctx)

	makeStreamRequest := func(request *access.SubscribeExecutionDataRequest) {
		sub := state_stream.NewSubscription(1)

		api.On("SubscribeExecutionData", mock.Anything, flow.ZeroID, uint64(0), mock.Anything).Return(sub)

		h := state_stream.NewHandler(api, flow.Localnet.Chain(), state_stream.EventFilterConfig{}, 1)

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			wg.Done()
			err := h.SubscribeExecutionData(request, stream)
			require.NoError(t, err)
			t.Log("subscription closed")
		}()
		wg.Wait()

		// send a single response
		blockHeight := uint64(1)
		executionData := unittest.BlockExecutionDataFixture(
			unittest.WithChunkExecutionDatas(
				unittest.ChunkExecutionDataFixture(t, 1024, unittest.WithChunkEvents(inputEvents)),
			),
		)

		err := sub.Send(ctx, &state_stream.ExecutionDataResponse{
			Height:        blockHeight,
			ExecutionData: executionData,
		}, 100*time.Millisecond)
		require.NoError(t, err)
		// notify end of data
		sub.Close()
	}

	t.Run("test default(JSON)", func(t *testing.T) {
		makeStreamRequest(&access.SubscribeExecutionDataRequest{})
		for {
			resp, err := stream.RecvToClient()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)

			convertedExecData, err := convert.MessageToBlockExecutionData(resp.GetBlockExecutionData(), flow.Testnet.Chain())
			require.NoError(t, err)

			// make sure the payload is valid JSON-CDC
			for _, chunk := range convertedExecData.ChunkExecutionDatas {
				for i, e := range chunk.Events {
					assert.Equal(t, expectedEvents[i], e)
				}
			}

			close(stream.sentFromServer)
		}
	})

	t.Run("test JSON event encoding", func(t *testing.T) {
		makeStreamRequest(&access.SubscribeExecutionDataRequest{
			EventEncodingVersion: &entities.EventEncodingVersionValue{Value: entities.EventEncodingVersion_JSON_CDC_V0},
		})
		for {
			resp, err := stream.RecvToClient()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)

			convertedExecData, err := convert.MessageToBlockExecutionData(resp.GetBlockExecutionData(), flow.Testnet.Chain())
			require.NoError(t, err)

			// make sure the payload is valid JSON-CDC
			for _, chunk := range convertedExecData.ChunkExecutionDatas {
				for i, e := range chunk.Events {
					assert.Equal(t, expectedEvents[i], e)
				}
			}
			close(stream.sentFromServer)
		}

	})

	t.Run("test CFF event encoding", func(t *testing.T) {
		makeStreamRequest(&access.SubscribeExecutionDataRequest{
			EventEncodingVersion: &entities.EventEncodingVersionValue{Value: entities.EventEncodingVersion_CCF_V0},
		})
		for {
			resp, err := stream.RecvToClient()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)

			convertedExecData, err := convert.MessageToBlockExecutionData(resp.GetBlockExecutionData(), flow.Testnet.Chain())
			require.NoError(t, err)

			// make sure the payload is valid JSON-CDC
			for _, chunk := range convertedExecData.ChunkExecutionDatas {
				for i, e := range chunk.Events {
					assert.Equal(t, inputEvents[i], e)
				}
			}
			close(stream.sentFromServer)
		}
	})
}

func TestEventStream(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api := ssmock.NewAPI(t)
	stream := makeStreamMock[access.SubscribeEventsRequest, access.SubscribeEventsResponse](ctx)
	sub := state_stream.NewSubscription(1)

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

	h := state_stream.NewHandler(api, flow.Localnet.Chain(), state_stream.EventFilterConfig{}, 1)

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
	err := sub.Send(ctx, &state_stream.EventsResponse{
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
