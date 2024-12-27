package pebble

import (
	"context"
	"fmt"
	"testing"
	"time"

	"golang.org/x/exp/rand"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	latestHeight   uint64 = 12
	pruneHeight    uint64 = 7
	pruneThreshold uint64 = 5
)

// testCase defines the structure for a single test case, including initial data setup,
// expected data after pruning, and the data that should be pruned from the database.
type testCase struct {
	name string
	// The initial register data for the test.
	initialData map[uint64]flow.RegisterEntries
	// The expected first register height in the database after pruning.
	expectedFirstHeight uint64
	// The data that is expected to be present in the database after pruning.
	expectedData map[uint64]flow.RegisterEntries
	// The data that should be pruned (i.e., removed) from the database after pruning.
	prunedData map[uint64]flow.RegisterEntries
}

// TestPrune validates the pruning functionality of the RegisterPruner.
//
// It runs multiple test cases, each of which initializes the database with specific
// register entries and then verifies that the pruning operation behaves as expected.
// The test cases check that:
// - Register entries below a certain height are pruned (i.e., removed) from the database.
// - The remaining data in the database matches the expected state after pruning.
// - The first height of the register entries in the database is correct after pruning.
//
// The test cases include:
// - Straight pruning, where register entries are pruned up to a specific height.
// - Pruning with different entries at varying heights, ensuring only the correct entries are kept.
func TestPrune(t *testing.T) {
	// Set up the test case with initial data, expected outcomes, and pruned data.
	straightPruneData := straightPruneTestCase()
	testCaseWithDiffData := testCaseWithDiffHeights()

	tests := []testCase{
		{
			name:                "straight pruning to a pruned height",
			initialData:         straightPruneData.initialData,
			expectedFirstHeight: straightPruneData.expectedFirstHeight,
			expectedData:        straightPruneData.expectedData,
			prunedData:          straightPruneData.prunedData,
		},
		{
			name:                "pruning with different entries to keep",
			initialData:         testCaseWithDiffData.initialData,
			expectedFirstHeight: testCaseWithDiffData.expectedFirstHeight,
			expectedData:        testCaseWithDiffData.expectedData,
			prunedData:          testCaseWithDiffData.prunedData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run the test with the provided initial data and Register storage
			RunWithRegistersStorageWithInitialData(t, tt.initialData, func(db *pebble.DB) {
				pruner, err := NewRegisterPruner(
					zerolog.Nop(),
					db,
					metrics.NewNoopCollector(),
					WithPruneThreshold(pruneThreshold),
					WithPruneTickerInterval(10*time.Millisecond),
				)
				require.NoError(t, err)

				ctx, cancel := context.WithCancel(context.Background())
				signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

				// Start the pruning process
				pruner.Start(signalerCtx)

				// Ensure pruning happens and the first height after pruning is as expected.
				requirePruning(t, db, tt.expectedFirstHeight)

				// Clean up pruner and check for any errors.
				cleanupPruner(t, pruner, cancel, errChan)

				// Verify that the data in the database matches the expected and pruned data.
				verifyData(t, db, tt)
			})
		})
	}
}

// TestRemainingValuesAfterPruning tests that pruning does not affect values above the pruned height.
//
// This test covers scenarios where:
// - The value entries in the database remains the same after pruning process
func TestRemainingValuesAfterPruning(t *testing.T) {
	// Generate random initial data across a range of heights.
	testData := generateRandomRegisterData()

	RunWithRegistersStorageWithInitialData(t, testData.initialData, func(db *pebble.DB) {
		pruner, err := NewRegisterPruner(
			zerolog.Nop(),
			db,
			metrics.NewNoopCollector(),
			WithPruneThreshold(pruneThreshold),
			WithPruneTickerInterval(10*time.Millisecond),
		)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

		// Start the pruning process.
		pruner.Start(signalerCtx)

		// Wait until pruning completes and verify that values above prune height remain unchanged.
		requirePruning(t, db, testData.expectedFirstHeight)

		// Clean up pruner and check for any errors.
		cleanupPruner(t, pruner, cancel, errChan)

		// Check that all heights above the prune height have unchanged values.
		for height := pruneHeight + 1; height <= latestHeight; height++ {
			entries, exists := testData.expectedData[height]
			if !exists {
				continue
			}

			for _, entry := range entries {
				val, closer, err := db.Get(newLookupKey(height, entry.Key).Bytes())
				require.NoError(t, err)
				require.Equal(t, entry.Value, val)
				require.NoError(t, closer.Close())
			}
		}
	})
}

