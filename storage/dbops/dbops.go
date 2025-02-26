package dbops

// The dbops feature flag is used to toggle between different database update operations.
// Currently, the existing database update operations use badger-transaction, which is default and deprecated.
// As part of the refactoring process to eventually transition to batch-update,

type DBOps string

const (
	// BadgerTransaction uses badger transactions (default and deprecated)
	BadgerTransaction DBOps = "badger-transaction"
	// BadgerBatch uses badger batch updates
	BatchUpdate DBOps = "batch-update"
)

func IsBadgerTransaction(op string) bool {
	return op == string(BadgerTransaction)
}

func IsBatchUpdate(op string) bool {
	return op == string(BatchUpdate)
}
