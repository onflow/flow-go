package remote

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
)

// RemoteLedgerFactory creates remote ledger instances via gRPC.
type RemoteLedgerFactory struct {
	grpcAddr string
	logger   zerolog.Logger
}

// NewRemoteLedgerFactory creates a new factory for remote ledger instances.
func NewRemoteLedgerFactory(
	grpcAddr string,
	logger zerolog.Logger,
) ledger.Factory {
	return &RemoteLedgerFactory{
		grpcAddr: grpcAddr,
		logger:   logger,
	}
}

func (f *RemoteLedgerFactory) NewLedger() (ledger.Ledger, error) {
	client, err := NewClient(f.grpcAddr, f.logger)
	if err != nil {
		return nil, err
	}
	return client, nil
}
