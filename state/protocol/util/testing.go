package util

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
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
	protocol_state "github.com/onflow/flow-go/state/protocol/protocol_state/state"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/util"
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
			last, _ := sealsDB.HighestInFork(candidate.Header.ParentID)
			return last
		},
		func(candidate *flow.Block) error {
			if len(candidate.Payload.Seals) > 0 {
				return nil
			}
			_, err := sealsDB.HighestInFork(candidate.Header.ParentID)
			return err
		}).Maybe()
	return validator
}

func RunWithBootstrapState(t testing.TB, rootSnapshot protocol.Snapshot, f func(*badger.DB, *pbadger.State)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		all := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.Setups,
			all.EpochCommits,
			all.ProtocolState,
			all.ProtocolKVStore,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		f(db, state)
	})
}

func RunWithFullProtocolState(t testing.TB, rootSnapshot protocol.Snapshot, f func(*badger.DB, *pbadger.ParticipantState)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.Setups,
			all.EpochCommits,
			all.ProtocolState,
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

func RunWithFullProtocolStateAndMetrics(t testing.TB, rootSnapshot protocol.Snapshot, metrics module.ComplianceMetrics, f func(*badger.DB, *pbadger.ParticipantState)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.Setups,
			all.EpochCommits,
			all.ProtocolState,
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

func RunWithFullProtocolStateAndValidator(t testing.TB, rootSnapshot protocol.Snapshot, validator module.ReceiptValidator, f func(*badger.DB, *pbadger.ParticipantState)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.Setups,
			all.EpochCommits,
			all.ProtocolState,
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

func RunWithFollowerProtocolState(t testing.TB, rootSnapshot protocol.Snapshot, f func(*badger.DB, *pbadger.FollowerState)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.Setups,
			all.EpochCommits,
			all.ProtocolState,
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

func RunWithFullProtocolStateAndConsumer(t testing.TB, rootSnapshot protocol.Snapshot, consumer protocol.Consumer, f func(*badger.DB, *pbadger.ParticipantState)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		all := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.Setups,
			all.EpochCommits,
			all.ProtocolState,
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

func RunWithFullProtocolStateAndMetricsAndConsumer(t testing.TB, rootSnapshot protocol.Snapshot, metrics module.ComplianceMetrics, consumer protocol.Consumer, f func(*badger.DB, *pbadger.ParticipantState, protocol.MutableProtocolState)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		all := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.Setups,
			all.EpochCommits,
			all.ProtocolState,
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
			all.ProtocolState,
			all.ProtocolKVStore,
			state.Params(),
			all.Headers,
			all.Results,
			all.Setups,
			all.EpochCommits,
		)
		f(db, fullState, mutableProtocolState)
	})
}

func RunWithFollowerProtocolStateAndHeaders(t testing.TB, rootSnapshot protocol.Snapshot, f func(*badger.DB, *pbadger.FollowerState, storage.Headers, storage.Index)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.Setups,
			all.EpochCommits,
			all.ProtocolState,
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

func RunWithFullProtocolStateAndMutator(t testing.TB, rootSnapshot protocol.Snapshot, f func(*badger.DB, *pbadger.ParticipantState, protocol.MutableProtocolState)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(
			metrics,
			db,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.Setups,
			all.EpochCommits,
			all.ProtocolState,
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
			all.ProtocolState,
			all.ProtocolKVStore,
			state.Params(),
			all.Headers,
			all.Results,
			all.Setups,
			all.EpochCommits,
		)
		f(db, fullState, mutableProtocolState)
	})
}
