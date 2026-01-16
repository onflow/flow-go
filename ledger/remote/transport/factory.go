package transport

import (
	"fmt"
	"net"

	"github.com/rs/zerolog"
)

// ClientFactory creates transport clients.
type ClientFactory interface {
	NewClient(addr string, logger zerolog.Logger, maxRequestSize, maxResponseSize, bufferSize uint) (ClientTransport, error)
}

// ServerFactory creates transport servers.
type ServerFactory interface {
	NewServer(listener net.Listener, logger zerolog.Logger, maxRequestSize, maxResponseSize, bufferSize uint) (ServerTransport, error)
}

// NewClient creates a new transport client based on the transport type.
// This function should be implemented by each transport package to avoid import cycles.
func NewClient(transportType TransportType, addr string, logger zerolog.Logger, maxRequestSize, maxResponseSize, bufferSize uint) (ClientTransport, error) {
	// This will be implemented by calling packages to avoid import cycles
	return nil, fmt.Errorf("NewClient must be implemented by transport-specific packages")
}

// NewServer creates a new transport server based on the transport type.
// This function should be implemented by each transport package to avoid import cycles.
func NewServer(transportType TransportType, listener net.Listener, logger zerolog.Logger, maxRequestSize, maxResponseSize, bufferSize uint) (ServerTransport, error) {
	// This will be implemented by calling packages to avoid import cycles
	return nil, fmt.Errorf("NewServer must be implemented by transport-specific packages")
}
