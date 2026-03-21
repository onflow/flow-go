package remote

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
)

// RemoteLedgerFactory creates remote ledger instances via gRPC.
type RemoteLedgerFactory struct {
	grpcAddr        string
	logger          zerolog.Logger
	maxRequestSize  uint
	maxResponseSize uint
}

// NewRemoteLedgerFactory creates a new factory for remote ledger instances.
// maxRequestSize and maxResponseSize specify the maximum message sizes in bytes.
// If both are 0, defaults to 1 GiB for both requests and responses.
func NewRemoteLedgerFactory(
	grpcAddr string,
	logger zerolog.Logger,
	maxRequestSize, maxResponseSize uint,
) ledger.Factory {
	return &RemoteLedgerFactory{
		grpcAddr:        grpcAddr,
		logger:          logger,
		maxRequestSize:  maxRequestSize,
		maxResponseSize: maxResponseSize,
	}
}

func (f *RemoteLedgerFactory) NewLedger() (ledger.Ledger, error) {
	var opts []ClientOption
	if f.maxRequestSize > 0 {
		opts = append(opts, WithMaxRequestSize(f.maxRequestSize))
	}
	if f.maxResponseSize > 0 {
		opts = append(opts, WithMaxResponseSize(f.maxResponseSize))
	}
	client, err := NewClient(f.grpcAddr, f.logger, opts...)
	if err != nil {
		return nil, err
	}
	return client, nil
}
