package state

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"runtime"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/atree"
)

const (
	storageIDSize = 16
)

// CollectionProvider provides access to collections
type CollectionProvider struct {
	rootAddr atree.Address
	storage  *atree.PersistentSlabStorage
}

// NewCollectionProvider constructs a new CollectionProvider
func NewCollectionProvider(
	rootAddr atree.Address,
	ledger atree.Ledger,
) (*CollectionProvider, error) {
	// empty address is not allowed (causes issues with atree)
	if rootAddr == atree.AddressUndefined {
		return nil, fmt.Errorf("empty address as root is not allowed")
	}
	baseStorage := atree.NewLedgerBaseStorage(ledger)
	storage, err := NewPersistentSlabStorage(baseStorage)
	return &CollectionProvider{
		rootAddr: rootAddr,
		storage:  storage,
	}, err
}

// CollectionByID returns the collection by collection ID
//
// if no collection is found with that collection id, it return error
// Warning: this method should only used only once for each collection and
// the returned pointer should be kept for the future.
// calling twice for the same collection might result in odd-behaviours
// currently collection provider doesn't do any internal caching to protect aginast these cases
func (cp *CollectionProvider) CollectionByID(collectionID []byte) (*Collection, error) {
	storageID, err := atree.NewStorageIDFromRawBytes(collectionID)
	if err != nil {
		return nil, err
	}
	omap, err := atree.NewMapWithRootID(cp.storage, storageID, atree.NewDefaultDigesterBuilder())
	if err != nil {
		return nil, err
	}
	return &Collection{
		omap:         omap,
		storage:      cp.storage,
		collectionID: collectionID,
	}, nil
}

// NewCollection constructs a new collection
func (cp *CollectionProvider) NewCollection() (*Collection, error) {
	omap, err := atree.NewMap(cp.storage, cp.rootAddr, atree.NewDefaultDigesterBuilder(), emptyTypeInfo{})
	if err != nil {
		return nil, err
	}
	storageIDBytes := make([]byte, storageIDSize)
	_, err = omap.StorageID().ToRawBytes(storageIDBytes)
	if err != nil {
		return nil, err
	}
	return &Collection{
		storage:      cp.storage,
		omap:         omap,
		collectionID: storageIDBytes, // we reuse the storageID bytes as collectionID
	}, nil
}

// Commit commits all changes to the collections with changes
func (cp *CollectionProvider) Commit() error {
	return cp.storage.FastCommit(runtime.NumCPU())
}

// Collection provides a persistent and compact way of storing key/value pairs
// each collection has a unique collectionID that can be used to fetch the collection
//
// TODO(ramtin): we might not need any extra hashing on the atree side
// and optimize this to just use the key given the keys are hashed ?
type Collection struct {
	omap         *atree.OrderedMap
	storage      *atree.PersistentSlabStorage
	collectionID []byte
}

// CollectionID returns the unique id for the collection
func (c *Collection) CollectionID() []byte {
	return c.collectionID
}

// Get gets the value for the given key
//
// if key doesn't exist it returns nil (no error)
func (c *Collection) Get(key []byte) ([]byte, error) {
	data, err := c.omap.Get(compare, hashInputProvider, NewByteStringValue(key))
	if err != nil {
		var keyNotFoundError *atree.KeyNotFoundError
		if errors.As(err, &keyNotFoundError) {
			return nil, nil
		}
		return nil, err
	}

	value, err := data.StoredValue(c.omap.Storage)
	if err != nil {
		return nil, err
	}

	return value.(ByteStringValue).Bytes(), nil
}

// Set sets the value for the given key
//
// if a value already stored at the given key it replaces the value
func (c *Collection) Set(key, value []byte) error {
	existingValueStorable, err := c.omap.Set(compare, hashInputProvider, NewByteStringValue(key), NewByteStringValue(value))
	if err != nil {
		return err
	}

	if id, ok := existingValueStorable.(atree.StorageIDStorable); ok {
		// NOTE: deep remove isn't necessary because value is ByteStringValue (not container)
		err := c.storage.Remove(atree.StorageID(id))
		if err != nil {
			return err
		}
	}
	return nil
}

// Remove removes a key from the collection
//
// if the key doesn't exist it return no error
func (c *Collection) Remove(key []byte) error {
	_, existingValueStorable, err := c.omap.Remove(compare, hashInputProvider, NewByteStringValue(key))
	if err != nil {
		var keyNotFoundError *atree.KeyNotFoundError
		if errors.As(err, &keyNotFoundError) {
			return nil
		}
		return err
	}

	if id, ok := existingValueStorable.(atree.StorageIDStorable); ok {
		// NOTE: deep remove isn't necessary because value is ByteStringValue (not container)
		err := c.storage.Remove(atree.StorageID(id))
		if err != nil {
			return err
		}
	}
	return nil
}

// Destroy destroys the whole collection
func (c *Collection) Destroy() error {
	var cachedErr error
	err := c.omap.PopIterate(func(_ atree.Storable, valueStorable atree.Storable) {
		if id, ok := valueStorable.(atree.StorageIDStorable); ok {
			err := c.storage.Remove(atree.StorageID(id))
			if err != nil && cachedErr == nil {
				cachedErr = err
			}
		}
	})
	if cachedErr != nil {
		return cachedErr
	}
	if err != nil {
		return err
	}
	return c.storage.Remove(c.omap.StorageID())
}

type ByteStringValue struct {
	data []byte
	size uint32
}

var _ atree.Value = &ByteStringValue{}
var _ atree.Storable = &ByteStringValue{}

