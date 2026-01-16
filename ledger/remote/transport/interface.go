package transport

import (
	"context"
	"fmt"

	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
)

// ClientTransport defines the interface for remote ledger communication.
// This abstraction allows switching between different transport mechanisms
// (gRPC, shmipc-go, etc.) without changing the client implementation.
type ClientTransport interface {
	// InitialState returns the initial state of the ledger.
	InitialState(ctx context.Context) (*ledgerpb.State, error)

	// HasState checks if the given state exists in the ledger.
	HasState(ctx context.Context, state *ledgerpb.State) (bool, error)

	// GetSingleValue returns a single value for a given key at a specific state.
	GetSingleValue(ctx context.Context, state *ledgerpb.State, key *ledgerpb.Key) (*ledgerpb.Value, error)

	// Get returns values for multiple keys at a specific state.
	Get(ctx context.Context, state *ledgerpb.State, keys []*ledgerpb.Key) ([]*ledgerpb.Value, error)

	// Set updates keys with new values at a specific state and returns the new state.
	Set(ctx context.Context, state *ledgerpb.State, keys []*ledgerpb.Key, values []*ledgerpb.Value) (*ledgerpb.State, []byte, error)

	// Prove returns proofs for the given keys at a specific state.
	Prove(ctx context.Context, state *ledgerpb.State, keys []*ledgerpb.Key) ([]byte, error)

	// Close closes the transport connection.
	Close() error

	// Ready returns a channel that is closed when the transport is ready.
	Ready() <-chan struct{}
}

// ServerTransport defines the interface for the server-side transport.
type ServerTransport interface {
	// Serve starts the transport server.
	Serve(handler ServerHandler) error

	// Stop stops the transport server.
	Stop()

	// Ready returns a channel that is closed when the server is ready.
	Ready() <-chan struct{}
}

// ServerHandler handles incoming requests from the transport.
type ServerHandler interface {
	InitialState(ctx context.Context) (*ledgerpb.State, error)
	HasState(ctx context.Context, state *ledgerpb.State) (bool, error)
	GetSingleValue(ctx context.Context, state *ledgerpb.State, key *ledgerpb.Key) (*ledgerpb.Value, error)
	Get(ctx context.Context, state *ledgerpb.State, keys []*ledgerpb.Key) ([]*ledgerpb.Value, error)
	Set(ctx context.Context, state *ledgerpb.State, keys []*ledgerpb.Key, values []*ledgerpb.Value) (*ledgerpb.State, []byte, error)
	Prove(ctx context.Context, state *ledgerpb.State, keys []*ledgerpb.Key) ([]byte, error)
}

// TransportType represents the type of transport to use.
type TransportType string

const (
	TransportTypeGRPC   TransportType = "grpc"
	TransportTypeShmipc TransportType = "shmipc"
)

// NewTransportTypeFromString converts a string to TransportType.
func NewTransportTypeFromString(s string) (TransportType, error) {
	switch s {
	case string(TransportTypeGRPC):
		return TransportTypeGRPC, nil
	case string(TransportTypeShmipc):
		return TransportTypeShmipc, nil
	default:
		return "", fmt.Errorf("invalid transport type: %s", s)
	}
}
