package database

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/fxamacker/cbor/v2"

	"github.com/onflow/atree"
)

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
	if uint64(v.ByteSize()) > maxInlineSize {

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

	return v, nil
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
	}

	return nil, fmt.Errorf("not supported type info")
}
