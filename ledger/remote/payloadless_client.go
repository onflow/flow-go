package remote

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
)

// PayloadlessClient is a gRPC client for a payloadless ledger server. Reads
// return leaf hashes (HashLeaf(path, value)) rather than payload values.
//
// PayloadlessClient mirrors [Client] in dial / retry / lifecycle behavior;
// the difference is the registered service it talks to and the leaf-hash
// return types on the read methods. Both clients share the same dial helper
// and the same mode-discovery check in Ready().
type PayloadlessClient struct {
	conn        *grpc.ClientConn
	client      ledgerpb.PayloadlessLedgerServiceClient
	infoClient  ledgerpb.LedgerInfoServiceClient
	logger      zerolog.Logger
	done        chan struct{}
	once        sync.Once
	ctx         context.Context
	cancel      context.CancelFunc
	callTimeout time.Duration
}

// NewPayloadlessClient creates a new payloadless remote ledger client.
//
// `grpcAddr` accepts the same forms as [NewClient]. Connection-establishment
// retries are identical: ~40 minutes of exponential backoff before
// log.Fatal. Mode verification happens in [Ready] (not here) — a wrong-mode
// server will be detected the first time the caller waits on Ready().
func NewPayloadlessClient(grpcAddr string, logger zerolog.Logger, opts ...ClientOption) (*PayloadlessClient, error) {
	logger = logger.With().Str("component", "remote_payloadless_ledger_client").Logger()

	cfg := defaultClientConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	conn := dialLedgerServer(grpcAddr, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())

	return &PayloadlessClient{
		conn:        conn,
		client:      ledgerpb.NewPayloadlessLedgerServiceClient(conn),
		infoClient:  ledgerpb.NewLedgerInfoServiceClient(conn),
		logger:      logger,
		done:        make(chan struct{}),
		ctx:         ctx,
		cancel:      cancel,
		callTimeout: cfg.callTimeout,
	}, nil
}

// Close closes the gRPC connection.
func (c *PayloadlessClient) Close() error {
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// callCtx returns a context for gRPC calls with the configured timeout,
// derived from the client's lifecycle context so cancellations propagate.
func (c *PayloadlessClient) callCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.ctx, c.callTimeout)
}

// InitialState returns the initial state of the payloadless ledger.
func (c *PayloadlessClient) InitialState() ledger.State {
	ctx, cancel := c.callCtx()
	defer cancel()
	resp, err := c.client.InitialState(ctx, &emptypb.Empty{})
	if err != nil {
		c.logger.Fatal().Err(err).Msg("failed to get initial state")
		return ledger.DummyState
	}

	var state ledger.State
	if len(resp.State.Hash) != len(state) {
		c.logger.Fatal().
			Int("expected", len(state)).
			Int("got", len(resp.State.Hash)).
			Msg("invalid state hash length")
		return ledger.DummyState
	}
	copy(state[:], resp.State.Hash)
	return state
}

// HasState returns true if the given state exists in the payloadless ledger.
func (c *PayloadlessClient) HasState(state ledger.State) bool {
	ctx, cancel := c.callCtx()
	defer cancel()
	req := &ledgerpb.StateRequest{State: &ledgerpb.State{Hash: state[:]}}

	resp, err := c.client.HasState(ctx, req)
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to check state")
		return false
	}
	return resp.HasState
}

// HasPaths reports, for each key in `query.Keys()`, whether the key has an
// allocated register at `query.State()`.
//
// Expected error returns during normal operation:
//   - generic error wrapping the underlying gRPC failure when the call fails.
func (c *PayloadlessClient) HasPaths(query *ledger.Query) ([]bool, error) {
	ctx, cancel := c.callCtx()
	defer cancel()
	state := query.State()
	req := &ledgerpb.GetRequest{
		State: &ledgerpb.State{Hash: state[:]},
		Keys:  make([]*ledgerpb.Key, len(query.Keys())),
	}
	for i, key := range query.Keys() {
		req.Keys[i] = ledgerKeyToProtoKey(key)
	}

	resp, err := c.client.HasPaths(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to check has paths: %w", err)
	}
	return resp.Exists, nil
}

