package database

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/fxamacker/cbor/v2"

	"github.com/onflow/atree"
)

const (
	cborTagUInt8Value  = 161
	cborTagUInt64Value = 164
)

type Uint8Value uint8

var _ atree.Value = Uint8Value(0)
var _ atree.Storable = Uint8Value(0)

func (v Uint8Value) ChildStorables() []atree.Storable {
	return nil
}

func (v Uint8Value) StoredValue(_ atree.SlabStorage) (atree.Value, error) {
	return v, nil
}

func (v Uint8Value) Storable(_ atree.SlabStorage, _ atree.Address, _ uint64) (atree.Storable, error) {
	return v, nil
}

// Encode encodes UInt8Value as
//
//	cbor.Tag{
//			Number:  cborTagUInt8Value,
//			Content: uint8(v),
//	}
func (v Uint8Value) Encode(enc *atree.Encoder) error {
	err := enc.CBOR.EncodeRawBytes([]byte{
		// tag number
		0xd8, cborTagUInt8Value,
	})
	if err != nil {
		return err
	}
	return enc.CBOR.EncodeUint8(uint8(v))
}

func (v Uint8Value) getHashInput(scratch []byte) ([]byte, error) {

	const cborTypePositiveInt = 0x00

	buf := scratch
	if len(scratch) < 4 {
		buf = make([]byte, 4)
	}

	buf[0], buf[1] = 0xd8, cborTagUInt8Value // Tag number

	if v <= 23 {
		buf[2] = cborTypePositiveInt | byte(v)
		return buf[:3], nil
	}

	buf[2] = cborTypePositiveInt | byte(24)
	buf[3] = byte(v)
	return buf[:4], nil
}

func (v Uint8Value) ByteSize() uint32 {
	// tag number (2 bytes) + encoded content
	return 2 + atree.GetUintCBORSize(uint64(v))
}

func (v Uint8Value) String() string {
	return fmt.Sprintf("%d", uint8(v))
}

type Uint64Value uint64

var _ atree.Value = Uint64Value(0)
var _ atree.Storable = Uint64Value(0)

func (v Uint64Value) ChildStorables() []atree.Storable {
	return nil
}

func (v Uint64Value) StoredValue(_ atree.SlabStorage) (atree.Value, error) {
	return v, nil
}

func (v Uint64Value) Storable(_ atree.SlabStorage, _ atree.Address, _ uint64) (atree.Storable, error) {
	return v, nil
}

// Encode encodes UInt64Value as
//
//	cbor.Tag{
//			Number:  cborTagUInt64Value,
//			Content: uint64(v),
//	}
func (v Uint64Value) Encode(enc *atree.Encoder) error {
	err := enc.CBOR.EncodeRawBytes([]byte{
		// tag number
		0xd8, cborTagUInt64Value,
	})
	if err != nil {
		return err
	}
	return enc.CBOR.EncodeUint64(uint64(v))
}

func (v Uint64Value) getHashInput(scratch []byte) ([]byte, error) {
	const cborTypePositiveInt = 0x00

	buf := scratch
	if len(buf) < 16 {
		buf = make([]byte, 16)
	}

	buf[0], buf[1] = 0xd8, cborTagUInt64Value // Tag number

	if v <= 23 {
		buf[2] = cborTypePositiveInt | byte(v)
		return buf[:3], nil
	}

	if v <= math.MaxUint8 {
		buf[2] = cborTypePositiveInt | byte(24)
		buf[3] = byte(v)
		return buf[:4], nil
	}

	if v <= math.MaxUint16 {
		buf[2] = cborTypePositiveInt | byte(25)
		binary.BigEndian.PutUint16(buf[3:], uint16(v))
		return buf[:5], nil
	}

	if v <= math.MaxUint32 {
		buf[2] = cborTypePositiveInt | byte(26)
		binary.BigEndian.PutUint32(buf[3:], uint32(v))
		return buf[:7], nil
	}

	buf[2] = cborTypePositiveInt | byte(27)
	binary.BigEndian.PutUint64(buf[3:], uint64(v))
	return buf[:11], nil
}

func (v Uint64Value) ByteSize() uint32 {
	// tag number (2 bytes) + encoded content
	return 2 + atree.GetUintCBORSize(uint64(v))
}

func (v Uint64Value) String() string {
	return fmt.Sprintf("%d", uint64(v))
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
		return NewByteStringValue([]byte(s)), nil

	case cbor.TagType:
		tagNumber, err := dec.DecodeTagNumber()
		if err != nil {
			return nil, err
		}

		switch tagNumber {

		case atree.CBORTagStorageID:
			return atree.DecodeStorageIDStorable(dec)

		case cborTagUInt8Value:
			n, err := dec.DecodeUint64()
			if err != nil {
				return nil, err
			}
			if n > math.MaxUint8 {
				return nil, fmt.Errorf("invalid data, got %d, expected max %d", n, math.MaxUint8)
			}
			return Uint8Value(n), nil

		case cborTagUInt64Value:
			n, err := dec.DecodeUint64()
			if err != nil {
				return nil, err
			}
			return Uint64Value(n), nil

		default:
			return nil, fmt.Errorf("invalid tag number %d", tagNumber)
		}

	default:
		return nil, fmt.Errorf("invalid cbor type %s for storable", t)
	}
}

func compare(storage atree.SlabStorage, value atree.Value, storable atree.Storable) (bool, error) {
	switch v := value.(type) {

	case Uint8Value:
		other, ok := storable.(Uint8Value)
		if !ok {
			return false, nil
		}
		return uint8(other) == uint8(v), nil

	case Uint64Value:
		other, ok := storable.(Uint64Value)
		if !ok {
			return false, nil
		}
		return uint64(other) == uint64(v), nil

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
	case Uint8Value:
	case Uint64Value:
	case ByteStringValue:
		return v.getHashInput(buffer)
	}

	return nil, fmt.Errorf("value %T not supported for hash input", value)
}

type testTypeInfo struct {
	value uint64
}

var _ atree.TypeInfo = testTypeInfo{}

func (i testTypeInfo) Encode(e *cbor.StreamEncoder) error {
	return e.EncodeUint64(i.value)
}

func (i testTypeInfo) Equal(other atree.TypeInfo) bool {
	otherTestTypeInfo, ok := other.(testTypeInfo)
	return ok && i.value == otherTestTypeInfo.value
}

func decodeTypeInfo(dec *cbor.StreamDecoder) (atree.TypeInfo, error) {
	value, err := dec.DecodeUint64()
	if err != nil {
		return nil, err
	}

	return testTypeInfo{value: value}, nil
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

type typeInfo struct{}

var _ atree.TypeInfo = typeInfo{}

func (typeInfo) Encode(e *cbor.StreamEncoder) error {
	return e.EncodeUint8(255)
}

func (i typeInfo) Equal(other atree.TypeInfo) bool {
	_, ok := other.(testTypeInfo)
	return ok
}
