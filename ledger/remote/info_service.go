package remote

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
)

// InfoService implements the gRPC LedgerInfoService interface. It is
// registered on every ledger gRPC server, regardless of the server's mode,
// so clients can discover the mode of the server they connected to before
// issuing mode-specific RPCs.
//
// InfoService is stateless and concurrency-safe.
type InfoService struct {
	ledgerpb.UnimplementedLedgerInfoServiceServer
	mode ledgerpb.LedgerMode
}

// NewInfoService creates a new info service that reports the given mode.
// Callers MUST pass either [ledgerpb.LedgerMode_LEDGER_MODE_FULL] or
// [ledgerpb.LedgerMode_LEDGER_MODE_PAYLOADLESS]; passing UNSPECIFIED produces
// a server that reports UNSPECIFIED, which clients will treat as a
// misconfigured server and refuse to use.
func NewInfoService(mode ledgerpb.LedgerMode) *InfoService {
	return &InfoService{mode: mode}
}

// ServerInfo returns the server's operating mode.
//
// No error returns are expected during normal operation.
func (s *InfoService) ServerInfo(_ context.Context, _ *emptypb.Empty) (*ledgerpb.ServerInfoResponse, error) {
	return &ledgerpb.ServerInfoResponse{Mode: s.mode}, nil
}
