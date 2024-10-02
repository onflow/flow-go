package sync

import (
	"github.com/onflow/flow-go/model/flow"
	gethTracers "github.com/onflow/go-ethereum/eth/tracers"
	"github.com/rs/zerolog"
)

// DryRunner allows dry run a call against a block height and a tx index
type DryRunner struct {
	chainID         flow.ChainID
	logger          zerolog.Logger
	storageProvider StorageProvider
	tracer          *gethTracers.Tracer
}

// NewDryRunner constructs a new DryRunner
func NewDryRunner(
	chainID flow.ChainID,
	sp StorageProvider,
	logger zerolog.Logger,
	tracer *gethTracers.Tracer,
) *ChainReplayer {
	return &ChainReplayer{
		chainID:         chainID,
		storageProvider: sp,
		logger:          logger,
		tracer:          tracer,
	}
}
