package util

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	pbadger "github.com/onflow/flow-go/state/protocol/pebble"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/procedure"
	"github.com/onflow/flow-go/storage/testingutils"
	"github.com/onflow/flow-go/utils/unittest"
)

func RunWithPebbleBootstrapState(t testing.TB, rootSnapshot protocol.Snapshot, f func(*pebble.DB, *pbadger.State)) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		all := testingutils.PebbleStorageLayer(t, db)
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
			all.Statuses,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		f(db, state)
	})
}

func RunWithPebbleFullProtocolState(t testing.TB, rootSnapshot protocol.Snapshot, f func(*pebble.DB, *pbadger.ParticipantState)) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := testingutils.PebbleStorageLayer(t, db)
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
			all.Statuses,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		receiptValidator := MockReceiptValidator()
		sealValidator := MockSealValidator(all.Seals)
		mockTimer := MockBlockTimer()
		fullState, err := pbadger.NewFullConsensusState(log, tracer, consumer, state, all.Index, all.Payloads, procedure.NewBlockIndexer(), mockTimer, receiptValidator, sealValidator)
		require.NoError(t, err)
		f(db, fullState)
	})
}

func RunWithPebbleFullProtocolStateAndMetrics(t testing.TB, rootSnapshot protocol.Snapshot, metrics module.ComplianceMetrics, f func(*pebble.DB, *pbadger.ParticipantState)) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := testingutils.PebbleStorageLayer(t, db)
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
			all.Statuses,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		receiptValidator := MockReceiptValidator()
		sealValidator := MockSealValidator(all.Seals)
		mockTimer := MockBlockTimer()
		fullState, err := pbadger.NewFullConsensusState(log, tracer, consumer, state, all.Index, all.Payloads, procedure.NewBlockIndexer(), mockTimer, receiptValidator, sealValidator)
		require.NoError(t, err)
		f(db, fullState)
	})
}

func RunWithPebbleFullProtocolStateAndValidator(t testing.TB, rootSnapshot protocol.Snapshot, validator module.ReceiptValidator, f func(*pebble.DB, *pbadger.ParticipantState)) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := testingutils.PebbleStorageLayer(t, db)
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
			all.Statuses,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		sealValidator := MockSealValidator(all.Seals)
		mockTimer := MockBlockTimer()
		fullState, err := pbadger.NewFullConsensusState(log, tracer, consumer, state, all.Index, all.Payloads, procedure.NewBlockIndexer(), mockTimer, validator, sealValidator)
		require.NoError(t, err)
		f(db, fullState)
	})
}

func RunWithPebbleFollowerProtocolState(t testing.TB, rootSnapshot protocol.Snapshot, f func(*pebble.DB, *pbadger.FollowerState)) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := testingutils.PebbleStorageLayer(t, db)
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
			all.Statuses,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		mockTimer := MockBlockTimer()
		followerState, err := pbadger.NewFollowerState(log, tracer, consumer, state, all.Index, all.Payloads, mockTimer)
		require.NoError(t, err)
		f(db, followerState)
	})
}

func RunWithPebbleFullProtocolStateAndConsumer(t testing.TB, rootSnapshot protocol.Snapshot, consumer protocol.Consumer, f func(*pebble.DB, *pbadger.ParticipantState)) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		all := testingutils.PebbleStorageLayer(t, db)
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
			all.Statuses,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		receiptValidator := MockReceiptValidator()
		sealValidator := MockSealValidator(all.Seals)
		mockTimer := MockBlockTimer()
		fullState, err := pbadger.NewFullConsensusState(log, tracer, consumer, state, all.Index, all.Payloads, procedure.NewBlockIndexer(), mockTimer, receiptValidator, sealValidator)
		require.NoError(t, err)
		f(db, fullState)
	})
}

func RunWithPebbleFullProtocolStateAndMetricsAndConsumer(t testing.TB, rootSnapshot protocol.Snapshot, metrics module.ComplianceMetrics, consumer protocol.Consumer, f func(*pebble.DB, *pbadger.ParticipantState)) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		all := testingutils.PebbleStorageLayer(t, db)
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
			all.Statuses,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		receiptValidator := MockReceiptValidator()
		sealValidator := MockSealValidator(all.Seals)
		mockTimer := MockBlockTimer()
		fullState, err := pbadger.NewFullConsensusState(log, tracer, consumer, state, all.Index, all.Payloads, procedure.NewBlockIndexer(), mockTimer, receiptValidator, sealValidator)
		require.NoError(t, err)
		f(db, fullState)
	})
}

func RunWithPebbleFollowerProtocolStateAndHeaders(t testing.TB, rootSnapshot protocol.Snapshot, f func(*pebble.DB, *pbadger.FollowerState, storage.Headers, storage.Index)) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := zerolog.Nop()
		consumer := events.NewNoop()
		all := testingutils.PebbleStorageLayer(t, db)
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
			all.Statuses,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		mockTimer := MockBlockTimer()
		followerState, err := pbadger.NewFollowerState(log, tracer, consumer, state, all.Index, all.Payloads, mockTimer)
		require.NoError(t, err)
		f(db, followerState, all.Headers, all.Index)
	})
}
