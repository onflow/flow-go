package remote

import (
	"fmt"
	"net"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger/remote/transport"
	"github.com/onflow/flow-go/ledger/remote/transport/grpc"
	shmipctransport "github.com/onflow/flow-go/ledger/remote/transport/shmipc"
)

// NewTransportClient creates a new transport client based on the transport type.
func NewTransportClient(
	transportType transport.TransportType,
	addr string,
	logger zerolog.Logger,
	maxRequestSize, maxResponseSize, bufferSize uint,
) (transport.ClientTransport, error) {
	switch transportType {
	case transport.TransportTypeGRPC:
		return grpc.NewClient(addr, logger, maxRequestSize, maxResponseSize)
	case transport.TransportTypeShmipc:
		return shmipctransport.NewClient(addr, logger, bufferSize)
	default:
		return nil, fmt.Errorf("unknown transport type: %s", transportType)
	}
}

// NewTransportServer creates a new transport server based on the transport type.
func NewTransportServer(
	transportType transport.TransportType,
	listener net.Listener,
	logger zerolog.Logger,
	maxRequestSize, maxResponseSize, bufferSize uint,
) (transport.ServerTransport, error) {
	switch transportType {
	case transport.TransportTypeGRPC:
		return grpc.NewServer(listener, logger, maxRequestSize, maxResponseSize), nil
	case transport.TransportTypeShmipc:
		return shmipctransport.NewServer(listener, logger, bufferSize)
	default:
		return nil, fmt.Errorf("unknown transport type: %s", transportType)
	}
}