func NewByteStringValue(data []byte) ByteStringValue {
	size := atree.GetUintCBORSize(uint64(len(data))) + uint32(len(data))
	return ByteStringValue{data: data, size: size}
}

func (v ByteStringValue) ChildStorables() []atree.Storable {
	return nil
}

func (v ByteStringValue) StoredValue(_ atree.SlabStorage) (atree.Value, error) {
	return v, nil
}

func (v ByteStringValue) Storable(storage atree.SlabStorage, address atree.Address, maxInlineSize uint64) (atree.Storable, error) {
	if uint64(v.ByteSize()) <= maxInlineSize {
		return v, nil
	}

	// Create StorableSlab
	id, err := storage.GenerateStorageID(address)
	if err != nil {
		return nil, err
	}

	slab := &atree.StorableSlab{
		StorageID: id,
		Storable:  v,
	}

	// Store StorableSlab in storage
	err = storage.Store(id, slab)
	if err != nil {
		return nil, err
	}

	// Return storage id as storable
	return atree.StorageIDStorable(id), nil
}

func (v ByteStringValue) Encode(enc *atree.Encoder) error {
	return enc.CBOR.EncodeBytes(v.data)
}

func (v ByteStringValue) getHashInput(scratch []byte) ([]byte, error) {

	const cborTypeByteString = 0x40

	buf := scratch
	if uint32(len(buf)) < v.size {
		buf = make([]byte, v.size)
	} else {
		buf = buf[:v.size]
	}

	slen := len(v.data)

	if slen <= 23 {
		buf[0] = cborTypeByteString | byte(slen)
		copy(buf[1:], v.data)
		return buf, nil
	}

	if slen <= math.MaxUint8 {
		buf[0] = cborTypeByteString | byte(24)
		buf[1] = byte(slen)
		copy(buf[2:], v.data)
		return buf, nil
	}

	if slen <= math.MaxUint16 {
		buf[0] = cborTypeByteString | byte(25)
		binary.BigEndian.PutUint16(buf[1:], uint16(slen))
		copy(buf[3:], v.data)
		return buf, nil
	}

	if slen <= math.MaxUint32 {
		buf[0] = cborTypeByteString | byte(26)
		binary.BigEndian.PutUint32(buf[1:], uint32(slen))
		copy(buf[5:], v.data)
		return buf, nil
	}

	buf[0] = cborTypeByteString | byte(27)
	binary.BigEndian.PutUint64(buf[1:], uint64(slen))
	copy(buf[9:], v.data)
	return buf, nil
}

func (v ByteStringValue) ByteSize() uint32 {
	return v.size
}

func (v ByteStringValue) String() string {
	return string(v.data)
}

func (v ByteStringValue) Bytes() []byte {
	return v.data
}

func decodeStorable(dec *cbor.StreamDecoder, _ atree.StorageID) (atree.Storable, error) {
	t, err := dec.NextType()
	if err != nil {
		return nil, err
	}

	switch t {
	case cbor.ByteStringType:
		s, err := dec.DecodeBytes()
		if err != nil {
			return nil, err
		}
		return NewByteStringValue(s), nil

	case cbor.TagType:
		tagNumber, err := dec.DecodeTagNumber()
		if err != nil {
			return nil, err
		}

		switch tagNumber {

		case atree.CBORTagStorageID:
			return atree.DecodeStorageIDStorable(dec)

		default:
			return nil, fmt.Errorf("invalid tag number %d", tagNumber)
		}

	default:
		return nil, fmt.Errorf("invalid cbor type %s for storable", t)
	}
}

func compare(storage atree.SlabStorage, value atree.Value, storable atree.Storable) (bool, error) {
	switch v := value.(type) {

	case ByteStringValue:
		other, ok := storable.(ByteStringValue)
		if ok {
			return bytes.Equal(other.data, v.data), nil
		}

		// Retrieve value from storage
		otherValue, err := storable.StoredValue(storage)
		if err != nil {
			return false, err
		}
		other, ok = otherValue.(ByteStringValue)
		if ok {
			return bytes.Equal(other.data, v.data), nil
		}

		return false, nil
	}

	return false, fmt.Errorf("value %T not supported for comparison", value)
}

func hashInputProvider(value atree.Value, buffer []byte) ([]byte, error) {
	switch v := value.(type) {
	case ByteStringValue:
		return v.getHashInput(buffer)
	}

	return nil, fmt.Errorf("value %T not supported for hash input", value)
}

func NewPersistentSlabStorage(baseStorage atree.BaseStorage) (*atree.PersistentSlabStorage, error) {
	encMode, err := cbor.EncOptions{}.EncMode()
	if err != nil {
		return nil, err
	}

	decMode, err := cbor.DecOptions{}.DecMode()
	if err != nil {
		return nil, err
	}

	return atree.NewPersistentSlabStorage(
		baseStorage,
		encMode,
		decMode,
		decodeStorable,
		decodeTypeInfo,
	), nil
}

type emptyTypeInfo struct{}

var _ atree.TypeInfo = emptyTypeInfo{}

func (emptyTypeInfo) Encode(e *cbor.StreamEncoder) error {
	return e.EncodeNil()
}

func (i emptyTypeInfo) Equal(other atree.TypeInfo) bool {
	_, ok := other.(emptyTypeInfo)
	return ok
}

func decodeTypeInfo(dec *cbor.StreamDecoder) (atree.TypeInfo, error) {
	ty, err := dec.NextType()
	if err != nil {
		return nil, err
	}
	switch ty {
	case cbor.NilType:
		err := dec.DecodeNil()
		if err != nil {
			return nil, err
		}
		return emptyTypeInfo{}, nil
	default:
	}

	return nil, fmt.Errorf("not supported type info")
}
