package grpcserver_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/module/grpcserver"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

func serverFixture(t *testing.T, listenAddr string) *grpcserver.GrpcServer {
	return grpcserver.NewGrpcServer(
		unittest.Logger(),
		listenAddr,
		grpc.NewServer(),
		atomic.NewPointer[irrecoverable.SignalerContext](nil),
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
