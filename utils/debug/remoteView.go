package debug

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/onflow/flow/protobuf/go/flow/executiondata"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
	BlockID flow.Identifier
}

var _ StorageSnapshot = &ExecutionNodeStorageSnapshot{}

func NewExecutionNodeStorageSnapshot(
	client execution.ExecutionAPIClient,
	blockID flow.Identifier,
) (
	*ExecutionNodeStorageSnapshot,
	error,
) {
	return &ExecutionNodeStorageSnapshot{
		Client:  client,
		BlockID: blockID,
	}, nil
}

func (snapshot *ExecutionNodeStorageSnapshot) Close() error {
	return nil
}

func (snapshot *ExecutionNodeStorageSnapshot) Get(
	id flow.RegisterID,
) (
	value flow.RegisterValue,
	err error,
) {
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
		if status.Code(err) == codes.NotFound {
			value = nil
		} else {
			return nil, err
		}
	} else {
		value = resp.Value
	}

	return value, nil
}

// ExecutionDataStorageSnapshot provides a storage snapshot connected
// to an access node to read the registers (via its execution data API).
type ExecutionDataStorageSnapshot struct {
	Client      executiondata.ExecutionDataAPIClient
	Chain       flow.Chain
	BlockHeight uint64
}

var _ StorageSnapshot = &ExecutionDataStorageSnapshot{}

func NewExecutionDataStorageSnapshot(
	client executiondata.ExecutionDataAPIClient,
	chain flow.Chain,
	blockHeight uint64,
) (
	*ExecutionDataStorageSnapshot,
	error,
) {
	return &ExecutionDataStorageSnapshot{
		Client:      client,
		Chain:       chain,
		BlockHeight: blockHeight,
	}, nil
}

func (snapshot *ExecutionDataStorageSnapshot) Close() error {
	return nil
}

func (snapshot *ExecutionDataStorageSnapshot) Get(
	id flow.RegisterID,
) (
	value flow.RegisterValue,
	err error,
) {
	owner := []byte(id.Owner)

	req := &executiondata.GetRegisterValuesRequest{
		BlockHeight: snapshot.BlockHeight,
		RegisterIds: []*entities.RegisterID{
			{
				Owner: owner,
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
		switch status.Code(err) {
		case codes.NotFound:
			value = nil
		case codes.InvalidArgument:
			// If the owner address is invalid for the chain, the AN returns InvalidArgument.
			// In this case, we return nil to indicate the register does not exist.
			if !snapshot.Chain.IsValid(flow.BytesToAddress(owner)) {
				value = nil
			} else {
				return nil, err
			}
		default:
			return nil, err
		}
	} else {
		value = resp.Values[0]
	}

	return value, nil
}

type UpdatableStorageSnapshot interface {
	StorageSnapshot
	Set(id flow.RegisterID, value flow.RegisterValue)
}

// CachingStorageSnapshot is a storage snapshot that caches register values
// in memory to avoid repeated calls to the backing snapshot.
type CachingStorageSnapshot struct {
	cache   *InMemoryRegisterCache
	backing StorageSnapshot
}

var _ StorageSnapshot = &CachingStorageSnapshot{}

func NewCachingStorageSnapshot(backing StorageSnapshot) *CachingStorageSnapshot {
	return &CachingStorageSnapshot{
		cache:   NewInMemoryRegisterCache(),
		backing: backing,
	}
}

func (s *CachingStorageSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	data, found := s.cache.Get(id.Key, id.Owner)
	if found {
		return data, nil
	}

	value, err := s.backing.Get(id)
	if err != nil {
		return nil, err
	}

	s.cache.Set(id.Key, id.Owner, value)

	return value, nil
}

func (s *CachingStorageSnapshot) Set(id flow.RegisterID, value flow.RegisterValue) {
	s.cache.Set(id.Key, id.Owner, value)
}

type CapturingStorageSnapshot struct {
	backing StorageSnapshot
	Reads   []struct {
		flow.RegisterID
		flow.RegisterValue
	}
	Writes []struct {
		flow.RegisterID
		flow.RegisterValue
	}
}

var _ UpdatableStorageSnapshot = &CapturingStorageSnapshot{}

func NewCapturingStorageSnapshot(backing StorageSnapshot) *CapturingStorageSnapshot {
	return &CapturingStorageSnapshot{
		backing: backing,
	}
}

func (s *CapturingStorageSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	value, err := s.backing.Get(id)
	if err != nil {
		return nil, err
	}

	s.Reads = append(
		s.Reads,
		struct {
			flow.RegisterID
			flow.RegisterValue
		}{
			RegisterID:    id,
			RegisterValue: value,
		},
	)

	return value, nil
}

func (s *CapturingStorageSnapshot) Set(id flow.RegisterID, value flow.RegisterValue) {
	s.Writes = append(
		s.Writes,
		struct {
			flow.RegisterID
			flow.RegisterValue
		}{
			RegisterID:    id,
			RegisterValue: value,
		},
	)

	if updatableBacking, ok := s.backing.(UpdatableStorageSnapshot); ok {
		updatableBacking.Set(id, value)
	}
}
