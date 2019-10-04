package client

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/grpc/services/observe"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/pkg/types/proto"
)

// RPCClient is an RPC client compatible with the Flow Observation API.
type RPCClient observe.ObserveServiceClient

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

	grpcClient := observe.NewObserveServiceClient(conn)

	return &Client{
		rpcClient: grpcClient,
		close:     func() error { return conn.Close() },
	}, nil
}

// NewFromRPCClient initializes a Flow client using a pre-configured gRPC provider.
func NewFromRPCClient(rpcClient RPCClient) *Client {
	return &Client{
		rpcClient: rpcClient,
		close:     func() error { return nil },
	}
}

// Close closes the client connection.
func (c *Client) Close() error {
	return c.close()
}

// SendTransaction submits a transaction to the network.
func (c *Client) SendTransaction(ctx context.Context, tx types.Transaction) error {
	txMsg := proto.TransactionToMessage(tx)

	_, err := c.rpcClient.SendTransaction(
		ctx,
		&observe.SendTransactionRequest{Transaction: txMsg},
	)

	return err
}

// CallScript executes a script against the current world state.
func (c *Client) CallScript(ctx context.Context, script []byte) (interface{}, error) {
	res, err := c.rpcClient.CallScript(ctx, &observe.CallScriptRequest{Script: script})
	if err != nil {
		return nil, err
	}

	// TODO: change to production encoding format
	var value interface{}
	err = json.Unmarshal(res.GetValue(), &value)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// GetTransaction fetches a transaction by hash.
func (c *Client) GetTransaction(ctx context.Context, h crypto.Hash) (*types.Transaction, error) {
	res, err := c.rpcClient.GetTransaction(
		ctx,
		&observe.GetTransactionRequest{Hash: h},
	)
	if err != nil {
		return nil, err
	}

	tx, err := proto.MessageToTransaction(res.GetTransaction())
	if err != nil {
		return nil, err
	}

	return &tx, nil
}

// GetAccount fetches an account by address.
func (c *Client) GetAccount(ctx context.Context, address types.Address) (*types.Account, error) {
	res, err := c.rpcClient.GetAccount(
		ctx,
		&observe.GetAccountRequest{Address: address.Bytes()},
	)
	if err != nil {
		return nil, err
	}

	account, err := proto.MessageToAccount(res.GetAccount())
	if err != nil {
		return nil, err
	}

	return &account, nil
}
