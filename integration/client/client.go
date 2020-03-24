package client

import (
	"context"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/ingress"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	"google.golang.org/grpc"
)

// RPCClient is an RPC client compatible with the Flow Observation API.
type RPCClient observation.ObserveServiceClient

// Client is a Flow user agent client.
type Client struct {
	rpcClient RPCClient
	close     func() error
}

// New initializes a Flow client with the default gRPC provider.
//
// An error will be returned if the host is unreachable.
func New(addr string) (*Client, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	grpcClient := observation.NewObserveServiceClient(conn)

	return &Client{
		rpcClient: grpcClient,
		close:     func() error { return conn.Close() },
	}, nil
}

// Close closes the client connection.
func (c *Client) Close() error {
	return c.close()
}

// Ping tests the connection to the Observation API.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.rpcClient.Ping(ctx, &observation.PingRequest{})
	return err
}

// SendTransaction submits a transaction to the network.
func (c *Client) SendTransaction(ctx context.Context, tx flow.TransactionBody) error {
	txMsg := ingress.TransactionToMessage(tx)

	_, err := c.rpcClient.SendTransaction(
		ctx,
		&observation.SendTransactionRequest{Transaction: txMsg},
	)

	return err
}
