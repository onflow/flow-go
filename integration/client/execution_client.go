package client

import (
	"context"

	"google.golang.org/grpc"

	"github.com/onflow/flow/protobuf/go/flow/execution"
)

// ExecutionClient is an execution RPC client.
type ExecutionClient struct {
	rpcClient execution.ExecutionAPIClient
	close     func() error
}

// NewExecutionClient initializes an execution client client with the default gRPC provider.
//
// An error will be returned if the host is unreachable.
func NewExecutionClient(addr string) (*ExecutionClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	grpcClient := execution.NewExecutionAPIClient(conn)

	return &ExecutionClient{
		rpcClient: grpcClient,
		close:     func() error { return conn.Close() },
	}, nil
}

// Close closes the client connection.
func (e *ExecutionClient) Close() error {
	return e.close()
}

// Ping tests the connection to the execution node
func (e *ExecutionClient) Ping(ctx context.Context) error {
	_, err := e.rpcClient.Ping(ctx, &execution.PingRequest{})
	return err
}
