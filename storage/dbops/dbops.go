package dbops

// The dbops feature flag is used to toggle between different database update operations.
// Currently, the existing database update operations use badger-transaction, which is default and deprecated.
// As part of the refactoring process to eventually transition to pebble-batch updates,
// an intermediate step is required to switch to badger-batch.
// This is why the feature flag has three possible values to facilitate the transition.
const DB_OPS_MSG = "database operations to use (badger-transaction, badger-batch, pebble-batch)"
const DB_OPS_DEFAULT = string(BadgerTransaction)

type DBOps string

const (
	// BadgerTransaction uses badger transactions (default and deprecated)
	BadgerTransaction DBOps = "badger-transaction"
	// BadgerBatch uses badger batch updates
	BadgerBatch DBOps = "badger-batch"
	// PebbleBatch uses pebble batch updates
	PebbleBatch DBOps = "pebble-batch"

	UsageErrMsg string = "invalid --dbops flag, expect badger-transaction/badger-batch/pebble-batch, but got: %v"
)

func IsBadgerBased(ops string) bool {
	return ops == string(BadgerTransaction) || ops == string(BadgerBatch)
}

func IsBadgerTransaction(ops string) bool {
	return ops == string(BadgerTransaction)
}

func IsBadgerBatch(ops string) bool {
	return ops == string(BadgerBatch)
}

func IsPebbleBatch(ops string) bool {
	return ops == string(PebbleBatch)
}

func IsBatchUpdate(ops string) bool {
	return ops == string(BadgerBatch) || ops == string(PebbleBatch)
}
