package ledger

// Factory creates ledger instances with internal compaction management.
// The compactor lifecycle is managed internally by the ledger.
type Factory interface {
	// NewLedger creates a new ledger instance with internal compactor.
	// The ledger's Ready() method will signal when initialization (WAL replay) is complete.
	NewLedger() (Ledger, error)
}
