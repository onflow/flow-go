package debug

import (
	"io"

	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/onflow/flow/protobuf/go/flow/executiondata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

type RemoteClient interface {
	io.Closer
	StorageSnapshot(blockHeight uint64, blockID flow.Identifier) (snapshot.StorageSnapshot, error)
}

// ExecutionNodeRemoteClient is a remote client that connects to an execution node
// and uses the execution API to fetch execution data.
type ExecutionNodeRemoteClient struct {
	conn *grpc.ClientConn
}

var _ RemoteClient = &ExecutionNodeRemoteClient{}

func NewExecutionNodeRemoteClient(address string) (*ExecutionNodeRemoteClient, error) {
	executionConn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &ExecutionNodeRemoteClient{
		conn: executionConn,
	}, nil
}

func (c *ExecutionNodeRemoteClient) StorageSnapshot(_ uint64, blockID flow.Identifier) (snapshot.StorageSnapshot, error) {
	executionClient := execution.NewExecutionAPIClient(c.conn)
	return NewExecutionNodeStorageSnapshot(executionClient, blockID)
}

func (c *ExecutionNodeRemoteClient) Close() error {
	return c.conn.Close()
}

// ExecutionDataRemoteClient is a remote client that connects to an access node
// and uses the execution data API to fetch execution data.
type ExecutionDataRemoteClient struct {
	conn  *grpc.ClientConn
	chain flow.Chain
}

var _ RemoteClient = &ExecutionDataRemoteClient{}

func NewExecutionDataRemoteClient(address string, chain flow.Chain) (*ExecutionDataRemoteClient, error) {
	accessConn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &ExecutionDataRemoteClient{
		conn:  accessConn,
		chain: chain,
	}, nil
}

func (c *ExecutionDataRemoteClient) StorageSnapshot(blockHeight uint64, _ flow.Identifier) (snapshot.StorageSnapshot, error) {
	executionDataClient := executiondata.NewExecutionDataAPIClient(c.conn)

	// The execution data API provides the *resulting* data,
	// so fetch the data for the parent block for the *initial* data.
	return NewExecutionDataStorageSnapshot(executionDataClient, c.chain, blockHeight-1)
}

func (c *ExecutionDataRemoteClient) Close() error {
	return c.conn.Close()
}
