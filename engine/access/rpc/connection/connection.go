package connection

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
)

// ConnectionFactory is an interface for creating access and execution API clients.
type ConnectionFactory interface {
	GetAccessAPIClient(address string) (access.AccessAPIClient, io.Closer, error)
	GetAccessAPIClientWithPort(address string, port uint) (access.AccessAPIClient, io.Closer, error)
	GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, io.Closer, error)
}

// ProxyConnectionFactory wraps an existing ConnectionFactory and allows getting API clients for a target address.
type ProxyConnectionFactory struct {
	ConnectionFactory
	targetAddress string
}

func (p *ProxyConnectionFactory) GetAccessAPIClient(address string) (access.AccessAPIClient, io.Closer, error) {
	return p.ConnectionFactory.GetAccessAPIClient(p.targetAddress)
}
func (p *ProxyConnectionFactory) GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, io.Closer, error) {
	return p.ConnectionFactory.GetExecutionAPIClient(p.targetAddress)
}

var _ ConnectionFactory = (*ConnectionFactoryImpl)(nil)

type ConnectionFactoryImpl struct {
	CollectionGRPCPort        uint
	ExecutionGRPCPort         uint
	CollectionNodeGRPCTimeout time.Duration
	ExecutionNodeGRPCTimeout  time.Duration
	AccessMetrics             module.AccessMetrics
	Log                       zerolog.Logger
	Manager                   Manager
}

// GetAccessAPIClient gets an access API client for the specified address using the default CollectionGRPCPort.
func (cf *ConnectionFactoryImpl) GetAccessAPIClient(address string) (access.AccessAPIClient, io.Closer, error) {
	return cf.GetAccessAPIClientWithPort(address, cf.CollectionGRPCPort)
}

// GetAccessAPIClientWithPort gets an access API client for the specified address and port.
func (cf *ConnectionFactoryImpl) GetAccessAPIClientWithPort(address string, port uint) (access.AccessAPIClient, io.Closer, error) {
	grpcAddress, err := getGRPCAddress(address, port)
	if err != nil {
		return nil, nil, err
	}

	conn, closer, err := cf.Manager.GetConnection(grpcAddress, cf.CollectionNodeGRPCTimeout, AccessClient)
	if err != nil {
		return nil, nil, err
	}

	return access.NewAccessAPIClient(conn), closer, nil
}

// GetExecutionAPIClient gets an execution API client for the specified address using the default ExecutionGRPCPort.
func (cf *ConnectionFactoryImpl) GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, io.Closer, error) {
	grpcAddress, err := getGRPCAddress(address, cf.ExecutionGRPCPort)
	if err != nil {
		return nil, nil, err
	}

	conn, closer, err := cf.Manager.GetConnection(grpcAddress, cf.ExecutionNodeGRPCTimeout, ExecutionClient)
	if err != nil {
		return nil, nil, err
	}

	return execution.NewExecutionAPIClient(conn), closer, nil
}

// getGRPCAddress translates the flow.Identity address to the GRPC address of the node by switching the port to the
// GRPC port from the libp2p port.
func getGRPCAddress(address string, grpcPort uint) (string, error) {
	// Split hostname and port
	hostnameOrIP, _, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}
	// Use the hostname from the identity list and the GRPC port number as the one passed in as an argument.
	grpcAddress := fmt.Sprintf("%s:%d", hostnameOrIP, grpcPort)

	return grpcAddress, nil
}
