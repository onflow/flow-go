package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertInstanceParams stores the consolidated instance params under a single key.
//
// CAUTION:
//   - This function is intended to be called exactly once during bootstrapping.
//     Overwrites are prevented by an explicit existence check; if data is already present, error is returned.
//   - To guarantee atomicity of existence-check plus database write, we require the caller to acquire
//     the [storage.LockInsertInstanceParams] lock and hold it until the database write has been committed.
//
// Expected errors during normal operations:
//   - [storage.ErrAlreadyExists] if instance params have already been stored.
func InsertInstanceParams(lctx lockctx.Proof, rw storage.ReaderBatchWriter, params flow.VersionedInstanceParams) error {
	if !lctx.HoldsLock(storage.LockInsertInstanceParams) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertInstanceParams)
	}
	key := MakePrefix(codeInstanceParams)
	exist, err := KeyExists(rw.GlobalReader(), key)
	if err != nil {
		return err
	}
	if exist {
		return fmt.Errorf("instance params is already stored: %w", storage.ErrAlreadyExists)
	}
	return UpsertByKey(rw.Writer(), key, params)
}

// RetrieveInstanceParams retrieves the consolidated instance params from storage.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if the key does not exist (not bootstrapped).
func RetrieveInstanceParams(r storage.Reader, params *flow.VersionedInstanceParams) error {
	return RetrieveByKey(r, MakePrefix(codeInstanceParams), params)
}
