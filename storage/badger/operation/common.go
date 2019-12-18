// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"bytes"
	"encoding/json"

	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"
)

// insert will encode the given entity using JSON and will insert the resulting
// binary data in the badger DB under the provided key. It will error if the
// key already exists and data under the key is different that one to be saved
func insert(key []byte, entity interface{}) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// check if the key already exists in the db
		item, err := tx.Get(key)
		if err == nil {
			err := item.Value(func(val []byte) error {

			})
			return errors.Errorf("key already exists (%x)", key)
		}
		if err != badger.ErrKeyNotFound {
			return errors.Wrap(err, "could not check key")
		}

		// serialize the entity data
		val, err := json.Marshal(entity)
		if err != nil {
			return errors.Wrap(err, "could not encode entity")
		}

		// insert the entity data into the DB
		err = tx.Set(key, val)
		if err != nil {
			return errors.Wrap(err, "could not store data")
		}

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
			return errors.Errorf("could not find key %x)", key)
		}
		if err != nil {
			return errors.Wrap(err, "could not check key")
		}

		// serialize the entity data
		val, err := json.Marshal(entity)
		if err != nil {
			return errors.Wrap(err, "could not encode entity")
		}

		// insert the entity data into the DB
		err = tx.Set(key, val)
		if err != nil {
			return errors.Wrap(err, "could not replace data")
		}

		return nil
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
			return errors.Wrap(err, "could not load data")
		}

		// get the value from the item
		err = item.Value(func(val []byte) error {
			err := json.Unmarshal(val, entity)
			return err
		})
		if err != nil {
			return errors.Wrap(err, "could not decode entity")
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

// iterationFunc is a function provided to our low-level iteration function that
// allows us to pass badger efficiencies across badger boundaries. By calling it
// for each iteration step, we can inject a function to check the key, a
// function to create the decode target and a function to process the current
// key-value pair. This a consumer of the API to decode when to skip the loading
// of values, the initialization of entities and the processing.
type iterationFunc func() (checkFunc, createFunc, handleFunc)

// iterate will start iteration at the start prefix and keep iterating through
// the badger DB up to and including the end prefix. On each iteration it will
// call the iteration function to initialize functions specific to processing
// the given key-value pair.
func iterate(start []byte, end []byte, iteration iterationFunc) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		it := tx.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(start); it.Valid(); it.Next() {

			// check if we have reached the end of our iteration
			item := it.Item()
			if bytes.Compare(item.Key(), end) > 0 {
				break
			}

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
					return errors.Wrap(err, "could not decode entity")
				}

				// process the entity
				err = handle()
				if err != nil {
					return errors.Wrap(err, "could not handle entity")
				}

				return nil
			})
			if err != nil {
				return errors.Wrap(err, "could not process value")
			}
		}

		return nil
	}
}
