package shmipc

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/cloudwego/shmipc-go"
	"github.com/golang/protobuf/proto"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger/remote/transport"
	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
)

// Server implements transport.ServerTransport using shmipc-go.
type Server struct {
	listener net.Listener
	config   *shmipc.Config
	handler  transport.ServerHandler
	logger   zerolog.Logger
	ready    chan struct{}
	stopCh   chan struct{}
	wg       sync.WaitGroup
	once     sync.Once
}

// NewServer creates a new shmipc-go transport server.
// addr can be a Unix domain socket path (e.g., "/tmp/ledger.sock") or TCP address (e.g., "0.0.0.0:9000").
// bufferSize specifies the shared memory buffer size in bytes (default: 1 GiB).
func NewServer(addr string, logger zerolog.Logger, bufferSize uint) (*Server, error) {
	logger = logger.With().Str("component", "shmipc_ledger_server").Logger()

	if bufferSize == 0 {
		bufferSize = 1 << 30 // 1 GiB default
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

	// Create listener
	listener, err := net.Listen(network, address)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Configure shmipc
	config := shmipc.DefaultConfig()
	config.ShareMemorySize = int(bufferSize)
	config.Network = network

	logger.Info().
		Str("address", addr).
		Str("network", network).
		Uint("buffer_size", bufferSize).
		Msg("shmipc server listening")

	return &Server{
		listener: listener,
		config:   config,
		logger:   logger,
		ready:    make(chan struct{}),
		stopCh:   make(chan struct{}),
	}, nil
}

// Serve starts the transport server.
func (s *Server) Serve(handler transport.ServerHandler) error {
	s.handler = handler

	// Signal ready
	s.once.Do(func() {
		close(s.ready)
	})

	s.logger.Info().Msg("shmipc server started")

	for {
		select {
		case <-s.stopCh:
			return nil
		default:
		}

		// Accept connection
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return nil
			default:
				s.logger.Error().Err(err).Msg("failed to accept connection")
				continue
			}
		}

		// Handle connection in goroutine
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// Stop stops the transport server.
func (s *Server) Stop() {
	close(s.stopCh)
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.wg.Wait()
	s.logger.Info().Msg("shmipc server stopped")
}

// Ready returns a channel that is closed when the server is ready.
func (s *Server) Ready() <-chan struct{} {
	return s.ready
}

// handleConnection handles a single client connection.
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	// Create shmipc session from connection
	session, err := shmipc.Server(conn, s.config)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to create shmipc session")
		return
	}
	defer session.Close()

	s.logger.Debug().Msg("new shmipc client connected")

	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		// Read request from shared memory
		stream := session.GetStream()
		reader := stream.BufferReader()

		reqBuf, err := reader.ReadBytes()
		if err != nil {
			if err == io.EOF {
				s.logger.Debug().Msg("client disconnected")
				return
			}
			s.logger.Error().Err(err).Msg("failed to read request")
			return
		}

		if len(reqBuf) < 5 {
			s.logger.Error().Int("len", len(reqBuf)).Msg("invalid request: too short")
			continue
		}

		// Parse message header
		msgType := messageType(reqBuf[0])
		reqLen := binary.BigEndian.Uint32(reqBuf[1:5])

		if len(reqBuf) < 5+int(reqLen) {
			s.logger.Error().Msg("invalid request: length mismatch")
			continue
		}

		reqData := reqBuf[5 : 5+reqLen]

		// Handle request
		respData, err := s.handleRequest(context.Background(), msgType, reqData)
		if err != nil {
			s.logger.Error().Err(err).Msg("failed to handle request")
			// Send error response
			errorResp := s.encodeError(err.Error())
			if err := s.sendResponse(stream, msgTypeError, errorResp); err != nil {
				s.logger.Error().Err(err).Msg("failed to send error response")
				return
			}
			continue
		}

		// Send response
		if err := s.sendResponse(stream, msgType, respData); err != nil {
			s.logger.Error().Err(err).Msg("failed to send response")
			return
		}
	}
}

