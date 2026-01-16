package shmipc

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/cloudwego/shmipc-go"
	"github.com/golang/protobuf/proto"
	"github.com/rs/zerolog"

	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
)

// Client implements transport.ClientTransport using shmipc-go.
type Client struct {
	sessionManager *shmipc.SessionManager
	logger         zerolog.Logger
	ready          chan struct{}
	once           sync.Once
}

// NewClient creates a new shmipc-go transport client.
// addr can be a Unix domain socket path (e.g., "/tmp/ledger.sock") or TCP address (e.g., "localhost:9000").
// bufferSize specifies the shared memory buffer size in bytes (default: 1 GiB).
func NewClient(addr string, logger zerolog.Logger, bufferSize uint) (*Client, error) {
	logger = logger.With().Str("component", "shmipc_ledger_client").Logger()

	if bufferSize == 0 {
		bufferSize = 2 << 30 // 2 GiB default (matches Docker shm_size)
	}

	// Parse address to determine network type
	var network string
	var address string
	if len(addr) > 7 && addr[:7] == "unix://" {
		network = "unix"
		address = addr[7:]
	} else if len(addr) > 6 && addr[:6] == "tcp://" {
		network = "tcp"
		address = addr[6:]
	} else {
		// Default to unix if no prefix
		network = "unix"
		address = addr
	}

	// Configure session manager
	config := shmipc.DefaultSessionManagerConfig()
	config.Address = address
	config.Network = network
	config.SessionNum = 1
	config.InitializeTimeout = 30 * time.Second

	// Configure shared memory buffer capacity (in bytes, uint32)
	// Note: ShareMemoryBufferCap is per-session, so we set it based on bufferSize
	if bufferSize > 1<<32-1 {
		bufferSize = 1<<32 - 1 // Max uint32
	}
	config.ShareMemoryBufferCap = uint32(bufferSize)

	// Retry connection with exponential backoff
	var sessionManager *shmipc.SessionManager
	retryDelay := 100 * time.Millisecond
	maxRetryDelay := 30 * time.Second

	for {
		var err error
		sessionManager, err = shmipc.NewSessionManager(config)
		if err == nil {
			logger.Info().
				Str("address", addr).
				Str("network", network).
				Uint("buffer_size", bufferSize).
				Msg("successfully connected to ledger service via shmipc")
			break
		}

		logger.Debug().
			Err(err).
			Dur("retry_delay", retryDelay).
			Str("address", addr).
			Msg("failed to connect to ledger service via shmipc, retrying...")

		time.Sleep(retryDelay)
		retryDelay = time.Duration(float64(retryDelay) * 1.5)
		if retryDelay > maxRetryDelay {
			retryDelay = maxRetryDelay
		}
	}

	return &Client{
		sessionManager: sessionManager,
		logger:         logger,
		ready:          make(chan struct{}),
	}, nil
}

// InitialState returns the initial state of the ledger.
func (c *Client) InitialState(ctx context.Context) (*ledgerpb.State, error) {
	req := &ledgerpb.StateRequest{} // Empty request for InitialState
	resp, err := c.sendRequest(ctx, msgTypeInitialState, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get initial state: %w", err)
	}

	stateResp := &ledgerpb.StateResponse{}
	if err := proto.Unmarshal(resp, stateResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state response: %w", err)
	}

	return stateResp.State, nil
}

// HasState checks if the given state exists in the ledger.
func (c *Client) HasState(ctx context.Context, state *ledgerpb.State) (bool, error) {
	req := &ledgerpb.StateRequest{State: state}
	resp, err := c.sendRequest(ctx, msgTypeHasState, req)
	if err != nil {
		return false, fmt.Errorf("failed to check state: %w", err)
	}

	hasStateResp := &ledgerpb.HasStateResponse{}
	if err := proto.Unmarshal(resp, hasStateResp); err != nil {
		return false, fmt.Errorf("failed to unmarshal has state response: %w", err)
	}

	return hasStateResp.HasState, nil
}

// GetSingleValue returns a single value for a given key at a specific state.
func (c *Client) GetSingleValue(ctx context.Context, state *ledgerpb.State, key *ledgerpb.Key) (*ledgerpb.Value, error) {
	req := &ledgerpb.GetSingleValueRequest{
		State: state,
		Key:   key,
	}
	resp, err := c.sendRequest(ctx, msgTypeGetSingleValue, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get single value: %w", err)
	}

	valueResp := &ledgerpb.ValueResponse{}
	if err := proto.Unmarshal(resp, valueResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal value response: %w", err)
	}

	return valueResp.Value, nil
}

// Get returns values for multiple keys at a specific state.
func (c *Client) Get(ctx context.Context, state *ledgerpb.State, keys []*ledgerpb.Key) ([]*ledgerpb.Value, error) {
	req := &ledgerpb.GetRequest{
		State: state,
		Keys:  keys,
	}
	resp, err := c.sendRequest(ctx, msgTypeGet, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get values: %w", err)
	}

	getResp := &ledgerpb.GetResponse{}
	if err := proto.Unmarshal(resp, getResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal get response: %w", err)
	}

	return getResp.Values, nil
}

