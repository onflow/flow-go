package interpreter

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/big"
	"strconv"

	"github.com/fxamacker/cbor/v2"

	"github.com/onflow/cadence-initial/runtime/ast"
	"github.com/onflow/cadence-initial/runtime/common"
	"github.com/onflow/cadence-initial/runtime/sema"
)

// A Decoder decodes CBOR-encoded representations of values.
//
type Decoder struct {
	decoder *cbor.Decoder
	owner   *common.Address
}

// Decode returns a value decoded from its CBOR-encoded representation,
// for the given owner (can be `nil`).
//
// The given path is used to identify values in the object graph.
// For example, path elements are appended for array elements (the index),
// dictionary values (the key), and composites (the field name).
//
func DecodeValue(b []byte, owner *common.Address, path []string) (Value, error) {
	reader := bytes.NewReader(b)

	decoder, err := NewDecoder(reader, owner)
	if err != nil {
		return nil, err
	}

	v, err := decoder.Decode(path)
	if err != nil {
		return nil, err
	}

	return v, nil
}

// NewDecoder initializes a Decoder that will decode CBOR-encoded bytes from the
// given io.Reader.
//
// It sets the given address as the owner (can be `nil`).
//
func NewDecoder(r io.Reader, owner *common.Address) (*Decoder, error) {
	decMode, err := cbor.DecOptions{}.DecModeWithTags(cborTagSet)
	if err != nil {
		return nil, err
	}

	return &Decoder{
		decoder: decMode.NewDecoder(r),
		owner:   owner,
	}, nil
}

// Decode reads CBOR-encoded bytes from the io.Reader and decodes them to a value.
//
func (d *Decoder) Decode(path []string) (Value, error) {
	var v interface{}
	err := d.decoder.Decode(&v)
	if err != nil {
		return nil, err
	}

	return d.decodeValue(v, path)
}

func (d *Decoder) decodeValue(v interface{}, path []string) (Value, error) {
	switch v := v.(type) {

	// CBOR Types

	case bool:
		return BoolValue(v), nil

	case string:
		return d.decodeString(v), nil

	case nil:
		return NilValue{}, nil

	case []interface{}:
		return d.decodeArray(v, path)

	case cbor.Tag:
		switch v.Number {

		case cborTagVoidValue:
			return VoidValue{}, nil

		case cborTagDictionaryValue:
			return d.decodeDictionary(v.Content, path)

		case cborTagSomeValue:
			return d.decodeSome(v.Content, path)

		case cborTagAddressValue:
			return d.decodeAddress(v.Content)

		case cborTagCompositeValue:
			return d.decodeComposite(v.Content, path)

		// Int*

		case cborTagIntValue:
			return d.decodeInt(v.Content)

		case cborTagInt8Value:
			return d.decodeInt8(v.Content)

		case cborTagInt16Value:
			return d.decodeInt16(v.Content)

		case cborTagInt32Value:
			return d.decodeInt32(v.Content)

		case cborTagInt64Value:
			return d.decodeInt64(v.Content)

		case cborTagInt128Value:
			return d.decodeInt128(v.Content)

		case cborTagInt256Value:
			return d.decodeInt256(v.Content)

		// UInt*

		case cborTagUIntValue:
			return d.decodeUInt(v.Content)

		case cborTagUInt8Value:
			return d.decodeUInt8(v.Content)

		case cborTagUInt16Value:
			return d.decodeUInt16(v.Content)

		case cborTagUInt32Value:
			return d.decodeUInt32(v.Content)

		case cborTagUInt64Value:
			return d.decodeUInt64(v.Content)

		case cborTagUInt128Value:
			return d.decodeUInt128(v.Content)

		case cborTagUInt256Value:
			return d.decodeUInt256(v.Content)

		// Word*

		case cborTagWord8Value:
			return d.decodeWord8(v.Content)

		case cborTagWord16Value:
			return d.decodeWord16(v.Content)

		case cborTagWord32Value:
			return d.decodeWord32(v.Content)

		case cborTagWord64Value:
			return d.decodeWord64(v.Content)

		// Fix*

		case cborTagFix64Value:
			return d.decodeFix64(v.Content)

		// UFix*

		case cborTagUFix64Value:
			return d.decodeUFix64(v.Content)

		// Storage

		case cborTagPathValue:
			return d.decodePath(v.Content)

		case cborTagCapabilityValue:
			return d.decodeCapability(v.Content)

		case cborTagStorageReferenceValue:
			return d.decodeStorageReference(v.Content)

		case cborTagLinkValue:
			return d.decodeLink(v.Content)

		default:
			return nil, fmt.Errorf("unsupported decoded tag: %d, %v", v.Number, v.Content)
		}

	default:
		return nil, fmt.Errorf("unsupported decoded type: %[1]T, %[1]v", v)
	}
}

