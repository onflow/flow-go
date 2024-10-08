package query

import (
	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// ViewProvider constructs views
// based on the requirements
type ViewProvider struct {
	chainID         flow.ChainID
	rootAddr        flow.Address
	storageProvider types.StorageProvider
	maxCallGasLimit uint64
}

// NewViewProvider constructs a new ViewProvider
func NewViewProvider(
	chainID flow.ChainID,
	rootAddr flow.Address,
	sp types.StorageProvider,
	maxCallGasLimit uint64,
) *ViewProvider {
	return &ViewProvider{
		chainID:         chainID,
		storageProvider: sp,
		rootAddr:        rootAddr,
		maxCallGasLimit: maxCallGasLimit,
	}
}

// GetBlockView returns the block view for the given height
func (evp *ViewProvider) GetBlockView(height uint64) (*View, error) {
	readOnly, err := evp.storageProvider.GetSnapshotAt(height)
	if err != nil {
		return nil, err
	}
	return &View{
		chainID:         evp.chainID,
		rootAddr:        evp.rootAddr,
		maxCallGasLimit: evp.maxCallGasLimit,
		storage: storage.NewEphemeralStorage(
			storage.NewReadOnlyStorage(readOnly),
		),
	}, nil
}
