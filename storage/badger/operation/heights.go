package operation

import (
	"github.com/dgraph-io/badger/v2"
)

func InsertRootHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeFinalizedRootHeight), height)
}

func RetrieveRootHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeFinalizedRootHeight), height)
}

func InsertSealedRootHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeSealedRootHeight), height)
}

func RetrieveSealedRootHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeSealedRootHeight), height)
}

func InsertFinalizedHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeFinalizedHeight), height)
}

func UpdateFinalizedHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(codeFinalizedHeight), height)
}

func RetrieveFinalizedHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeFinalizedHeight), height)
}

func InsertSealedHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeSealedHeight), height)
}

func UpdateSealedHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(codeSealedHeight), height)
}

func RetrieveSealedHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeSealedHeight), height)
}

// InsertEpochFirstHeight inserts the height of the first block in the given epoch.
// The first block of an epoch E is the finalized block with view >= E.FirstView.
// Although we don't store the final height of an epoch, it can be inferred from this index.
// Returns storage.ErrAlreadyExists if the height has already been indexed.
func InsertEpochFirstHeight(epoch, height uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochFirstHeight, epoch), height)
}

// RetrieveEpochFirstHeight retrieves the height of the first block in the given epoch.
// Returns storage.ErrNotFound if the first block of the epoch has not yet been finalized.
func RetrieveEpochFirstHeight(epoch uint64, height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochFirstHeight, epoch), height)
}

// RetrieveEpochLastHeight retrieves the height of the last block in the given epoch.
// It's a more readable, but equivalent query to RetrieveEpochFirstHeight when interested in the last height of an epoch.
// Returns storage.ErrNotFound if the first block of the epoch has not yet been finalized.
func RetrieveEpochLastHeight(epoch uint64, height *uint64) func(*badger.Txn) error {
	var nextEpochFirstHeight uint64
	return func(tx *badger.Txn) error {
		if err := retrieve(makePrefix(codeEpochFirstHeight, epoch+1), &nextEpochFirstHeight)(tx); err != nil {
			return err
		}
		*height = nextEpochFirstHeight - 1
		return nil
	}
}

// InsertLastCompleteBlockHeightIfNotExists inserts the last full block height if it is not already set.
// Calling this function multiple times is a no-op and returns no expected errors.
func InsertLastCompleteBlockHeightIfNotExists(height uint64) func(*badger.Txn) error {
	return SkipDuplicates(InsertLastCompleteBlockHeight(height))
}

func InsertLastCompleteBlockHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeLastCompleteBlockHeight), height)
}

func UpdateLastCompleteBlockHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(codeLastCompleteBlockHeight), height)
}

func RetrieveLastCompleteBlockHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeLastCompleteBlockHeight), height)
}
