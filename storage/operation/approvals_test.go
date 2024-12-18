package operation_test

import (
	"encoding/gob"
	"log"
	"os"
	"sync"
	"testing"

	"github.com/golang/snappy"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

// This benchmark compare the read performance of badger and pebble
// the result shows that their read performance are similar
//
// go test -bench=BenchmarkRetrieveApprovals -run=TestNothing
// pkg: github.com/onflow/flow-go/storage/operation
// BenchmarkRetrieveApprovals/BadgerStorage-10               849144              1417 ns/op
// BenchmarkRetrieveApprovals/PebbleStorage-10              1101733              1080 ns/op
// PASS
// ok      github.com/onflow/flow-go/storage/operation     5.655s
func BenchmarkRetrieveApprovals(b *testing.B) {
	dbtest.BenchWithDB(b, func(b *testing.B, db storage.DB) {
		approval := unittest.ResultApprovalFixture()
		require.NoError(b, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertResultApproval(rw.Writer(), approval)
		}))

		b.ResetTimer()

		approvalID := approval.ID()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				var stored flow.ResultApproval
				require.NoError(b, operation.RetrieveResultApproval(db.Reader(), approvalID, &stored))
			}
		})

	})
}

// This benchmark is to compare the write performance of badger and pebble batch writes
// The result shows that pebble batch write with batch.Commit(pebble.Sync) is 100 times
// slower than badger batch write

// go test -bench=BenchmarkInsertApprovals -run=TestNothing
// pkg: github.com/onflow/flow-go/storage/operation
// BenchmarkInsertApprovals/BadgerStorage-10                  24312             45819 ns/op
// BenchmarkInsertApprovals/PebbleStorage-10                    226           4468405 ns/op
// PASS
// ok      github.com/onflow/flow-go/storage/operation     4.202s
func BenchmarkInsertApprovals(b *testing.B) {
	dbtest.BenchWithDB(b, func(b *testing.B, db storage.DB) {
		for i := 0; i < b.N; i++ {
			approval := unittest.ResultApprovalFixture()
			require.NoError(b, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertResultApproval(rw.Writer(), approval)
			}))
		}
	})
}

// insert approvals one at a time, wait until Commmit returns to insert the next one.
// when pebble batch is commited with `b.batch.Commit(pebble.Sync)`, then it's ~50 times slower than badger
//
// go test -run=TestInsertApprovalOneAtATime
//
// === RUN   TestInsertApprovalOneAtATime
// === RUN   TestInsertApprovalOneAtATime/BadgerStorage
// === RUN   TestInsertApprovalOneAtATime/PebbleStorage
// --- PASS: TestInsertApprovalOneAtATime (4.36s)
//
//	--- PASS: TestInsertApprovalOneAtATime/BadgerStorage (0.09s)
//	--- PASS: TestInsertApprovalOneAtATime/PebbleStorage (4.27s)
//
// PASS
// ok      github.com/onflow/flow-go/storage/operation     5.340s

// when pebble batch is commited with `b.batch.Commit(pebble.NoSync)`, badger and pebble are similar
// === RUN   TestInsertApprovalOneAtATime
// === RUN   TestInsertApprovalOneAtATime/BadgerStorage
// === RUN   TestInsertApprovalOneAtATime/PebbleStorage
// --- PASS: TestInsertApprovalOneAtATime (0.18s)
//
//	--- PASS: TestInsertApprovalOneAtATime/BadgerStorage (0.12s)
//	--- PASS: TestInsertApprovalOneAtATime/PebbleStorage (0.05s)
//
// PASS
// ok      github.com/onflow/flow-go/storage/operation     1.270s
func TestInsertApprovalOneAtATime(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, ww dbtest.WithWriter) {
		N := 1000
		approvals := make([]*flow.ResultApproval, N)
		for i := 0; i < N; i++ {
			approval := unittest.ResultApprovalFixture()
			approvals[i] = approval
		}

		for _, approval := range approvals {
			require.NoError(t, ww(func(w storage.Writer) error {
				return operation.InsertResultApproval(w, approval)
			}))
		}
	})
}