// TestPruningInterruption tests that interrupted pruning does not affect any register values.
//
// This test covers scenarios where:
// - The all values entries in the database remains the same when pruning was interrupted in the middle of the process
func TestPruningInterruption(t *testing.T) {
	// Generate random initial data across a range of heights.
	testData := generateRandomRegisterData()

	RunWithRegistersStorageWithInitialData(t, testData.initialData, func(db *pebble.DB) {
		pruner, err := NewRegisterPruner(
			zerolog.Nop(),
			db,
			metrics.NewNoopCollector(),
			WithPruneThreshold(pruneThreshold),
			WithPruneTickerInterval(10*time.Millisecond),
		)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

		// Start the pruning process but cancel it immediately to simulate interruption.
		go pruner.Start(signalerCtx)

		cleanupPruner(t, pruner, cancel, errChan)

		// Verify that all values remain unchanged after the interrupted pruning attempt.
		for height, entries := range testData.initialData {
			for _, entry := range entries {
				val, closer, err := db.Get(newLookupKey(height, entry.Key).Bytes())
				require.NoError(t, err)
				require.Equal(t, entry.Value, val)
				require.NoError(t, closer.Close())
			}
		}
	})
}

// TestPruneErrors checks the error handling behavior of the RegisterPruner when certain
// conditions cause failures during the pruning process.
//
// This test covers scenarios where:
// - The first stored height in the database cannot be retrieved, simulating a failure to locate it.
// - The latest height in the database cannot be retrieved, simulating a missing entry.
//
// The tests ensure that the RegisterPruner handles these error conditions correctly by
// triggering the appropriate irrecoverable errors and shutting down gracefully.
func TestPruneErrors(t *testing.T) {
	t.Run("not found first height", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			// Run the test with the Register storage
			db, err := OpenRegisterPebbleDB(dir)
			require.NoError(t, err)

			pruner, err := NewRegisterPruner(
				zerolog.Nop(),
				db,
				metrics.NewNoopCollector(),
				WithPruneThreshold(pruneThreshold),
				WithPruneTickerInterval(10*time.Millisecond),
			)
			require.NoError(t, err)

			err = fmt.Errorf("key not found")
			signCtxErr := fmt.Errorf("failed to get first height from register storage: %w", err)
			ctx := irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), signCtxErr)

			// Start the pruning process
			pruner.Start(ctx)

			unittest.AssertClosesBefore(t, pruner.Done(), 2*time.Second)
		})
	})

	t.Run("not found latest height", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			// Run the test with the Register storage
			db, err := OpenRegisterPebbleDB(dir)
			require.NoError(t, err)

			// insert initial first height to pebble
			require.NoError(t, db.Set(firstHeightKey, encodedUint64(1), nil))

			pruner, err := NewRegisterPruner(
				zerolog.Nop(),
				db,
				metrics.NewNoopCollector(),
				WithPruneThreshold(pruneThreshold),
				WithPruneTickerInterval(10*time.Millisecond),
			)
			require.NoError(t, err)

			err = fmt.Errorf("key not found")
			signCtxErr := fmt.Errorf("failed to get latest height from register storage: %w", err)
			ctx := irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), signCtxErr)

			// Start the pruning process
			pruner.Start(ctx)

			unittest.AssertClosesBefore(t, pruner.Done(), 2*time.Second)
		})
	})
}

// requirePruning checks if the first stored height in the database matches the expected height after pruning.
func requirePruning(t *testing.T, db *pebble.DB, expectedFirstHeightAfterPruning uint64) {
	require.Eventually(t, func() bool {
		actualFirstHeight, err := firstStoredHeight(db)
		require.NoError(t, err)
		return expectedFirstHeightAfterPruning == actualFirstHeight
	}, 2*time.Second, 15*time.Millisecond)
}

