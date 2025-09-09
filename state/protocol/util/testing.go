package util

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	mmetrics "github.com/onflow/flow-go/module/metrics"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	pbadger "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	protocol_state "github.com/onflow/flow-go/state/protocol/protocol_state/state"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
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
			last, _ := sealsDB.HighestInFork(candidate.ParentID)
			return last
		},
		func(candidate *flow.Block) error {
			if len(candidate.Payload.Seals) > 0 {
				return nil
			}
			_, err := sealsDB.HighestInFork(candidate.ParentID)
			return err
		}).Maybe()
	return validator
}

func RunWithBootstrapState(t testing.TB, rootSnapshot protocol.Snapshot, f func(storage.DB, *pbadger.State)) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		db := pebbleimpl.ToDB(pdb)
		metrics := metrics.NewNoopCollector()
		all := store.InitAll(metrics, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			lockManager,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.EpochSetups,
			all.EpochCommits,
			all.EpochProtocolStateEntries,
			all.ProtocolKVStore,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		f(db, state)
	})
}

func RunWithFullProtocolState(t testing.TB, rootSnapshot protocol.Snapshot, f func(storage.DB, *pbadger.ParticipantState)) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		db := pebbleimpl.ToDB(pdb)
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := store.InitAll(metrics, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			lockManager,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.EpochSetups,
			all.EpochCommits,
			all.EpochProtocolStateEntries,
			all.ProtocolKVStore,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		receiptValidator := MockReceiptValidator()
		sealValidator := MockSealValidator(all.Seals)
		mockTimer := MockBlockTimer()
		fullState, err := pbadger.NewFullConsensusState(
			log,
			tracer,
			consumer,
			state,
			all.Index,
			all.Payloads,
			mockTimer,
			receiptValidator,
			sealValidator,
		)
		require.NoError(t, err)
		f(db, fullState)
	})
}

func RunWithFullProtocolStateAndMetrics(t testing.TB, rootSnapshot protocol.Snapshot, metrics module.ComplianceMetrics, f func(storage.DB, *pbadger.ParticipantState)) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		db := pebbleimpl.ToDB(pdb)
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := store.InitAll(mmetrics.NewNoopCollector(), db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			lockManager,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.EpochSetups,
			all.EpochCommits,
			all.EpochProtocolStateEntries,
			all.ProtocolKVStore,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		receiptValidator := MockReceiptValidator()
		sealValidator := MockSealValidator(all.Seals)
		mockTimer := MockBlockTimer()

		fullState, err := pbadger.NewFullConsensusState(
			log,
			tracer,
			consumer,
			state,
			all.Index,
			all.Payloads,
			mockTimer,
			receiptValidator,
			sealValidator,
		)
		require.NoError(t, err)
		f(db, fullState)
	})
}

func RunWithFullProtocolStateAndValidator(t testing.TB, rootSnapshot protocol.Snapshot, validator module.ReceiptValidator, f func(storage.DB, *pbadger.ParticipantState)) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		db := pebbleimpl.ToDB(pdb)
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := store.InitAll(metrics, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			lockManager,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.EpochSetups,
			all.EpochCommits,
			all.EpochProtocolStateEntries,
			all.ProtocolKVStore,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		sealValidator := MockSealValidator(all.Seals)
		mockTimer := MockBlockTimer()
		fullState, err := pbadger.NewFullConsensusState(
			log,
			tracer,
			consumer,
			state,
			all.Index,
			all.Payloads,
			mockTimer,
			validator,
			sealValidator,
		)
		require.NoError(t, err)
		f(db, fullState)
	})
}

func RunWithFollowerProtocolState(t testing.TB, rootSnapshot protocol.Snapshot, f func(storage.DB, *pbadger.FollowerState)) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		db := pebbleimpl.ToDB(pdb)
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := store.InitAll(metrics, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			lockManager,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.EpochSetups,
			all.EpochCommits,
			all.EpochProtocolStateEntries,
			all.ProtocolKVStore,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		mockTimer := MockBlockTimer()
		followerState, err := pbadger.NewFollowerState(
			log,
			tracer,
			consumer,
			state,
			all.Index,
			all.Payloads,
			mockTimer,
		)
		require.NoError(t, err)
		f(db, followerState)
	})
}

