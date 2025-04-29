package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

func InsertExecutionForkEvidence(conflictingSeals []*flow.IncorporatedResultSeal) func(*badger.Txn) error {
	return insert(makePrefix(codeExecutionFork), conflictingSeals)
}

func RemoveExecutionForkEvidence() func(*badger.Txn) error {
	return remove(makePrefix(codeExecutionFork))
}

func RetrieveExecutionForkEvidence(conflictingSeals *[]*flow.IncorporatedResultSeal) func(*badger.Txn) error {
	return retrieve(makePrefix(codeExecutionFork), conflictingSeals)
}