// handleRequest processes a request and returns the response.
func (s *Server) handleRequest(ctx context.Context, msgType messageType, reqData []byte) ([]byte, error) {
	switch msgType {
	case msgTypeInitialState:
		state, err := s.handler.InitialState(ctx)
		if err != nil {
			return nil, fmt.Errorf("initial state: %w", err)
		}
		resp := &ledgerpb.StateResponse{State: state}
		return proto.Marshal(resp)

	case msgTypeHasState:
		req := &ledgerpb.StateRequest{}
		if err := proto.Unmarshal(reqData, req); err != nil {
			return nil, fmt.Errorf("unmarshal has state request: %w", err)
		}
		hasState, err := s.handler.HasState(ctx, req.State)
		if err != nil {
			return nil, fmt.Errorf("has state: %w", err)
		}
		resp := &ledgerpb.HasStateResponse{HasState: hasState}
		return proto.Marshal(resp)

	case msgTypeGetSingleValue:
		req := &ledgerpb.GetSingleValueRequest{}
		if err := proto.Unmarshal(reqData, req); err != nil {
			return nil, fmt.Errorf("unmarshal get single value request: %w", err)
		}
		value, err := s.handler.GetSingleValue(ctx, req.State, req.Key)
		if err != nil {
			return nil, fmt.Errorf("get single value: %w", err)
		}
		resp := &ledgerpb.ValueResponse{Value: value}
		return proto.Marshal(resp)

	case msgTypeGet:
		req := &ledgerpb.GetRequest{}
		if err := proto.Unmarshal(reqData, req); err != nil {
			return nil, fmt.Errorf("unmarshal get request: %w", err)
		}
		values, err := s.handler.Get(ctx, req.State, req.Keys)
		if err != nil {
			return nil, fmt.Errorf("get: %w", err)
		}
		resp := &ledgerpb.GetResponse{Values: values}
		return proto.Marshal(resp)

	case msgTypeSet:
		req := &ledgerpb.SetRequest{}
		if err := proto.Unmarshal(reqData, req); err != nil {
			return nil, fmt.Errorf("unmarshal set request: %w", err)
		}
		newState, trieUpdate, err := s.handler.Set(ctx, req.State, req.Keys, req.Values)
		if err != nil {
			return nil, fmt.Errorf("set: %w", err)
		}
		resp := &ledgerpb.SetResponse{
			NewState:   newState,
			TrieUpdate: trieUpdate,
		}
		return proto.Marshal(resp)

	case msgTypeProve:
		req := &ledgerpb.ProveRequest{}
		if err := proto.Unmarshal(reqData, req); err != nil {
			return nil, fmt.Errorf("unmarshal prove request: %w", err)
		}
		proof, err := s.handler.Prove(ctx, req.State, req.Keys)
		if err != nil {
			return nil, fmt.Errorf("prove: %w", err)
		}
		resp := &ledgerpb.ProofResponse{Proof: proof}
		return proto.Marshal(resp)

	default:
		return nil, fmt.Errorf("unknown message type: %d", msgType)
	}
}

// sendResponse sends a response using zero-copy.
func (s *Server) sendResponse(stream *shmipc.Stream, msgType messageType, respData []byte) error {
	writer := stream.BufferWriter()

	// Reserve space for header and data
	headerSize := 1 + 4 // 1 byte for type, 4 bytes for length
	totalSize := headerSize + len(respData)

	buf := writer.Reserve(totalSize)
	if len(buf) < totalSize {
		return fmt.Errorf("insufficient buffer space: need %d, got %d", totalSize, len(buf))
	}

	// Write header
	buf[0] = byte(msgType)
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(respData)))

	// Write response data
	copy(buf[5:], respData)

	// Flush (zero-copy)
	return writer.Flush()
}

// encodeError encodes an error message.
func (s *Server) encodeError(errorMsg string) []byte {
	return []byte(errorMsg)
}
