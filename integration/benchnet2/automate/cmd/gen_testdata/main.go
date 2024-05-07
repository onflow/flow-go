package main

import (
	"encoding/json"
	"os"

	"github.com/onflow/flow-go/utils/unittest"
)

// Use this tool to re-generate testdata for Benchnet2 tests if the snapshot model changes.
// For example, generate a new snapshot test file with:
//
//	go run .../cmd/gen_testdata/main.go > new_snapshot_test_data.json
func main() {
	participants := unittest.IdentityListFixture(10, unittest.WithAllRoles())
	snapshot := unittest.RootSnapshotFixture(participants)

	err := json.NewEncoder(os.Stdout).Encode(snapshot.Encodable())
	if err != nil {
		panic(err)
	}
}
