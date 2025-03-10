package grpcutils_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/onflow/crypto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/onflow/flow-go/utils/grpcutils"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestTLSConnection tests a simple gRPC connection using our default client and server TLS configs
func TestTLSConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	serverCreds, serverCert := generateServerCredentials(t)
	clientCreds := generateClientCredentials(t, serverCert)

	serverURL, serverDone := runServer(t, ctx, serverCreds)

	conn, err := grpc.NewClient(serverURL, grpc.WithTransportCredentials(clientCreds))
	require.NoError(t, err)
	defer conn.Close()

	healthClient := grpc_health_v1.NewHealthClient(conn)

	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)

	require.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.GetStatus())

	cancel()
	unittest.RequireCloseBefore(t, serverDone, time.Second, "server did not stop on time")
}

func generateServerCredentials(t *testing.T) (credentials.TransportCredentials, crypto.PublicKey) {
	netPriv := unittest.NetworkingPrivKeyFixture()

	x509Certificate, err := grpcutils.X509Certificate(netPriv)
	require.NoError(t, err)

	tlsConfig := grpcutils.DefaultServerTLSConfig(x509Certificate)
	return credentials.NewTLS(tlsConfig), netPriv.PublicKey()
}

func generateClientCredentials(t *testing.T, serverCert crypto.PublicKey) credentials.TransportCredentials {
	clientTLSConfig, err := grpcutils.DefaultClientTLSConfig(serverCert)
	require.NoError(t, err)

	return credentials.NewTLS(clientTLSConfig)
}

func runServer(t *testing.T, ctx context.Context, creds credentials.TransportCredentials) (string, <-chan struct{}) {
	grpcServer := grpc.NewServer(grpc.Creds(creds))

	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	lis, err := net.Listen("tcp", "127.0.0.1:0") // OS-assigned free port
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		err := grpcServer.Serve(lis)
		require.NoError(t, err)
	}()

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	return lis.Addr().String(), done
}