// cleanupPruner stops the pruner and verifies there are no errors in the error channel.
func cleanupPruner(t *testing.T, pruner *RegisterPruner, cancel context.CancelFunc, errChan <-chan error) {
	cancel()
	<-pruner.Done()

	select {
	case err := <-errChan:
		require.NoError(t, err)
	default:
	}
}

// straightPruneTestCase initializes and returns a testCase with predefined data for straight pruning.
func straightPruneTestCase() testCase {
	initialData := emptyRegistersData(latestHeight)

	key1 := flow.RegisterID{Owner: "owner1", Key: "key1"}
	key2 := flow.RegisterID{Owner: "owner2", Key: "key2"}
	key3 := flow.RegisterID{Owner: "owner3", Key: "key3"}

	value1 := []byte("value1")
	value2 := []byte("value2")
	value3 := []byte("value3")

	// Set up initial register entries for different heights.
	initialData[3] = flow.RegisterEntries{
		{Key: key1, Value: value1},
		{Key: key2, Value: value2},
	}
	initialData[4] = flow.RegisterEntries{
		{Key: key2, Value: value2},
		{Key: key3, Value: value3},
	}
	initialData[7] = flow.RegisterEntries{
		{Key: key1, Value: value1},
		{Key: key2, Value: value2},
		{Key: key3, Value: value3},
	}
	initialData[8] = flow.RegisterEntries{
		{Key: key1, Value: value1},
		{Key: key3, Value: value3},
	}
	initialData[9] = flow.RegisterEntries{
		{Key: key2, Value: value2},
		{Key: key3, Value: value3},
	}
	initialData[10] = flow.RegisterEntries{
		{Key: key1, Value: value1},
		{Key: key2, Value: value2},
	}
	initialData[12] = flow.RegisterEntries{
		{Key: key1, Value: value1},
		{Key: key3, Value: value3},
	}

	// Define the expected data after pruning.
	expectedData := map[uint64]flow.RegisterEntries{
		7: {
			{Key: key1, Value: value1}, // keep, first row <= 7
			{Key: key2, Value: value2}, // keep, first row <= 7
			{Key: key3, Value: value3}, // keep, first row <= 7
		},
		8: {
			{Key: key1, Value: value1}, // keep, height > 7
			{Key: key3, Value: value3}, // keep, height > 7
		},
		9: {
			{Key: key2, Value: value2}, // keep, height > 7
			{Key: key3, Value: value3}, // keep, height > 7
		},
		10: {
			{Key: key1, Value: value1}, // keep, height > 7
			{Key: key2, Value: value2}, // keep, height > 7
		},
		12: {
			{Key: key1, Value: value1}, // keep, height > 7
			{Key: key3, Value: value3}, // keep, height > 7
		},
	}

	// Define the data that should be pruned (i.e., removed) from the database after pruning.
	prunedData := map[uint64]flow.RegisterEntries{
		3: {
			{Key: key1, Value: value1},
			{Key: key2, Value: value2},
		},
		4: {
			{Key: key2, Value: value2},
			{Key: key3, Value: value3},
		},
	}

	return testCase{
		initialData:         initialData,
		expectedFirstHeight: pruneHeight,
		expectedData:        expectedData,
		prunedData:          prunedData,
	}
}

