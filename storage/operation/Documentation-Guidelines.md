# Guidelines for documenting low-level primitives for persistent storage

The folder `storage/operation` contains low-level primitives for persistent storage and retrieval of data structures from a database.
We accept that these functions have to be used _carefully_ by engineers that are knowledgeable about the
safety limitations of these functions to avoid data corruption . In order to facilitate correct usage, we need to diligently document
what aspects have to be paid attention to when calling these functions.

Proceed as follows
1. look at one file in `storage/operation` at a time (skip test files for now)
2. Go over the functions contained in the file one by one and for each function decide whether it is for writing or reading data.
3. For each function, provide a concise yet precise documentation covering
    - what this function is for
    - the assumptions this function makes about its inputs
    - what has to be payed attention to when calling this function
    - expected error returns during normal operations
    - follow our godocs policy `docs/agents/GoDocs.md`

Guidelines:
- Tune your documentation on a case by case basis to reflect the function's specific detail.
- Avoid overly generic documentation.
- Stick to a uniform framing and wording.
- Be very concise and precise.
- Analyze the implementation to make the correct statements!
- Double check your work.

## High level structure

On the highest level, there are function for storing data and other functions for retrieving data. The naming indicate which class
a function belongs to, though there is no absolutely uniform convention. For example, some function for loading data start with `Retrieve`,
while others start with `Lookup`, and additional names might be used as well. So pay close attention to the naming of the function.

Conceptually, we have data structures that contain certain fields. Furthermore, most data structures we deal with provide the functionality
to compute a cryptographic hash of their contents, which is typically referred to as "ID". We store data as key-value pairs.
(i) keys are either: the cryptographic hashes of the data structures.
(ii) Frequently, we break up the storage of compound objects, storing their sub-data structures individually. For example, a block contains the payload, the payload contains Seals. Frequently, we create mappings from the ID of the high-level data structure (e.g. block ID) to the IDs of the lower-level objects it contains (e.g. Seals). For example, `operation.IndexPayloadSeals`.

(i) and (ii) are fundamentally different: In case (i) the key is derived from the value in a collision-resistant manner (via cryptographic hash).
Meaning, if we change the value, the key should also change. Hence, unchecked overwrites pose no risk of data corruption, because for the same key,
we expect the same value. In comparison, for case (ii) we derive the key from the _context_ of the value. Note that the Flow protocol mandates that
for a previously persisted key, the data is never changed to a different value. Changing data could cause the node to publish inconsistent data and
to be slashed, or the protocol to be compromised as a whole. In many cases, the caller has to be cautious about avoiding usages causing data
corruption. This is because we don't wan't to implement override protections in all low-level storage functions of type (ii) for performance
reasons. Rather, we delegate the responsibility for cohesive checks to the caller, which must be clearly documented.


### Functions for reading data

When generating documentation for functions that read data, carefully differentiate between functions of type (i) and (ii).

#### Type (i) functions for reading data

As an example for functions of type (i), consider `operation.RetrieveSeal`:
```golang
// RetrieveSeal retrieves [flow.Seal] by its ID.
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if no seal with the specified `sealID` is known.
func RetrieveSeal(r storage.Reader, sealID flow.Identifier, seal *flow.Seal) error
```
* We document the struct type that is retrieved, here flow.Seal. Be mindful whether we are retrieving an individual struct or a slice.
* We document that the key we look up is the struct's own ID.
* We document the "Expected errors during normal operations:" (use this phrase)
    - in all cases, this will be the error storage.ErrNotFound, followed by a short description that no object with the specified ID is known.

#### Type (ii) functions for reading data

As an example for functions of type (ii), consider `operation.LookupPayloadSeals `:
```golang
// LookupPayloadSeals retrieves the list of Seals that were included in the payload
// of the specified block. For every known block, this index should be populated (at or above the root block).
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if `blockID` does not refer to a known block
func LookupPayloadSeals(r storage.Reader, blockID flow.Identifier, sealIDs *[]flow.Identifier) error error
```
* We document the struct type that is retrieved, here list of Seals. Be mindful whether we are retrieving an individual struct or a slice.
* Document that the lookup key is the ID of the bock containing the data structure. You can use our standard shorthand in this case, and just write that we are looking up X by Y containing X.
* We state if the index is populated for every known struct (which is typically the case). Consult the places in the code, where the corresponding index is written, to determine when the index is populated.
* We document the "Expected errors during normal operations" (use this phrase). Typically, the error explanation is that no struct Y is known, which contains X.


### Functions for writing data

When generating documentation for functions that write data, carefully differentiate between functions of type (i) and (ii).
For type (i), you need to carefully differentiate two sub-cases (i.a) and (i.b). Analogously, for type (ii),
you need to carefully differentiate two sub-cases (ii.a) and (ii.b)

#### Type (i.a) functions for writing data

As an example for functions of type (i.a), consider `operation.UpsertCollection`:
```golang
// UpsertCollection inserts a light collection into the storage, keyed by its ID.
//
// If the collection already exists, it will be overwritten. Note that here, the key (collection ID) is derived
// from the value (collection) via a collision-resistant hash function. Hence, unchecked overwrites pose no risk
// of data corruption, because for the same key, we expect the same value.
//
// No error returns are expected during normal operation.
func UpsertCollection(w storage.Writer, collection *flow.LightCollection) error {
	return UpsertByKey(w, MakePrefix(codeCollection, collection.ID()), collection)
}
```
Analyze the implementation! Here, the method itself computes the ID (i.e. cryptographic hash).
In this case, the function contains internal protections against the caller accidentally corrupting data.
Only functions that store the struct by its own ID _and_ contain internal safeguards against accidentally corrupting data are of type (i.a)!

