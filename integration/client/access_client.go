// Package client implements a GRPC client to the Flow network API. This
// matches the client exposed by the SDK is intended to be replaced by the SDK
// once its protobuf definitions are up-to-date.
package client

import (
	"context"

	//sdk "github.com/onflow/flow-go-sdk"
	//sdkConvert "github.com/onflow/flow-go-sdk/client/convert"

	"google.golang.org/grpc"

	"github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/dapperlabs/flow-go/engine/common/rpc/convert"
	"github.com/dapperlabs/flow-go/model/flow"
)

// AccessClient is a Flow user agent client.
// NOTE: This is a stop gap solution till the flow-go-sdk also starts using the latest access node API
type AccessClient struct {
	rpcClient access.AccessAPIClient
	close     func() error
}

// NewAccessClient initializes a Flow client with the default gRPC provider.
//
// An error will be returned if the host is unreachable.
func NewAccessClient(addr string) (*AccessClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	grpcClient := access.NewAccessAPIClient(conn)

	return &AccessClient{
		rpcClient: grpcClient,
		close:     func() error { return conn.Close() },
	}, nil
}

// Close closes the client connection.
func (c *AccessClient) Close() error {
	return c.close()
}

// Ping tests the connection to the Observation API.
func (c *AccessClient) Ping(ctx context.Context) error {
	_, err := c.rpcClient.Ping(ctx, &access.PingRequest{})
	return err
}

// SendTransaction submits a transaction to the network.
func (c *AccessClient) SendTransaction(ctx context.Context, tx flow.TransactionBody) error {
	txMsg := convert.TransactionToMessage(tx)

	_, err := c.rpcClient.SendTransaction(
		ctx,
		&access.SendTransactionRequest{Transaction: txMsg},
	)

	return err
}

// ExecuteScript executes a script against the latest sealed world state.
func (c *AccessClient) ExecuteScript(ctx context.Context, script []byte) ([]byte, error) {
	res, err := c.rpcClient.ExecuteScriptAtLatestBlock(ctx, &access.ExecuteScriptAtLatestBlockRequest{Script: script})
	if err != nil {
		return nil, err
	}

	return res.GetValue(), nil
}

func (c *AccessClient) GetEvents(ctx context.Context, typ string) ([]*access.EventsResponse_Result, error) {

	events, err := c.rpcClient.GetEventsForHeightRange(ctx, &access.GetEventsForHeightRangeRequest{
		Type:        typ,
		StartHeight: 0,
		EndHeight:   1000,
	})

	if err != nil {
		return nil, err
	}

	return events.Results, nil
}
