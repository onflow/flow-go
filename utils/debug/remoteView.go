package debug

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/execution"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/model/flow"
)

// RemoteStorageSnapshot provides a storage snapshot connected to a live
// execution node to read the registers.
type RemoteStorageSnapshot struct {
	Cache              registerCache
	BlockID            []byte
	BlockHeader        *flow.Header
	connection         *grpc.ClientConn
	executionAPIclient execution.ExecutionAPIClient
}

// A RemoteStorageSnapshotOption sets a configuration parameter for the remote
// snapshot
type RemoteStorageSnapshotOption func(*RemoteStorageSnapshot) *RemoteStorageSnapshot

// WithFileCache sets the output path to store
// register values so can be fetched from a file cache
// it loads the values from the cache upon object construction
func WithCache(cache registerCache) RemoteStorageSnapshotOption {
	return func(snapshot *RemoteStorageSnapshot) *RemoteStorageSnapshot {
		snapshot.Cache = cache
		return snapshot
	}
}

// WithBlockID sets the blockID for the remote snapshot, if not used
// remote snapshot will use the latest sealed block
func WithBlockID(blockID flow.Identifier) RemoteStorageSnapshotOption {
	return func(snapshot *RemoteStorageSnapshot) *RemoteStorageSnapshot {
		snapshot.BlockID = blockID[:]
		var err error
		snapshot.BlockHeader, err = snapshot.getBlockHeader(blockID)
		if err != nil {
			panic(err)
		}
		return snapshot
	}
}

func NewRemoteStorageSnapshot(
	grpcAddress string,
	opts ...RemoteStorageSnapshotOption,
) *RemoteStorageSnapshot {
	conn, err := grpc.Dial(
		grpcAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	snapshot := &RemoteStorageSnapshot{
		connection:         conn,
		executionAPIclient: execution.NewExecutionAPIClient(conn),
		Cache:              newMemRegisterCache(),
	}

	snapshot.BlockID, snapshot.BlockHeader, err = snapshot.getLatestBlockID()
	if err != nil {
		panic(err)
	}

	for _, applyOption := range opts {
		snapshot = applyOption(snapshot)
	}
	return snapshot
}

func (snapshot *RemoteStorageSnapshot) Close() error {
	return snapshot.connection.Close()
}

func (snapshot *RemoteStorageSnapshot) getLatestBlockID() (
	[]byte,
	*flow.Header,
	error,
) {
	req := &execution.GetLatestBlockHeaderRequest{
		IsSealed: true,
	}

	resp, err := snapshot.executionAPIclient.GetLatestBlockHeader(
		context.Background(),
		req)
	if err != nil {
		return nil, nil, err
	}

	// TODO set chainID and parentID
	header := &flow.Header{
		Height:    resp.Block.Height,
		Timestamp: resp.Block.Timestamp.AsTime(),
	}

	return resp.Block.Id, header, nil
}

func (snapshot *RemoteStorageSnapshot) getBlockHeader(
	blockID flow.Identifier,
) (
	*flow.Header,
	error,
) {
	req := &execution.GetBlockHeaderByIDRequest{
		Id: blockID[:],
	}

	resp, err := snapshot.executionAPIclient.GetBlockHeaderByID(
		context.Background(),
		req)
	if err != nil {
		return nil, err
	}

	// TODO set chainID and parentID
	header := &flow.Header{
		Height:    resp.Block.Height,
		Timestamp: resp.Block.Timestamp.AsTime(),
	}

	return header, nil
}

func (snapshot *RemoteStorageSnapshot) Get(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	// then check the read cache
	value, found := snapshot.Cache.Get(id.Owner, id.Key)
	if found {
		return value, nil
	}

	// last use the grpc api the
	req := &execution.GetRegisterAtBlockIDRequest{
		BlockId:       []byte(snapshot.BlockID),
		RegisterOwner: []byte(id.Owner),
		RegisterKey:   []byte(id.Key),
	}

	// TODO use a proper context for timeouts
	resp, err := snapshot.executionAPIclient.GetRegisterAtBlockID(
		context.Background(),
		req)
	if err != nil {
		return nil, err
	}

	snapshot.Cache.Set(id.Owner, id.Key, resp.Value)

	// append value to the file cache

	return resp.Value, nil
}
