// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

// insert will encode the given entity using JSON and will insert the resulting
// binary data in the badger DB under the provided key. It will error if the
// key already exists.
func insert(key []byte, entity interface{}) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// check if the key already exists in the db
		_, err := tx.Get(key)
		if err == nil {
			return storage.ErrAlreadyExists
		}

		if !errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("could not check key: %w", err)
		}

		// serialize the entity data
		val, err := json.Marshal(entity)
		if err != nil {
			return fmt.Errorf("could not encode entity: %w", err)
		}

		// persist the entity data into the DB
		err = tx.Set(key, val)
		if err != nil {
			return fmt.Errorf("could not store data: %w", err)
		}

		return nil
	}
}

// check will simply check if the entry with the given key exists in the DB.
func check(key []byte, exists *bool) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// retrieve the item from the key-value store
		_, err := tx.Get(key)
		if err == badger.ErrKeyNotFound {
			*exists = false
			return nil
		}
		if err != nil {
			return fmt.Errorf("could not check existence: %w", err)
		}
		*exists = true
		return nil
	}
}

// update will encode the given entity with JSON and update the binary data
// under the given key in the badger DB. It will error if the key does not exist
// yet.
func update(key []byte, entity interface{}) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// retrieve the item from the key-value store
		_, err := tx.Get(key)
		if err == badger.ErrKeyNotFound {
			return storage.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("could not check key: %w", err)
		}

		// serialize the entity data
		val, err := json.Marshal(entity)
		if err != nil {
			return fmt.Errorf("could not encode entity: %w", err)
		}

		// persist the entity data into the DB
		err = tx.Set(key, val)
		if err != nil {
			return fmt.Errorf("could not replace data: %w", err)
		}

		return nil
	}
}

// remove removes the entity with the given key, if it exists. If it doesn't
// exist, this is a no-op.
func remove(key []byte) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		// retrieve the item from the key-value store
		_, err := tx.Get(key)
		if err == badger.ErrKeyNotFound {
			return fmt.Errorf("could not find key %x): %w", key, err)
		}
		if err != nil {
			return fmt.Errorf("could not check key: %w", err)
		}

		err = tx.Delete(key)
		return err
	}
}

// retrieve will retrieve the binary data under the given key from the badger DB
// and decode it into the given entity. The provided entity needs to be a
// pointer to an initialized entity of the correct type.
func retrieve(key []byte, entity interface{}) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// retrieve the item from the key-value store
		item, err := tx.Get(key)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return storage.ErrNotFound
			}
			return fmt.Errorf("could not load data: %w", err)
		}

		// get the value from the item
		err = item.Value(func(val []byte) error {
			err := json.Unmarshal(val, entity)
			return err
		})
		if err != nil {
			return fmt.Errorf("could not decode entity: %w", err)
		}

		return nil
	}
}

// checkFunc is called during key iteration through the badger DB in order to
// check whether we should process the given key-value pair. It can be used to
// avoid loading the value if its not of interest, as well as storing the key
// for the current iteration step.
type checkFunc func(key []byte) bool

// createFunc returns a pointer to an initialized entity that we can potentially
// decode the next value into during a badger DB iteration.
type createFunc func() interface{}

// handleFunc is a function that starts the processing of the current key-value
// pair during a badger iteration. It should be called after the key was checked
// and the entity was decoded.
type handleFunc func() error

// handleKeyFunc is a function that process the current key during a badger iteration.
type handleKeyFunc func(key []byte)

// iterationFunc is a function provided to our low-level iteration function that
// allows us to pass badger efficiencies across badger boundaries. By calling it
// for each iteration step, we can inject a function to check the key, a
// function to create the decode target and a function to process the current
// key-value pair. This a consumer of the API to decode when to skip the loading
// of values, the initialization of entities and the processing.
type iterationFunc func() (checkFunc, createFunc, handleFunc)

// lookup is the default iteration function allowing us to collect a list of
// entity IDs from an index.
func lookup(entityIDs *[]flow.Identifier) func() (checkFunc, createFunc, handleFunc) {
	*entityIDs = make([]flow.Identifier, 0, len(*entityIDs))
	return func() (checkFunc, createFunc, handleFunc) {
		check := func(key []byte) bool {
			return true
		}
		var entityID flow.Identifier
		create := func() interface{} {
			return &entityID
		}
		handle := func() error {
			*entityIDs = append(*entityIDs, entityID)
			return nil
		}
		return check, create, handle
	}
}