func (d *Decoder) decodeString(v string) Value {
	value := NewStringValue(v)
	value.modified = false
	return value
}

func (d *Decoder) decodeArray(v []interface{}, path []string) (*ArrayValue, error) {
	values := make([]Value, len(v))
	for i, value := range v {
		valuePath := append(path[:], strconv.Itoa(i))
		res, err := d.decodeValue(value, valuePath)
		if err != nil {
			return nil, fmt.Errorf("invalid array element encoding: %w", err)
		}
		values[i] = res
	}

	return &ArrayValue{
		Values:   values,
		Owner:    d.owner,
		modified: false,
	}, nil
}

func (d *Decoder) decodeDictionary(v interface{}, path []string) (*DictionaryValue, error) {
	encoded, ok := v.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid dictionary encoding: %T", v)
	}

	encodedKeys, ok := encoded[uint64(0)].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid dictionary keys encoding")
	}

	keysPath := append(path[:], dictionaryKeyPathPrefix)
	keys, err := d.decodeArray(encodedKeys, keysPath)
	if err != nil {
		return nil, fmt.Errorf("invalid dictionary keys encoding: %w", err)
	}

	encodedEntries, ok := encoded[uint64(1)].(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid dictionary entries encoding")
	}

	keyCount := keys.Count()
	entryCount := len(encodedEntries)

	// The number of entries must either match the number of keys,
	// or be zero in case the values are deferred

	countMismatch := entryCount != keyCount
	if countMismatch && entryCount != 0 {
		return nil, fmt.Errorf(
			"invalid dictionary encoding: key and entry count mismatch: expected %d, got %d",
			keyCount,
			entryCount,
		)
	}

	var entries map[string]Value
	var deferred map[string]string
	var deferredOwner *common.Address

	// Are the values in the dictionary deferred, i.e. are they encoded
	// separately and stored in separate storage keys?

	isDeferred := countMismatch && entryCount == 0

	if isDeferred {

		deferred = make(map[string]string, keyCount)
		entries = map[string]Value{}
		deferredOwner = d.owner
		for _, keyValue := range keys.Values {
			key := dictionaryKey(keyValue)
			deferred[key] = joinPath(append(path[:], dictionaryValuePathPrefix, key))
		}

	} else {
		entries = make(map[string]Value, entryCount)

		for key, value := range encodedEntries {

			keyString, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("invalid dictionary key encoding")
			}

			valuePath := append(path[:], dictionaryValuePathPrefix, keyString)
			decodedValue, err := d.decodeValue(value, valuePath)
			if err != nil {
				return nil, fmt.Errorf("invalid dictionary value encoding: %w", err)
			}

			entries[keyString] = decodedValue
		}
	}

	return &DictionaryValue{
		Keys:          keys,
		Entries:       entries,
		Owner:         d.owner,
		modified:      false,
		DeferredOwner: deferredOwner,
		DeferredKeys:  deferred,
	}, nil
}

func (d *Decoder) decodeLocation(l interface{}) (ast.Location, error) {
	tag, ok := l.(cbor.Tag)
	if !ok {
		return nil, fmt.Errorf("invalid location encoding: %T", l)
	}

	content := tag.Content

	switch tag.Number {
	case cborTagAddressLocation:
		return d.decodeAddressLocation(content)

	case cborTagStringLocation:
		return d.decodeStringLocation(content)

	default:
		return nil, fmt.Errorf("invalid location encoding tag: %d", tag.Number)
	}
}