* We document the struct type that is stored, here light collection. Be mindful whether we are storing an individual struct or a slice.
* We state whether the method will overwrite existing data. And then explain why this is safe.
* We state which errors are expected during normal operations (here none).

#### Type (i.b) functions for writing data

As an example for functions of type (i.b), consider `operation.InsertSeal`:

```golang
// InsertSeal inserts a [flow.Seal] into the database, keyed by its ID.
//
// CAUTION: The caller must ensure sealID is a collision-resistant hash of the provided seal!
// This method silently overrides existing data, which is safe only if for the same key, we
// always write the same value.
//
// No error returns are expected during normal operation.
func InsertSeal(w storage.Writer, sealID flow.Identifier, seal *flow.Seal) error {
	return UpsertByKey(w, MakePrefix(codeSeal, sealID), seal)
}
```
Analyze the implementation! Here, the method itself receives the ID (i.e. the cryptographic hash) if the object it is storing as an input. In this case, the function requires the caller to precompute the ID of the struct and provide it as an input. Only functions that store the struct by its own ID _but_ require the caller to provide this ID are of type (i.b)!

* We document the struct type that is stored, here flow.Seal. Be mindful whether we are storing an individual struct or a slice.
* We document that the key which we use (here "its ID").
* With a "CAUTION" statement, we document the requirement that the caller must ensure that the key is a collision-resistant hash of the provided data struct.
* We state which errors are expected during normal operations (here none).

#### Type (ii.a) functions for writing data

As an example for functions of type (ii.a), consider `operation.IndexStateCommitment`:

```golang
// IndexStateCommitment indexes a state commitment by the block ID whose execution results in that state.
// The function ensures data integrity by first checking if a commitment already exists for the given block
// and rejecting overwrites with different values. This function is idempotent, i.e. repeated calls with the
// *initially* indexed value are no-ops.
//
// CAUTION:
//   - Confirming that no value is already stored and the subsequent write must be atomic to prevent data corruption.
//     The caller must acquire the [storage.LockInsertOwnReceipt] and hold it until the database write has been committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrDataMismatch] if a *different* state commitment is already indexed for the same block ID
func IndexStateCommitment(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, commit flow.StateCommitment) error {
	if !lctx.HoldsLock(storage.LockInsertOwnReceipt) {
		return fmt.Errorf("cannot index state commitment without holding lock %s", storage.LockInsertOwnReceipt)
	}

	var existingCommit flow.StateCommitment
	err := LookupStateCommitment(rw.GlobalReader(), blockID, &existingCommit) // on happy path, i.e. nothing stored yet, we expect `storage.ErrNotFound`
	if err == nil {                                                           // Value for this key already exists! Need to check for data mismatch:
		if existingCommit == commit {
			return nil // The commit already exists, no need to index again
		}
		return fmt.Errorf("commit for block %v already exists with different value, (existing: %v, new: %v), %w", blockID, existingCommit, commit, storage.ErrDataMismatch)
	} else if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not check existing state commitment: %w", err)
	}

	return UpsertByKey(rw.Writer(), MakePrefix(codeCommit, blockID), commit)
}
```
Analyze the implementation! Only functions that internally implement safeguards against overwriting a key-value pair with _different_ data for the same key are of type (ii.a)!

* We document the struct type that is stored, here `flow.StateCommitment`. If applicable, we also document additional key-value pairs that are persisted as part of this function (here, none). Analyze the implementation.
* We concisely document by which means the implementation ensures data integrity. For functions of type (ii.a), we typically just attempt to read the value for the respective key. You may adapt the explanation from this example to reflect the specifics of the implementation. Note that the behaviour might be different if a value has previously been stored. Analyze the implementation.
* With a "CAUTION" statement, we concisely document the requirement that the read for the data integrity check and the subsequent write must happen atomically. This requires synchronization, and hence locking. We document which locks are required to be held by the caller.
* Analyze the implementation to decide whether additional cautionary statements are required to reduce the probability of accidental bugs.
* We state which errors are expected during normal operations (here `storage.ErrDataMismatch`) and the condition under which they occur. Analyze the implementation to make the correct statements!



#### Type (ii.b) functions for writing data

As an example for functions of type (ii.b), consider `operation.IndexPayloadSeals`:

```golang
// IndexPayloadSeals indexes the given Seal IDs by the block ID.
//
// CAUTION:
//   - The caller must acquire the [storage.LockInsertBlock] and hold it until the database write has been committed.
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exits.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption. The lock proof
//     serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
//     ATOMICALLY with this write operation.
//
// No error returns are expected during normal operation.
func IndexPayloadSeals(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, sealIDs []flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("cannot index seal for blockID %v without holding lock %s",
			blockID, storage.LockInsertBlock)
	}
	return UpsertByKey(w, MakePrefix(codePayloadSeals, blockID), sealIDs)
}
```

Analyze the implementation! Only functions are of type (ii.b) that delegate the check whether an entry with the specified key already exists to the caller!

* We document the struct type that is stored, here "the given Seal". If applicable, we also document additional key-value pairs that are persisted as part of this function (here none). Analyze the implementation.
* With a "CAUTION" statement, we document that the caller must provide protections against accidental overrides. Typically, those protections require reads happening in one atomic operation with the writes. To perform those reads atomically with the writes, the caller is intended to hold the specified locks and only release them after the database writes have been committed.
    - The first bullet point in the CAUTION statement specifies which locks the caller must hold and that those locks are to be held until the writes have been committed.
    - The second bullet point in the CAUTION statement emphasizes that the caller must provide protections against accidental overrides with different data. You may copy the wording of the second bullet point. It is generic enough, so it should apply in the majority of cases.
* We state which errors are expected during normal operations (here none) and the condition under which they occur. Analyze the implementation to make the correct statements!


