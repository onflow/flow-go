package connection

import (
	"fmt"
	"io"
	"net"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
)

// DefaultClientTimeout is used when making a GRPC request to a collection node or an execution node
const DefaultClientTimeout = 3 * time.Second

// ConnectionFactory is used to create an access api client
type ConnectionFactory interface {
	GetAccessAPIClient(address string) (access.AccessAPIClient, io.Closer, error)
	GetAccessAPIClientWithPort(address string, port uint) (access.AccessAPIClient, io.Closer, error)
	InvalidateAccessAPIClient(address string)
	GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, io.Closer, error)
	InvalidateExecutionAPIClient(address string)
}

type ProxyConnectionFactory struct {
	ConnectionFactory
	targetAddress string
}

type noopCloser struct{}

func (c *noopCloser) Close() error {
	return nil
}

func (p *ProxyConnectionFactory) GetAccessAPIClient(address string) (access.AccessAPIClient, io.Closer, error) {
	return p.ConnectionFactory.GetAccessAPIClient(p.targetAddress)
}
func (p *ProxyConnectionFactory) GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, io.Closer, error) {
	return p.ConnectionFactory.GetExecutionAPIClient(p.targetAddress)
}

type ConnectionFactoryImpl struct {
	CollectionGRPCPort        uint
	ExecutionGRPCPort         uint
	CollectionNodeGRPCTimeout time.Duration
	ExecutionNodeGRPCTimeout  time.Duration
	ConnectionsCache          *lru.Cache
	CacheSize                 uint
	MaxMsgSize                uint
	AccessMetrics             module.AccessMetrics
	Log                       zerolog.Logger

	manager *Manager
}

func (cf *ConnectionFactoryImpl) GetAccessAPIClient(address string) (access.AccessAPIClient, io.Closer, error) {
	return cf.GetAccessAPIClientWithPort(address, cf.CollectionGRPCPort)
}

func (cf *ConnectionFactoryImpl) GetAccessAPIClientWithPort(address string, port uint) (access.AccessAPIClient, io.Closer, error) {

	grpcAddress, err := getGRPCAddress(address, port)
	if err != nil {
		return nil, nil, err
	}

	conn, closer, err := cf.manager.GetConnection(grpcAddress, cf.CollectionNodeGRPCTimeout)
	if err != nil {
		return nil, nil, err
	}

	return access.NewAccessAPIClient(conn), closer, nil
}

func (cf *ConnectionFactoryImpl) InvalidateAccessAPIClient(address string) {
	if cf.ConnectionsCache == nil {
		return
	}

	cf.Log.Debug().Str("cached_access_client_invalidated", address).Msg("invalidating cached access client")
	cf.invalidateAPIClient(address, cf.CollectionGRPCPort)
}

func (cf *ConnectionFactoryImpl) GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, io.Closer, error) {

	grpcAddress, err := getGRPCAddress(address, cf.ExecutionGRPCPort)
	if err != nil {
		return nil, nil, err
	}

	conn, closer, err := cf.manager.GetConnection(grpcAddress, cf.ExecutionNodeGRPCTimeout)
	if err != nil {
		return nil, nil, err
	}

	return execution.NewExecutionAPIClient(conn), closer, nil
}

func (cf *ConnectionFactoryImpl) InvalidateExecutionAPIClient(address string) {
	if cf.ConnectionsCache == nil {
		return
	}

	cf.Log.Debug().Str("cached_execution_client_invalidated", address).Msg("invalidating cached execution client")
	cf.invalidateAPIClient(address, cf.ExecutionGRPCPort)
}

// invalidateAPIClient invalidates the access API client associated with the given address and port.
// It removes the cached client from the ConnectionsCache and closes the connection.
func (cf *ConnectionFactoryImpl) invalidateAPIClient(address string, port uint) {
	grpcAddress, err := getGRPCAddress(address, port)
	if err != nil {
		// TODO: return and handle the error
		panic(err)
	}

	if !cf.manager.Remove(grpcAddress) {
		return
	}

	if cf.AccessMetrics != nil {
		cf.AccessMetrics.ConnectionFromPoolInvalidated()
	}
}

// getExecutionNodeAddress translates flow.Identity address to the GRPC address of the node by switching the port to the
// GRPC port from the libp2p port
func getGRPCAddress(address string, grpcPort uint) (string, error) {
	// split hostname and port
	hostnameOrIP, _, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}
	// use the hostname from identity list and port number as the one passed in as argument
	grpcAddress := fmt.Sprintf("%s:%d", hostnameOrIP, grpcPort)

	return grpcAddress, nil
}
