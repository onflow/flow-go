package unittest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/cluster"
)

const dontCheck = -1

// ClusterStateChecker is a test utility for checking cluster state. First, prepare it
// with expectations about the state, using `Expect*` functions, then use
// `Check` to assert the expectations.
//
// Duplicates are checked automatically without setting any expectations.
type ClusterStateChecker struct {
	state cluster.State

	// expectations about state
	count    int
	contains []flow.Identifier
	omits    []flow.Identifier
}

// NewClusterStateChecker returns a state checker for the given state.
func NewClusterStateChecker(state cluster.State) *ClusterStateChecker {
	checker := &ClusterStateChecker{
		state:    state,
		count:    dontCheck,
		contains: nil,
		omits:    nil,
	}
	return checker
}

// ExpectTxCount adds an expectation for the total count of transactions in
// the cluster state.
func (checker *ClusterStateChecker) ExpectTxCount(n int) *ClusterStateChecker {
	checker.count = n
	return checker
}

// ExpectContainsTx adds an expectation that the given transaction exists in
// the cluster state.
func (checker *ClusterStateChecker) ExpectContainsTx(txIDs ...flow.Identifier) *ClusterStateChecker {
	checker.contains = append(checker.contains, txIDs...)
	return checker
}

// ExpectOmitsTx adds an expectation that the given  transaction does not exist
// in the cluster state.
func (checker *ClusterStateChecker) ExpectOmitsTx(txIDs ...flow.Identifier) *ClusterStateChecker {
	checker.omits = append(checker.omits, txIDs...)
	return checker
}

// Assert checks all assertions against the cluster state. If any assertions
// fail, the test will fail.
func (checker *ClusterStateChecker) Assert(t *testing.T) {

	// start at the state head
	head, err := checker.state.Final().Head()
	assert.Nil(t, err)

	// track properties of the state we will later compare against expectations
	var (
		count        = 0                                  // total number of transactions
		transactions = make(map[flow.Identifier]struct{}) // unique transactions
		dupes        []flow.Identifier                    // duplicate transactions
	)

	// walk the chain state from head to genesis
	for head.Height > 0 {
		collection, err := checker.state.AtBlockID(head.ID()).Collection()
		assert.Nil(t, err)

		head, err = checker.state.AtBlockID(head.ParentID).Head()
		assert.Nil(t, err)

		if collection.Len() == 0 {
			continue
		}

		for _, txID := range collection.Light().Transactions {
			count++

			_, isDupe := transactions[txID]
			if isDupe {
				dupes = append(dupes, txID)
			}

			transactions[txID] = struct{}{}
		}
	}

	// ensure there are no duplicates
	if !assert.Len(t, dupes, 0) {
		t.Log("found duplicates: ", dupes)
	}

	// check that all manually set expectations are true
	if checker.count != dontCheck {
		assert.Equal(t, checker.count, count, "unexpected total number of transactions")
	}

	for _, txID := range checker.contains {
		_, exists := transactions[txID]
		assert.True(t, exists, "missing expected transaction: %x", txID)
	}

	for _, txID := range checker.omits {
		_, exists := transactions[txID]
		assert.False(t, exists, "found unexpected transaction: %x", txID)
	}
}
