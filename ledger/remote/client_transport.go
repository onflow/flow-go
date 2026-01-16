package remote

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/remote/transport"
	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
)

// TransportClient implements ledger.Ledger interface using a transport abstraction.
// This allows switching between gRPC and shmipc-go without changing the client code.
type TransportClient struct {
	transport transport.ClientTransport
	logger    zerolog.Logger
	done      chan struct{}
}

// NewTransportLedgerClient creates a new remote ledger client using the specified transport.
// This returns a ledger.Ledger implementation that uses the transport abstraction.
func NewTransportLedgerClient(
	transportType transport.TransportType,
	addr string,
	logger zerolog.Logger,
	maxRequestSize, maxResponseSize, bufferSize uint,
) (*TransportClient, error) {
	logger = logger.With().Str("component", "remote_ledger_client").Logger()

	clientTransport, err := NewTransportClient(transportType, addr, logger, maxRequestSize, maxResponseSize, bufferSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport client: %w", err)
	}

	return &TransportClient{
		transport: clientTransport,
		logger:    logger,
		done:      make(chan struct{}),
	}, nil
}

// InitialState returns the initial state of the ledger.
func (c *TransportClient) InitialState() ledger.State {
	ctx := context.Background()
	state, err := c.transport.InitialState(ctx)
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to get initial state")
		return ledger.DummyState
	}

	var result ledger.State
	if len(state.Hash) != len(result) {
		c.logger.Error().
			Int("expected", len(result)).
			Int("got", len(state.Hash)).
			Msg("invalid state hash length")
		return ledger.DummyState
	}
	copy(result[:], state.Hash)
	return result
}

// HasState returns true if the given state exists in the ledger.
func (c *TransportClient) HasState(state ledger.State) bool {
	ctx := context.Background()
	protoState := &ledgerpb.State{Hash: state[:]}
	hasState, err := c.transport.HasState(ctx, protoState)
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to check state")
		return false
	}
	return hasState
}

// GetSingleValue returns a single value for a given key at a specific state.
func (c *TransportClient) GetSingleValue(query *ledger.QuerySingleValue) (ledger.Value, error) {
	ctx := context.Background()
	state := query.State()
	protoState := &ledgerpb.State{Hash: state[:]}
	protoKey := ledgerKeyToProtoKey(query.Key())

	protoValue, err := c.transport.GetSingleValue(ctx, protoState, protoKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get single value: %w", err)
	}

	// Reconstruct the original value type using is_nil flag
	if len(protoValue.Data) == 0 {
		if protoValue.IsNil {
			return nil, nil
		}
		return ledger.Value([]byte{}), nil
	}
	return ledger.Value(protoValue.Data), nil
}

// Get returns values for multiple keys at a specific state.
func (c *TransportClient) Get(query *ledger.Query) ([]ledger.Value, error) {
	ctx := context.Background()
	state := query.State()
	protoState := &ledgerpb.State{Hash: state[:]}

	protoKeys := make([]*ledgerpb.Key, len(query.Keys()))
	for i, key := range query.Keys() {
		protoKeys[i] = ledgerKeyToProtoKey(key)
	}

	protoValues, err := c.transport.Get(ctx, protoState, protoKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to get values: %w", err)
	}

	values := make([]ledger.Value, len(protoValues))
	for i, protoValue := range protoValues {
		if len(protoValue.Data) == 0 {
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
func (c *TransportClient) Set(update *ledger.Update) (ledger.State, *ledger.TrieUpdate, error) {
	ctx := context.Background()
	state := update.State()
	protoState := &ledgerpb.State{Hash: state[:]}

	protoKeys := make([]*ledgerpb.Key, len(update.Keys()))
	for i, key := range update.Keys() {
		protoKeys[i] = ledgerKeyToProtoKey(key)
	}

	protoValues := make([]*ledgerpb.Value, len(update.Values()))
	for i, value := range update.Values() {
		isNil := value == nil
		protoValues[i] = &ledgerpb.Value{
			Data:  value,
			IsNil: isNil,
		}
	}

	newProtoState, trieUpdateBytes, err := c.transport.Set(ctx, protoState, protoKeys, protoValues)
	if err != nil {
		return ledger.DummyState, nil, fmt.Errorf("failed to set values: %w", err)
	}

	var newState ledger.State
	if len(newProtoState.Hash) != len(newState) {
		return ledger.DummyState, nil, fmt.Errorf("invalid new state hash length")
	}
	copy(newState[:], newProtoState.Hash)

	trieUpdate, err := decodeTrieUpdateFromTransport(trieUpdateBytes)
	if err != nil {
		return ledger.DummyState, nil, fmt.Errorf("failed to decode trie update: %w", err)
	}

	return newState, trieUpdate, nil
}

// Prove returns proofs for the given keys at a specific state.
func (c *TransportClient) Prove(query *ledger.Query) (ledger.Proof, error) {
	ctx := context.Background()
	state := query.State()
	protoState := &ledgerpb.State{Hash: state[:]}

	protoKeys := make([]*ledgerpb.Key, len(query.Keys()))
	for i, key := range query.Keys() {
		protoKeys[i] = ledgerKeyToProtoKey(key)
	}

	proof, err := c.transport.Prove(ctx, protoState, protoKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to generate proof: %w", err)
	}

	return ledger.Proof(proof), nil
}

// Ready returns a channel that is closed when the client is ready.
func (c *TransportClient) Ready() <-chan struct{} {
	return c.transport.Ready()
}

// Done returns a channel that is closed when the client is done.
func (c *TransportClient) Done() <-chan struct{} {
	return c.done
}

// Close closes the transport connection.
func (c *TransportClient) Close() error {
	close(c.done)
	return c.transport.Close()
}

// StateCount returns the number of states in the ledger.
// This is not supported for remote clients.
func (c *TransportClient) StateCount() int {
	return 0
}

// StateByIndex returns the state at the given index.
// This is not supported for remote clients.
func (c *TransportClient) StateByIndex(index int) (ledger.State, error) {
	return ledger.DummyState, fmt.Errorf("StateByIndex is not supported for remote ledger clients")
}
