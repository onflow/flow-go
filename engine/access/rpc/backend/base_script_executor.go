package backend

import (
	"context"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/common/version"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/storage"
)

// baseScriptExecutor holds common logic for all ScriptExecutors.
type baseScriptExecutor struct {
	log zerolog.Logger

	// scripts is used to interact with execution state
	scripts *execution.Scripts

	// versionControl provides information about the current version beacon for each block
	versionControl *version.VersionControl

	// minCompatibleHeight and maxCompatibleHeight define block range compatible with node version
	minCompatibleHeight *atomic.Uint64
	maxCompatibleHeight *atomic.Uint64
}

func NewBaseScriptExecutor(log zerolog.Logger, minHeight, maxHeight uint64) *baseScriptExecutor {
	log.Info().
		Uint64("min_height", minHeight).
		Uint64("max_height", maxHeight).
		Msg("base script executor created")

	return &baseScriptExecutor{
		log:                 log,
		minCompatibleHeight: atomic.NewUint64(minHeight),
		maxCompatibleHeight: atomic.NewUint64(maxHeight),
	}
}

// SetMinCompatibleHeight sets the lowest block height (inclusive).
func (b *baseScriptExecutor) SetMinCompatibleHeight(height uint64) {
	b.minCompatibleHeight.Store(height)
	b.log.Info().Uint64("height", height).Msg("minimum compatible height set")
}

// SetMaxCompatibleHeight sets the highest block height (inclusive).
func (b *baseScriptExecutor) SetMaxCompatibleHeight(height uint64) {
	b.maxCompatibleHeight.Store(height)
	b.log.Info().Uint64("height", height).Msg("maximum compatible height set")
}

func (b *baseScriptExecutor) ExecuteAtBlockHeight(ctx context.Context, script []byte, arguments [][]byte, height uint64, registers storage.RegisterIndexReader) ([]byte, error) {
	return b.scripts.ExecuteAtBlockHeight(ctx, script, arguments, height, registers)
}

func (b *baseScriptExecutor) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64, registers storage.RegisterIndexReader) (*flow.Account, error) {
	return b.scripts.GetAccountAtBlockHeight(ctx, address, height, registers)
}

func (b *baseScriptExecutor) GetAccountBalance(ctx context.Context, address flow.Address, height uint64, registers storage.RegisterIndexReader) (uint64, error) {
	return b.scripts.GetAccountBalance(ctx, address, height, registers)
}

func (b *baseScriptExecutor) GetAccountAvailableBalance(ctx context.Context, address flow.Address, height uint64, registers storage.RegisterIndexReader) (uint64, error) {
	return b.scripts.GetAccountAvailableBalance(ctx, address, height, registers)
}

func (b *baseScriptExecutor) GetAccountKeys(ctx context.Context, address flow.Address, height uint64, registers storage.RegisterIndexReader) ([]flow.AccountPublicKey, error) {
	return b.scripts.GetAccountKeys(ctx, address, height, registers)
}

func (b *baseScriptExecutor) GetAccountKey(ctx context.Context, address flow.Address, keyIndex uint32, height uint64, registers storage.RegisterIndexReader) (*flow.AccountPublicKey, error) {
	return b.scripts.GetAccountKey(ctx, address, keyIndex, height, registers)
}
