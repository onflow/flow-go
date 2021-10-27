package util

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	pbadger "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/unittest"

	bstorage "github.com/onflow/flow-go/storage/badger"
)

// MockReceiptValidator returns a ReceiptValidator that accepts
// all receipts without performing any
// integrity checks.
func MockReceiptValidator() module.ReceiptValidator {
	validator := &modulemock.ReceiptValidator{}
	validator.On("Validate", mock.Anything).Return(nil)
	validator.On("ValidatePayload", mock.Anything).Return(nil)
	return validator
}

// MockBlockTimer returns BlockTimer that accepts all timestamps
// without performing any checks.
func MockBlockTimer() protocol.BlockTimer {
	blockTimer := &mockprotocol.BlockTimer{}
	blockTimer.On("Validate", mock.Anything, mock.Anything).Return(nil)
	return blockTimer
}

// MockSealValidator returns a SealValidator that accepts
// all seals without performing any
// integrity checks, returns first seal in block as valid one
func MockSealValidator(sealsDB storage.Seals) module.SealValidator {
	validator := &modulemock.SealValidator{}
	validator.On("Validate", mock.Anything).Return(
		func(candidate *flow.Block) *flow.Seal {
			if len(candidate.Payload.Seals) > 0 {
				return candidate.Payload.Seals[0]
			}
			last, _ := sealsDB.ByBlockID(candidate.Header.ParentID)
			return last
		},
		func(candidate *flow.Block) error {
			if len(candidate.Payload.Seals) > 0 {
				return nil
			}
			_, err := sealsDB.ByBlockID(candidate.Header.ParentID)
			return err
		}).Maybe()
	return validator
}

func RunWithBootstrapState(t testing.TB, rootSnapshot protocol.Snapshot, f func(*badger.DB, *pbadger.State)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		headers, _, seals, _, _, blocks, setups, commits, statuses, results := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(metrics, db, headers, seals, results, blocks, setups, commits, statuses, rootSnapshot)
		require.NoError(t, err)
		f(db, state)
	})
}

// RunWithFollowerProtocolStateAndResults will insert some results into results db. Used to test sealing segment edge cases.
func RunWithFollowerProtocolStateAndResults(t testing.TB, rootSnapshot protocol.Snapshot, exeResults []*flow.ExecutionResult, f func(*badger.DB, *pbadger.FollowerState)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		consumer := events.NewNoop()
		headers, guarantees, seals, index, _, blocks, setups, commits, statuses, results := util.StorageLayer(t, db)
		for _, r := range exeResults {
			err := results.Store(r)
			require.NoError(t, err)
		}

		receipts := bstorage.NewExecutionReceipts(metrics, db, results, bstorage.DefaultCacheSize)
		payloads := bstorage.NewPayloads(db, index, guarantees, seals, receipts, results)

		state, err := pbadger.Bootstrap(metrics, db, headers, seals, results, blocks, setups, commits, statuses, rootSnapshot)
		require.NoError(t, err)
		mockTimer := MockBlockTimer()
		followerState, err := pbadger.NewFollowerState(state, index, payloads, tracer, consumer, mockTimer)
		require.NoError(t, err)
		f(db, followerState)
	})
}

func RunWithFullProtocolState(t testing.TB, rootSnapshot protocol.Snapshot, f func(*badger.DB, *pbadger.MutableState)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		consumer := events.NewNoop()
		headers, _, seals, index, payloads, blocks, setups, commits, statuses, results := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(metrics, db, headers, seals, results, blocks, setups, commits, statuses, rootSnapshot)
		require.NoError(t, err)
		receiptValidator := MockReceiptValidator()
		sealValidator := MockSealValidator(seals)
		mockTimer := MockBlockTimer()
		fullState, err := pbadger.NewFullConsensusState(state, index, payloads, tracer, consumer, mockTimer, receiptValidator, sealValidator)
		require.NoError(t, err)
		f(db, fullState)
	})
}

func RunWithFullProtocolStateAndValidator(t testing.TB, rootSnapshot protocol.Snapshot, validator module.ReceiptValidator, f func(*badger.DB, *pbadger.MutableState)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		consumer := events.NewNoop()
		headers, _, seals, index, payloads, blocks, setups, commits, statuses, results := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(metrics, db, headers, seals, results, blocks, setups, commits, statuses, rootSnapshot)
		require.NoError(t, err)
		sealValidator := MockSealValidator(seals)
		mockTimer := MockBlockTimer()
		fullState, err := pbadger.NewFullConsensusState(state, index, payloads, tracer, consumer, mockTimer, validator, sealValidator)
		require.NoError(t, err)
		f(db, fullState)
	})
}

func RunWithFollowerProtocolState(t testing.TB, rootSnapshot protocol.Snapshot, f func(*badger.DB, *pbadger.FollowerState)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		consumer := events.NewNoop()
		headers, _, seals, index, payloads, blocks, setups, commits, statuses, results := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(metrics, db, headers, seals, results, blocks, setups, commits, statuses, rootSnapshot)
		require.NoError(t, err)
		mockTimer := MockBlockTimer()
		followerState, err := pbadger.NewFollowerState(state, index, payloads, tracer, consumer, mockTimer)
		require.NoError(t, err)
		f(db, followerState)
	})
}

func RunWithFullProtocolStateAndConsumer(t testing.TB, rootSnapshot protocol.Snapshot, consumer protocol.Consumer, f func(*badger.DB, *pbadger.MutableState)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		headers, _, seals, index, payloads, blocks, setups, commits, statuses, results := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(metrics, db, headers, seals, results, blocks, setups, commits, statuses, rootSnapshot)
		require.NoError(t, err)
		receiptValidator := MockReceiptValidator()
		sealValidator := MockSealValidator(seals)
		mockTimer := MockBlockTimer()
		fullState, err := pbadger.NewFullConsensusState(state, index, payloads, tracer, consumer, mockTimer, receiptValidator, sealValidator)
		require.NoError(t, err)
		f(db, fullState)
	})
}

func RunWithFollowerProtocolStateAndHeaders(t testing.TB, rootSnapshot protocol.Snapshot, f func(*badger.DB, *pbadger.FollowerState, storage.Headers, storage.Index)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		consumer := events.NewNoop()
		headers, _, seals, index, payloads, blocks, setups, commits, statuses, results := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(metrics, db, headers, seals, results, blocks, setups, commits, statuses, rootSnapshot)
		require.NoError(t, err)
		mockTimer := MockBlockTimer()
		followerState, err := pbadger.NewFollowerState(state, index, payloads, tracer, consumer, mockTimer)
		require.NoError(t, err)
		f(db, followerState, headers, index)
	})
}
