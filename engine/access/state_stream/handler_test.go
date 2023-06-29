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
	access "github.com/onflow/flow/protobuf/go/flow/executiondata"

	"github.com/onflow/flow-go/engine/access/state_stream"
	ssmock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/generator"
)

func TestExecutionDataStream(t *testing.T) {
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

	sub.Send(ctx, &state_stream.ExecutionDataResponse{
		Height:        blockHeight,
		ExecutionData: executionData,
	}, 100*time.Millisecond)

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
	sub.Send(ctx, &state_stream.EventsResponse{
		BlockID: blockID,
		Height:  blockHeight,
		Events:  inputEvents,
	}, 100*time.Millisecond)

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
