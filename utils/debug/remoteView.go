package debug

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/onflow/flow/protobuf/go/flow/executiondata"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

type StorageSnapshot interface {
	snapshot.StorageSnapshot
}

// ExecutionNodeStorageSnapshot provides a storage snapshot connected
// to an execution node to read the registers.
type ExecutionNodeStorageSnapshot struct {
	Client  execution.ExecutionAPIClient
	Cache   RegisterCache
	BlockID flow.Identifier
}

var _ StorageSnapshot = &ExecutionNodeStorageSnapshot{}

func NewExecutionNodeStorageSnapshot(
	client execution.ExecutionAPIClient,
	cache RegisterCache,
	blockID flow.Identifier,
) (
	*ExecutionNodeStorageSnapshot,
	error,
) {
	if cache == nil {
		cache = NewInMemoryRegisterCache()
	}

	return &ExecutionNodeStorageSnapshot{
		Client:  client,
		Cache:   cache,
		BlockID: blockID,
	}, nil
}

func (snapshot *ExecutionNodeStorageSnapshot) Close() error {
	return snapshot.Cache.Persist()
}

func (snapshot *ExecutionNodeStorageSnapshot) Get(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	// first, check the cache
	value, found := snapshot.Cache.Get(id.Owner, id.Key)
	if found {
		return value, nil
	}

	// if the register is not cached, fetch it from the execution node
	req := &execution.GetRegisterAtBlockIDRequest{
		BlockId:       snapshot.BlockID[:],
		RegisterOwner: []byte(id.Owner),
		RegisterKey:   []byte(id.Key),
	}

	// TODO use a proper context for timeouts
	resp, err := snapshot.Client.GetRegisterAtBlockID(
		context.Background(),
		req,
	)
	if err != nil {
		return nil, err
	}

	// append register to the cache
	snapshot.Cache.Set(id.Owner, id.Key, resp.Value)

	return resp.Value, nil
}

// ExecutionDataStorageSnapshot provides a storage snapshot connected
// to an access node to read the registers (via its execution data API).
type ExecutionDataStorageSnapshot struct {
	Client      executiondata.ExecutionDataAPIClient
	Cache       RegisterCache
	BlockHeight uint64
}

var _ StorageSnapshot = &ExecutionDataStorageSnapshot{}

func NewExecutionDataStorageSnapshot(
	client executiondata.ExecutionDataAPIClient,
	cache RegisterCache,
	blockHeight uint64,
) (
	*ExecutionDataStorageSnapshot,
	error,
) {
	if cache == nil {
		cache = NewInMemoryRegisterCache()
	}

	return &ExecutionDataStorageSnapshot{
		Client:      client,
		Cache:       cache,
		BlockHeight: blockHeight,
	}, nil
}

func (snapshot *ExecutionDataStorageSnapshot) Close() error {
	return snapshot.Cache.Persist()
}

func (snapshot *ExecutionDataStorageSnapshot) Get(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	// first, check the cache
	value, found := snapshot.Cache.Get(id.Owner, id.Key)
	if found {
		return value, nil
	}

	// if the register is not cached, fetch it from the execution data API
	req := &executiondata.GetRegisterValuesRequest{
		BlockHeight: snapshot.BlockHeight,
		RegisterIds: []*entities.RegisterID{
			{
				Owner: []byte(id.Owner),
				Key:   []byte(id.Key),
			},
		},
	}

	// TODO use a proper context for timeouts
	resp, err := snapshot.Client.GetRegisterValues(
		context.Background(),
		req,
	)
	if err != nil {
		return nil, err
	}

	value = resp.Values[0]

	// append register to the cache
	snapshot.Cache.Set(id.Owner, id.Key, value)

	return value, nil
}
