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
	infoClient  ledgerpb.LedgerInfoServiceClient
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

	conn := dialLedgerServer(grpcAddr, cfg, logger)

	client := ledgerpb.NewLedgerServiceClient(conn)
	infoClient := ledgerpb.NewLedgerInfoServiceClient(conn)

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		conn:        conn,
		client:      client,
		infoClient:  infoClient,
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
//
// Readiness has two phases. First, this client waits for the ledger service
// to finish initialization by calling InitialState() with retries (the
// server may still be replaying its WAL). Second, after the service responds,
// the client calls LedgerInfoService.ServerInfo to verify the server is
// running in FULL mode. A mode mismatch is treated as a configuration error
// and crashes the process via log.Fatal — clients of a payloadless server
// must use [PayloadlessClient], not [Client].
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
				// Mode check is a configuration-correctness gate. If we
				// connected to a payloadless server while expecting full,
				// crash now rather than fail later on every Get call.
				verifyServerMode(c.ctx, c.infoClient, c.callTimeout, ledgerpb.LedgerMode_LEDGER_MODE_FULL, c.logger)
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

// verifyServerMode calls LedgerInfoService.ServerInfo via `infoClient` and
// crashes the process via log.Fatal if the reported mode does not match
// `expected`.
//
// A mode mismatch is a configuration error (the deployment paired a client
// of one mode with a server of the other); it is not retryable and not
// safe to ignore — every subsequent RPC against a wrong-mode server would
// either return gRPC UNIMPLEMENTED or, worse, succeed against a method that
// happens to share its name but returns incompatibly-typed data.
//
// `expected` should be either FULL or PAYLOADLESS; UNSPECIFIED is treated
// as "client is misconfigured" and also crashes.
//
// If the ServerInfo call itself fails, this function logs a warning and
// returns without crashing — the underlying connection may be transiently
// flaky, and the failure mode of the next real RPC will surface a clearer
// error. Crashing on a transport error here would prevent the client from
// ever recovering from a brief network blip during startup.
func verifyServerMode(
	ctx context.Context,
	infoClient ledgerpb.LedgerInfoServiceClient,
	callTimeout time.Duration,
	expected ledgerpb.LedgerMode,
	logger zerolog.Logger,
) {
	infoCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	resp, err := infoClient.ServerInfo(infoCtx, &emptypb.Empty{})
	if err != nil {
		// Transport-level failure; surface as a warning. The real RPCs will
		// hit the same error if it persists.
		logger.Warn().Err(err).Msg("ledger info ServerInfo call failed; skipping mode check")
		return
	}

	if resp.Mode != expected {
		logger.Fatal().
			Str("expected", expected.String()).
			Str("actual", resp.Mode.String()).
			Msg("ledger server mode mismatch: client connected to wrong-mode server")
	}

	logger.Info().Str("mode", resp.Mode.String()).Msg("ledger server mode verified")
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

// dialLedgerServer establishes a gRPC connection to a ledger server with
// retry. `grpcAddr` may be a TCP address (e.g. "localhost:9000") or a Unix
// domain socket (either "unix:///path" or just "/path" — the prefix is
// auto-added for the latter).
//
// Retries with exponential backoff (capped at 30s) for up to ~40 minutes. If
// the server does not become reachable in that window, the process exits
// via log.Fatal.
//
// Used by both [NewClient] and [NewPayloadlessClient]; the two share the
// same dial behavior because the underlying gRPC server is the same in both
// modes — only the registered services differ.
func dialLedgerServer(grpcAddr string, cfg *clientConfig, logger zerolog.Logger) *grpc.ClientConn {
	if strings.HasPrefix(grpcAddr, "/") {
		grpcAddr = "unix://" + grpcAddr
		logger.Debug().Str("address", grpcAddr).Msg("using Unix domain socket (auto-prefixed)")
	} else if strings.HasPrefix(grpcAddr, "unix://") {
		logger.Debug().Str("address", grpcAddr).Msg("using Unix domain socket")
	}

	// Default to 1 GiB (instead of standard 4 MiB) to handle large proofs.
	retryDelay := 100 * time.Millisecond
	maxRetryDelay := 30 * time.Second
	maxRetries := 90 // ~40 minutes total wait

	for attempt := 0; ; attempt++ {
		conn, err := grpc.NewClient(
			grpcAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(
				grpc.MaxCallRecvMsgSize(int(cfg.maxResponseSize)),
				grpc.MaxCallSendMsgSize(int(cfg.maxRequestSize)),
			),
		)
		if err == nil {
			logger.Info().Str("address", grpcAddr).Msg("successfully connected to ledger service")
			return conn
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
		retryDelay = min(maxRetryDelay, time.Duration(float64(retryDelay)*1.5))
	}
}
