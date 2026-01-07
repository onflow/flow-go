package remote

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/onflow/flow-go/ledger"
	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
)

// Client implements ledger.Ledger interface using gRPC calls to a remote ledger service.
type Client struct {
	conn   *grpc.ClientConn
	client ledgerpb.LedgerServiceClient
	logger zerolog.Logger
	done   chan struct{}
	once   sync.Once
}

// NewClient creates a new remote ledger client.
func NewClient(grpcAddr string, logger zerolog.Logger) (*Client, error) {
	logger = logger.With().Str("component", "remote_ledger_client").Logger()

	// Create gRPC connection
	conn, err := grpc.NewClient(
		grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ledger service: %w", err)
	}

	client := ledgerpb.NewLedgerServiceClient(conn)

	return &Client{
		conn:   conn,
		client: client,
		logger: logger,
		done:   make(chan struct{}),
	}, nil
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// InitialState returns the initial state of the ledger.
func (c *Client) InitialState() ledger.State {
	ctx := context.Background()
	resp, err := c.client.InitialState(ctx, &emptypb.Empty{})
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to get initial state")
		return ledger.DummyState
	}

	var state ledger.State
	if len(resp.State.Hash) != len(state) {
		c.logger.Error().
			Int("expected", len(state)).
			Int("got", len(resp.State.Hash)).
			Msg("invalid state hash length")
		return ledger.DummyState
	}
	copy(state[:], resp.State.Hash)
	return state
}

// HasState returns true if the given state exists in the ledger.
func (c *Client) HasState(state ledger.State) bool {
	ctx := context.Background()
	req := &ledgerpb.StateRequest{
		State: &ledgerpb.State{
			Hash: state[:],
		},
	}

	resp, err := c.client.HasState(ctx, req)
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to check state")
		return false
	}

	return resp.HasState
}

// GetSingleValue returns a single value for a given key at a specific state.
func (c *Client) GetSingleValue(query *ledger.QuerySingleValue) (ledger.Value, error) {
	ctx := context.Background()
	state := query.State()
	req := &ledgerpb.GetSingleValueRequest{
		State: &ledgerpb.State{
			Hash: state[:],
		},
		Key: ledgerKeyToProtoKey(query.Key()),
	}

	resp, err := c.client.GetSingleValue(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get single value: %w", err)
	}

	// Reconstruct the original value type using is_nil flag
	// This preserves the distinction between nil and []byte{} that protobuf loses
	if resp.Value.Data == nil || len(resp.Value.Data) == 0 {
		if resp.Value.IsNil {
			return nil, nil
		}
		return ledger.Value([]byte{}), nil
	}
	return ledger.Value(resp.Value.Data), nil
}

// Get returns values for multiple keys at a specific state.
func (c *Client) Get(query *ledger.Query) ([]ledger.Value, error) {
	ctx := context.Background()
	state := query.State()
	req := &ledgerpb.GetRequest{
		State: &ledgerpb.State{
			Hash: state[:],
		},
		Keys: make([]*ledgerpb.Key, len(query.Keys())),
	}

	for i, key := range query.Keys() {
		req.Keys[i] = ledgerKeyToProtoKey(key)
	}

	resp, err := c.client.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get values: %w", err)
	}

	values := make([]ledger.Value, len(resp.Values))
	for i, protoValue := range resp.Values {
		// Reconstruct the original value type using is_nil flag
		// This preserves the distinction between nil and []byte{} that protobuf loses
		if protoValue.Data == nil || len(protoValue.Data) == 0 {
			if protoValue.IsNil {
				values[i] = nil
			} else {
				values[i] = ledger.Value([]byte{})
			}
		} else {
			values[i] = ledger.Value(protoValue.Data)
		}
	}

	return values, nil
}

