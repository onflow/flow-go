package testutils

import (
	"testing"

	"github.com/onflow/atree"
	"github.com/stretchr/testify/require"

	evmEvents "github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/offchain/sync"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

func ValidateEventsReplayability(
	t *testing.T,
	chainID flow.ChainID,
	preSnapshot snapshot.StorageSnapshot,
	transactionEvents []evmEvents.TransactionEventPayload,
	blockEvent *evmEvents.BlockEventPayload,
) {
	_, err := sync.ReplayBlockExecution(
		chainID,
		newSnapShotWrapper(preSnapshot),
		nil,
		transactionEvents,
		blockEvent,
		true,
	)
	require.NoError(t, err)
}

// snapShotWrapper wraps an snapshot and provides a backend storage
type snapShotWrapper struct {
	preSnapshot snapshot.StorageSnapshot
}

func newSnapShotWrapper(preSnapshot snapshot.StorageSnapshot) *snapShotWrapper {
	return &snapShotWrapper{preSnapshot: preSnapshot}
}

var _ types.BackendStorage = &snapShotWrapper{}

func (s *snapShotWrapper) GetValue(owner []byte, key []byte) ([]byte, error) {
	regID := storage.RegisterID(owner, key)
	return s.preSnapshot.Get(regID)
}

func (s *snapShotWrapper) ValueExists(owner []byte, key []byte) (bool, error) {
	ret, err := s.GetValue(owner, key)
	return len(ret) > 0, err
}

func (s *snapShotWrapper) SetValue(owner []byte, key []byte, value []byte) error {
	panic("not supported")
}

func (s *snapShotWrapper) AllocateSlabIndex(owner []byte) (atree.SlabIndex, error) {
	panic("not supported")
}