func (d *Decoder) decodeAddressLocation(content interface{}) (ast.Location, error) {
	b, ok := content.([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid address location encoding: %T", content)
	}

	err := d.checkAddressLength(b)
	if err != nil {
		return nil, err
	}

	return ast.AddressLocation(b), nil
}

func (d *Decoder) decodeStringLocation(content interface{}) (ast.Location, error) {
	s, ok := content.(string)
	if !ok {
		return nil, fmt.Errorf("invalid string location encoding: %T", content)
	}
	return ast.StringLocation(s), nil
}

func (d *Decoder) decodeComposite(v interface{}, path []string) (*CompositeValue, error) {

	encoded, ok := v.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid composite encoding: %T", v)
	}

	// Location

	location, err := d.decodeLocation(encoded[uint64(0)])
	if err != nil {
		return nil, fmt.Errorf("invalid composite location encoding: %w", err)
	}

	// Type ID

	field2 := encoded[uint64(1)]
	encodedTypeID, ok := field2.(string)
	if !ok {
		return nil, fmt.Errorf("invalid composite type ID encoding: %T", field2)
	}
	typeID := sema.TypeID(encodedTypeID)

	// Kind

	field3 := encoded[uint64(2)]
	encodedKind, ok := field3.(uint64)
	if !ok {
		return nil, fmt.Errorf("invalid composite kind encoding: %T", field3)
	}
	kind := common.CompositeKind(encodedKind)

	// Fields

	field4 := encoded[uint64(3)]
	encodedFields, ok := field4.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid composite fields encoding")
	}

	fields := make(map[string]Value, len(encodedFields))

	for name, value := range encodedFields {
		nameString, ok := name.(string)
		if !ok {
			return nil, fmt.Errorf("invalid dictionary field name encoding: %T", name)
		}
		valuePath := append(path[:], nameString)
		decodedValue, err := d.decodeValue(value, valuePath)
		if err != nil {
			return nil, fmt.Errorf("invalid dictionary field value encoding: %w", err)
		}

		fields[nameString] = decodedValue
	}

	compositeValue := NewCompositeValue(location, typeID, kind, fields, d.owner)
	compositeValue.modified = false
	return compositeValue, nil
}

