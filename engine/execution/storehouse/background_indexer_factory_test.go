package storehouse_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol/events"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestLoadRegisterStore_Disabled tests that LoadRegisterStore returns nil when enableStorehouse is false
func TestLoadRegisterStore_Disabled(t *testing.T) {
	t.Parallel()

	log := unittest.Logger()
	state := protocolmock.NewState(t)
	headers := storagemock.NewHeaders(t)
	protocolEvents := events.NewDistributor()
	lastFinalizedHeight := uint64(100)
	collector := &metrics.NoopCollector{}
	enableStorehouse := false
	registerDir := t.TempDir()
	triedir := t.TempDir()
	importCheckpointWorkerCount := 1
	var importFunc storehouse.ImportRegistersFromCheckpoint = nil

	registerStore, closer, err := storehouse.LoadRegisterStore(
		log,
		state,
		headers,
		protocolEvents,
		lastFinalizedHeight,
		collector,
		enableStorehouse,
		registerDir,
		triedir,
		importCheckpointWorkerCount,
		importFunc,
	)

	require.NoError(t, err)
	require.Nil(t, registerStore)
	require.Nil(t, closer)
}

// TestLoadBackgroundIndexerEngine_StorehouseEnabled tests that LoadBackgroundIndexerEngine returns nil when enableStorehouse is true
func TestLoadBackgroundIndexerEngine_StorehouseEnabled(t *testing.T) {
	t.Parallel()

	log := unittest.Logger()
	enableStorehouse := true
	enableBackgroundStorehouseIndexing := true
	state := protocolmock.NewState(t)
	headers := storagemock.NewHeaders(t)
	protocolEvents := events.NewDistributor()
	lastFinalizedHeight := uint64(100)
	collector := &metrics.NoopCollector{}
	registerDir := t.TempDir()
	triedir := t.TempDir()
	importCheckpointWorkerCount := 1
	var importFunc storehouse.ImportRegistersFromCheckpoint = nil
	var executionDataStore execution_data.ExecutionDataGetter = nil
	resultsReader := storagemock.NewExecutionResults(t)
	var blockExecutedNotifier storehouse.BlockExecutedNotifier = nil
	followerDistributor := pubsub.NewFollowerDistributor()

	engine, created, err := storehouse.LoadBackgroundIndexerEngine(
		log,
		enableStorehouse,
		enableBackgroundStorehouseIndexing,
		state,
		headers,
		protocolEvents,
		lastFinalizedHeight,
		collector,
		registerDir,
		triedir,
		importCheckpointWorkerCount,
		importFunc,
		executionDataStore,
		resultsReader,
		blockExecutedNotifier,
		followerDistributor,
	)

	require.NoError(t, err)
	require.Nil(t, engine)
	require.False(t, created)
}

// TestLoadBackgroundIndexerEngine_BackgroundIndexingDisabled tests that LoadBackgroundIndexerEngine returns nil when enableBackgroundStorehouseIndexing is false
func TestLoadBackgroundIndexerEngine_BackgroundIndexingDisabled(t *testing.T) {
	t.Parallel()

	log := unittest.Logger()
	enableStorehouse := false
	enableBackgroundStorehouseIndexing := false
	state := protocolmock.NewState(t)
	headers := storagemock.NewHeaders(t)
	protocolEvents := events.NewDistributor()
	lastFinalizedHeight := uint64(100)
	collector := &metrics.NoopCollector{}
	registerDir := t.TempDir()
	triedir := t.TempDir()
	importCheckpointWorkerCount := 1
	var importFunc storehouse.ImportRegistersFromCheckpoint = nil
	var executionDataStore execution_data.ExecutionDataGetter = nil
	resultsReader := storagemock.NewExecutionResults(t)
	var blockExecutedNotifier storehouse.BlockExecutedNotifier = nil
	followerDistributor := pubsub.NewFollowerDistributor()

	engine, created, err := storehouse.LoadBackgroundIndexerEngine(
		log,
		enableStorehouse,
		enableBackgroundStorehouseIndexing,
		state,
		headers,
		protocolEvents,
		lastFinalizedHeight,
		collector,
		registerDir,
		triedir,
		importCheckpointWorkerCount,
		importFunc,
		executionDataStore,
		resultsReader,
		blockExecutedNotifier,
		followerDistributor,
	)

	require.NoError(t, err)
	require.Nil(t, engine)
	require.False(t, created)
}
