package backend

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"google.golang.org/grpc"

	grpcutils "github.com/onflow/flow-go/utils/grpc"
)

const defaultClientTimeout = 2 * time.Second // the timeout used when connecting to a collection node or an execution node

var grcpClientTimeout = defaultClientTimeout

// ConnectionFactory is used to create an access api client
type ConnectionFactory interface {
	GetAccessAPIClient(address string) (access.AccessAPIClient, io.Closer, error)
	GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, io.Closer, error)
}

type ConnectionFactoryImpl struct {
	CollectionGRPCPort uint
	ExecutionGRPCPort  uint
}

// createConnection creates new gRPC connections to remote node
func (cf *ConnectionFactoryImpl) createConnection(address string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(
		address,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
		grpc.WithInsecure(),
		withClientUnaryInterceptor())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to address %s: %w", address, err)
	}
	return conn, nil
}

func (cf *ConnectionFactoryImpl) GetAccessAPIClient(address string) (access.AccessAPIClient, io.Closer, error) {

	grpcAddress, err := getGRPCAddress(address, cf.CollectionGRPCPort)
	if err != nil {
		return nil, nil, err
	}
	conn, err := cf.createConnection(grpcAddress)
	if err != nil {
		return nil, nil, err
	}
	accessAPIClient := access.NewAccessAPIClient(conn)
	closer := io.Closer(conn)
	return accessAPIClient, closer, nil
}

func (cf *ConnectionFactoryImpl) GetExecutionAPIClient(address string) (execution.ExecutionAPIClient, io.Closer, error) {

	grpcAddress, err := getGRPCAddress(address, cf.ExecutionGRPCPort)
	if err != nil {
		return nil, nil, err
	}

	conn, err := cf.createConnection(grpcAddress)
	if err != nil {
		return nil, nil, err
	}
	executionAPIClient := execution.NewExecutionAPIClient(conn)
	closer := io.Closer(conn)
	return executionAPIClient, closer, nil
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

func withClientUnaryInterceptor() grpc.DialOption {
	return grpc.WithUnaryInterceptor(clientTimeoutInterceptor)
}

func clientTimeoutInterceptor(
	ctx context.Context,
	method string,
	req interface{},
	reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {

	// create a context that expires after timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, grcpClientTimeout)

	defer cancel()

	// call the remote GRPC using the short context
	err := invoker(ctxWithTimeout, method, req, reply, cc, opts...)

	return err
}
