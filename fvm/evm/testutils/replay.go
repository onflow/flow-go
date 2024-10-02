package testutils

import (
	"fmt"
	"testing"

	"github.com/onflow/atree"
	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
	evmEvents "github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/sync"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

func ValidateEventsReplayability(
	t *testing.T,
	chainID flow.ChainID,
	preSnapshot snapshot.StorageSnapshot,
	transactionEvents []evmEvents.TransactionEventPayload,
	blockEvent evmEvents.BlockEventPayload,
) {
	err := sync.ReplayBlockExecution(
		chainID,
		newStorage(preSnapshot),
		nil,
		transactionEvents,
		blockEvent,
		true,
		func(h gethCommon.Hash) {},
	)
	require.NoError(t, err)
}

type storage struct {
	preSnapshot snapshot.StorageSnapshot
	deltas      map[flow.RegisterID]flow.RegisterValue
}

func newStorage(preSnapshot snapshot.StorageSnapshot) *storage {
	deltas := make(map[flow.RegisterID]flow.RegisterValue)
	return &storage{preSnapshot: preSnapshot, deltas: deltas}
}

var _ types.BackendStorage = &storage{}

func (s *storage) GetValue(owner []byte, key []byte) ([]byte, error) {
	// check delta first
	regID := registerID(owner, key)
	ret, found := s.deltas[regID]
	if found {
		return ret, nil
	}
	return s.preSnapshot.Get(regID)
}

func (s *storage) SetValue(owner, key, value []byte) error {
	s.deltas[registerID(owner, key)] = value
	return nil
}

func (s *storage) ValueExists(owner []byte, key []byte) (bool, error) {
	ret, err := s.GetValue(owner, key)
	return len(ret) > 0, err
}

func (s *storage) AllocateSlabIndex(owner []byte) (atree.SlabIndex, error) {

	statusBytes, err := s.GetValue(owner, []byte(flow.AccountStatusKey))
	if err != nil {
		return atree.SlabIndex{}, err
	}
	if len(statusBytes) == 0 {
		return atree.SlabIndex{}, fmt.Errorf("state for account not found")
	}

	status, err := environment.AccountStatusFromBytes(statusBytes)
	if err != nil {
		return atree.SlabIndex{}, err
	}

	// get and increment the index
	index := status.SlabIndex()
	newIndexBytes := index.Next()

	// update the storageIndex bytes
	status.SetStorageIndex(newIndexBytes)
	err = s.SetValue(owner, []byte(flow.AccountStatusKey), status.ToBytes())
	if err != nil {
		return atree.SlabIndex{}, err
	}
	return index, nil
}

func registerID(owner []byte, key []byte) flow.RegisterID {
	return flow.NewRegisterID(flow.BytesToAddress(owner), string(key))
}
