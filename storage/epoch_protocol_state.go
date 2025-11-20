package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

// EpochProtocolStateEntries represents persistent, fork-aware storage for the Epoch-related
// sub-state of the overall of the overall Protocol State (KV Store).
type EpochProtocolStateEntries interface {

	// BatchStore persists the given epoch protocol state entry as part of a DB batch. Per convention, the identities in
	// the flow.MinEpochStateEntry must be in canonical order for the current and next epoch (if present), otherwise an
	// exception is returned.
	//
	// CAUTION: The caller must ensure `epochProtocolStateID` is a collision-resistant hash of the provided
	// `epochProtocolStateEntry`! This method silently overrides existing data, which is safe only if for the same
	// key, we always write the same value.
	//
	// No errors are expected during normal operation.
	BatchStore(w Writer, epochProtocolStateID flow.Identifier, epochProtocolStateEntry *flow.MinEpochStateEntry) error

	// BatchIndex persists the specific map entry in the node's database.
	// In a nutshell, we want to maintain a map from `blockID` to `epochStateEntry`, where `blockID` references the
	// block that _proposes_ the referenced epoch protocol state entry.
	// Protocol convention:
	//   - Consider block B, whose ingestion might potentially lead to an updated protocol state. For example,
	//     the protocol state changes if we seal some execution results emitting service events.
	//   - For the key `blockID`, we use the identity of block B which _proposes_ this Protocol State. As value,
	//     the hash of the resulting protocol state at the end of processing B is to be used.
	//   - IMPORTANT: The protocol state requires confirmation by a QC and will only become active at the child block,
	//     _after_ validating the QC.
	//
	// CAUTION:
	//   - The caller must acquire the lock [storage.LockInsertBlock] and hold it until the database write has been committed.
	//   - OVERWRITES existing data (potential for data corruption):
	//     The lock proof serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
	//     ATOMICALLY within this write operation. Currently it's done by operation.InsertHeader where it performs a check
	//     to ensure the blockID is new, therefore any data indexed by this blockID is new as well.
	//
	// Expected errors during normal operations:
	// No expected errors during normal operations.
	BatchIndex(lctx lockctx.Proof, rw ReaderBatchWriter, blockID flow.Identifier, epochProtocolStateID flow.Identifier) error

	// ByID returns the flow.RichEpochStateEntry by its ID.
	// Expected errors during normal operations:
	//   - storage.ErrNotFound if no epoch state entry with the given Identifier is known.
	ByID(id flow.Identifier) (*flow.RichEpochStateEntry, error)

	// ByBlockID retrieves the flow.RichEpochStateEntry that the block with the given ID proposes.
	// CAUTION: this protocol state requires confirmation by a QC and will only become active at the child block,
	// _after_ validating the QC. Protocol convention:
	//   - Consider block B, whose ingestion might potentially lead to an updated protocol state. For example,
	//     the protocol state changes if we seal some execution results emitting service events.
	//   - For the key `blockID`, we use the identity of block B which _proposes_ this Protocol State. As value,
	//     the hash of the resulting protocol state at the end of processing B is to be used.
	//   - CAUTION: The protocol state requires confirmation by a QC and will only become active at the child block,
	//     _after_ validating the QC.
	//
	// Expected errors during normal operations:
	//   - storage.ErrNotFound if no epoch state entry has been indexed for the given block.
	ByBlockID(blockID flow.Identifier) (*flow.RichEpochStateEntry, error)
}
