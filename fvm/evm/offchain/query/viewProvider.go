package query

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

type ViewProvider struct {
	chainID         flow.ChainID
	rootAddr        flow.Address
	logger          zerolog.Logger
	storageProvider types.StorageProvider
}

// NewViewProvider constructs a new ViewProvider
func NewViewProvider(
	chainID flow.ChainID,
	rootAddr flow.Address,
	sp types.StorageProvider,
	logger zerolog.Logger,
) *ViewProvider {
	return &ViewProvider{
		chainID:         chainID,
		storageProvider: sp,
		rootAddr:        rootAddr,
		logger:          logger,
	}
}

func (evp *ViewProvider) GetBlockView(height uint64) (*View, error) {
	readOnly, err := evp.storageProvider.GetSnapshotAt(height)
	if err != nil {
		return nil, err
	}
	return &View{
		chainID:  evp.chainID,
		rootAddr: evp.rootAddr,
		storage: storage.NewEphemeralStorage(
			storage.NewReadOnlyStorage(readOnly),
		),
	}, nil
}