// testCaseWithDiffHeights initializes and returns a testCase with predefined data for different entries to keep
func testCaseWithDiffHeights() testCase {
	initialData := emptyRegistersData(latestHeight)

	key1 := flow.RegisterID{Owner: "owner1", Key: "key1"}
	key2 := flow.RegisterID{Owner: "owner2", Key: "key2"}
	key3 := flow.RegisterID{Owner: "owner3", Key: "key3"}

	value1 := []byte("value1")
	value2 := []byte("value2")
	value3 := []byte("value3")

	// Set up initial register entries for different heights.
	initialData[1] = flow.RegisterEntries{
		{Key: key1, Value: value1},
		{Key: key2, Value: value2},
		{Key: key3, Value: value3},
	}
	initialData[2] = flow.RegisterEntries{
		{Key: key1, Value: value1},
		{Key: key2, Value: value2},
	}
	initialData[5] = flow.RegisterEntries{
		{Key: key1, Value: value1},
		{Key: key3, Value: value3},
	}
	initialData[6] = flow.RegisterEntries{
		{Key: key1, Value: value1},
		{Key: key2, Value: value2},
	}
	initialData[10] = flow.RegisterEntries{
		{Key: key1, Value: value1},
		{Key: key2, Value: value2},
		{Key: key3, Value: value3},
	}
	initialData[11] = flow.RegisterEntries{
		{Key: key1, Value: value1},
		{Key: key3, Value: value3},
	}

	initialData[12] = flow.RegisterEntries{
		{Key: key1, Value: value1},
		{Key: key3, Value: value3},
	}

	// Define the expected data after pruning.
	expectedData := map[uint64]flow.RegisterEntries{
		5: {
			{Key: key3, Value: value3}, // keep, first row <= 7
		},
		6: {
			{Key: key1, Value: value1}, // keep, first row <= 7
			{Key: key2, Value: value2}, // keep, first row <= 7
		},
		10: {
			{Key: key1, Value: value1}, // keep, height > 7
			{Key: key2, Value: value2}, // keep, height > 7
			{Key: key3, Value: value3}, // keep, height > 7
		},
		11: {
			{Key: key1, Value: value1}, // keep, height > 7
			{Key: key3, Value: value3}, // keep, height > 7
		},
		12: {
			{Key: key1, Value: value1}, // keep, height > 7
			{Key: key3, Value: value3}, // keep, height > 7
		},
	}

	// Define the data that should be pruned (i.e., removed) from the database after pruning.
	prunedData := map[uint64]flow.RegisterEntries{
		1: {
			{Key: key1, Value: value1},
			{Key: key2, Value: value2},
			{Key: key3, Value: value3},
		},
		2: {
			{Key: key1, Value: value1},
			{Key: key2, Value: value2},
		},
		5: {
			{Key: key1, Value: value1},
		},
	}

	return testCase{
		initialData:         initialData,
		expectedFirstHeight: pruneHeight,
		expectedData:        expectedData,
		prunedData:          prunedData,
	}
}

// generateRandomRegisterData generates random register entries up to a given number of heights.
func generateRandomRegisterData() testCase {
	initialData := emptyRegistersData(latestHeight)

	keys := []flow.RegisterID{
		{Owner: "owner1", Key: "key1"},
		{Owner: "owner2", Key: "key2"},
		{Owner: "owner3", Key: "key3"},
	}

	values := [][]byte{
		[]byte("value1"),
		[]byte("value2"),
		[]byte("value3"),
	}

	rand.Seed(uint64(time.Now().UnixNano()))
	expectedData := make(map[uint64]flow.RegisterEntries)

	for height := uint64(1); height <= latestHeight; height++ {
		var entries flow.RegisterEntries

		// Randomly assign register entries for each height.
		for j, key := range keys {
			if rand.Intn(2) == 0 {
				entries = append(entries, flow.RegisterEntry{
					Key:   key,
					Value: values[j],
				})
			}
		}

		initialData[height] = entries
		if height > pruneHeight {
			expectedData[height] = entries
		}
	}

	return testCase{
		initialData:         initialData,
		expectedFirstHeight: pruneHeight,
		expectedData:        expectedData,
	}
}

// emptyRegistersData initializes an empty map for storing register entries.
func emptyRegistersData(count uint64) map[uint64]flow.RegisterEntries {
	data := make(map[uint64]flow.RegisterEntries, count)
	for i := uint64(1); i <= count; i++ {
		data[i] = flow.RegisterEntries{}
	}

	return data
}

// verifyData verifies that the data in the database matches the expected and pruned data after pruning.
func verifyData(t *testing.T,
	db *pebble.DB,
	data testCase,
) {
	for height, entries := range data.expectedData {
		for _, entry := range entries {
			val, closer, err := db.Get(newLookupKey(height, entry.Key).Bytes())
			require.NoError(t, err)
			require.Equal(t, entry.Value, val)
			require.NoError(t, closer.Close())
		}
	}

	for height, entries := range data.prunedData {
		for _, entry := range entries {
			_, _, err := db.Get(newLookupKey(height, entry.Key).Bytes())
			require.ErrorIs(t, err, pebble.ErrNotFound)
		}
	}
}