// iterate iterates over a range of keys defined by a start and end key.
//
// The start key is the first key in the iteration and the lexicographically
// smallest key in the iteration. The end key is last key in the iteration
// and the lexicographically largest key in the iteration.
//
// On each iteration, it will call the iteration function to initialize
// functions specific to processing the given key-value pair.
//
// TODO: this function is unbounded – pass context.Context to this or calling
// functions to allow timing functions out.
func iterate(start []byte, end []byte, iteration iterationFunc) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// initialize the default options and comparison modifier for iteration
		modifier := 1
		options := badger.DefaultIteratorOptions

		// If start is bigger than end, we have a backwards iteration:
		// 1) Seek will go to the first key that is equal or lesser than the
		// start prefix. However, this will only include the first key with the
		// start prefix; we need to add 0xff to the start prefix to cover all
		// items with the start prefix.
		// 2) We break the loop upon hitting the first item that has a key
		// higher than the end prefix. In order to reverse this, we use a
		// modifier for the comparison that reverses the check and makes it
		// stop upon the first item before the end prefix.
		// 3) We also set the reverse option on the iterator, so we step through
		// all of the keys backwards.
		if bytes.Compare(start, end) > 0 {
			options.Reverse = true      // make sure to go in reverse order
			start = append(start, 0xff) // make sure to start after start prefix
			modifier = -1               // make sure to stop before end prefix
		}

		it := tx.NewIterator(options)
		defer it.Close()

		for it.Seek(start); it.Valid(); it.Next() {

			item := it.Item()

			// check if we have reached the end of our iteration
			// NOTE: we have to cut down the prefix to the same length as the
			// end prefix in order to go through all items that have the same
			// partial prefix before breaking
			key := item.Key()
			prefix := key[0:len(end)]
			if bytes.Compare(prefix, end)*modifier > 0 {
				break
			}

			// initialize processing functions for iteration
			check, create, handle := iteration()

			// check if we should process the item at all
			ok := check(key)
			if !ok {
				continue
			}

			// process the actual item
			err := item.Value(func(val []byte) error {

				// decode into the entity
				entity := create()
				err := json.Unmarshal(val, entity)
				if err != nil {
					return fmt.Errorf("could not decode entity: %w", err)
				}

				// process the entity
				err = handle()
				if err != nil {
					return fmt.Errorf("could not handle entity: %w", err)
				}

				return nil
			})
			if err != nil {
				return fmt.Errorf("could not process value: %w", err)
			}
		}

		return nil
	}
}

// iterateKey iterate only the keys from the given `start` prefix to the given `end` prefix.
// On each iteration, the key will be passed to the handle function to be handled.
// It is useful to traverse through the index when the data needed are all included in the
// key itself without reading the value.
func iterateKey(start []byte, end []byte, handle handleKeyFunc) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// initialize the default options and comparison modifier for iteration
		modifier := 1
		options := badger.DefaultIteratorOptions

		// If start is bigger than end, we have a backwards iteration:
		// 1) Seek will go to the first key that is equal or lesser than the
		// start prefix. However, this will only include the first key with the
		// start prefix; we need to add 0xff to the start prefix to cover all
		// items with the start prefix.
		// 2) We break the loop upon hitting the first item that has a key
		// higher than the end prefix. In order to reverse this, we use a
		// modifier for the comparison that reverses the check and makes it
		// stop upon the first item before the end prefix.
		// 3) We also set the reverse option on the iterator, so we step through
		// all of the keys backwards.
		if bytes.Compare(start, end) > 0 {
			options.Reverse = true      // make sure to go in reverse order
			start = append(start, 0xff) // make sure to start after start prefix
			modifier = -1               // make sure to stop before end prefix
		}

		it := tx.NewIterator(options)
		defer it.Close()

		for it.Seek(start); it.Valid(); it.Next() {

			item := it.Item()

			// check if we have reached the end of our iteration
			// NOTE: we have to cut down the prefix to the same length as the
			// end prefix in order to go through all items that have the same
			// partial prefix before breaking
			key := item.Key()
			prefix := key[0:len(end)]
			if bytes.Compare(prefix, end)*modifier > 0 {
				break
			}

			handle(key)
		}

		return nil
	}
}

// traverse iterates over a range of keys defined by a prefix.
//
// The prefix must be shared by all keys in the iteration.
//
// On each iteration, it will call the iteration function to initialize
// functions specific to processing the given key-value pair.
func traverse(prefix []byte, iteration iterationFunc) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		if len(prefix) == 0 {
			return fmt.Errorf("prefix must not be empty")
		}

		opts := badger.DefaultIteratorOptions
		// NOTE: this is an optimization only, it does not enforce that all
		// results in the iteration have this prefix.
		opts.Prefix = prefix

		it := tx.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {

			item := it.Item()

			// initialize processing functions for iteration
			check, create, handle := iteration()

			// check if we should process the item at all
			key := item.Key()
			ok := check(key)
			if !ok {
				continue
			}

			// process the actual item
			err := item.Value(func(val []byte) error {

				// decode into the entity
				entity := create()
				err := json.Unmarshal(val, entity)
				if err != nil {
					return fmt.Errorf("could not decode entity: %w", err)
				}

				// process the entity
				err = handle()
				if err != nil {
					return fmt.Errorf("could not handle entity: %w", err)
				}

				return nil
			})
			if err != nil {
				return fmt.Errorf("could not process value: %w", err)
			}
		}

		return nil
	}
}

