package grpcserver_test

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/module/grpcserver"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

// blockingStreamService is the interface gRPC uses to type-check the registered handler.
type blockingStreamService interface {
	Stream(grpc.ServerStream) error
}

// blockingStreamServiceDesc is a gRPC service descriptor with a single server-streaming method.
// The handler blocks until the stream context is cancelled, simulating a long-lived subscription.
var blockingStreamServiceDesc = grpc.ServiceDesc{
	ServiceName: "test.BlockingStream",
	HandlerType: (*blockingStreamService)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Stream",
			Handler:       blockingStreamHandler,
			ServerStreams: true,
		},
	},
}

type blockingStreamServer struct {
	// started is closed when the stream handler has been entered.
	started chan struct{}
}

var _ blockingStreamService = (*blockingStreamServer)(nil)

func (s *blockingStreamServer) Stream(stream grpc.ServerStream) error {
	close(s.started)
	<-stream.Context().Done()
	return nil
}

func blockingStreamHandler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(blockingStreamService).Stream(stream)
}

// TestGrpcServerShutdown_WithActiveStream verifies that GrpcServer shuts down within
// gracefulStopTimeout even when a long-lived streaming RPC is active and the client
// has not disconnected. Without the fix, GracefulStop() would block indefinitely.
func TestGrpcServerShutdown_WithActiveStream(t *testing.T) {
	gracefulStopTimeout := 200 * time.Millisecond

	rawServer := grpc.NewServer()
	handler := &blockingStreamServer{started: make(chan struct{})}
	rawServer.RegisterService(&blockingStreamServiceDesc, handler)

	signalerCtx := atomic.NewPointer[irrecoverable.SignalerContext](nil)
	server := grpcserver.NewGrpcServer(
		zerolog.Nop(),
		"localhost:0",
		rawServer,
		signalerCtx,
		gracefulStopTimeout,
	)

	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
	server.Start(ctx)
	unittest.RequireComponentsReadyBefore(t, 2*time.Second, server)

	conn, err := grpc.NewClient(
		server.GRPCAddress().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	// Open a stream; do not cancel clientCtx so the stream stays open indefinitely.
	clientCtx, cancelClient := context.WithCancel(context.Background())
	defer cancelClient()
	_, err = conn.NewStream(clientCtx, &grpc.StreamDesc{ServerStreams: true}, "/test.BlockingStream/Stream")
	require.NoError(t, err)

	// Wait until the server-side handler is running.
	unittest.RequireCloseBefore(t, handler.started, 2*time.Second, "stream handler did not start")

	// Trigger node shutdown. The stream is still open on the client side.
	cancel()

	// The server must complete shutdown within gracefulStopTimeout plus a small buffer.
	// Before the fix, this would hang indefinitely because GracefulStop() waits for all
	// active streaming RPCs to finish, and the client never disconnects.
	unittest.RequireComponentsDoneBefore(t, gracefulStopTimeout+500*time.Millisecond, server)
}

// TestGrpcServerShutdown_NoActiveStreams verifies that when no streaming RPCs are active,
// GrpcServer shuts down promptly via GracefulStop without waiting for the timeout.
func TestGrpcServerShutdown_NoActiveStreams(t *testing.T) {
	gracefulStopTimeout := 5 * time.Second

	rawServer := grpc.NewServer()
	rawServer.RegisterService(&blockingStreamServiceDesc, &blockingStreamServer{started: make(chan struct{})})

	signalerCtx := atomic.NewPointer[irrecoverable.SignalerContext](nil)
	server := grpcserver.NewGrpcServer(
		zerolog.Nop(),
		"localhost:0",
		rawServer,
		signalerCtx,
		gracefulStopTimeout,
	)

	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
	server.Start(ctx)
	unittest.RequireComponentsReadyBefore(t, 2*time.Second, server)

	cancel()

	// With no active streams, GracefulStop() completes immediately — well under the 5s timeout.
	unittest.RequireComponentsDoneBefore(t, 500*time.Millisecond, server)
}
