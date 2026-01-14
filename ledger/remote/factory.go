package remote

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
)

// RemoteLedgerFactory creates remote ledger instances via gRPC.
type RemoteLedgerFactory struct {
	grpcAddr          string
	logger            zerolog.Logger
	maxRequestSize    uint
	maxResponseSize   uint
	enableCompression bool
}

// NewRemoteLedgerFactory creates a new factory for remote ledger instances.
// maxRequestSize and maxResponseSize specify the maximum message sizes in bytes.
// If both are 0, defaults to 1 GiB for both requests and responses.
// enableCompression enables gzip compression for proof operations (useful for testing/benchmarking).
func NewRemoteLedgerFactory(
	grpcAddr string,
	logger zerolog.Logger,
	maxRequestSize, maxResponseSize uint,
	enableCompression bool,
) ledger.Factory {
	return &RemoteLedgerFactory{
		grpcAddr:          grpcAddr,
		logger:            logger,
		maxRequestSize:    maxRequestSize,
		maxResponseSize:   maxResponseSize,
		enableCompression: enableCompression,
	}
}

func (f *RemoteLedgerFactory) NewLedger() (ledger.Ledger, error) {
	client, err := NewClient(f.grpcAddr, f.logger, f.maxRequestSize, f.maxResponseSize, f.enableCompression)
	if err != nil {
		return nil, err
	}
	return client, nil
}
