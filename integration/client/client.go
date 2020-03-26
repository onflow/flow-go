// Package client implements a GRPC client to the Flow network API. This
// matches the client exposed by the SDK is intended to be replaced by the SDK
// once its protobuf definitions are up-to-date.
package client

import (
	"context"

	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
)

// Client is a Flow user agent client.
// NOTE: This is a stop gap solution till the flow-go-sdk also starts using the latest access node API
type Client struct {
	rpcClient observation.ObserveServiceClient
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
	txMsg := convert.TransactionToMessage(tx)

	_, err := c.rpcClient.SendTransaction(
		ctx,
		&observation.SendTransactionRequest{Transaction: txMsg},
	)

	return err
}

// ExecuteScript executes a script against the latest sealed world state.
func (c *Client) ExecuteScript(ctx context.Context, script []byte) ([]byte, error) {
	res, err := c.rpcClient.ExecuteScript(ctx, &observation.ExecuteScriptRequest{Script: script})
	if err != nil {
		return nil, err
	}

	return res.GetValue(), nil
}
