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

// NewServer creates a new shmipc-go transport server using the provided listener.
// bufferSize specifies the shared memory buffer size in bytes (default: 1 GiB).
func NewServer(listener net.Listener, logger zerolog.Logger, bufferSize uint) (*Server, error) {
	logger = logger.With().Str("component", "shmipc_ledger_server").Logger()

	if bufferSize == 0 {
		bufferSize = 2 << 30 // 2 GiB default (matches Docker shm_size)
	}

	// Configure shmipc
	config := shmipc.DefaultConfig()
	// Configure shared memory buffer capacity (in bytes, uint32)
	if bufferSize > 1<<32-1 {
		bufferSize = 1<<32 - 1 // Max uint32
	}
	config.ShareMemoryBufferCap = uint32(bufferSize)

	logger.Info().
		Str("address", listener.Addr().String()).
		Str("network", listener.Addr().Network()).
		Uint("buffer_size_bytes", bufferSize).
		Uint32("buffer_size_configured", config.ShareMemoryBufferCap).
		Str("buffer_size_mb", fmt.Sprintf("%.2f", float64(bufferSize)/(1024*1024))).
		Msg("shmipc server listening with configured buffer size")

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

	// Accept and handle streams from this session
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		stream, err := session.AcceptStream()
		if err != nil {
			s.logger.Debug().Err(err).Msg("failed to accept stream or connection closed")
			return
		}

		// Handle stream in goroutine
		s.wg.Add(1)
		go s.handleStream(stream)
	}
}

// handleStream handles a single stream.
func (s *Server) handleStream(stream *shmipc.Stream) {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		// Read request header from shared memory (5 bytes: 1 byte type + 4 bytes length)
		reader := stream.BufferReader()

		headerBuf, err := reader.ReadBytes(5)
		if err != nil {
			if err == io.EOF {
				s.logger.Debug().Msg("stream closed")
				return
			}
			s.logger.Error().Err(err).Msg("failed to read request header")
			return
		}

		// Parse message header
		msgType := messageType(headerBuf[0])
		reqLen := binary.BigEndian.Uint32(headerBuf[1:5])

		// Read request data
		reqData, err := reader.ReadBytes(int(reqLen))
		if err != nil {
			s.logger.Error().Err(err).Msg("failed to read request data")
			return
		}

		// Release the read buffer after we've processed the data
		reader.ReleasePreviousRead()

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

	buf, err := writer.Reserve(totalSize)
	if err != nil {
		return fmt.Errorf("failed to reserve buffer: %w", err)
	}

	// Write header
	buf[0] = byte(msgType)
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(respData)))

	// Write response data
	copy(buf[5:], respData)

	// Flush (zero-copy)
	return stream.Flush(false)
}

// encodeError encodes an error message.
func (s *Server) encodeError(errorMsg string) []byte {
	return []byte(errorMsg)
}