// GetSingleLeafHash returns the leaf hash for a single key at a specific
// state. Returns nil if the key has no allocated register.
//
// Expected error returns during normal operation:
//   - generic error wrapping the underlying gRPC failure when the call fails.
func (c *PayloadlessClient) GetSingleLeafHash(query *ledger.QuerySingleValue) (*hash.Hash, error) {
	ctx, cancel := c.callCtx()
	defer cancel()
	state := query.State()
	req := &ledgerpb.GetSingleValueRequest{
		State: &ledgerpb.State{Hash: state[:]},
		Key:   ledgerKeyToProtoKey(query.Key()),
	}

	resp, err := c.client.GetSingleLeafHash(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get single leaf hash: %w", err)
	}

	return decodeProtoLeafHash(resp.LeafHash)
}

// GetLeafHashes returns leaf hashes for multiple keys at a specific state.
// A nil entry in the returned slice indicates an unallocated register.
//
// Expected error returns during normal operation:
//   - generic error wrapping the underlying gRPC failure when the call fails.
func (c *PayloadlessClient) GetLeafHashes(query *ledger.Query) ([]*hash.Hash, error) {
	ctx, cancel := c.callCtx()
	defer cancel()
	state := query.State()
	req := &ledgerpb.GetRequest{
		State: &ledgerpb.State{Hash: state[:]},
		Keys:  make([]*ledgerpb.Key, len(query.Keys())),
	}
	for i, key := range query.Keys() {
		req.Keys[i] = ledgerKeyToProtoKey(key)
	}

	resp, err := c.client.GetLeafHashes(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get leaf hashes: %w", err)
	}

	leafHashes := make([]*hash.Hash, len(resp.LeafHashes))
	for i, protoLH := range resp.LeafHashes {
		lh, err := decodeProtoLeafHash(protoLH)
		if err != nil {
			return nil, fmt.Errorf("failed to decode leaf hash at index %d: %w", i, err)
		}
		leafHashes[i] = lh
	}
	return leafHashes, nil
}

// Set updates keys with new values at a specific state and returns the new
// state plus the trie update that was applied.
//
// Expected error returns during normal operation:
//   - generic error wrapping the underlying gRPC failure when the call fails,
//     or when the response is malformed.
func (c *PayloadlessClient) Set(update *ledger.Update) (ledger.State, *ledger.TrieUpdate, error) {
	// Empty updates short-circuit, matching the behavior of the local ledger.
	if update.Size() == 0 {
		return update.State(),
			&ledger.TrieUpdate{
				RootHash: ledger.RootHash(update.State()),
				Paths:    []ledger.Path{},
				Payloads: []*ledger.Payload{},
			},
			nil
	}

	ctx, cancel := c.callCtx()
	defer cancel()
	state := update.State()
	req := &ledgerpb.SetRequest{
		State:  &ledgerpb.State{Hash: state[:]},
		Keys:   make([]*ledgerpb.Key, len(update.Keys())),
		Values: make([]*ledgerpb.Value, len(update.Values())),
	}

	for i, key := range update.Keys() {
		req.Keys[i] = ledgerKeyToProtoKey(key)
	}
	for i, value := range update.Values() {
		req.Values[i] = &ledgerpb.Value{
			Data:  value,
			IsNil: value == nil,
		}
	}

	resp, err := c.client.Set(ctx, req)
	if err != nil {
		return ledger.DummyState, nil, fmt.Errorf("failed to set values: %w", err)
	}

	if resp == nil || resp.NewState == nil {
		return ledger.DummyState, nil, fmt.Errorf("invalid response: missing new state")
	}

	var newState ledger.State
	if len(resp.NewState.Hash) != len(newState) {
		return ledger.DummyState, nil, fmt.Errorf("invalid new state hash length")
	}
	copy(newState[:], resp.NewState.Hash)

	trieUpdate, err := decodeTrieUpdateFromTransport(resp.TrieUpdate)
	if err != nil {
		return ledger.DummyState, nil, fmt.Errorf("failed to decode trie update: %w", err)
	}

	return newState, trieUpdate, nil
}