func (d *Decoder) decodeBig(v interface{}) (*big.Int, error) {
	tag, ok := v.(cbor.Tag)
	if !ok {
		return nil, fmt.Errorf("invalid bignum encoding: %T", v)
	}

	b, ok := tag.Content.([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid bignum content encoding: %T", v)
	}

	bigInt := new(big.Int).SetBytes(b)

	switch tag.Number {
	case cborTagPositiveBignum:
		break
	case cborTagNegativeBignum:
		bigInt.Neg(bigInt)
	default:
		return nil, fmt.Errorf("invalid Int content tag: %v", tag.Number)
	}

	return bigInt, nil
}

func (d *Decoder) decodeInt(v interface{}) (IntValue, error) {
	bigInt, err := d.decodeBig(v)
	if err != nil {
		return IntValue{}, fmt.Errorf("invalid Int encoding: %w", err)
	}

	return NewIntValueFromBigInt(bigInt), nil
}

func (d *Decoder) decodeInt8(v interface{}) (Int8Value, error) {
	switch v := v.(type) {
	case uint64:
		const max = math.MaxInt8
		if v > max {
			return 0, fmt.Errorf("invalid Int8: got %d, expected max %d", v, max)
		}
		return Int8Value(v), nil

	case int64:
		const min = math.MinInt8
		if v < min {
			return 0, fmt.Errorf("invalid Int8: got %d, expected min %d", v, min)
		}
		const max = math.MaxInt8
		if v > max {
			return 0, fmt.Errorf("invalid Int8: got %d, expected max %d", v, max)
		}
		return Int8Value(v), nil

	default:
		return 0, fmt.Errorf("unknown Int8 encoding: %T", v)
	}
}

func (d *Decoder) decodeInt16(v interface{}) (Int16Value, error) {
	switch v := v.(type) {
	case uint64:
		const max = math.MaxInt16
		if v > max {
			return 0, fmt.Errorf("invalid Int16: got %d, expected max %d", v, max)
		}
		return Int16Value(v), nil

	case int64:
		const min = math.MinInt16
		if v < min {
			return 0, fmt.Errorf("invalid Int16: got %d, expected min %d", v, min)
		}
		const max = math.MaxInt16
		if v > max {
			return 0, fmt.Errorf("invalid Int16: got %d, expected max %d", v, max)
		}
		return Int16Value(v), nil

	default:
		return 0, fmt.Errorf("unknown Int16 encoding: %T", v)
	}
}

func (d *Decoder) decodeInt32(v interface{}) (Int32Value, error) {
	switch v := v.(type) {
	case uint64:
		const max = math.MaxInt32
		if v > max {
			return 0, fmt.Errorf("invalid Int32: got %d, expected max %d", v, max)
		}
		return Int32Value(v), nil

	case int64:
		const min = math.MinInt32
		if v < min {
			return 0, fmt.Errorf("invalid Int32: got %d, expected min %d", v, min)
		}
		const max = math.MaxInt32
		if v > max {
			return 0, fmt.Errorf("invalid Int32: got %d, expected max %d", v, max)
		}
		return Int32Value(v), nil

	default:
		return 0, fmt.Errorf("unknown Int32 encoding: %T", v)
	}
}

func (d *Decoder) decodeInt64(v interface{}) (Int64Value, error) {
	switch v := v.(type) {
	case uint64:
		const max = math.MaxInt64
		if v > max {
			return 0, fmt.Errorf("invalid Int64: got %d, expected max %d", v, max)
		}
		return Int64Value(v), nil

	case int64:
		return Int64Value(v), nil

	default:
		return 0, fmt.Errorf("unknown Int64 encoding: %T", v)
	}
}

func (d *Decoder) decodeInt128(v interface{}) (Int128Value, error) {
	bigInt, err := d.decodeBig(v)
	if err != nil {
		return Int128Value{}, fmt.Errorf("invalid Int encoding: %w", err)
	}

	min := sema.Int128TypeMinIntBig
	if bigInt.Cmp(min) < 0 {
		return Int128Value{}, fmt.Errorf("invalid Int128: got %s, expected min %s", bigInt, min)
	}

	max := sema.Int128TypeMaxIntBig
	if bigInt.Cmp(max) > 0 {
		return Int128Value{}, fmt.Errorf("invalid Int128: got %s, expected max %s", bigInt, max)
	}

	return NewInt128ValueFromBigInt(bigInt), nil
}

func (d *Decoder) decodeInt256(v interface{}) (Int256Value, error) {
	bigInt, err := d.decodeBig(v)
	if err != nil {
		return Int256Value{}, fmt.Errorf("invalid Int encoding: %w", err)
	}

	min := sema.Int256TypeMinIntBig
	if bigInt.Cmp(min) < 0 {
		return Int256Value{}, fmt.Errorf("invalid Int256: got %s, expected min %s", bigInt, min)
	}

	max := sema.Int256TypeMaxIntBig
	if bigInt.Cmp(max) > 0 {
		return Int256Value{}, fmt.Errorf("invalid Int256: got %s, expected max %s", bigInt, max)
	}

	return NewInt256ValueFromBigInt(bigInt), nil
}

func (d *Decoder) decodeUInt(v interface{}) (UIntValue, error) {
	bigInt, err := d.decodeBig(v)
	if err != nil {
		return UIntValue{}, fmt.Errorf("invalid UInt encoding: %w", err)
	}

	if bigInt.Sign() < 0 {
		return UIntValue{}, fmt.Errorf("invalid UInt: got %s, expected positive", bigInt)
	}

	return NewUIntValueFromBigInt(bigInt), nil
}

func (d *Decoder) decodeUInt8(v interface{}) (UInt8Value, error) {
	value, ok := v.(uint64)
	if !ok {
		return 0, fmt.Errorf("unknown UInt8 encoding: %T", v)
	}
	const max = math.MaxUint8
	if value > max {
		return 0, fmt.Errorf("invalid UInt8: got %d, expected max %d", v, max)
	}
	return UInt8Value(value), nil
}

func (d *Decoder) decodeUInt16(v interface{}) (UInt16Value, error) {
	value, ok := v.(uint64)
	if !ok {
		return 0, fmt.Errorf("unknown UInt16 encoding: %T", v)
	}
	const max = math.MaxUint16
	if value > max {
		return 0, fmt.Errorf("invalid UInt16: got %d, expected max %d", v, max)
	}
	return UInt16Value(value), nil
}

func (d *Decoder) decodeUInt32(v interface{}) (UInt32Value, error) {
	value, ok := v.(uint64)
	if !ok {
		return 0, fmt.Errorf("unknown UInt32 encoding: %T", v)
	}
	const max = math.MaxUint32
	if value > max {
		return 0, fmt.Errorf("invalid UInt32: got %d, expected max %d", v, max)
	}
	return UInt32Value(value), nil
}

func (d *Decoder) decodeUInt64(v interface{}) (UInt64Value, error) {
	value, ok := v.(uint64)
	if !ok {
		return 0, fmt.Errorf("unknown UInt64 encoding: %T", v)
	}
	return UInt64Value(value), nil
}

func (d *Decoder) decodeUInt128(v interface{}) (UInt128Value, error) {
	bigInt, err := d.decodeBig(v)
	if err != nil {
		return UInt128Value{}, fmt.Errorf("invalid Int encoding: %w", err)
	}

	if bigInt.Sign() < 0 {
		return UInt128Value{}, fmt.Errorf("invalid UInt128: got %s, expected positive", bigInt)
	}

	max := sema.UInt128TypeMaxIntBig
	if bigInt.Cmp(max) > 0 {
		return UInt128Value{}, fmt.Errorf("invalid UInt128: got %s, expected max %s", bigInt, max)
	}

	return NewUInt128ValueFromBigInt(bigInt), nil
}

func (d *Decoder) decodeUInt256(v interface{}) (UInt256Value, error) {
	bigInt, err := d.decodeBig(v)
	if err != nil {
		return UInt256Value{}, fmt.Errorf("invalid Int encoding: %w", err)
	}

	if bigInt.Sign() < 0 {
		return UInt256Value{}, fmt.Errorf("invalid UInt256: got %s, expected positive", bigInt)
	}

	max := sema.UInt256TypeMaxIntBig
	if bigInt.Cmp(max) > 0 {
		return UInt256Value{}, fmt.Errorf("invalid UInt256: got %s, expected max %s", bigInt, max)
	}

	return NewUInt256ValueFromBigInt(bigInt), nil
}

func (d *Decoder) decodeWord8(v interface{}) (Word8Value, error) {
	value, ok := v.(uint64)
	if !ok {
		return 0, fmt.Errorf("unknown Word8 encoding: %T", v)
	}
	const max = math.MaxUint8
	if value > max {
		return 0, fmt.Errorf("invalid Word8: got %d, expected max %d", v, max)
	}
	return Word8Value(value), nil
}

func (d *Decoder) decodeWord16(v interface{}) (Word16Value, error) {
	value, ok := v.(uint64)
	if !ok {
		return 0, fmt.Errorf("unknown Word16 encoding: %T", v)
	}
	const max = math.MaxUint16
	if value > max {
		return 0, fmt.Errorf("invalid Word16: got %d, expected max %d", v, max)
	}
	return Word16Value(value), nil
}

func (d *Decoder) decodeWord32(v interface{}) (Word32Value, error) {
	value, ok := v.(uint64)
	if !ok {
		return 0, fmt.Errorf("unknown Word32 encoding: %T", v)
	}
	const max = math.MaxUint32
	if value > max {
		return 0, fmt.Errorf("invalid Word32: got %d, expected max %d", v, max)
	}
	return Word32Value(value), nil
}

func (d *Decoder) decodeWord64(v interface{}) (Word64Value, error) {
	value, ok := v.(uint64)
	if !ok {
		return 0, fmt.Errorf("unknown Word64 encoding: %T", v)
	}
	return Word64Value(value), nil
}

func (d *Decoder) decodeFix64(v interface{}) (Fix64Value, error) {
	switch v := v.(type) {
	case uint64:
		const max = math.MaxInt64
		if v > max {
			return 0, fmt.Errorf("invalid Fix64: got %d, expected max %d", v, max)
		}
		return Fix64Value(v), nil

	case int64:
		return Fix64Value(v), nil

	default:
		return 0, fmt.Errorf("unknown Fix64 encoding: %T", v)
	}
}

func (d *Decoder) decodeUFix64(v interface{}) (UFix64Value, error) {
	value, ok := v.(uint64)
	if !ok {
		return 0, fmt.Errorf("unknown UFix64 encoding: %T", v)
	}
	return UFix64Value(value), nil
}

func (d *Decoder) decodeSome(v interface{}, path []string) (*SomeValue, error) {
	value, err := d.decodeValue(v, path)
	if err != nil {
		return nil, fmt.Errorf("invalid some value encoding: %w", err)
	}

	return &SomeValue{
		Value: value,
		Owner: d.owner,
	}, nil
}

func (d *Decoder) decodeStorageReference(v interface{}) (*StorageReferenceValue, error) {
	encoded, ok := v.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid storage reference encoding: %T", v)
	}

	authorized, ok := encoded[uint64(0)].(bool)
	if !ok {
		return nil, fmt.Errorf("invalid storage reference authorized encoding: %T", authorized)
	}

	targetStorageAddressBytes, ok := encoded[uint64(1)].([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid storage reference target storage address encoding: %T", authorized)
	}

	targetStorageAddress := common.BytesToAddress(targetStorageAddressBytes)

	targetKey, ok := encoded[uint64(2)].(string)
	if !ok {
		return nil, fmt.Errorf("invalid storage reference target key encoding: %T", targetKey)
	}

	return &StorageReferenceValue{
		Authorized:           authorized,
		TargetStorageAddress: targetStorageAddress,
		TargetKey:            targetKey,
	}, nil
}

func (d *Decoder) checkAddressLength(addressBytes []byte) error {
	actualLength := len(addressBytes)
	const expectedLength = common.AddressLength
	if actualLength > expectedLength {
		return fmt.Errorf(
			"invalid address length: got %d, expected max %d",
			actualLength,
			expectedLength,
		)
	}
	return nil
}

func (d *Decoder) decodeAddress(v interface{}) (AddressValue, error) {
	addressBytes, ok := v.([]byte)
	if !ok {
		return AddressValue{}, fmt.Errorf("invalid address encoding: %T", v)
	}

	err := d.checkAddressLength(addressBytes)
	if err != nil {
		return AddressValue{}, err
	}

	address := NewAddressValueFromBytes(addressBytes)
	return address, nil
}

func (d *Decoder) decodePath(v interface{}) (PathValue, error) {
	encoded, ok := v.(map[interface{}]interface{})
	if !ok {
		return PathValue{}, fmt.Errorf("invalid path encoding: %T", v)
	}

	field1 := encoded[uint64(0)]
	domain, ok := field1.(uint64)
	if !ok {
		return PathValue{}, fmt.Errorf("invalid path domain encoding: %T", field1)
	}

	field2 := encoded[uint64(1)]
	identifier, ok := field2.(string)
	if !ok {
		return PathValue{}, fmt.Errorf("invalid path identifier encoding: %T", field2)
	}

	return PathValue{
		Domain:     common.PathDomain(domain),
		Identifier: identifier,
	}, nil
}

func (d *Decoder) decodeCapability(v interface{}) (CapabilityValue, error) {
	encoded, ok := v.(map[interface{}]interface{})
	if !ok {
		return CapabilityValue{}, fmt.Errorf("invalid capability encoding: %T", v)
	}

	// address

	field1 := encoded[uint64(0)]
	field1Value, err := d.decodeValue(field1, nil)
	if err != nil {
		return CapabilityValue{}, fmt.Errorf("invalid capability address: %w", err)
	}

	address, ok := field1Value.(AddressValue)
	if !ok {
		return CapabilityValue{}, fmt.Errorf("invalid capability address: %T", address)
	}

	// path

	field2 := encoded[uint64(1)]
	field2Value, err := d.decodeValue(field2, nil)
	if err != nil {
		return CapabilityValue{}, fmt.Errorf("invalid capability path: %w", err)
	}

	path, ok := field2Value.(PathValue)
	if !ok {
		return CapabilityValue{}, fmt.Errorf("invalid capability path: %T", path)
	}

	return CapabilityValue{
		Address: address,
		Path:    path,
	}, nil
}

func (d *Decoder) decodeLink(v interface{}) (LinkValue, error) {
	encoded, ok := v.(map[interface{}]interface{})
	if !ok {
		return LinkValue{}, fmt.Errorf("invalid link encoding")
	}

	decodedPath, err := d.decodeValue(encoded[uint64(0)], nil)
	if err != nil {
		return LinkValue{}, fmt.Errorf("invalid link target path encoding: %w", err)
	}

	pathValue, ok := decodedPath.(PathValue)
	if !ok {
		return LinkValue{}, fmt.Errorf("invalid link target path encoding: %T", decodedPath)
	}

	decodedStaticType, err := d.decodeStaticType(encoded[uint64(1)])
	if err != nil {
		return LinkValue{}, fmt.Errorf("invalid link type encoding: %w", err)
	}

	staticType, ok := decodedStaticType.(StaticType)
	if !ok {
		return LinkValue{}, fmt.Errorf("invalid link type encoding: %T", decodedStaticType)
	}

	return LinkValue{
		TargetPath: pathValue,
		Type:       staticType,
	}, nil
}

func (d *Decoder) decodeStaticType(v interface{}) (StaticType, error) {
	tag, ok := v.(cbor.Tag)
	if !ok {
		return nil, fmt.Errorf("invalid static type encoding: %T", v)
	}

	content := tag.Content

	switch tag.Number {
	case cborTagPrimitiveStaticType:
		return d.decodePrimitiveStaticType(content)

	case cborTagOptionalStaticType:
		return d.decodeOptionalStaticType(content)

	case cborTagCompositeStaticType:
		return d.decodeCompositeStaticType(content)

	case cborTagInterfaceStaticType:
		return d.decodeInterfaceStaticType(content)

	case cborTagVariableSizedStaticType:
		return d.decodeVariableSizedStaticType(content)

	case cborTagConstantSizedStaticType:
		return d.decodeConstantSizedStaticType(content)

	case cborTagReferenceStaticType:
		return d.decodeReferenceStaticType(content)

	case cborTagDictionaryStaticType:
		return d.decodeDictionaryStaticType(content)

	case cborTagRestrictedStaticType:
		return d.decodeRestrictedStaticType(content)

	default:
		return nil, fmt.Errorf("invalid static type encoding tag: %d", tag.Number)
	}
}

func (d *Decoder) decodePrimitiveStaticType(v interface{}) (PrimitiveStaticType, error) {
	encoded, ok := v.(uint64)
	if !ok {
		return PrimitiveStaticTypeUnknown,
			fmt.Errorf("invalid primitive static type encoding: %T", v)
	}
	return PrimitiveStaticType(encoded), nil
}

func (d *Decoder) decodeOptionalStaticType(v interface{}) (StaticType, error) {
	staticType, err := d.decodeStaticType(v)
	if err != nil {
		return nil, fmt.Errorf("invalid optional static type inner type encoding: %w", err)
	}
	return OptionalStaticType{
		Type: staticType,
	}, nil
}

func (d *Decoder) decodeLocationAndTypeID(v interface{}) (ast.Location, sema.TypeID, error) {
	encoded, ok := v.(map[interface{}]interface{})
	if !ok {
		return nil, "", fmt.Errorf("invalid static type encoding: %T", v)
	}

	location, err := d.decodeLocation(encoded[uint64(0)])
	if err != nil {
		return nil, "", fmt.Errorf("invalid static type location encoding: %w", err)
	}

	field2 := encoded[uint64(1)]
	encodedTypeID, ok := field2.(string)
	if !ok {
		return nil, "", fmt.Errorf("invalid static type type ID encoding: %T", field2)
	}
	typeID := sema.TypeID(encodedTypeID)

	return location, typeID, nil
}

func (d *Decoder) decodeCompositeStaticType(v interface{}) (StaticType, error) {
	location, typeID, err := d.decodeLocationAndTypeID(v)
	if err != nil {
		return nil, fmt.Errorf("invalid composite static type encoding: %w", err)
	}

	return CompositeStaticType{
		Location: location,
		TypeID:   typeID,
	}, nil
}

func (d *Decoder) decodeInterfaceStaticType(v interface{}) (StaticType, error) {
	location, typeID, err := d.decodeLocationAndTypeID(v)
	if err != nil {
		return nil, fmt.Errorf("invalid interface static type encoding: %w", err)
	}

	return InterfaceStaticType{
		Location: location,
		TypeID:   typeID,
	}, nil
}

func (d *Decoder) decodeVariableSizedStaticType(v interface{}) (StaticType, error) {
	staticType, err := d.decodeStaticType(v)
	if err != nil {
		return nil, fmt.Errorf("invalid optional static type encoding: %w", err)
	}
	return VariableSizedStaticType{
		Type: staticType,
	}, nil
}

func (d *Decoder) decodeConstantSizedStaticType(v interface{}) (StaticType, error) {
	encoded, ok := v.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid constant-sized static type encoding: %T", v)
	}

	field1 := encoded[uint64(0)]
	size, ok := field1.(uint64)
	if !ok {
		return nil, fmt.Errorf("invalid constant-sized static type size encoding: %T", field1)
	}

	const max = math.MaxInt64
	if size > max {
		return nil, fmt.Errorf(
			"invalid constant-sized static type size: got %d, expected max %d",
			size,
			max,
		)
	}

	staticType, err := d.decodeStaticType(encoded[uint64(1)])
	if err != nil {
		return nil, fmt.Errorf("invalid constant-sized static type inner type encoding: %w", err)
	}

	return ConstantSizedStaticType{
		Type: staticType,
		Size: int64(size),
	}, nil
}