func toPayloadIndex(code uint8, height uint64, blockID flow.Identifier, parentID flow.Identifier) []byte {
	return makePrefix(code, height, blockID, parentID)
}

func fromPayloadIndex(key []byte) (uint64, flow.Identifier, flow.Identifier) {
	height := binary.BigEndian.Uint64(key[1:9])
	blockID := flow.HashToID(key[9:41])
	parentID := flow.HashToID(key[41:73])
	return height, blockID, parentID
}

// validatepayload creates an iteration function used for ensuring no entities in
// a new payload have already been included in an ancestor payload. The input
// set of entity IDs represents the IDs of all entities in the candidate block
// payload. If any of these candidate entities have already been included in an
// ancestor block, a sentinel error is returned.
//
// This is useful for verifying an existing payload, for example in a block
// proposal from another node, where the desired output is accept/reject.
func validatepayload(blockID flow.Identifier, checkIDs []flow.Identifier) iterationFunc {

	// build lookup table for payload entities
	lookup := make(map[flow.Identifier]struct{})
	for _, checkID := range checkIDs {
		lookup[checkID] = struct{}{}
	}

	return func() (checkFunc, createFunc, handleFunc) {

		// check will check whether we are on the next block we want to check
		// if we are not on the block we care about, we ignore the entry
		// otherwise, we forward to the next parent and check the entry
		check := func(key []byte) bool {
			_, currentID, nextID := fromPayloadIndex(key)
			if currentID != blockID {
				return false
			}
			blockID = nextID
			return true
		}

		// create returns a slice of IDs to decode the payload index entry into
		var entityIDs []flow.Identifier
		create := func() interface{} {
			return &entityIDs
		}

		// handle will check the payload index entry entity IDs against the
		// entity IDs we are checking as a new payload; if any of them matches,
		// the payload is not valid, as it contains a duplicate entity
		handle := func() error {
			for _, entityID := range entityIDs {
				_, ok := lookup[entityID]
				if ok {
					return fmt.Errorf("duplicate payload entity (%s): %w", entityID, storage.ErrAlreadyIndexed)
				}
			}
			return nil
		}

		return check, create, handle
	}
}

// searchduplicates creates an iteration function similar to validatepayload. Rather
// than returning an error when ANY duplicate IDs are found, searchduplicates
// tracks any duplicate IDs and populates a map containing all invalid IDs
// from the candidate set.
//
// This is useful when building a payload locally, where we want to know which
// entities are valid for inclusion so we can produce a valid block proposal.
func searchduplicates(blockID flow.Identifier, candidateIDs []flow.Identifier, invalidIDs *map[flow.Identifier]struct{}) iterationFunc {

	// build lookup table for candidate payload entities
	lookup := make(map[flow.Identifier]struct{})
	for _, id := range candidateIDs {
		lookup[id] = struct{}{}
	}

	// ensure the map is instantiated and empty
	*invalidIDs = make(map[flow.Identifier]struct{})

	return func() (checkFunc, createFunc, handleFunc) {

		// check will check whether we are on the next block we want to check
		// if we are not on the block we care about, we ignore the entry
		// otherwise, we forward to the next parent and check the entry
		check := func(key []byte) bool {
			_, currentID, nextID := fromPayloadIndex(key)
			if currentID != blockID {
				return false
			}
			blockID = nextID
			return true
		}

		// create returns a slice of IDs to decode the payload index entry into
		var entityIDs []flow.Identifier
		create := func() interface{} {
			return &entityIDs
		}

		// handle will check the payload index entry entity IDs against the
		// entity IDs we are checking as a new payload; if any of them matches,
		// the payload is not valid, as it contains a duplicate entity
		handle := func() error {
			for _, entityID := range entityIDs {
				_, isInvalid := lookup[entityID]
				if isInvalid {
					(*invalidIDs)[entityID] = struct{}{}
				}
			}
			return nil
		}

		return check, create, handle
	}
}

// finddescendant returns an handleKey function for iteration. It iterates keys in badger
// and find the descendants blocks (blocks with higher view) that connected to the block
// by the given blockID.
func finddescendant(blockID flow.Identifier, descendants *[]flow.Identifier) handleKeyFunc {
	incorporated := make(map[flow.Identifier]struct{})

	// set the input block as incorporated. (The scenario for the input block is usually the
	// finalized block)
	incorporated[blockID] = struct{}{}

	return func(key []byte) {
		_, blockID, parentID := fromPayloadIndex(key)

		_, ok := incorporated[parentID]
		if ok {
			// if parent is incorporated, then this block is incorporated too.
			// adding it to the descendants list.
			(*descendants) = append((*descendants), blockID)
			incorporated[blockID] = struct{}{}
		}

		// if a block's parent isn't found in the incorporated list, then it's not incorporated.
		// And it will never be incorporated, because we are traversing blocks with height in
		// the increasing order. So future blocks will have higher height, and is not possible
		// to fill the gap between this block and any existing incorporated block. Even if there is,
		// that block must be an invalid block anyway, which is fine not being included in the
		// descendants list.
	}
}