// Set updates keys with new values at a specific state and returns the new state.
func (c *Client) Set(update *ledger.Update) (ledger.State, *ledger.TrieUpdate, error) {
	ctx := context.Background()
	state := update.State()
	req := &ledgerpb.SetRequest{
		State: &ledgerpb.State{
			Hash: state[:],
		},
		Keys:   make([]*ledgerpb.Key, len(update.Keys())),
		Values: make([]*ledgerpb.Value, len(update.Values())),
	}

	for i, key := range update.Keys() {
		req.Keys[i] = ledgerKeyToProtoKey(key)
	}

	for i, value := range update.Values() {
		// Distinguish between nil and []byte{} for protobuf encoding
		// Protobuf cannot distinguish them, so we use is_nil flag
		isNil := value == nil
		req.Values[i] = &ledgerpb.Value{
			Data:  value,
			IsNil: isNil,
		}
	}

	resp, err := c.client.Set(ctx, req)
	if err != nil {
		return ledger.DummyState, nil, fmt.Errorf("failed to set values: %w", err)
	}

	var newState ledger.State
	if len(resp.NewState.Hash) != len(newState) {
		return ledger.DummyState, nil, fmt.Errorf("invalid new state hash length")
	}
	copy(newState[:], resp.NewState.Hash)

	// Decode trie update if present
	trieUpdate, err := ledger.DecodeTrieUpdate(resp.TrieUpdate)

	return newState, trieUpdate, nil
}

// Prove returns proofs for the given keys at a specific state.
func (c *Client) Prove(query *ledger.Query) (ledger.Proof, error) {
	ctx := context.Background()
	state := query.State()
	req := &ledgerpb.ProveRequest{
		State: &ledgerpb.State{
			Hash: state[:],
		},
		Keys: make([]*ledgerpb.Key, len(query.Keys())),
	}

	for i, key := range query.Keys() {
		req.Keys[i] = ledgerKeyToProtoKey(key)
	}

	resp, err := c.client.Prove(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate proof: %w", err)
	}

	return ledger.Proof(resp.Proof), nil
}

// Ready returns a channel that is closed when the client is ready.
// For a remote client, this waits for the ledger service to be ready by
// calling InitialState() with retries to ensure the service has finished initialization.
func (c *Client) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		defer close(ready)
		// Wait for the ledger service to be ready by calling InitialState()
		// This ensures the service has finished WAL replay and is ready to serve requests
		// Retry with exponential backoff for up to 30 seconds
		ctx := context.Background()
		maxRetries := 30
		retryDelay := 100 * time.Millisecond

		for i := 0; i < maxRetries; i++ {
			_, err := c.client.InitialState(ctx, &emptypb.Empty{})
			if err == nil {
				c.logger.Info().Msg("ledger service ready")
				return
			}

			if i < maxRetries-1 {
				c.logger.Debug().
					Err(err).
					Int("attempt", i+1).
					Dur("retry_delay", retryDelay).
					Msg("ledger service not ready, retrying...")
				time.Sleep(retryDelay)
				retryDelay = time.Duration(float64(retryDelay) * 1.5) // exponential backoff
			} else {
				c.logger.Warn().Err(err).Msg("ledger service not ready after retries, proceeding anyway")
				// Still close the channel to avoid blocking forever
				// The execution node will fail later with a more specific error if the service is truly not ready
			}
		}
	}()
	return ready
}

// Done returns a channel that is closed when the client is done.
// This closes the gRPC connection. The method is idempotent - multiple calls
// return the same channel.
func (c *Client) Done() <-chan struct{} {
	c.once.Do(func() {
		go func() {
			defer close(c.done)
			if err := c.Close(); err != nil {
				c.logger.Error().Err(err).Msg("error closing gRPC connection")
			}
		}()
	})
	return c.done
}

// ledgerKeyToProtoKey converts a ledger.Key to a protobuf Key.
func ledgerKeyToProtoKey(key ledger.Key) *ledgerpb.Key {
	parts := make([]*ledgerpb.KeyPart, len(key.KeyParts))
	for i, part := range key.KeyParts {
		parts[i] = &ledgerpb.KeyPart{
			Type:  uint32(part.Type),
			Value: part.Value,
		}
	}
	return &ledgerpb.Key{
		Parts: parts,
	}
}