// when pebble batch are commited concurrently with `b.batch.Commit(pebble.Sync)`,
// then badger and pebble are similar
//
// === RUN   TestInsertApprovalConcurrently
// === RUN   TestInsertApprovalConcurrently/BadgerStorage
// === RUN   TestInsertApprovalConcurrently/PebbleStorage
// --- PASS: TestInsertApprovalConcurrently (0.12s)
//
//	--- PASS: TestInsertApprovalConcurrently/BadgerStorage (0.06s)
//	--- PASS: TestInsertApprovalConcurrently/PebbleStorage (0.06s)
//
// PASS
// ok      github.com/onflow/flow-go/storage/operation     1.275s
func TestInsertApprovalConcurrently(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, ww dbtest.WithWriter) {
		N := 1000
		approvals := make([]*flow.ResultApproval, N)
		for i := 0; i < N; i++ {
			approval := unittest.ResultApprovalFixture()
			approvals[i] = approval
		}

		var wg sync.WaitGroup
		errCh := make(chan error, N) // Buffered channel to capture errors

		for _, approval := range approvals {
			wg.Add(1)
			go func(app *flow.ResultApproval) {
				defer wg.Done()
				err := ww(func(w storage.Writer) error {
					return operation.InsertResultApproval(w, app)
				})
				if err != nil {
					errCh <- err // Send any errors to the channel
				}
			}(approval)
		}

		wg.Wait()
		close(errCh)

		// Check if there were any errors
		for err := range errCh {
			require.NoError(t, err) // Fails the test on the first error
		}
	})
}

// This test is to verify if the batch.Commit is durable.
// It needs to run with `TestWriteApprovalToDiskAsGob` and `TestReadApproval`
// Before running this test, run `TestWriteApprovalToDiskAsGob` first to write approvals to disk,
// so that we can later verify if the approvals are saved to disk after the crash and restart.
// After running this test, run `TestReadApproval` to verify if the approvals are saved to disk.
// A run_badger.sh or run_pebble.sh script runs the above three tests in sequence
// the result did not find a case where batch.Commit is not durable, since running 100 times
// and each time crash right after batch.Commit, and restart, all approvals are still accessiable
// after restart.

// Run #100
// ============================
//
//		Step 1: Cleaning directories...
//		Step 2: Running TestInsertApprovalDurability/Pebble (ignoring errors)...
//		=== RUN   TestInsertApprovalDurability
//		=== RUN   TestInsertApprovalDurability/PebbleStorage
//		--- FAIL: TestInsertApprovalDurability (0.07s)
//		    --- FAIL: TestInsertApprovalDurability/PebbleStorage (0.06s)
//		panic: crash as soon as batch.Commit is returned to test if pending writes are saved to disk [recovered]
//		        panic: crash as soon as batch.Commit is returned to test if pending writes are saved to disk
//		FAIL    github.com/onflow/flow-go/storage/operation     0.795s
//		Step 3: Running TestReadApproval/Pebble...
//		=== RUN   TestReadApproval
//		=== RUN   TestReadApproval/PebbleStorage
//		2024/12/18 13:43:19 [JOB 1] WAL file pebbletest/000002.log with log number 000002 stopped reading at offset: 0; replayed 0 keys in 0 batches
//		2024/12/18 13:43:19 [JOB 1] WAL file pebbletest/000004.log with log number 000004 stopped reading at offset: 0; replayed 0 keys in 0 batches
//		2024/12/18 13:43:19 [JOB 1] WAL file pebbletest/000005.log with log number 000005 stopped reading at offset: 349133; replayed 1000 keys in 1 batches
//		--- PASS: TestReadApproval (0.07s)
//		    --- PASS: TestReadApproval/PebbleStorage (0.07s)
//		PASS
//		ok      github.com/onflow/flow-go/storage/operation     0.792s
//
//	 Run #100 completed successfully.
//	 All 100 runs completed successfully!
func TestInsertApprovalDurability(t *testing.T) {

	gobFile := "approvals_test.gob"

	// Read approvals back from Gob
	approvals, err := readApprovalsFromGob(t, gobFile)
	require.NoError(t, err, "Reading approvals from Gob file should not fail")

	dbtest.RunWithStoragesAt(t, "./", func(t *testing.T, r storage.Reader, ww dbtest.WithWriter) {
		require.NoError(t, ww(func(w storage.Writer) error {
			for _, approval := range approvals {
				err := operation.InsertResultApproval(w, approval)
				if err != nil {
					return err
				}
			}
			return nil
		}))

		// the panic here is to verify that if the batch.Commit returns before the writes are saved to disk,
		// if after the crash and restart, some approvals are missing, then it means some pending writes are
		// not saved to disk, the batch.Commit is not durable
		panic("crash as soon as batch.Commit is returned to test if pending writes are saved to disk")

	})

}

