package connection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"

	"google.golang.org/grpc/encoding/gzip"

	"github.com/onflow/flow-go/engine/common/grpc/compressor/deflate"
	"github.com/onflow/flow-go/engine/common/grpc/compressor/snappy"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/grpcutils"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow/protobuf/go/flow/execution"
)

// BenchmarkWithGzipCompression benchmarks the gRPC request to execution nodes using gzip compressor.
func BenchmarkWithGzipCompression(b *testing.B) {
	runBenchmark(b, gzip.Name)
}

// BenchmarkWithSnappyCompression benchmarks the gRPC request to execution nodes using snappy compressor.
func BenchmarkWithSnappyCompression(b *testing.B) {
	runBenchmark(b, snappy.Name)
}

// BenchmarkWithDeflateCompression benchmarks the gRPC request to execution nodes using deflate compressor.
func BenchmarkWithDeflateCompression(b *testing.B) {
	runBenchmark(b, deflate.Name)
}

// runBenchmark is a helper function that performs the benchmarking for different compressors.
func runBenchmark(b *testing.B, compressorName string) {
	// create an execution node
	en := new(executionNode)
	en.start(b)
	defer en.stop(b)

	blockHeaders := getHeaders(5)
	exeResults := make([]*execution.GetEventsForBlockIDsResponse_Result, len(blockHeaders))
	for i := 0; i < len(blockHeaders); i++ {
		exeResults[i] = &execution.GetEventsForBlockIDsResponse_Result{
			BlockId:     convert.IdentifierToMessage(blockHeaders[i].ID()),
			BlockHeight: blockHeaders[i].Height,
			Events:      convert.EventsToMessages(getEvents(10)),
		}
	}
	expectedEventsResponse := &execution.GetEventsForBlockIDsResponse{
		Results: exeResults,
	}

	blockIDs := make([]flow.Identifier, len(blockHeaders))
	for i, header := range blockHeaders {
		blockIDs[i] = header.ID()
	}
	eventsReq := &execution.GetEventsForBlockIDsRequest{
		BlockIds: convert.IdentifiersToMessages(blockIDs),
		Type:     string(flow.EventAccountCreated),
	}

	en.handler.On("GetEventsForBlockIDs", testifymock.Anything, testifymock.Anything).
		Return(expectedEventsResponse, nil)

	// create the factory
	connectionFactory := new(ConnectionFactoryImpl)
	// set the execution grpc port
	connectionFactory.ExecutionGRPCPort = en.port

	// set metrics reporting
	connectionFactory.AccessMetrics = metrics.NewNoopCollector()
	connectionFactory.Manager = NewManager(
		nil,
		unittest.Logger(),
		connectionFactory.AccessMetrics,
		grpcutils.DefaultMaxMsgSize,
		CircuitBreakerConfig{},
		compressorName,
	)

	proxyConnectionFactory := ProxyConnectionFactory{
		ConnectionFactory: connectionFactory,
		targetAddress:     en.listener.Addr().String(),
	}

	// get an execution API client
	client, _, err := proxyConnectionFactory.GetExecutionAPIClient("foo")
	assert.NoError(b, err)

	ctx := context.Background()
	b.ResetTimer()
	// make the call to the execution node
	for i := 0; i < b.N; i++ {
		_, err := client.GetEventsForBlockIDs(ctx, eventsReq)
		assert.NoError(b, err)
	}
}

// getEvents generates a slice of flow events with a specified length.
func getEvents(n int) []flow.Event {
	events := make([]flow.Event, n)
	for i := range events {
		events[i] = flow.Event{Type: flow.EventAccountCreated}
	}
	return events
}

// getHeaders generates a slice of flow headers with a specified length.
func getHeaders(n int) []*flow.Header {
	headers := make([]*flow.Header, n)
	for i := range headers {
		b := unittest.BlockFixture()
		headers[i] = b.Header

	}
	return headers
}
