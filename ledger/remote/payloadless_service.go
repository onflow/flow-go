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

// PayloadlessService implements the gRPC PayloadlessLedgerService interface
// on top of a [ledger.PayloadlessLedger]. Reads return leaf hashes rather
// than payload values.
//
// A ledger gRPC server registers either [Service] (full mode) or
// [PayloadlessService] (payloadless mode), never both. The mode is chosen at
// startup config.
type PayloadlessService struct {
	ledgerpb.UnimplementedPayloadlessLedgerServiceServer
	ledger ledger.PayloadlessLedger
	logger zerolog.Logger
}

// NewPayloadlessService creates a new payloadless ledger gRPC service.
// In production the ledger argument is a *complete.PayloadlessLedger; tests
// may pass any value that satisfies [ledger.PayloadlessLedger].
func NewPayloadlessService(l ledger.PayloadlessLedger, logger zerolog.Logger) *PayloadlessService {
	return &PayloadlessService{
		ledger: l,
		logger: logger,
	}
}

// InitialState returns the initial state of the payloadless ledger.
//
// No error returns are expected during normal operation.
func (s *PayloadlessService) InitialState(_ context.Context, _ *emptypb.Empty) (*ledgerpb.StateResponse, error) {
	state := s.ledger.InitialState()
	return &ledgerpb.StateResponse{
		State: &ledgerpb.State{Hash: state[:]},
	}, nil
}

// HasState checks if the given state exists in the payloadless ledger.
//
// Expected error returns during normal operation:
//   - gRPC InvalidArgument: when `req.State` is nil or has the wrong length.
func (s *PayloadlessService) HasState(_ context.Context, req *ledgerpb.StateRequest) (*ledgerpb.HasStateResponse, error) {
	if req.State == nil || len(req.State.Hash) != len(ledger.State{}) {
		return nil, status.Error(codes.InvalidArgument, "invalid state")
	}
	var state ledger.State
	copy(state[:], req.State.Hash)
	return &ledgerpb.HasStateResponse{HasState: s.ledger.HasState(state)}, nil
}

// HasPaths reports, for each key in `req.Keys`, whether the key has an
// allocated register at `req.State`.
//
// Expected error returns during normal operation:
//   - gRPC InvalidArgument: when `req.State` is nil or has the wrong length,
//     or when `req.Keys` is empty.
func (s *PayloadlessService) HasPaths(_ context.Context, req *ledgerpb.GetRequest) (*ledgerpb.HasPathsResponse, error) {
	if req.State == nil || len(req.State.Hash) != len(ledger.State{}) {
		return nil, status.Error(codes.InvalidArgument, "invalid state")
	}
	if len(req.Keys) == 0 {
		return nil, status.Error(codes.InvalidArgument, "keys cannot be empty")
	}

	var state ledger.State
	copy(state[:], req.State.Hash)

	keys, err := protoKeysToLedgerKeys(req.Keys)
	if err != nil {
		return nil, err
	}

	query, err := ledger.NewQuery(state, keys)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	exists, err := s.ledger.HasPaths(query)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ledgerpb.HasPathsResponse{Exists: exists}, nil
}

// GetSingleLeafHash returns the leaf hash for a single key. An unallocated
// register is reported as an empty `hash` field.
//
// Expected error returns during normal operation:
//   - gRPC InvalidArgument: when `req.State` is nil or has the wrong length,
//     or when `req.Key` is nil.
func (s *PayloadlessService) GetSingleLeafHash(_ context.Context, req *ledgerpb.GetSingleValueRequest) (*ledgerpb.LeafHashResponse, error) {
	if req.State == nil || len(req.State.Hash) != len(ledger.State{}) {
		return nil, status.Error(codes.InvalidArgument, "invalid state")
	}

	var state ledger.State
	copy(state[:], req.State.Hash)

	key, err := protoKeyToLedgerKey(req.Key)
	if err != nil {
		return nil, err
	}

	query, err := ledger.NewQuerySingleValue(state, key)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	leafHash, err := s.ledger.GetSingleLeafHash(query)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &ledgerpb.LeafHashResponse{LeafHash: &ledgerpb.LeafHash{}}
	if leafHash != nil {
		resp.LeafHash.Hash = leafHash[:]
	}
	return resp, nil
}

