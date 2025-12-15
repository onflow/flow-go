package complete

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	realWAL "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module"
)

// LedgerWithCompactor wraps a Ledger and its internal Compactor,
// managing both as a single component. This hides the compactor
// as an implementation detail.
type LedgerWithCompactor struct {
	ledger    *Ledger
	compactor *Compactor
	logger    zerolog.Logger
}

// NewLedgerWithCompactor creates a new ledger with an internal compactor.
// The compactor lifecycle is managed by this wrapper.
// Use Ready() to wait for the ledger and compactor to be ready.
func NewLedgerWithCompactor(
	diskWAL realWAL.LedgerWAL,
	ledgerCapacity int,
	compactorConfig *ledger.CompactorConfig,
	metrics module.LedgerMetrics,
	logger zerolog.Logger,
	pathFinderVersion uint8,
) (*LedgerWithCompactor, error) {
	logger = logger.With().Str("ledger_mod", "complete").Logger()

	// Create the ledger
	l, err := NewLedger(diskWAL, ledgerCapacity, metrics, logger, pathFinderVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to create ledger: %w", err)
	}

	// Create the compactor (internal to ledger)
	compactor, err := NewCompactor(
		l,
		diskWAL,
		logger.With().Str("subcomponent", "compactor").Logger(),
		compactorConfig.CheckpointCapacity,
		compactorConfig.CheckpointDistance,
		compactorConfig.CheckpointsToKeep,
		compactorConfig.TriggerCheckpointOnNextSegmentFinish,
		compactorConfig.Metrics,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create compactor: %w", err)
	}

	return &LedgerWithCompactor{
		ledger:    l,
		compactor: compactor,
		logger:    logger,
	}, nil
}

// Implement ledger.Ledger interface - delegate to underlying ledger
func (lwc *LedgerWithCompactor) InitialState() ledger.State {
	return lwc.ledger.InitialState()
}

func (lwc *LedgerWithCompactor) HasState(state ledger.State) bool {
	return lwc.ledger.HasState(state)
}

func (lwc *LedgerWithCompactor) GetSingleValue(query *ledger.QuerySingleValue) (ledger.Value, error) {
	return lwc.ledger.GetSingleValue(query)
}

func (lwc *LedgerWithCompactor) Get(query *ledger.Query) ([]ledger.Value, error) {
	return lwc.ledger.Get(query)
}

func (lwc *LedgerWithCompactor) Set(update *ledger.Update) (ledger.State, *ledger.TrieUpdate, error) {
	return lwc.ledger.Set(update)
}

func (lwc *LedgerWithCompactor) Prove(query *ledger.Query) (ledger.Proof, error) {
	return lwc.ledger.Prove(query)
}

// Ready manages lifecycle of both ledger and compactor.
// Signals when initialization (WAL replay) is complete and compactor is ready.
func (lwc *LedgerWithCompactor) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		defer close(ready)

		// Wait for ledger initialization (WAL replay) to complete
		<-lwc.ledger.Ready()

		// Start compactor
		<-lwc.compactor.Ready()

		lwc.logger.Info().Msg("ledger with compactor ready")
	}()
	return ready
}

// Done manages shutdown of both ledger and compactor.
func (lwc *LedgerWithCompactor) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)

		// Stop compactor first (it needs to finish WAL writes)
		<-lwc.compactor.Done()

		// Then stop ledger
		<-lwc.ledger.Done()

		lwc.logger.Info().Msg("ledger with compactor stopped")
	}()
	return done
}