func (d *Decoder) decodeReferenceStaticType(v interface{}) (StaticType, error) {
	encoded, ok := v.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid reference static type encoding: %T", v)
	}

	field1 := encoded[uint64(0)]
	authorized, ok := field1.(bool)
	if !ok {
		return nil, fmt.Errorf("invalid reference static type authorized encoding: %T", field1)
	}

	staticType, err := d.decodeStaticType(encoded[uint64(1)])
	if err != nil {
		return nil, fmt.Errorf("invalid reference static type inner type encoding: %w", err)
	}

	return ReferenceStaticType{
		Authorized: authorized,
		Type:       staticType,
	}, nil
}

func (d *Decoder) decodeDictionaryStaticType(v interface{}) (StaticType, error) {
	encoded, ok := v.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid dictionary static type encoding: %T", v)
	}

	keyType, err := d.decodeStaticType(encoded[uint64(0)])
	if err != nil {
		return nil, fmt.Errorf("invalid dictionary static type key type encoding: %w", err)
	}

	valueType, err := d.decodeStaticType(encoded[uint64(1)])
	if err != nil {
		return nil, fmt.Errorf("invalid dictionary static type value type encoding: %w", err)
	}

	return DictionaryStaticType{
		KeyType:   keyType,
		ValueType: valueType,
	}, nil
}

func (d *Decoder) decodeRestrictedStaticType(v interface{}) (StaticType, error) {
	encoded, ok := v.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid restricted static type encoding: %T", v)
	}

	restrictedType, err := d.decodeStaticType(encoded[uint64(0)])
	if err != nil {
		return nil, fmt.Errorf("invalid restricted static type key type encoding: %w", err)
	}

	field2 := encoded[uint64(1)]
	encodedRestrictions, ok := field2.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid restricted static type restrictions encoding: %T", field2)
	}

	restrictions := make([]InterfaceStaticType, len(encodedRestrictions))
	for i, encodedRestriction := range encodedRestrictions {
		r, err := d.decodeStaticType(encodedRestriction)
		if err != nil {
			return nil, err
		}
		restriction, ok := r.(InterfaceStaticType)
		if !ok {
			return nil, fmt.Errorf("invalid restricted static type restriction encoding: %T", r)
		}
		restrictions[i] = restriction
	}

	return RestrictedStaticType{
		Type:         restrictedType,
		Restrictions: restrictions,
	}, nil
}
