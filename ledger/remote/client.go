package remote

import (
	"context"
	"fmt"
	"strings"
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
	conn        *grpc.ClientConn
	client      ledgerpb.LedgerServiceClient
	logger      zerolog.Logger
	done        chan struct{}
	once        sync.Once
	ctx         context.Context
	cancel      context.CancelFunc
	callTimeout time.Duration
}

// clientConfig holds configuration options for the Client.
type clientConfig struct {
	maxRequestSize  uint
	maxResponseSize uint
	callTimeout     time.Duration
}

// defaultClientConfig returns the default configuration.
func defaultClientConfig() *clientConfig {
	return &clientConfig{
		maxRequestSize:  1 << 30, // 1 GiB
		maxResponseSize: 1 << 30, // 1 GiB
		callTimeout:     time.Minute,
	}
}

// ClientOption is a function that configures a Client.
type ClientOption func(*clientConfig)

// WithMaxRequestSize sets the maximum request message size in bytes.
func WithMaxRequestSize(size uint) ClientOption {
	return func(cfg *clientConfig) {
		cfg.maxRequestSize = size
	}
}

// WithMaxResponseSize sets the maximum response message size in bytes.
func WithMaxResponseSize(size uint) ClientOption {
	return func(cfg *clientConfig) {
		cfg.maxResponseSize = size
	}
}

// WithCallTimeout sets the timeout for individual gRPC calls.
func WithCallTimeout(timeout time.Duration) ClientOption {
	return func(cfg *clientConfig) {
		cfg.callTimeout = timeout
	}
}

// NewClient creates a new remote ledger client.
// grpcAddr can be either a TCP address (e.g., "localhost:9000") or a Unix domain socket.
// For Unix sockets, you can use either the full gRPC format (e.g., "unix:///tmp/ledger.sock")
// or just the absolute path (e.g., "/tmp/ledger.sock") - the unix:// prefix will be added automatically.
// Options can be provided to customize the client configuration.
// By default, max request and response sizes are 1 GiB.
func NewClient(grpcAddr string, logger zerolog.Logger, opts ...ClientOption) (*Client, error) {
	logger = logger.With().Str("component", "remote_ledger_client").Logger()

	cfg := defaultClientConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Handle Unix domain socket addresses
	// gRPC client accepts "unix:///absolute/path" or "unix://relative/path" format
	// For convenience, if an absolute path is provided (starts with /), automatically add the unix:// prefix
	if strings.HasPrefix(grpcAddr, "/") {
		grpcAddr = "unix://" + grpcAddr
		logger.Debug().Str("address", grpcAddr).Msg("using Unix domain socket (auto-prefixed)")
	} else if strings.HasPrefix(grpcAddr, "unix://") {
		logger.Debug().Str("address", grpcAddr).Msg("using Unix domain socket")
	}

	// Create gRPC connection with max message size configuration.
	// Default to 1 GiB (instead of standard 4 MiB) to handle large proofs that can exceed 4MB.
	// This was increased to fix "grpc: received message larger than max" errors when generating
	// proofs for blocks with many state changes.
	// Retry connection with exponential backoff until the service becomes available.
	// After approximately 40 minutes of retrying (90 attempts), the client will give up and crash.
	var conn *grpc.ClientConn
	retryDelay := 100 * time.Millisecond
	maxRetryDelay := 30 * time.Second
	maxRetries := 90 // ~40 minutes total wait time with exponential backoff capped at 30s

	for attempt := 0; ; attempt++ {
		var err error
		conn, err = grpc.NewClient(
			grpcAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(
				grpc.MaxCallRecvMsgSize(int(cfg.maxResponseSize)),
				grpc.MaxCallSendMsgSize(int(cfg.maxRequestSize)),
			),
		)
		if err == nil {
			logger.Info().Str("address", grpcAddr).Msg("successfully connected to ledger service")
			break
		}

		if attempt >= maxRetries {
			logger.Fatal().
				Err(err).
				Int("attempts", attempt).
				Str("address", grpcAddr).
				Msg("failed to connect to ledger service after maximum retries, crashing node")
		}

		logger.Warn().
			Err(err).
			Int("attempt", attempt+1).
			Int("max_attempts", maxRetries).
			Dur("retry_delay", retryDelay).
			Time("retry_at", time.Now().Add(retryDelay)).
			Str("address", grpcAddr).
			Msg("failed to connect to ledger service, retrying...")

		time.Sleep(retryDelay)
		// Exponential backoff with max cap
		retryDelay = min(maxRetryDelay, time.Duration(float64(retryDelay)*1.5))
	}

	client := ledgerpb.NewLedgerServiceClient(conn)

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		conn:        conn,
		client:      client,
		logger:      logger,
		done:        make(chan struct{}),
		ctx:         ctx,
		cancel:      cancel,
		callTimeout: cfg.callTimeout,
	}, nil
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// callCtx returns a context for gRPC calls with the configured timeout.
// The context is also cancelled when the client is shut down via Done().
func (c *Client) callCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.ctx, c.callTimeout)
}

