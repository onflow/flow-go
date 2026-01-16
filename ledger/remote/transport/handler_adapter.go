package transport

import (
	"context"

	"github.com/onflow/flow-go/ledger"
	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
)

// LedgerHandlerAdapter adapts a ledger.Ledger to ServerHandler interface.
type LedgerHandlerAdapter struct {
	ledger ledger.Ledger
}

// NewLedgerHandlerAdapter creates a new adapter.
func NewLedgerHandlerAdapter(l ledger.Ledger) *LedgerHandlerAdapter {
	return &LedgerHandlerAdapter{ledger: l}
}

// InitialState returns the initial state of the ledger.
func (a *LedgerHandlerAdapter) InitialState(ctx context.Context) (*ledgerpb.State, error) {
	state := a.ledger.InitialState()
	return &ledgerpb.State{Hash: state[:]}, nil
}

// HasState checks if the given state exists in the ledger.
func (a *LedgerHandlerAdapter) HasState(ctx context.Context, state *ledgerpb.State) (bool, error) {
	var ledgerState ledger.State
	if len(state.Hash) != len(ledgerState) {
		return false, nil
	}
	copy(ledgerState[:], state.Hash)
	return a.ledger.HasState(ledgerState), nil
}

// GetSingleValue returns a single value for a given key at a specific state.
func (a *LedgerHandlerAdapter) GetSingleValue(ctx context.Context, state *ledgerpb.State, key *ledgerpb.Key) (*ledgerpb.Value, error) {
	var ledgerState ledger.State
	if len(state.Hash) != len(ledgerState) {
		return nil, nil
	}
	copy(ledgerState[:], state.Hash)

	ledgerKey, err := protoKeyToLedgerKey(key)
	if err != nil {
		return nil, err
	}

	query, err := ledger.NewQuerySingleValue(ledgerState, ledgerKey)
	if err != nil {
		return nil, err
	}

	value, err := a.ledger.GetSingleValue(query)
	if err != nil {
		return nil, err
	}

	return &ledgerpb.Value{
		Data:  value,
		IsNil: value == nil,
	}, nil
}

// Get returns values for multiple keys at a specific state.
func (a *LedgerHandlerAdapter) Get(ctx context.Context, state *ledgerpb.State, keys []*ledgerpb.Key) ([]*ledgerpb.Value, error) {
	var ledgerState ledger.State
	if len(state.Hash) != len(ledgerState) {
		return nil, nil
	}
	copy(ledgerState[:], state.Hash)

	ledgerKeys := make([]ledger.Key, len(keys))
	for i, protoKey := range keys {
		ledgerKey, err := protoKeyToLedgerKey(protoKey)
		if err != nil {
			return nil, err
		}
		ledgerKeys[i] = ledgerKey
	}

	query, err := ledger.NewQuery(ledgerState, ledgerKeys)
	if err != nil {
		return nil, err
	}

	values, err := a.ledger.Get(query)
	if err != nil {
		return nil, err
	}

	protoValues := make([]*ledgerpb.Value, len(values))
	for i, v := range values {
		protoValues[i] = &ledgerpb.Value{
			Data:  v,
			IsNil: v == nil,
		}
	}

	return protoValues, nil
}

// Set updates keys with new values at a specific state and returns the new state.
func (a *LedgerHandlerAdapter) Set(ctx context.Context, state *ledgerpb.State, keys []*ledgerpb.Key, values []*ledgerpb.Value) (*ledgerpb.State, []byte, error) {
	var ledgerState ledger.State
	if len(state.Hash) != len(ledgerState) {
		return nil, nil, nil
	}
	copy(ledgerState[:], state.Hash)

	ledgerKeys := make([]ledger.Key, len(keys))
	for i, protoKey := range keys {
		ledgerKey, err := protoKeyToLedgerKey(protoKey)
		if err != nil {
			return nil, nil, err
		}
		ledgerKeys[i] = ledgerKey
	}

	ledgerValues := make([]ledger.Value, len(values))
	for i, protoValue := range values {
		if len(protoValue.Data) == 0 {
			if protoValue.IsNil {
				ledgerValues[i] = nil
			} else {
				ledgerValues[i] = ledger.Value([]byte{})
			}
		} else {
			ledgerValues[i] = ledger.Value(protoValue.Data)
		}
	}

	update, err := ledger.NewUpdate(ledgerState, ledgerKeys, ledgerValues)
	if err != nil {
		return nil, nil, err
	}

	newState, trieUpdate, err := a.ledger.Set(update)
	if err != nil {
		return nil, nil, err
	}

	// Encode trie update using centralized encoding function
	trieUpdateBytes := encodeTrieUpdateForTransport(trieUpdate)

	return &ledgerpb.State{Hash: newState[:]}, trieUpdateBytes, nil
}

// Prove returns proofs for the given keys at a specific state.
func (a *LedgerHandlerAdapter) Prove(ctx context.Context, state *ledgerpb.State, keys []*ledgerpb.Key) ([]byte, error) {
	var ledgerState ledger.State
	if len(state.Hash) != len(ledgerState) {
		return nil, nil
	}
	copy(ledgerState[:], state.Hash)

	ledgerKeys := make([]ledger.Key, len(keys))
	for i, protoKey := range keys {
		ledgerKey, err := protoKeyToLedgerKey(protoKey)
		if err != nil {
			return nil, err
		}
		ledgerKeys[i] = ledgerKey
	}

	query, err := ledger.NewQuery(ledgerState, ledgerKeys)
	if err != nil {
		return nil, err
	}

	proof, err := a.ledger.Prove(query)
	if err != nil {
		return nil, err
	}

	return proof, nil
}

// Helper functions (these should be moved to a shared location)

func protoKeyToLedgerKey(protoKey *ledgerpb.Key) (ledger.Key, error) {
	if protoKey == nil {
		return ledger.Key{}, nil
	}

	keyParts := make([]ledger.KeyPart, len(protoKey.Parts))
	for i, part := range protoKey.Parts {
		if part.Type > 65535 {
			return ledger.Key{}, nil
		}
		keyParts[i] = ledger.NewKeyPart(uint16(part.Type), part.Value)
	}

	return ledger.NewKey(keyParts), nil
}

// encodeTrieUpdateForTransport is a placeholder - should use the one from remote package
func encodeTrieUpdateForTransport(trieUpdate *ledger.TrieUpdate) []byte {
	return ledger.EncodeTrieUpdateCBOR(trieUpdate)
}