// Set updates keys with new values at a specific state and returns the new state.
func (c *Client) Set(ctx context.Context, state *ledgerpb.State, keys []*ledgerpb.Key, values []*ledgerpb.Value) (*ledgerpb.State, []byte, error) {
	req := &ledgerpb.SetRequest{
		State:  state,
		Keys:   keys,
		Values: values,
	}
	resp, err := c.sendRequest(ctx, msgTypeSet, req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to set values: %w", err)
	}

	setResp := &ledgerpb.SetResponse{}
	if err := proto.Unmarshal(resp, setResp); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal set response: %w", err)
	}

	return setResp.NewState, setResp.TrieUpdate, nil
}

// Prove returns proofs for the given keys at a specific state.
func (c *Client) Prove(ctx context.Context, state *ledgerpb.State, keys []*ledgerpb.Key) ([]byte, error) {
	req := &ledgerpb.ProveRequest{
		State: state,
		Keys:  keys,
	}
	resp, err := c.sendRequest(ctx, msgTypeProve, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate proof: %w", err)
	}

	proofResp := &ledgerpb.ProofResponse{}
	if err := proto.Unmarshal(resp, proofResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal proof response: %w", err)
	}

	return proofResp.Proof, nil
}

// Close closes the transport connection.
func (c *Client) Close() error {
	if c.sessionManager != nil {
		return c.sessionManager.Close()
	}
	return nil
}

// Ready returns a channel that is closed when the transport is ready.
func (c *Client) Ready() <-chan struct{} {
	c.once.Do(func() {
		go func() {
			defer close(c.ready)
			// Wait for the ledger service to be ready by calling InitialState()
			ctx := context.Background()
			maxRetries := 30
			retryDelay := 100 * time.Millisecond

			for i := 0; i < maxRetries; i++ {
				_, err := c.InitialState(ctx)
				if err == nil {
					c.logger.Info().Msg("ledger service ready via shmipc")
					return
				}

				if i < maxRetries-1 {
					c.logger.Debug().
						Err(err).
						Int("attempt", i+1).
						Dur("retry_delay", retryDelay).
						Msg("ledger service not ready via shmipc, retrying...")
					time.Sleep(retryDelay)
					retryDelay = time.Duration(float64(retryDelay) * 1.5)
				} else {
					c.logger.Warn().Err(err).Msg("ledger service not ready via shmipc after retries, proceeding anyway")
				}
			}
		}()
	})
	return c.ready
}

// sendRequest sends a request and receives a response using shmipc-go.
func (c *Client) sendRequest(ctx context.Context, msgType messageType, req proto.Message) ([]byte, error) {
	// Get stream from session manager
	stream, err := c.sessionManager.GetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to get stream: %w", err)
	}
	defer c.sessionManager.PutBack(stream)

	// Serialize request
	reqData, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Get buffer writer for zero-copy communication
	writer := stream.BufferWriter()

	// Reserve space for message header (type + length) and data
	headerSize := 1 + 4 // 1 byte for type, 4 bytes for length
	totalSize := headerSize + len(reqData)

	// Reserve space in shared memory buffer
	buf, err := writer.Reserve(totalSize)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve buffer: %w", err)
	}

	// Write message header
	buf[0] = byte(msgType)
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(reqData)))

	// Write request data
	copy(buf[5:], reqData)

	// Flush to send (zero-copy)
	if err := stream.Flush(false); err != nil {
		return nil, fmt.Errorf("failed to flush request: %w", err)
	}

	// Read response header (5 bytes: 1 byte type + 4 bytes length)
	reader := stream.BufferReader()
	headerBuf, err := reader.ReadBytes(5)
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("connection closed")
		}
		return nil, fmt.Errorf("failed to read response header: %w", err)
	}

	// Check for error response
	respType := messageType(headerBuf[0])
	respLen := binary.BigEndian.Uint32(headerBuf[1:5])

	if respType == msgTypeError {
		errorMsgBuf, err := reader.ReadBytes(int(respLen))
		if err != nil {
			return nil, fmt.Errorf("failed to read error message: %w", err)
		}
		errorMsg := string(errorMsgBuf)
		reader.ReleasePreviousRead()
		return nil, fmt.Errorf("server error: %s", errorMsg)
	}

	// Read response data
	respData, err := reader.ReadBytes(int(respLen))
	if err != nil {
		return nil, fmt.Errorf("failed to read response data: %w", err)
	}

	// Release the read buffer after we've copied the data
	reader.ReleasePreviousRead()

	return respData, nil
}

// messageType represents the type of message in the protocol.
type messageType byte

const (
	msgTypeInitialState messageType = iota + 1
	msgTypeHasState
	msgTypeGetSingleValue
	msgTypeGet
	msgTypeSet
	msgTypeProve
	msgTypeError
)
