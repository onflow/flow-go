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
	blockProvider   types.BlockSnapshotProvider
	maxCallGasLimit uint64
}

// NewViewProvider constructs a new ViewProvider
func NewViewProvider(
	chainID flow.ChainID,
	rootAddr flow.Address,
	sp types.StorageProvider,
	bp types.BlockSnapshotProvider,
	maxCallGasLimit uint64,
) *ViewProvider {
	return &ViewProvider{
		chainID:         chainID,
		storageProvider: sp,
		blockProvider:   bp,
		rootAddr:        rootAddr,
		maxCallGasLimit: maxCallGasLimit,
	}
}

// GetBlockView returns the block view for the given height (at the end of a block!)
// The `GetSnapshotAt` function of `storageProvider`, will return
// the block state at its start, before any transaction executions.
// This is the intended functionality, when replaying & verifying blocks.
// However, when reading the state from a block, we are interested
// in its end state, after all transaction executions.
// That is why we fetch the block snapshot at the next height.
func (evp *ViewProvider) GetBlockView(height uint64) (*View, error) {
	readOnly, err := evp.storageProvider.GetSnapshotAt(height + 1)
	if err != nil {
		return nil, err
	}
	blockSnapshot, err := evp.blockProvider.GetSnapshotAt(height)
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
		blockSnapshot: blockSnapshot,
	}, nil
}
