package connection

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/onflow/crypto"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"

	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/module"
)

// ConnectionFactory is an interface for creating access and execution API clients.
type ConnectionFactory interface {
	// GetCollectionAPIClient gets an access API client for the specified address using the default CollectionGRPCPort, networkPubKey is optional,
	// and it is used for secure gRPC connection. Can be nil for an unsecured connection.
	// The returned io.Closer should close the connection after the call if no error occurred during client creation.
	GetCollectionAPIClient(address string, networkPubKey crypto.PublicKey) (access.AccessAPIClient, io.Closer, error)
	// GetAccessAPIClientWithPort gets an access API client for the specified address with port, networkPubKey is optional,
	// and it is used for secure gRPC connection. Can be nil for an unsecured connection.
	// The returned io.Closer should close the connection after the call if no error occurred during client creation.
	GetAccessAPIClientWithPort(address string, networkPubKey crypto.PublicKey) (access.AccessAPIClient, io.Closer, error)
	// GetExecutionAPIClient gets an execution API client for the specified address using the default ExecutionGRPCPort.
	// The returned io.Closer should close the connection after the call if no error occurred during client creation.
	GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, io.Closer, error)
}

// ProxyConnectionFactory wraps an existing ConnectionFactory and allows getting API clients for a target address.
type ProxyConnectionFactory struct {
	ConnectionFactory
	targetAddress string
}

// GetCollectionAPIClient gets an access API client for a target address using the default CollectionGRPCPort.
// The networkPubKey is the public key used for a secure gRPC connection. It can be nil for an unsecured connection.
// The returned io.Closer should close the connection after the call if no error occurred during client creation.
func (p *ProxyConnectionFactory) GetCollectionAPIClient(_ string, networkPubKey crypto.PublicKey) (access.AccessAPIClient, io.Closer, error) {
	return p.ConnectionFactory.GetCollectionAPIClient(p.targetAddress, networkPubKey)
}

// GetExecutionAPIClient gets an execution API client for a target address using the default ExecutionGRPCPort.
// The returned io.Closer should close the connection after the call if no error occurred during client creation.
func (p *ProxyConnectionFactory) GetExecutionAPIClient(_ string) (execution.ExecutionAPIClient, io.Closer, error) {
	return p.ConnectionFactory.GetExecutionAPIClient(p.targetAddress)
}

var _ ConnectionFactory = (*ConnectionFactoryImpl)(nil)

type ConnectionFactoryImpl struct {
	AccessMetrics module.AccessMetrics
	Log           zerolog.Logger
	Manager       Manager

	AccessConfig     Config
	ExecutionConfig  Config
	CollectionConfig Config
}

type Config struct {
	GRPCPort           uint
	Timeout            time.Duration
	MaxRequestMsgSize  uint
	MaxResponseMsgSize uint
}

// DefaultExecutionConfig returns the default execution client config.
func DefaultAccessConfig() Config {
	return Config{
		GRPCPort:           9000,
		Timeout:            3 * time.Second,
		MaxRequestMsgSize:  commonrpc.DefaultAccessMaxRequestSize,
		MaxResponseMsgSize: commonrpc.DefaultAccessMaxResponseSize,
	}
}

// DefaultCollectionConfig returns the default collection client config.
func DefaultCollectionConfig() Config {
	return Config{
		GRPCPort:           9000,
		Timeout:            3 * time.Second,
		MaxRequestMsgSize:  commonrpc.DefaultCollectionMaxRequestSize,
		MaxResponseMsgSize: commonrpc.DefaultCollectionMaxResponseSize,
	}
}

// DefaultExecutionConfig returns the default execution client config.
func DefaultExecutionConfig() Config {
	return Config{
		GRPCPort:           9000,
		Timeout:            3 * time.Second,
		MaxRequestMsgSize:  commonrpc.DefaultExecutionMaxRequestSize,
		MaxResponseMsgSize: commonrpc.DefaultExecutionMaxResponseSize,
	}
}

// GetCollectionAPIClient gets an access API client for the specified collection node address using
// the default CollectionConfig.
// The networkPubKey is the public key used for secure gRPC connection. Can be nil for an unsecured connection.
// The returned io.Closer should close the connection after the call if no error occurred during client creation.
func (cf *ConnectionFactoryImpl) GetCollectionAPIClient(address string, networkPubKey crypto.PublicKey) (access.AccessAPIClient, io.Closer, error) {
	address, err := getGRPCAddress(address, cf.CollectionConfig.GRPCPort)
	if err != nil {
		return nil, nil, err
	}

	conn, closer, err := cf.Manager.GetConnection(address, cf.CollectionConfig, networkPubKey)
	if err != nil {
		return nil, nil, err
	}

	return access.NewAccessAPIClient(conn), closer, nil
}

// GetAccessAPIClientWithPort gets an access API client for the specified address with port.
// The networkPubKey is the public key used for secure gRPC connection. Can be nil for an unsecured connection.
// The returned io.Closer should close the connection after the call if no error occurred during client creation.
func (cf *ConnectionFactoryImpl) GetAccessAPIClientWithPort(address string, networkPubKey crypto.PublicKey) (access.AccessAPIClient, io.Closer, error) {
	conn, closer, err := cf.Manager.GetConnection(address, cf.AccessConfig, networkPubKey)
	if err != nil {
		return nil, nil, err
	}

	return access.NewAccessAPIClient(conn), closer, nil
}

// GetExecutionAPIClient gets an execution API client for the specified address using the ExecutionConfig.
// The returned io.Closer should close the connection after the call if no error occurred during client creation.
func (cf *ConnectionFactoryImpl) GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, io.Closer, error) {
	grpcAddress, err := getGRPCAddress(address, cf.ExecutionConfig.GRPCPort)
	if err != nil {
		return nil, nil, err
	}

	conn, closer, err := cf.Manager.GetConnection(grpcAddress, cf.ExecutionConfig, nil)
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
