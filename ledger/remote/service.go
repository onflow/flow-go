package remote

import (
	"context"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/onflow/flow-go/ledger"
	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
)

// Service implements the gRPC LedgerService interface
type Service struct {
	ledgerpb.UnimplementedLedgerServiceServer
	ledger ledger.Ledger
	logger zerolog.Logger
}

// NewService creates a new ledger service
func NewService(l ledger.Ledger, logger zerolog.Logger) *Service {
	return &Service{
		ledger: l,
		logger: logger,
	}
}

// InitialState returns the initial state of the ledger
func (s *Service) InitialState(ctx context.Context, req *emptypb.Empty) (*ledgerpb.StateResponse, error) {
	state := s.ledger.InitialState()
	return &ledgerpb.StateResponse{
		State: &ledgerpb.State{
			Hash: state[:],
		},
	}, nil
}

// HasState checks if the given state exists in the ledger
func (s *Service) HasState(ctx context.Context, req *ledgerpb.StateRequest) (*ledgerpb.HasStateResponse, error) {
	if req.State == nil || len(req.State.Hash) != len(ledger.State{}) {
		return nil, status.Error(codes.InvalidArgument, "invalid state")
	}

	var state ledger.State
	copy(state[:], req.State.Hash)

	hasState := s.ledger.HasState(state)
	return &ledgerpb.HasStateResponse{
		HasState: hasState,
	}, nil
}

// GetSingleValue returns a single value for a given key at a specific state
func (s *Service) GetSingleValue(ctx context.Context, req *ledgerpb.GetSingleValueRequest) (*ledgerpb.ValueResponse, error) {
	if req.State == nil || len(req.State.Hash) != len(ledger.State{}) {
		return nil, status.Error(codes.InvalidArgument, "invalid state")
	}

	var state ledger.State
	copy(state[:], req.State.Hash)

	key, err := protoKeyToLedgerKey(req.Key)
	if err != nil {
		return nil, err // protoKeyToLedgerKey already returns status.Error
	}

	query, err := ledger.NewQuerySingleValue(state, key)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	value, err := s.ledger.GetSingleValue(query)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ledgerpb.ValueResponse{
		Value: &ledgerpb.Value{
			Data:  value,
			IsNil: value == nil,
		},
	}, nil
}

// Get returns values for multiple keys at a specific state
func (s *Service) Get(ctx context.Context, req *ledgerpb.GetRequest) (*ledgerpb.GetResponse, error) {
	if req.State == nil || len(req.State.Hash) != len(ledger.State{}) {
		return nil, status.Error(codes.InvalidArgument, "invalid state")
	}

	if len(req.Keys) == 0 {
		return nil, status.Error(codes.InvalidArgument, "keys cannot be empty")
	}

	var state ledger.State
	copy(state[:], req.State.Hash)

	keys := make([]ledger.Key, len(req.Keys))
	for i, protoKey := range req.Keys {
		key, err := protoKeyToLedgerKey(protoKey)
		if err != nil {
			return nil, err // protoKeyToLedgerKey already returns status.Error
		}
		keys[i] = key
	}

	query, err := ledger.NewQuery(state, keys)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	values, err := s.ledger.Get(query)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoValues := make([]*ledgerpb.Value, len(values))
	for i, v := range values {
		protoValues[i] = &ledgerpb.Value{
			Data:  v,
			IsNil: v == nil,
		}
	}

	return &ledgerpb.GetResponse{
		Values: protoValues,
	}, nil
}

// Set updates keys with new values at a specific state and returns the new state
func (s *Service) Set(ctx context.Context, req *ledgerpb.SetRequest) (*ledgerpb.SetResponse, error) {
	if req.State == nil || len(req.State.Hash) != len(ledger.State{}) {
		return nil, status.Error(codes.InvalidArgument, "invalid state")
	}

	if len(req.Keys) == 0 {
		return nil, status.Error(codes.InvalidArgument, "keys cannot be empty")
	}

	if len(req.Keys) != len(req.Values) {
		return nil, status.Error(codes.InvalidArgument, "keys and values length mismatch")
	}

	var state ledger.State
	copy(state[:], req.State.Hash)

	keys := make([]ledger.Key, len(req.Keys))
	for i, protoKey := range req.Keys {
		key, err := protoKeyToLedgerKey(protoKey)
		if err != nil {
			return nil, err // protoKeyToLedgerKey already returns status.Error
		}
		keys[i] = key
	}

	values := make([]ledger.Value, len(req.Values))
	for i, protoValue := range req.Values {
		var value ledger.Value
		// Reconstruct the original value type using is_nil flag
		// This preserves the distinction between nil and []byte{} that protobuf loses
		if len(protoValue.Data) == 0 {
			if protoValue.IsNil {
				// Original value was nil
				value = nil
			} else {
				// Original value was []byte{} (empty slice)
				value = ledger.Value([]byte{})
			}
		} else {
			// Non-empty value, use data as-is
			value = ledger.Value(protoValue.Data)
		}
		values[i] = value
	}

	update, err := ledger.NewUpdate(state, keys, values)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	newState, trieUpdate, err := s.ledger.Set(update)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Encode trie update using centralized encoding function to ensure
	// client and server use the same encoding method
	trieUpdateBytes := encodeTrieUpdateForTransport(trieUpdate)

	return &ledgerpb.SetResponse{
		NewState: &ledgerpb.State{
			Hash: newState[:],
		},
		TrieUpdate: trieUpdateBytes,
	}, nil
}

// Prove returns proofs for the given keys at a specific state
func (s *Service) Prove(ctx context.Context, req *ledgerpb.ProveRequest) (*ledgerpb.ProofResponse, error) {
	if req.State == nil || len(req.State.Hash) != len(ledger.State{}) {
		return nil, status.Error(codes.InvalidArgument, "invalid state")
	}

	if len(req.Keys) == 0 {
		return nil, status.Error(codes.InvalidArgument, "keys cannot be empty")
	}

	var state ledger.State
	copy(state[:], req.State.Hash)

	keys := make([]ledger.Key, len(req.Keys))
	for i, protoKey := range req.Keys {
		key, err := protoKeyToLedgerKey(protoKey)
		if err != nil {
			return nil, err // protoKeyToLedgerKey already returns status.Error
		}
		keys[i] = key
	}

	query, err := ledger.NewQuery(state, keys)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	proof, err := s.ledger.Prove(query)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ledgerpb.ProofResponse{
		Proof: proof,
	}, nil
}

// protoKeyToLedgerKey converts a protobuf Key to a ledger.Key
func protoKeyToLedgerKey(protoKey *ledgerpb.Key) (ledger.Key, error) {
	if protoKey == nil {
		return ledger.Key{}, status.Error(codes.InvalidArgument, "key is nil")
	}

	keyParts := make([]ledger.KeyPart, len(protoKey.Parts))
	for i, part := range protoKey.Parts {
		if part.Type > 65535 {
			return ledger.Key{}, status.Error(codes.InvalidArgument, "key part type exceeds uint16")
		}
		keyParts[i] = ledger.NewKeyPart(uint16(part.Type), part.Value)
	}

	return ledger.NewKey(keyParts), nil
}
