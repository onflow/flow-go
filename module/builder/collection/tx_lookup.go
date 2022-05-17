package collection

import "github.com/onflow/flow-go/model/flow"

// transactionLookup encapsulates state and logic for checking chain history
// to avoid transaction duplication while building collections.
type transactionLookup struct {
	// set of transaction IDs in finalized ancestry
	finalized map[flow.Identifier]struct{}
	// set of transaction IDs in unfinalized ancestry
	unfinalized map[flow.Identifier]struct{}
}

func newTransactionLookup() *transactionLookup {
	lookup := &transactionLookup{
		finalized:   make(map[flow.Identifier]struct{}),
		unfinalized: make(map[flow.Identifier]struct{}),
	}
	return lookup
}

// note the existence of a transaction in a finalized noteAncestor collection
func (lookup *transactionLookup) addFinalizedAncestor(txID flow.Identifier) {
	lookup.finalized[txID] = struct{}{}
}

// note the existence of a transaction in a unfinalized noteAncestor collection
func (lookup *transactionLookup) addUnfinalizedAncestor(txID flow.Identifier) {
	lookup.unfinalized[txID] = struct{}{}
}

// checks whether the given transaction ID is in a finalized noteAncestor collection
func (lookup *transactionLookup) isFinalizedAncestor(txID flow.Identifier) bool {
	_, exists := lookup.finalized[txID]
	return exists
}

// checks whether the given transaction ID is in a unfinalized noteAncestor collection
func (lookup *transactionLookup) isUnfinalizedAncestor(txID flow.Identifier) bool {
	_, exists := lookup.unfinalized[txID]
	return exists
}