// GetLeafHashes returns leaf hashes for multiple keys. Unallocated registers
// are reported as `LeafHash` entries with empty `hash` fields.
//
// Expected error returns during normal operation:
//   - gRPC InvalidArgument: when `req.State` is nil or has the wrong length,
//     or when `req.Keys` is empty.
func (s *PayloadlessService) GetLeafHashes(_ context.Context, req *ledgerpb.GetRequest) (*ledgerpb.LeafHashesResponse, error) {
	if req.State == nil || len(req.State.Hash) != len(ledger.State{}) {
		return nil, status.Error(codes.InvalidArgument, "invalid state")
	}
	if len(req.Keys) == 0 {
		return nil, status.Error(codes.InvalidArgument, "keys cannot be empty")
	}

	var state ledger.State
	copy(state[:], req.State.Hash)

	keys, err := protoKeysToLedgerKeys(req.Keys)
	if err != nil {
		return nil, err
	}

	query, err := ledger.NewQuery(state, keys)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	leafHashes, err := s.ledger.GetLeafHashes(query)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoHashes := make([]*ledgerpb.LeafHash, len(leafHashes))
	for i, lh := range leafHashes {
		entry := &ledgerpb.LeafHash{}
		if lh != nil {
			entry.Hash = lh[:]
		}
		protoHashes[i] = entry
	}

	return &ledgerpb.LeafHashesResponse{LeafHashes: protoHashes}, nil
}

// Set updates keys with new values at a specific state and returns the new
// state. The server discards the keys after hashing; only the values
// contribute to the trie.
//
// Expected error returns during normal operation:
//   - gRPC InvalidArgument: when `req.State` is nil/wrong length, keys are
//     empty, or keys/values lengths mismatch.
func (s *PayloadlessService) Set(_ context.Context, req *ledgerpb.SetRequest) (*ledgerpb.SetResponse, error) {
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

	keys, err := protoKeysToLedgerKeys(req.Keys)
	if err != nil {
		return nil, err
	}

	values := make([]ledger.Value, len(req.Values))
	for i, protoValue := range req.Values {
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

	update, err := ledger.NewUpdate(state, keys, values)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	newState, trieUpdate, err := s.ledger.Set(update)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	trieUpdateBytes := encodeTrieUpdateForTransport(trieUpdate)

	return &ledgerpb.SetResponse{
		NewState:   &ledgerpb.State{Hash: newState[:]},
		TrieUpdate: trieUpdateBytes,
	}, nil
}

// Prove returns a payloadless batch proof for the given keys at a specific
// state. The proof is encoded with [ledger.EncodePayloadlessTrieBatchProof].
//
// Expected error returns during normal operation:
//   - gRPC InvalidArgument: when `req.State` is nil/wrong length or `req.Keys`
//     is empty.
func (s *PayloadlessService) Prove(_ context.Context, req *ledgerpb.ProveRequest) (*ledgerpb.ProofResponse, error) {
	if req.State == nil || len(req.State.Hash) != len(ledger.State{}) {
		return nil, status.Error(codes.InvalidArgument, "invalid state")
	}
	if len(req.Keys) == 0 {
		return nil, status.Error(codes.InvalidArgument, "keys cannot be empty")
	}

	var state ledger.State
	copy(state[:], req.State.Hash)

	keys, err := protoKeysToLedgerKeys(req.Keys)
	if err != nil {
		return nil, err
	}

	query, err := ledger.NewQuery(state, keys)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	batchProof, err := s.ledger.Prove(query)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ledgerpb.ProofResponse{
		Proof: ledger.EncodePayloadlessTrieBatchProof(batchProof),
	}, nil
}

// protoKeysToLedgerKeys decodes a slice of proto keys via protoKeyToLedgerKey.
// Returns the first conversion error (already wrapped as a gRPC status).
func protoKeysToLedgerKeys(protoKeys []*ledgerpb.Key) ([]ledger.Key, error) {
	keys := make([]ledger.Key, len(protoKeys))
	for i, pk := range protoKeys {
		k, err := protoKeyToLedgerKey(pk)
		if err != nil {
			return nil, err
		}
		keys[i] = k
	}
	return keys, nil
}