// InitialState returns the initial state of the ledger.
func (c *Client) InitialState() ledger.State {
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

// HasState returns true if the given state exists in the ledger.
func (c *Client) HasState(state ledger.State) bool {
	ctx, cancel := c.callCtx()
	defer cancel()
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
	ctx, cancel := c.callCtx()
	defer cancel()
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
	if len(resp.Value.Data) == 0 {
		if resp.Value.IsNil {
			return nil, nil
		}
		return ledger.Value([]byte{}), nil
	}
	// Copy the data to avoid holding reference to the gRPC response buffer
	return ledger.Value(append([]byte{}, resp.Value.Data...)), nil
}

// Get returns values for multiple keys at a specific state.
func (c *Client) Get(query *ledger.Query) ([]ledger.Value, error) {
	ctx, cancel := c.callCtx()
	defer cancel()
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
		if len(protoValue.Data) == 0 {
			if protoValue.IsNil {
				values[i] = nil
			} else {
				values[i] = ledger.Value([]byte{})
			}
		} else {
			// Copy the data to avoid holding reference to the gRPC response buffer
			values[i] = ledger.Value(append([]byte{}, protoValue.Data...))
		}
	}

	return values, nil
}

// Set updates keys with new values at a specific state and returns the new state.
func (c *Client) Set(update *ledger.Update) (ledger.State, *ledger.TrieUpdate, error) {
	// Handle empty updates locally without RPC call.
	// This matches the behavior of the local ledger implementations which return
	// the same state when there are no keys to update.
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

	if resp == nil || resp.NewState == nil {
		return ledger.DummyState, nil, fmt.Errorf("invalid response: missing new state")
	}

	var newState ledger.State
	if len(resp.NewState.Hash) != len(newState) {
		return ledger.DummyState, nil, fmt.Errorf("invalid new state hash length")
	}
	copy(newState[:], resp.NewState.Hash)

	// Decode trie update using centralized decoding function to ensure
	// client and server use the same encoding method
	trieUpdate, err := decodeTrieUpdateFromTransport(resp.TrieUpdate)
	if err != nil {
		return ledger.DummyState, nil, fmt.Errorf("failed to decode trie update: %w", err)
	}

	return newState, trieUpdate, nil
}

// Prove returns proofs for the given keys at a specific state.
func (c *Client) Prove(query *ledger.Query) (ledger.Proof, error) {
	ctx, cancel := c.callCtx()
	defer cancel()
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

	// Copy the proof to avoid holding reference to the gRPC response buffer
	return ledger.Proof(append([]byte{}, resp.Proof...)), nil
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
		// Retry with exponential backoff (delay capped at 30s)
		maxRetries := 30
		retryDelay := 100 * time.Millisecond
		maxRetryDelay := 30 * time.Second

		for i := 0; i < maxRetries; i++ {
			ctx, cancel := c.callCtx()
			_, err := c.client.InitialState(ctx, &emptypb.Empty{})
			cancel()
			if err == nil {
				c.logger.Info().Msg("ledger service ready")
				return
			}

			// Check if the client context was cancelled (shutdown in progress)
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
					Msg("ledger service not ready, retrying...")
				time.Sleep(retryDelay)
				retryDelay = min(time.Duration(float64(retryDelay)*1.5), maxRetryDelay)
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
// This cancels any in-flight gRPC calls and closes the connection.
// The method is idempotent - multiple calls return the same channel.
func (c *Client) Done() <-chan struct{} {
	c.once.Do(func() {
		go func() {
			defer close(c.done)
			// Cancel context first to abort any in-flight calls
			c.cancel()
			if err := c.Close(); err != nil {
				c.logger.Error().Err(err).Msg("error closing gRPC connection")
			}
		}()
	})
	return c.done
}

// StateCount returns the number of states in the ledger.
// This is not supported for remote clients as it requires gRPC methods that are not yet implemented.
func (c *Client) StateCount() int {
	// Remote client doesn't have access to state count without additional gRPC methods
	// Return 0 to indicate no states are available (or unknown)
	// This will cause the health check to fail, which is appropriate
	return 0
}

// StateByIndex returns the state at the given index.
// -1 is the last index.
// This is not supported for remote clients as it requires gRPC methods that are not yet implemented.
func (c *Client) StateByIndex(index int) (ledger.State, error) {
	return ledger.DummyState, fmt.Errorf("StateByIndex is not supported for remote ledger clients")
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
