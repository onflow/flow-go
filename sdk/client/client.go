package client

import (
	"context"

	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/proto/services/observation"
	"github.com/dapperlabs/flow-go/sdk/convert"
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

// Ping tests the connection to the Observation API.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.rpcClient.Ping(ctx, &observation.PingRequest{})
	return err
}

// SendTransaction submits a transaction to the network.
func (c *Client) SendTransaction(ctx context.Context, tx flow.Transaction) error {
	txMsg := convert.TransactionToMessage(tx)

	_, err := c.rpcClient.SendTransaction(
		ctx,
		&observation.SendTransactionRequest{Transaction: txMsg},
	)

	return err
}

// GetLatestBlock gets the header of the latest sealed or unsealed block.
func (c *Client) GetLatestBlock(ctx context.Context, isSealed bool) (*flow.BlockHeader, error) {
	res, err := c.rpcClient.GetLatestBlock(
		ctx,
		&observation.GetLatestBlockRequest{IsSealed: isSealed},
	)
	if err != nil {
		return nil, err
	}

	blockHeader := convert.MessageToBlockHeader(res.GetBlock())

	return &blockHeader, nil
}

// ExecuteScript executes a script against the latest sealed world state.
func (c *Client) ExecuteScript(ctx context.Context, script []byte) ([]byte, error) {
	res, err := c.rpcClient.ExecuteScript(ctx, &observation.ExecuteScriptRequest{Script: script})
	if err != nil {
		return nil, err
	}

	return res.GetValue(), nil
}

// GetTransaction fetches a transaction by hash.
func (c *Client) GetTransaction(ctx context.Context, h crypto.Hash) (*flow.Transaction, error) {
	res, err := c.rpcClient.GetTransaction(
		ctx,
		&observation.GetTransactionRequest{Hash: h},
	)
	if err != nil {
		return nil, err
	}

	tx, err := convert.MessageToTransaction(res.GetTransaction())
	if err != nil {
		return nil, err
	}

	eventMessages := res.GetEvents()
	events := make([]flow.Event, len(eventMessages))

	for i, m := range eventMessages {
		events[i] = convert.MessageToEvent(m)
	}

	tx.Events = events

	return &tx, nil
}

// GetAccount fetches an account by address.
func (c *Client) GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error) {
	res, err := c.rpcClient.GetAccount(
		ctx,
		&observation.GetAccountRequest{Address: address.Bytes()},
	)
	if err != nil {
		return nil, err
	}

	account, err := convert.MessageToAccount(res.GetAccount())
	if err != nil {
		return nil, err
	}

	return &account, nil
}

// EventQuery defines a query for Flow events.
type EventQuery struct {
	// The event type to search for. If empty, no filtering by type is done.
	Type string
	// The block to begin looking for events
	StartBlock uint64
	// The block to end looking for events (inclusive)
	EndBlock uint64
}

// GetEvents queries the Observation API for events and returns the results.
func (c *Client) GetEvents(ctx context.Context, query EventQuery) ([]flow.Event, error) {
	req := &observation.GetEventsRequest{
		Type:       query.Type,
		StartBlock: query.StartBlock,
		EndBlock:   query.EndBlock,
	}

	res, err := c.rpcClient.GetEvents(ctx, req)
	if err != nil {
		return nil, err
	}

	eventMessages := res.GetEvents()
	events := make([]flow.Event, len(eventMessages))

	for i, m := range eventMessages {
		events[i] = convert.MessageToEvent(m)
	}

	return events, nil
}