// Prove returns a payloadless batch proof for the given keys at a specific
// state. The proof is decoded with [ledger.DecodePayloadlessTrieBatchProof].
//
// Expected error returns during normal operation:
//   - generic error wrapping the underlying gRPC failure or a decode failure.
func (c *PayloadlessClient) Prove(query *ledger.Query) (*ledger.PayloadlessTrieBatchProof, error) {
	ctx, cancel := c.callCtx()
	defer cancel()
	state := query.State()
	req := &ledgerpb.ProveRequest{
		State: &ledgerpb.State{Hash: state[:]},
		Keys:  make([]*ledgerpb.Key, len(query.Keys())),
	}
	for i, key := range query.Keys() {
		req.Keys[i] = ledgerKeyToProtoKey(key)
	}

	resp, err := c.client.Prove(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate proof: %w", err)
	}

	bp, err := ledger.DecodePayloadlessTrieBatchProof(resp.Proof)
	if err != nil {
		return nil, fmt.Errorf("failed to decode payloadless batch proof: %w", err)
	}
	return bp, nil
}

// Ready returns a channel that is closed when the client is ready.
//
// Readiness has two phases. First, the client waits for the ledger service
// to finish initialization by calling InitialState() with retries. Second,
// it calls LedgerInfoService.ServerInfo to verify the server is running in
// PAYLOADLESS mode. A mode mismatch is treated as a configuration error and
// crashes the process via log.Fatal — clients of a full server must use
// [Client], not [PayloadlessClient].
func (c *PayloadlessClient) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		defer close(ready)
		maxRetries := 30
		retryDelay := 100 * time.Millisecond
		maxRetryDelay := 30 * time.Second

		for i := 0; i < maxRetries; i++ {
			ctx, cancel := c.callCtx()
			_, err := c.client.InitialState(ctx, &emptypb.Empty{})
			cancel()
			if err == nil {
				c.logger.Info().Msg("payloadless ledger service ready")
				verifyServerMode(c.ctx, c.infoClient, c.callTimeout, ledgerpb.LedgerMode_LEDGER_MODE_PAYLOADLESS, c.logger)
				return
			}

			if c.ctx.Err() != nil {
				c.logger.Info().Msg("client shutdown during ready check")
				return
			}

			if i < maxRetries-1 {
				c.logger.Warn().
					Err(err).
					Int("attempt", i+1).
					Dur("retry_delay", retryDelay).
					Time("retry_at", time.Now().Add(retryDelay)).
					Msg("payloadless ledger service not ready, retrying...")
				time.Sleep(retryDelay)
				retryDelay = min(time.Duration(float64(retryDelay)*1.5), maxRetryDelay)
			} else {
				c.logger.Warn().Err(err).Msg("payloadless ledger service not ready after retries, proceeding anyway")
			}
		}
	}()
	return ready
}

// Done returns a channel that is closed when the client is done. Idempotent.
func (c *PayloadlessClient) Done() <-chan struct{} {
	c.once.Do(func() {
		go func() {
			defer close(c.done)
			c.cancel()
			if err := c.Close(); err != nil {
				c.logger.Error().Err(err).Msg("error closing gRPC connection")
			}
		}()
	})
	return c.done
}

// decodeProtoLeafHash converts a proto LeafHash to a *hash.Hash. An empty
// `hash` field (length 0) represents an unallocated register and returns nil.
//
// Expected error returns during normal operation:
//   - generic error when the hash field has an unexpected (non-zero, non-HashLen) length.
func decodeProtoLeafHash(protoLH *ledgerpb.LeafHash) (*hash.Hash, error) {
	if protoLH == nil || len(protoLH.Hash) == 0 {
		return nil, nil
	}
	if len(protoLH.Hash) != hash.HashLen {
		return nil, fmt.Errorf("invalid leaf hash length: got %d, want %d", len(protoLH.Hash), hash.HashLen)
	}
	var h hash.Hash
	copy(h[:], protoLH.Hash)
	return &h, nil
}
