package pebble

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"github.com/cockroachdb/pebble/v2"
	"github.com/pkg/errors"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// Registers library that implements pebble storage for registers
// given a pebble instance with root block and root height populated
type Registers struct {
	db             *pebble.DB
	firstHeight    uint64
	latestHeight   *atomic.Uint64
	pruneThreshold uint64
}

// PruningDisabled represents the absence of a pruning threshold.
const PruningDisabled = math.MaxUint64

var _ storage.RegisterIndex = (*Registers)(nil)

// NewRegisters takes a populated pebble instance with LatestHeight and FirstHeight set.
// return storage.ErrNotBootstrapped if they those two keys are unavailable as it implies a uninitialized state
// return other error if database is in a corrupted state
func NewRegisters(db *pebble.DB, pruneThreshold uint64) (*Registers, error) {
	// check height keys and populate cache. These two variables will have been set
	firstHeight, latestHeight, err := ReadHeightsFromBootstrappedDB(db)
	if err != nil {
		// first height is found, but latest height is not found, this means that the DB is in a corrupted state
		return nil, fmt.Errorf("unable to initialize register storage, latest height unavailable in db: %w", err)
	}

	// If no pruning threshold is provided, disable pruning.
	if pruneThreshold == 0 {
		pruneThreshold = PruningDisabled
	}

	// All registers between firstHeight and lastHeight have been indexed
	return &Registers{
		db:             db,
		firstHeight:    firstHeight,
		latestHeight:   atomic.NewUint64(latestHeight),
		pruneThreshold: pruneThreshold,
	}, nil
}

// Get returns the most recent updated payload for the given RegisterID.
// "most recent" means the updates happens most recent up the given height.
//
// For example, if there are 2 values stored for register A at height 6 and 11, then
// GetPayload(13, A) would return the value at height 11.
//
// - storage.ErrNotFound if no register values are found
// - storage.ErrHeightNotIndexed if the requested height is out of the range of stored heights
func (s *Registers) Get(
	reg flow.RegisterID,
	height uint64,
) (flow.RegisterValue, error) {
	latestHeight := s.LatestHeight()
	if height > latestHeight {
		return nil, fmt.Errorf("height %d not indexed, latestHeight: %d, %w", height, latestHeight, storage.ErrHeightNotIndexed)
	}

	firstHeight := s.calculateFirstHeight(latestHeight)
	if height < firstHeight {
		return nil, fmt.Errorf("height %d not indexed, indexed range: [%d-%d], %w", height, firstHeight, latestHeight, storage.ErrHeightNotIndexed)
	}
	key := newLookupKey(height, reg)
	return s.lookupRegister(key.Bytes())
}

func (s *Registers) lookupRegister(key []byte) (flow.RegisterValue, error) {
	iter, err := s.db.NewIter(&pebble.IterOptions{
		UseL6Filters: true,
	})
	if err != nil {
		return nil, err
	}

	defer iter.Close()

	ok := iter.SeekPrefixGE(key)
	if !ok {
		// no such register found
		return nil, storage.ErrNotFound
	}

	binaryValue, err := iter.ValueAndErr()
	if err != nil {
		return nil, fmt.Errorf("failed to get value: %w", err)
	}
	// preventing caller from modifying the iterator's value slices
	valueCopy := make([]byte, len(binaryValue))
	copy(valueCopy, binaryValue)

	return valueCopy, nil
}

// Store sets the given entries in a batch.
// This function is expected to be called at one batch per height, sequentially. Under normal conditions,
// it should be called wth the value of height set to LatestHeight + 1
// CAUTION: This function is not safe for concurrent use.
func (s *Registers) Store(
	entries flow.RegisterEntries,
	height uint64,
) error {
	latestHeight := s.latestHeight.Load()
	// This check is for a special case for the execution node.
	// Upon restart, it may be in a state where registers are indexed in pebble for the latest height
	// but the remaining execution data in badger is not, so we skip the indexing step without throwing an error
	if height == latestHeight {
		// already updated
		return nil
	}

	nextHeight := latestHeight + 1
	if height != nextHeight {
		return fmt.Errorf("must store registers with the next height %v, but got %v", nextHeight, height)
	}
	batch := s.db.NewBatch()
	defer batch.Close()

	for _, entry := range entries {
		encoded := newLookupKey(height, entry.Key).Bytes()

		err := batch.Set(encoded, entry.Value, nil)
		if err != nil {
			return fmt.Errorf("failed to set key: %w", err)
		}
	}
	// increment height and commit
	err := batch.Set(latestHeightKey, encodedUint64(height), nil)
	if err != nil {
		return fmt.Errorf("failed to update latest height %d", height)
	}
	err = batch.Commit(pebble.Sync)
	if err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	s.latestHeight.Store(height)

	return nil
}