func TestReadApproval(t *testing.T) {

	gobFile := "approvals_test.gob"

	// Read approvals back from Gob
	approvals, err := readApprovalsFromGob(t, gobFile)
	require.NoError(t, err, "Reading approvals from Gob file should not fail")

	dbtest.RunWithStoragesAt(t, "./", func(t *testing.T, r storage.Reader, ww dbtest.WithWriter) {
		for _, approval := range approvals {
			// Read the approval back and verify its the same
			var stored flow.ResultApproval
			require.NoError(t, operation.RetrieveResultApproval(r, approval.ID(), &stored))
			require.Equal(t, approval, &stored, "Approval should match")
		}
	})
}

// this benchmark is to check the performance of compressing the key and data before
// writing to the database
// the result shows the compression make non-significant difference in performance
//
// go test -bench=BenchmarkCompressionPerformance -run=TestNothing
// goos: darwin
// goarch: arm64
// pkg: github.com/onflow/flow-go/storage/operation
// BenchmarkCompressionPerformance/Compressed-10                481           2530317 ns/op
// BenchmarkCompressionPerformance/NotCompressed-10             556           2185808 ns/op
// PASS
// ok      github.com/onflow/flow-go/storage/operation     3.986s
func BenchmarkCompressionPerformance(b *testing.B) {
	gobFile := "approvals_test.gob"

	// Read approvals back from Gob
	approvals, err := readApprovalsFromGob(nil, gobFile)
	if err != nil {
		b.Fatalf("Failed to read approvals from Gob file: %v", err)
	}

	b.Run("Compressed", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			compressedIDs := make([]flow.Identifier, len(approvals))
			compressedValues := make([][]byte, len(approvals))
			for i, approval := range approvals {
				id := approval.ID()

				// Marshal the approval
				value, err := msgpack.Marshal(approval)
				if err != nil {
					b.Fatalf("Failed to marshal approval: %v", err)
				}

				// Compress with Snappy
				compressed := snappy.Encode(nil, value)
				log.Printf("Compressed size: %d -> %d", len(value), len(compressed))

				// Store the compressed data
				compressedIDs[i] = id
				compressedValues[i] = compressed
			}
		}
	})

	b.Run("NotCompressed", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ids := make([]flow.Identifier, len(approvals))
			values := make([][]byte, len(approvals))
			for i, approval := range approvals {
				id := approval.ID()

				// Marshal the approval
				value, err := msgpack.Marshal(approval)
				if err != nil {
					b.Fatalf("Failed to marshal approval: %v", err)
				}

				// Store the uncompressed data
				ids[i] = id
				values[i] = value
			}
		}
	})
}

// TestWriteApprovalToDiskAsGob tests writing and reading approvals as Gob
// necessary to run the tests
func TestWriteApprovalToDiskAsGob(t *testing.T) {
	N := 1000
	approvals := make([]*flow.ResultApproval, N)
	for i := 0; i < N; i++ {
		approval := unittest.ResultApprovalFixture()
		approvals[i] = approval
	}

	// File path for testing
	gobFile := "approvals_test.gob"

	// Write approvals to Gob
	err := writeApprovalsToGob(t, gobFile, approvals)
	require.NoError(t, err, "Writing approvals to Gob file should not fail")

	// Read approvals back from Gob
	readApprovals, err := readApprovalsFromGob(t, gobFile)
	require.NoError(t, err, "Reading approvals from Gob file should not fail")

	// Check lengths match
	require.Equal(t, len(approvals), len(readApprovals), "Number of approvals should match after read")

	// Optionally: Verify content equality
	for i := 0; i < N; i++ {
		require.Equal(t, approvals[i], readApprovals[i], "Approval at index %d should match", i)
	}
}

// writeApprovalsToGob writes a slice of ResultApproval to a Gob file
func writeApprovalsToGob(t *testing.T, gobFile string, approvals []*flow.ResultApproval) error {
	file, err := os.Create(gobFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(approvals); err != nil {
		return err
	}

	return nil
}

// readApprovalsFromGob reads a slice of ResultApproval from a Gob file
func readApprovalsFromGob(t *testing.T, gobFile string) ([]*flow.ResultApproval, error) {
	file, err := os.Open(gobFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var approvals []*flow.ResultApproval
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&approvals); err != nil {
		return nil, err
	}

	return approvals, nil
}
