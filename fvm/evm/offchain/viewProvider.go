package sync

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/model/flow"
)

type EphemeralViewProvider struct {
	chainID         flow.ChainID
	logger          zerolog.Logger
	storageProvider StorageProvider
}

// NewEphemeralViewProvider constructs a new EphemeralViewProvider
func NewEphemeralViewProvider(
	chainID flow.ChainID,
	sp StorageProvider,
	logger zerolog.Logger,
) *EphemeralViewProvider {
	return &EphemeralViewProvider{
		chainID:         chainID,
		storageProvider: sp,
		logger:          logger,
	}
}

func (evp *EphemeralViewProvider) GetBlockView(height uint64) (*EphemeralView, error) {
	readOnly, err := evp.storageProvider.GetSnapshotAt(height)
	if err != nil {
		return nil, err
	}
	return &EphemeralView{
		chainID:  evp.chainID,
		rootAddr: evm.StorageAccountAddress(evp.chainID),
		storage:  NewEphemeralStorage(NewReadOnlyStorage(readOnly)),
	}, nil
}