// LatestHeight Gets the latest height of complete registers available
func (s *Registers) LatestHeight() uint64 {
	return s.latestHeight.Load()
}

// FirstHeight first indexed height found in the store, typically root block for the spork
func (s *Registers) FirstHeight() uint64 {
	return s.calculateFirstHeight(s.LatestHeight())
}

// ByKeyPrefix returns an iterator over all registers whose key starts with keyPrefix,
// at or before height, across all owners. It uses a single pebble iterator that seeks
// monotonically forward through the address space.
//
// When keyPrefix is an exact key (e.g. "contract_names"), the iterator yields one entry
// per owner that has that key. When keyPrefix is a partial key (e.g. "code."), it yields
// one entry per unique matching (owner, key) pair. Using the "code." example, it yields
// the most recent contract code for each contract in each account as of the given height.
//
// If cursor is provided, the iterator will start from the next register key after the cursor.
// Use this to resume iteration from a previous position. This is useful when performing long running
// iterations, so you can close and reopen the iterator to avoid pausing compaction for too long.
//
// No error returns are expected during normal operation.
func (s *Registers) ByKeyPrefix(keyPrefix string, height uint64, cursor *flow.RegisterID) storage.IndexIterator[flow.RegisterValue, flow.RegisterID] {
	return func(yield func(storage.IteratorEntry[flow.RegisterValue, flow.RegisterID], error) bool) {
		var lowerBound []byte
		if cursor == nil {
			lowerBound = []byte{codeRegister}
		} else {
			lowerBound = nextRegisterKeyStart(cursor.Owner, cursor.Key)
		}

		iter, err := s.db.NewIter(&pebble.IterOptions{
			LowerBound: lowerBound,
			UpperBound: []byte{codeRegister + 1}, // exclusive upper bound
		})
		if err != nil {
			yield(nil, err)
			return
		}
		defer iter.Close()

		// starting from the first register for the first account, iterate over all registers with the
		// provided `keyPrefix` at or before the target height across all accounts.
		for ok := iter.First(); ok; {
			rawKey := iter.Key()
			entryHeight, reg, err := lookupKeyToRegisterID(rawKey)
			if err != nil {
				if !yield(nil, fmt.Errorf("invalid register key in scan: %w", err)) {
					return
				}
				// in practice, the caller is most likely going to break from the loop if there's an error.
				// continue here since that is the intuitive behavior.
				ok = iter.Next()
				continue
			}

			if reg.Key < keyPrefix {
				// Before the prefix range for this owner. Seek to the first matching key.
				ok = iter.SeekGE(newLookupKey(height, flow.RegisterID{Owner: reg.Owner, Key: keyPrefix}).Bytes())
				continue
			}

			if strings.HasPrefix(reg.Key, keyPrefix) {
				if entryHeight > height {
					// In range but newer than the target height. Seek to the right height for this key.
					ok = iter.SeekGE(newLookupKey(height, reg).Bytes())
					continue
				}

				// Found the most recent value at or before height. Yield it.
				entry := registerEntry{id: reg, iter: iter}
				if !yield(entry, nil) {
					return
				}

				// Choose the next seek based on whether this was an exact or prefix match.
				// An exact match means no other key for this owner can match the prefix, so
				// we can skip the entire owner. A prefix match may have further matches.
				if reg.Key == keyPrefix {
					ok = iter.SeekGE(nextOwnerStart(reg.Owner))
				} else {
					ok = iter.SeekGE(nextRegisterKeyStart(reg.Owner, reg.Key))
				}
				continue
			}

			// Past the prefix range for this owner; no match exists.
			ok = iter.SeekGE(nextOwnerStart(reg.Owner))
		}
	}
}

