package remote

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/remote/transport"
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
	client, err := NewClient(f.grpcAddr, f.logger, f.maxRequestSize, f.maxResponseSize)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// TransportLedgerFactory creates remote ledger instances using the transport abstraction.
type TransportLedgerFactory struct {
	transportType  transport.TransportType
	addr           string
	logger         zerolog.Logger
	maxRequestSize uint
	maxResponseSize uint
	bufferSize     uint
}

// NewTransportLedgerFactory creates a new factory for remote ledger instances using the transport abstraction.
func NewTransportLedgerFactory(
	transportType transport.TransportType,
	addr string,
	logger zerolog.Logger,
	maxRequestSize, maxResponseSize, bufferSize uint,
) ledger.Factory {
	return &TransportLedgerFactory{
		transportType:  transportType,
		addr:            addr,
		logger:          logger,
		maxRequestSize:  maxRequestSize,
		maxResponseSize: maxResponseSize,
		bufferSize:      bufferSize,
	}
}

func (f *TransportLedgerFactory) NewLedger() (ledger.Ledger, error) {
	client, err := NewTransportLedgerClient(
		f.transportType,
		f.addr,
		f.logger,
		f.maxRequestSize,
		f.maxResponseSize,
		f.bufferSize,
	)
	if err != nil {
		return nil, err
	}
	return client, nil
}