func RunWithFullProtocolStateAndConsumer(t testing.TB, rootSnapshot protocol.Snapshot, consumer protocol.Consumer, f func(storage.DB, *pbadger.ParticipantState)) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		db := pebbleimpl.ToDB(pdb)
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		all := store.InitAll(metrics, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			lockManager,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.EpochSetups,
			all.EpochCommits,
			all.EpochProtocolStateEntries,
			all.ProtocolKVStore,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		receiptValidator := MockReceiptValidator()
		sealValidator := MockSealValidator(all.Seals)
		mockTimer := MockBlockTimer()
		fullState, err := pbadger.NewFullConsensusState(
			log,
			tracer,
			consumer,
			state,
			all.Index,
			all.Payloads,
			mockTimer,
			receiptValidator,
			sealValidator,
		)
		require.NoError(t, err)
		f(db, fullState)
	})
}

func RunWithFullProtocolStateAndMetricsAndConsumer(t testing.TB, rootSnapshot protocol.Snapshot, metrics module.ComplianceMetrics, consumer protocol.Consumer, f func(storage.DB, *pbadger.ParticipantState, protocol.MutableProtocolState)) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		db := pebbleimpl.ToDB(pdb)
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		all := store.InitAll(mmetrics.NewNoopCollector(), db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			lockManager,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.EpochSetups,
			all.EpochCommits,
			all.EpochProtocolStateEntries,
			all.ProtocolKVStore,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		receiptValidator := MockReceiptValidator()
		sealValidator := MockSealValidator(all.Seals)
		mockTimer := MockBlockTimer()
		fullState, err := pbadger.NewFullConsensusState(
			log,
			tracer,
			consumer,
			state,
			all.Index,
			all.Payloads,
			mockTimer,
			receiptValidator,
			sealValidator,
		)
		require.NoError(t, err)
		mutableProtocolState := protocol_state.NewMutableProtocolState(
			log,
			all.EpochProtocolStateEntries,
			all.ProtocolKVStore,
			state.Params(),
			all.Headers,
			all.Results,
			all.EpochSetups,
			all.EpochCommits,
		)
		f(db, fullState, mutableProtocolState)
	})
}

func RunWithFollowerProtocolStateAndHeaders(t testing.TB, rootSnapshot protocol.Snapshot, f func(storage.DB, *pbadger.FollowerState, storage.Headers, storage.Index)) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		db := pebbleimpl.ToDB(pdb)
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := store.InitAll(metrics, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			lockManager,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.EpochSetups,
			all.EpochCommits,
			all.EpochProtocolStateEntries,
			all.ProtocolKVStore,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		mockTimer := MockBlockTimer()
		followerState, err := pbadger.NewFollowerState(
			log,
			tracer,
			consumer,
			state,
			all.Index,
			all.Payloads,
			mockTimer,
		)
		require.NoError(t, err)
		f(db, followerState, all.Headers, all.Index)
	})
}

func RunWithFullProtocolStateAndMutator(t testing.TB, rootSnapshot protocol.Snapshot, f func(storage.DB, *pbadger.ParticipantState, protocol.MutableProtocolState)) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		db := pebbleimpl.ToDB(pdb)
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := store.InitAll(metrics, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			lockManager,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.EpochSetups,
			all.EpochCommits,
			all.EpochProtocolStateEntries,
			all.ProtocolKVStore,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		receiptValidator := MockReceiptValidator()
		sealValidator := MockSealValidator(all.Seals)
		mockTimer := MockBlockTimer()
		fullState, err := pbadger.NewFullConsensusState(
			log,
			tracer,
			consumer,
			state,
			all.Index,
			all.Payloads,
			mockTimer,
			receiptValidator,
			sealValidator,
		)
		require.NoError(t, err)

		mutableProtocolState := protocol_state.NewMutableProtocolState(
			log,
			all.EpochProtocolStateEntries,
			all.ProtocolKVStore,
			state.Params(),
			all.Headers,
			all.Results,
			all.EpochSetups,
			all.EpochCommits,
		)
		f(db, fullState, mutableProtocolState)
	})
}