// nextRegisterKeyStart returns a seek key that lands on the first pebble entry of the
// register immediately following (owner, regKey), skipping all height-variants of regKey.
func nextRegisterKeyStart(owner, regKey string) []byte {
	// keys are sorted descending by height. using height=0 as the first possible entry.
	lastPossibleKey := newLookupKey(0, flow.RegisterID{Owner: owner, Key: regKey}).Bytes()
	return storage.PrefixUpperBound(lastPossibleKey) // increment key by 1
}

// nextOwnerStart returns the seek key for the first register entry of the owner
// immediately following owner in lexicographic order. It is used to skip all remaining
// register entries for the current owner after processing their target key register (or
// determining they have none).
func nextOwnerStart(owner string) []byte {
	// Include the '/' separator in the prefix so that PrefixUpperBound targets exactly
	// the entries for this owner. Pebble keys have the form:
	//
	//   [codeRegister] [owner] '/' [key] '/' [^height]
	//
	// All entries for a given owner share the prefix [codeRegister][owner]['/']. Using
	// that full prefix ensures PrefixUpperBound returns the first key strictly after
	// all of that owner's entries, regardless of owner length.
	//
	// The empty-owner case illustrates why the separator is required. Global registers
	// (owner="") have keys of the form [codeRegister, '/' (0x2F), ...]. Without the
	// separator, the prefix would be just [codeRegister] and PrefixUpperBound would
	// return [codeRegister+1] = [0x03], which equals the iterator's upper bound,
	// terminating the scan immediately and silently skipping every account whose first
	// address byte is > '/' (0x2F). With the separator:
	//
	//   PrefixUpperBound([codeRegister, '/']) = [codeRegister, 0x30]
	//
	// That seek target is strictly after all global-register keys ([codeRegister, 0x2F,
	// ...]) and before any account whose first address byte is >= 0x30, so the scan
	// correctly continues into those accounts.
	ownerPrefix := make([]byte, 2+len(owner))
	ownerPrefix[0] = codeRegister
	copy(ownerPrefix[1:], []byte(owner))
	ownerPrefix[1+len(owner)] = '/'
	return storage.PrefixUpperBound(ownerPrefix) // increment key by 1
}

// registerEntry is the IteratorEntry implementation for the Registers index.
type registerEntry struct {
	id   flow.RegisterID
	iter *pebble.Iterator
}

func (e registerEntry) Cursor() flow.RegisterID {
	return e.id
}

func (e registerEntry) Value() (flow.RegisterValue, error) {
	rawVal, err := e.iter.ValueAndErr()
	if err != nil {
		return nil, err
	}

	val := make(flow.RegisterValue, len(rawVal))
	copy(val, rawVal)
	return val, nil
}

// calculateFirstHeight calculates the first indexed height that is stored in the register index, based on the
// latest height and the configured pruning threshold. If the latest height is below the pruning threshold, the
// first indexed height will be the same as the initial height when the store was initialized. If the pruning
// threshold has been exceeded, the first indexed height is adjusted accordingly.
//
// Parameters:
// - latestHeight: the most recent height of complete registers available.
//
// Returns:
// - The first indexed height, either as the initialized height or adjusted for pruning.
func (s *Registers) calculateFirstHeight(latestHeight uint64) uint64 {
	if latestHeight < s.pruneThreshold {
		return s.firstHeight
	}

	pruneHeight := latestHeight - s.pruneThreshold
	if pruneHeight < s.firstHeight {
		return s.firstHeight
	}

	return pruneHeight
}

func firstStoredHeight(db *pebble.DB) (uint64, error) {
	return heightLookup(db, firstHeightKey)
}

func latestStoredHeight(db *pebble.DB) (uint64, error) {
	return heightLookup(db, latestHeightKey)
}

func heightLookup(db *pebble.DB, key []byte) (uint64, error) {
	res, closer, err := db.Get(key)
	if err != nil {
		return 0, convertNotFoundError(err)
	}
	defer closer.Close()
	return binary.BigEndian.Uint64(res), nil
}

// convert pebble NotFound error to storage NotFound error
func convertNotFoundError(err error) error {
	if errors.Is(err, pebble.ErrNotFound) {
		return storage.ErrNotFound
	}
	return err
}
