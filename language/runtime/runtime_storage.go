package runtime

import (
	"bytes"
	"encoding/gob"

	"github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/interpreter"
)

type storageKey struct {
	storageIdentifier string
	key               string
}

type interpreterRuntimeStorage struct {
	runtimeInterface Interface
	cache            map[storageKey]interpreter.Value
}

func newInterpreterRuntimeStorage(runtimeInterface Interface) *interpreterRuntimeStorage {
	return &interpreterRuntimeStorage{
		runtimeInterface: runtimeInterface,
		cache:            map[storageKey]interpreter.Value{},
	}
}

// readValue is the StorageReadHandlerFunc for the interpreter.
//
// It checks the cache for values which were already previously loaded/deserialized
// from storage (through the runtime interface) and returns the cached value if it exists.
//
// If there is a cache miss, the key is read from storage through the runtime interface,
// places in the cache, and returned.
//
func (s *interpreterRuntimeStorage) readValue(
	storageIdentifier string,
	key string,
) interpreter.OptionalValue {

	storageKey := storageKey{
		storageIdentifier: storageIdentifier,
		key:               key,
	}

	// Check cache. Returned cached value, if any

	if cachedValue, ok := s.cache[storageKey]; ok {
		if cachedValue == nil {
			return interpreter.NilValue{}
		}

		return interpreter.NewSomeValueOwningNonCopying(cachedValue)
	}

	// Cache miss: Load and deserialize the stored value (if any)
	// through the runtime interface

	// TODO: fix controller
	storedData, err := s.runtimeInterface.GetValue([]byte(storageIdentifier), []byte{}, []byte(key))
	if err != nil {
		panic(err)
	}

	var storedValue interpreter.Value
	if len(storedData) == 0 {
		s.cache[storageKey] = nil
		return interpreter.NilValue{}
	}

	decoder := gob.NewDecoder(bytes.NewReader(storedData))
	err = decoder.Decode(&storedValue)
	if err != nil {
		panic(err)
	}

	s.cache[storageKey] = storedValue
	return interpreter.NewSomeValueOwningNonCopying(storedValue)
}

// writeValue is the StorageWriteHandlerFunc for the interpreter.
//
// It only places the written value in the cache.
//
// It does *not* serialize/save the value in  storage (through the runtime interface).
// (The Cache is finally written back through the runtime interface in `writeCached`.)
//
func (s *interpreterRuntimeStorage) writeValue(
	storageIdentifier string,
	key string,
	value interpreter.OptionalValue,
) {
	storageKey := storageKey{
		storageIdentifier: storageIdentifier,
		key:               key,
	}

	// Only write the value to the cache.
	// The Cache is finally written back through the runtime interface in `writeCached`

	switch typedValue := value.(type) {
	case *interpreter.SomeValue:
		s.cache[storageKey] = typedValue.Value
	case interpreter.NilValue:
		s.cache[storageKey] = nil
	default:
		panic(errors.NewUnreachableError())
	}
}

// writeCached serializes/saves all values in the cache in storage (through the runtime interface).
//
func (s *interpreterRuntimeStorage) writeCached() {

	for storageKey, value := range s.cache {

		var newData []byte
		if value != nil {
			var newStoredData bytes.Buffer
			encoder := gob.NewEncoder(&newStoredData)
			err := encoder.Encode(&value)
			if err != nil {
				panic(err)
			}
			newData = newStoredData.Bytes()
		}

		// TODO: fix controller
		err := s.runtimeInterface.SetValue(
			[]byte(storageKey.storageIdentifier),
			[]byte{},
			[]byte(storageKey.key),
			newData,
		)
		if err != nil {
			panic(err)
		}
	}
}
