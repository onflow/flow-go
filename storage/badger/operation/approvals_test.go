package operation

import (
	"encoding/gob"
	"fmt"
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// This test is a benchmark for inserting approvals into a BadgerDB
// The result shows inserting approvals with badger transaction is similar
// to inserting with badger batch update.

// go test -run=TestInsertApproval -v -count=1
// === RUN   TestInsertApproval
// Approvals count:  1000
// --- PASS: TestInsertApproval (0.09s)
// PASS
// ok      github.com/onflow/flow-go/storage/badger/operation      1.375s
func TestInsertApproval(t *testing.T) {

	gobFile := "approvals_test.gob"

	// Read approvals back from Gob
	approvals, err := readApprovalsFromGob(t, gobFile)
	require.NoError(t, err, "Reading approvals from Gob file should not fail")

	fmt.Println("Approvals count: ", len(approvals))

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		for _, approval := range approvals {
			err := db.Update(InsertResultApproval(approval))
			require.NoError(t, err, "Inserting approval should not fail")
		}

	})

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
