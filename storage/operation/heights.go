package operation

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/storage"
)

func InsertRootHeight(w storage.Writer, height uint64) error {
	return UpsertByKey(w, MakePrefix(codeFinalizedRootHeight), height)
}

func RetrieveRootHeight(r storage.Reader, height *uint64) error {
	return RetrieveByKey(r, MakePrefix(codeFinalizedRootHeight), height)
}

func InsertSealedRootHeight(w storage.Writer, height uint64) error {
	return UpsertByKey(w, MakePrefix(codeSealedRootHeight), height)
}

func RetrieveSealedRootHeight(r storage.Reader, height *uint64) error {
	return RetrieveByKey(r, MakePrefix(codeSealedRootHeight), height)
}

func UpsertFinalizedHeight(w storage.Writer, height uint64) error {
	return UpsertByKey(w, MakePrefix(codeFinalizedHeight), height)
}

func RetrieveFinalizedHeight(r storage.Reader, height *uint64) error {
	return RetrieveByKey(r, MakePrefix(codeFinalizedHeight), height)
}

func UpsertSealedHeight(w storage.Writer, height uint64) error {
	return UpsertByKey(w, MakePrefix(codeSealedHeight), height)
}

func RetrieveSealedHeight(r storage.Reader, height *uint64) error {
	return RetrieveByKey(r, MakePrefix(codeSealedHeight), height)
}

// InsertEpochFirstHeight inserts the height of the first block in the given epoch.
// The first block of an epoch E is the finalized block with view >= E.FirstView.
// Although we don't store the final height of an epoch, it can be inferred from this index.
// Returns storage.ErrAlreadyExists if the height has already been indexed.
func InsertEpochFirstHeight(inserting *sync.Mutex, rw storage.ReaderBatchWriter, epoch, height uint64) error {
	inserting.Lock()
	rw.AddCallback(func(err error) {
		inserting.Unlock()
	})

	var exisingHeight uint64
	err := RetrieveEpochFirstHeight(rw.GlobalReader(), epoch, &exisingHeight)
	if err == nil {
		return storage.ErrAlreadyExists
	}

	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("failed to check existing epoch first height: %w", err)
	}

	return UpsertByKey(rw.Writer(), MakePrefix(codeEpochFirstHeight, epoch), height)
}

// RetrieveEpochFirstHeight retrieves the height of the first block in the given epoch.
// Returns storage.ErrNotFound if the first block of the epoch has not yet been finalized.
func RetrieveEpochFirstHeight(r storage.Reader, epoch uint64, height *uint64) error {
	return RetrieveByKey(r, MakePrefix(codeEpochFirstHeight, epoch), height)
}

// RetrieveEpochLastHeight retrieves the height of the last block in the given epoch.
// It's a more readable, but equivalent query to RetrieveEpochFirstHeight when interested in the last height of an epoch.
// Returns storage.ErrNotFound if the first block of the epoch has not yet been finalized.
func RetrieveEpochLastHeight(r storage.Reader, epoch uint64, height *uint64) error {
	var nextEpochFirstHeight uint64
	if err := RetrieveByKey(r, MakePrefix(codeEpochFirstHeight, epoch+1), &nextEpochFirstHeight); err != nil {
		return err
	}
	*height = nextEpochFirstHeight - 1
	return nil
}

func InsertLastCompleteBlockHeightIfNotExists(inserting *sync.Mutex, rw storage.ReaderBatchWriter, height uint64) error {
	inserting.Lock()
	rw.AddCallback(func(err error) {
		inserting.Unlock()
	})
	var existingHeight uint64
	err := RetrieveLastCompleteBlockHeight(rw.GlobalReader(), &existingHeight)
	if err == nil {
		// already exists, do not insert
		return nil
	}

	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("failed to check existing last complete block height: %w", err)
	}

	return UpsertByKey(rw.Writer(), MakePrefix(codeLastCompleteBlockHeight), height)
}

func UpsertLastCompleteBlockHeight(w storage.Writer, height uint64) error {
	return UpsertByKey(w, MakePrefix(codeLastCompleteBlockHeight), height)
}

func RetrieveLastCompleteBlockHeight(r storage.Reader, height *uint64) error {
	return RetrieveByKey(r, MakePrefix(codeLastCompleteBlockHeight), height)
}
