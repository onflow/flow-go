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
	// blockDuration will cause `Stream` to block for the set duration
	blockDuration time.Duration
}

var _ blockingStreamService = (*blockingStreamServer)(nil)

func (s *blockingStreamServer) Stream(stream grpc.ServerStream) error {
	close(s.started)
	if s.blockDuration > 0 {
		// this is to simulate the case that after `grpcServer.Stop()` is called,
		// `<-gracefulDone` channel is still blocking, so that we can verify
		// the caller is not waiting for `<-gracefulDone` return before shutdown,
		// otherwise, the waiting might be still blocking for longer or indefinitely.
		time.Sleep(s.blockDuration)
	}
	<-stream.Context().Done()
	return nil
}

func blockingStreamHandler(srv any, stream grpc.ServerStream) error {
	return srv.(blockingStreamService).Stream(stream)
}

// TestGrpcServerShutdown_WithActiveStream verifies that GrpcServer shuts down within
// gracefulStopTimeout even when a long-lived streaming RPC is active and the client
// has not disconnected. Without the fix, GracefulStop() would block indefinitely.
func TestGrpcServerShutdown_WithActiveStream(t *testing.T) {
	gracefulStopTimeout := 200 * time.Millisecond

	rawServer := grpc.NewServer()
	handler := &blockingStreamServer{
		started:       make(chan struct{}),
		blockDuration: gracefulStopTimeout * 10,
	}
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
	clientCtx := t.Context()
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

// TestGrpcServerShutdown_ShutdownStreamInterceptor verifies that when the
// [grpcserver.ShutdownStreamInterceptor] is registered, an active streaming RPC's
// stream.Context() is cancelled as soon as the node's SignalerContext is cancelled.
// This allows GracefulStop to complete cleanly — well under gracefulStopTimeout —
// even when the client has not disconnected, so the force-stop fallback is not needed.
func TestGrpcServerShutdown_ShutdownStreamInterceptor(t *testing.T) {
	// Give the graceful path a generous window so we can prove that the interceptor —
	// not the force-stop fallback — is what unblocks shutdown.
	gracefulStopTimeout := 10 * time.Second

	signalerCtx := atomic.NewPointer[irrecoverable.SignalerContext](nil)
	rawServer := grpc.NewServer(
		grpc.ChainStreamInterceptor(grpcserver.ShutdownStreamInterceptor(signalerCtx)),
	)
	handler := &blockingStreamServer{
		started:       make(chan struct{}),
		blockDuration: time.Second,
	}
	rawServer.RegisterService(&blockingStreamServiceDesc, handler)

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

	// Open a stream and never cancel the client-side context — mimicking a long-lived
	// subscription that a well-behaved client is happy to keep open indefinitely.
	clientCtx := t.Context()
	_, err = conn.NewStream(clientCtx, &grpc.StreamDesc{ServerStreams: true}, "/test.BlockingStream/Stream")
	require.NoError(t, err)

	unittest.RequireCloseBefore(t, handler.started, 2*time.Second, "stream handler did not start")

	// Trigger node shutdown. The interceptor should cancel the stream's context, the
	// handler should return, and GracefulStop should complete immediately.
	cancel()

	// Shutdown must complete well under gracefulStopTimeout; otherwise the force-stop
	// fallback is what unblocked us, not the interceptor.
	unittest.RequireComponentsDoneBefore(t, 2*time.Second, server)
}

// TestGrpcServerShutdown_NoActiveStreams verifies that when no streaming RPCs are active,
// GrpcServer shuts down promptly via GracefulStop without waiting for the timeout.
func TestGrpcServerShutdown_NoActiveStreams(t *testing.T) {
	gracefulStopTimeout := 5 * time.Second

	rawServer := grpc.NewServer()
	rawServer.RegisterService(
		&blockingStreamServiceDesc,
		&blockingStreamServer{
			started:       make(chan struct{}),
			blockDuration: gracefulStopTimeout * 10,
		},
	)

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

func serverFixture(t *testing.T, listenAddr string) *grpcserver.GrpcServer {
	return grpcserver.NewGrpcServer(
		unittest.Logger(),
		listenAddr,
		grpc.NewServer(),
		atomic.NewPointer[irrecoverable.SignalerContext](nil),
		0, // use DefaultGracefulStopTimeout
	)
}

// TestGrpcServer_StartStop verifies a normal server lifecycle: the server starts, becomes
// ready, and shuts down without throwing an irrecoverable error.
func TestGrpcServer_StartStop(t *testing.T) {
	server := serverFixture(t, "localhost:0")

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx) // fails the test on any Throw

	server.Start(signalerCtx)
	unittest.RequireCloseBefore(t, server.Ready(), 5*time.Second, "server did not start on time")
	require.NotNil(t, server.GRPCAddress())

	cancel()
	unittest.RequireCloseBefore(t, server.Done(), 5*time.Second, "server did not stop on time")
}

// TestGrpcServer_ImmediateShutdown is a regression test for the shutdown race between the
// server's two workers: if the shutdown worker completes GracefulStop before the serve worker
// reaches Serve, Serve returns ErrServerStopped. This is a normal shutdown, and must NOT be
// thrown as an irrecoverable error. The mock signaler context fails the test on any Throw;
// repeated immediate shutdowns make the race likely enough to be exercised.
func TestGrpcServer_ImmediateShutdown(t *testing.T) {
	for i := 0; i < 50; i++ {
		server := serverFixture(t, "localhost:0")

		ctx, cancel := context.WithCancel(context.Background())
		signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx) // fails the test on any Throw

		server.Start(signalerCtx)
		// cancel without waiting for the server to become ready, so that the shutdown races
		// the startup
		cancel()
		unittest.RequireCloseBefore(t, server.Done(), 5*time.Second, "server did not stop on time")
	}
}

// TestGrpcServer_ListenErrorThrown verifies that a genuine startup failure (the listen address
// cannot be bound) is still thrown as an irrecoverable error.
func TestGrpcServer_ListenErrorThrown(t *testing.T) {
	server := serverFixture(t, "invalid-listen-address")

	thrown := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalerCtx := irrecoverable.NewMockSignalerContextWithCallback(t, ctx, func(err error) {
		select {
		case thrown <- err:
		default:
		}
	})

	server.Start(signalerCtx)

	unittest.RequireReturnsBefore(t, func() {
		err := <-thrown
		require.Error(t, err)
	}, 5*time.Second, "expected listen error was not thrown")
}
