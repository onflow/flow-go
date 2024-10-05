package query

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

type EphemeralViewProvider struct {
	chainID         flow.ChainID
	rootAddr        flow.Address
	logger          zerolog.Logger
	storageProvider types.StorageProvider
}

// NewEphemeralViewProvider constructs a new EphemeralViewProvider
func NewEphemeralViewProvider(
	chainID flow.ChainID,
	rootAddr flow.Address,
	sp types.StorageProvider,
	logger zerolog.Logger,
) *EphemeralViewProvider {
	return &EphemeralViewProvider{
		chainID:         chainID,
		storageProvider: sp,
		rootAddr:        rootAddr,
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
		rootAddr: evp.rootAddr,
		storage: storage.NewEphemeralStorage(
			storage.NewReadOnlyStorage(readOnly),
		),
	}, nil
}
