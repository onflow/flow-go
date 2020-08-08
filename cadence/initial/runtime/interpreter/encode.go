package interpreter

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	"github.com/fxamacker/cbor/v2"

	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/common"
)

// !!! *WARNING* !!!
//
// Only add new fields to encoded structs by
// appending new fields with the next highest key.
//
// DO *NOT* REPLACE EXISTING FIELDS!

var cborTagSet cbor.TagSet

const cborTagPositiveBignum = 0x2
const cborTagNegativeBignum = 0x3

const cborTagBase = 128

// !!! *WARNING* !!!
//
// Only add new types by:
// - replacing existing placeholders (`_`) with new types
// - appending new types
//
// Only remove types by:
// - replace existing types with a placeholder `_`
//
// DO *NOT* REPLACE EXISTING TYPES!
// DO *NOT* ADD NEW TYPES IN BETWEEN!

const (
	cborTagVoidValue = cborTagBase + iota
	cborTagDictionaryValue
	cborTagSomeValue
	cborTagAddressValue
	cborTagCompositeValue
	_
	_
	_
	_
	_
	_
	_
	_
	_
	_
	_
	_
	_
	_
	_
	_
	_
	_
	_

	// Int*
	cborTagIntValue
	cborTagInt8Value
	cborTagInt16Value
	cborTagInt32Value
	cborTagInt64Value
	cborTagInt128Value
	cborTagInt256Value
	_

	// UInt*
	cborTagUIntValue
	cborTagUInt8Value
	cborTagUInt16Value
	cborTagUInt32Value
	cborTagUInt64Value
	cborTagUInt128Value
	cborTagUInt256Value
	_

	// Word*
	_
	cborTagWord8Value
	cborTagWord16Value
	cborTagWord32Value
	cborTagWord64Value
	_ // future: Word128
	_ // future: Word256
	_

	// Fix*
	_
	_ // future: Fix8
	_ // future: Fix16
	_ // future: Fix32
	cborTagFix64Value
	_ // future: Fix128
	_ // future: Fix256
	_

	// UFix*
	_
	_ // future: UFix8
	_ // future: UFix16
	_ // future: UFix32
	cborTagUFix64Value
	_ // future: UFix128
	_ // future: UFix256
	_

	// Locations
	cborTagAddressLocation
	cborTagStringLocation
	_
	_
	_
	_
	_
	_

	// Storage

	cborTagPathValue
	cborTagCapabilityValue
	cborTagStorageReferenceValue
	cborTagLinkValue
	_
	_
	_
	_
	_
	_
	_
	_

	// Static Types
	cborTagPrimitiveStaticType
	cborTagCompositeStaticType
	cborTagInterfaceStaticType
	cborTagVariableSizedStaticType
	cborTagConstantSizedStaticType
	cborTagDictionaryStaticType
	cborTagOptionalStaticType
	cborTagReferenceStaticType
	cborTagRestrictedStaticType
)

func init() {
	cborTagSet = cbor.NewTagSet()
	tagOptions := cbor.TagOptions{
		EncTag: cbor.EncTagRequired,
		DecTag: cbor.DecTagRequired,
	}

	register := func(tag uint64, encodingType interface{}) {
		err := cborTagSet.Add(
			tagOptions,
			reflect.TypeOf(encodingType),
			tag,
		)
		if err != nil {
			panic(err)
		}
	}

	types := map[uint64]interface{}{
		cborTagDictionaryValue:         encodedDictionaryValue{},
		cborTagCompositeValue:          encodedCompositeValue{},
		cborTagPathValue:               encodedPathValue{},
		cborTagCapabilityValue:         encodedCapabilityValue{},
		cborTagStorageReferenceValue:   encodedStorageReferenceValue{},
		cborTagLinkValue:               encodedLinkValue{},
		cborTagCompositeStaticType:     encodedCompositeStaticType{},
		cborTagInterfaceStaticType:     encodedInterfaceStaticType{},
		cborTagVariableSizedStaticType: encodedVariableSizedStaticType{},
		cborTagConstantSizedStaticType: encodedConstantSizedStaticType{},
		cborTagDictionaryStaticType:    encodedDictionaryStaticType{},
		cborTagRestrictedStaticType:    encodedRestrictedStaticType{},
		cborTagReferenceStaticType:     encodedReferenceStaticType{},
	}

	// Register types
	for tag, encodedType := range types {
		register(tag, encodedType)
	}
}

type EncodingDeferralMove struct {
	DeferredOwner      common.Address
	DeferredStorageKey string
	NewOwner           common.Address
	NewStorageKey      string
}

type EncodingDeferrals struct {
	Values map[string]Value
	Moves  []EncodingDeferralMove
}

// Encoder converts Values into CBOR-encoded bytes.
//
type Encoder struct {
	enc      *cbor.Encoder
	deferred bool
}

// EncodeValue returns the CBOR-encoded representation of the given value.
//
// The given path is used to identify values in the object graph.
// For example, path elements are appended for array elements (the index),
// dictionary values (the key), and composites (the field name).
//
// The deferred flag determines if child values should be deferred,
// i.e. should not be encoded into the result,
// but e.g. be eventually written to separate storage keys.
// If true, the deferrals result will contain the values
// which have not been encoded, and which values need to be moved
// from a previous storage key to another storage key.
//
func EncodeValue(value Value, path []string, deferred bool) (
	encoded []byte,
	deferrals *EncodingDeferrals,
	err error,
) {
	var w bytes.Buffer
	enc, err := NewEncoder(&w, deferred)
	if err != nil {
		return nil, nil, err
	}

	deferrals = &EncodingDeferrals{
		Values: map[string]Value{},
	}

	err = enc.Encode(value, path, deferrals)
	if err != nil {
		return nil, nil, err
	}

	return w.Bytes(), deferrals, nil
}

// NewEncoder initializes an Encoder that will write CBOR-encoded bytes
// to the given io.Writer.
//
func NewEncoder(w io.Writer, deferred bool) (*Encoder, error) {
	encMode, err := cbor.CanonicalEncOptions().EncModeWithTags(cborTagSet)
	if err != nil {
		return nil, err
	}
	enc := encMode.NewEncoder(w)
	return &Encoder{
		enc:      enc,
		deferred: deferred,
	}, nil
}

// Encode writes the CBOR-encoded representation of the given value to this
// encoder's io.Writer.
//
// This function returns an error if the given value's type is not supported
// by this encoder.
//
func (e *Encoder) Encode(
	v Value,
	path []string,
	deferrals *EncodingDeferrals,
) error {
	prepared, err := e.prepare(v, path, deferrals)
	if err != nil {
		return err
	}

	return e.enc.Encode(prepared)
}

// prepare traverses the object graph of the provided value and returns
// the representation for the value that can be marshalled to CBOR.
//
func (e *Encoder) prepare(
	v Value,
	path []string,
	deferrals *EncodingDeferrals,
) (
	interface{},
	error,
) {
	switch v := v.(type) {

	case NilValue:
		return e.prepareNil(), nil

	case VoidValue:
		return e.prepareVoid(), nil

	case BoolValue:
		return e.prepareBool(v), nil

	case AddressValue:
		return e.prepareAddressValue(v), nil

	// Int*

	case IntValue:
		return e.prepareInt(v), nil

	case Int8Value:
		return e.prepareInt8(v), nil

	case Int16Value:
		return e.prepareInt16(v), nil

	case Int32Value:
		return e.prepareInt32(v), nil

	case Int64Value:
		return e.prepareInt64(v), nil

	case Int128Value:
		return e.prepareInt128(v), nil

	case Int256Value:
		return e.prepareInt256(v), nil

	// UInt*

	case UIntValue:
		return e.prepareUInt(v), nil

	case UInt8Value:
		return e.prepareUInt8(v), nil

	case UInt16Value:
		return e.prepareUInt16(v), nil

	case UInt32Value:
		return e.prepareUInt32(v), nil

	case UInt64Value:
		return e.prepareUInt64(v), nil

	case UInt128Value:
		return e.prepareUInt128(v), nil

	case UInt256Value:
		return e.prepareUInt256(v), nil

	// Word*

	case Word8Value:
		return e.prepareWord8(v), nil

	case Word16Value:
		return e.prepareWord16(v), nil

	case Word32Value:
		return e.prepareWord32(v), nil

	case Word64Value:
		return e.prepareWord64(v), nil

	// Fix*

	case Fix64Value:
		return e.prepareFix64(v), nil

	// UFix*

	case UFix64Value:
		return e.prepareUFix64(v), nil

	// String

	case *StringValue:
		return e.prepareString(v), nil

	// Collections

	case *ArrayValue:
		return e.prepareArray(v, path, deferrals)

	case *DictionaryValue:
		return e.prepareDictionaryValue(v, path, deferrals)

	// Composites

	case *CompositeValue:
		return e.prepareCompositeValue(v, path, deferrals)

	// Some

	case *SomeValue:
		return e.prepareSomeValue(v, path, deferrals)

	// Storage

	case *StorageReferenceValue:
		return e.prepareStorageReferenceValue(v), nil

	case PathValue:
		return e.preparePathValue(v), nil

	case CapabilityValue:
		return e.prepareCapabilityValue(v), nil

	case LinkValue:
		return e.prepareLinkValue(v)

	default:
		return nil, fmt.Errorf("unsupported value: %[1]T, %[1]v", v)
	}
}

func (e *Encoder) prepareNil() interface{} {
	return nil
}

func (e *Encoder) prepareVoid() cbor.Tag {

	// TODO: optimize: use 0xf7, but decoded by github.com/fxamacker/cbor/v2 as Go `nil`:
	//   https://github.com/fxamacker/cbor/blob/a6ed6ff68e99cbb076997a08d19f03c453851555/README.md#limitations

	return cbor.Tag{
		Number: cborTagVoidValue,
	}
}

func (e *Encoder) prepareBool(v BoolValue) bool {
	return bool(v)
}

func (e *Encoder) prepareBig(bigInt *big.Int) cbor.Tag {
	b := bigInt.Bytes()
	// positive bignum
	var tag uint64 = cborTagPositiveBignum
	if bigInt.Sign() < 0 {
		// negative bignum
		tag = cborTagNegativeBignum
	}
	return cbor.Tag{Number: tag, Content: b}
}

func (e *Encoder) prepareInt(v IntValue) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagIntValue,
		Content: e.prepareBig(v.BigInt),
	}
}

func (e *Encoder) prepareInt8(v Int8Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagInt8Value,
		Content: v,
	}
}

func (e *Encoder) prepareInt16(v Int16Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagInt16Value,
		Content: v,
	}
}

func (e *Encoder) prepareInt32(v Int32Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagInt32Value,
		Content: v,
	}
}

func (e *Encoder) prepareInt64(v Int64Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagInt64Value,
		Content: v,
	}
}
func (e *Encoder) prepareInt128(v Int128Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagInt128Value,
		Content: e.prepareBig(v.BigInt),
	}
}

func (e *Encoder) prepareInt256(v Int256Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagInt256Value,
		Content: e.prepareBig(v.BigInt),
	}
}

func (e *Encoder) prepareUInt(v UIntValue) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagUIntValue,
		Content: e.prepareBig(v.BigInt),
	}
}

func (e *Encoder) prepareUInt8(v UInt8Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagUInt8Value,
		Content: v,
	}
}

func (e *Encoder) prepareUInt16(v UInt16Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagUInt16Value,
		Content: v,
	}
}

func (e *Encoder) prepareUInt32(v UInt32Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagUInt32Value,
		Content: v,
	}
}

func (e *Encoder) prepareUInt64(v UInt64Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagUInt64Value,
		Content: v,
	}
}

func (e *Encoder) prepareUInt128(v UInt128Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagUInt128Value,
		Content: e.prepareBig(v.BigInt),
	}
}

func (e *Encoder) prepareUInt256(v UInt256Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagUInt256Value,
		Content: e.prepareBig(v.BigInt),
	}
}

func (e *Encoder) prepareWord8(v Word8Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagWord8Value,
		Content: v,
	}
}

func (e *Encoder) prepareWord16(v Word16Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagWord16Value,
		Content: v,
	}
}

func (e *Encoder) prepareWord32(v Word32Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagWord32Value,
		Content: v,
	}
}

func (e *Encoder) prepareWord64(v Word64Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagWord64Value,
		Content: v,
	}
}

func (e *Encoder) prepareFix64(v Fix64Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagFix64Value,
		Content: v,
	}
}

func (e *Encoder) prepareUFix64(v UFix64Value) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagUFix64Value,
		Content: v,
	}
}

func (e *Encoder) prepareString(v *StringValue) string {
	return v.Str
}

// joinPath returns the path for a nested item, for example the index of an array,
// the key of a dictionary, or the field name of a composite.
//
// \x1F = Information Separator One
//
func joinPath(elements []string) string {
	return strings.Join(elements, "\x1F")
}

func (e *Encoder) prepareArray(
	v *ArrayValue,
	path []string,
	deferrals *EncodingDeferrals,
) (
	[]interface{},
	error,
) {
	result := make([]interface{}, len(v.Values))

	for i, value := range v.Values {
		valuePath := append(path[:], strconv.Itoa(i))
		prepared, err := e.prepare(value, valuePath, deferrals)
		if err != nil {
			return nil, err
		}
		result[i] = prepared
	}

	return result, nil
}

type encodedDictionaryValue struct {
	Keys    []interface{}          `cbor:"0,keyasint"`
	Entries map[string]interface{} `cbor:"1,keyasint"`
}

const dictionaryKeyPathPrefix = "k"
const dictionaryValuePathPrefix = "v"

// TODO: optimize: use CBOR map, but unclear how to preserve ordering
func (e *Encoder) prepareDictionaryValue(
	v *DictionaryValue,
	path []string,
	deferrals *EncodingDeferrals,
) (
	interface{},
	error,
) {
	keysPath := append(path[:], dictionaryKeyPathPrefix)

	keys, err := e.prepareArray(v.Keys, keysPath, deferrals)
	if err != nil {
		return nil, err
	}

	entries := make(map[string]interface{}, len(v.Entries))

	// Deferring the encoding of values is only supported if all
	// values are resources: resource typed dictionaries are moved

	deferred := e.deferred
	if deferred {
		for _, value := range v.Entries {
			compositeValue, ok := value.(*CompositeValue)
			if !ok || compositeValue.Kind != common.CompositeKindResource {
				deferred = false
				break
			}
		}
	}

	for _, keyValue := range v.Keys.Values {
		key := dictionaryKey(keyValue)
		entryValue := v.Entries[key]
		valuePath := append(path[:], dictionaryValuePathPrefix, key)

		if deferred {

			deferredStorageKey, isDeferred := v.DeferredKeys[key]

			// If the value is not deferred, i.e. it is in memory,
			// then it must be stored under a separate storage key
			// in the owner's storage.

			if !isDeferred {
				deferrals.Values[joinPath(valuePath)] = entryValue
			} else {

				// If the value is deferred, and the deferred value
				// is stored in another account's storage,
				// it must be moved.

				deferredOwner := *v.DeferredOwner
				owner := *v.Owner

				if deferredOwner != owner {

					deferrals.Moves = append(deferrals.Moves,
						EncodingDeferralMove{
							DeferredOwner:      deferredOwner,
							DeferredStorageKey: deferredStorageKey,
							NewOwner:           owner,
							NewStorageKey:      joinPath(valuePath),
						},
					)
				}
			}
		} else {
			var prepared interface{}
			prepared, err = e.prepare(entryValue, valuePath, deferrals)
			if err != nil {
				return nil, err
			}
			entries[key] = prepared
		}
	}

	return encodedDictionaryValue{
		Keys:    keys,
		Entries: entries,
	}, nil
}

type encodedCompositeValue struct {
	Location interface{}            `cbor:"0,keyasint"`
	TypeID   string                 `cbor:"1,keyasint"`
	Kind     uint                   `cbor:"2,keyasint"`
	Fields   map[string]interface{} `cbor:"3,keyasint"`
}

func (e *Encoder) prepareCompositeValue(
	v *CompositeValue,
	path []string,
	deferrals *EncodingDeferrals,
) (
	interface{},
	error,
) {

	fields := make(map[string]interface{}, len(v.Fields))

	for name, value := range v.Fields {
		valuePath := append(path[:], name)

		prepared, err := e.prepare(value, valuePath, deferrals)
		if err != nil {
			return nil, err
		}
		fields[name] = prepared
	}

	location, err := e.prepareLocation(v.Location)
	if err != nil {
		return nil, err
	}

	return encodedCompositeValue{
		Location: location,
		TypeID:   string(v.TypeID),
		Kind:     uint(v.Kind),
		Fields:   fields,
	}, nil
}

func (e *Encoder) prepareSomeValue(
	v *SomeValue,
	path []string,
	deferrals *EncodingDeferrals,
) (
	interface{},
	error,
) {
	prepared, err := e.prepare(v.Value, path, deferrals)
	if err != nil {
		return nil, err
	}

	return cbor.Tag{
		Number:  cborTagSomeValue,
		Content: prepared,
	}, nil
}

type encodedStorageReferenceValue struct {
	Authorized           bool   `cbor:"0,keyasint"`
	TargetStorageAddress []byte `cbor:"1,keyasint"`
	TargetKey            string `cbor:"2,keyasint"`
}

func (e *Encoder) prepareStorageReferenceValue(v *StorageReferenceValue) interface{} {
	return encodedStorageReferenceValue{
		Authorized:           v.Authorized,
		TargetStorageAddress: v.TargetStorageAddress.Bytes(),
		TargetKey:            v.TargetKey,
	}
}

func (e *Encoder) prepareAddressValue(v AddressValue) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagAddressValue,
		Content: v.ToAddress().Bytes(),
	}
}

type encodedPathValue struct {
	Domain     uint   `cbor:"0,keyasint"`
	Identifier string `cbor:"1,keyasint"`
}

func (e *Encoder) preparePathValue(v PathValue) encodedPathValue {
	return encodedPathValue{
		Domain:     uint(v.Domain),
		Identifier: v.Identifier,
	}
}

type encodedCapabilityValue struct {
	Address cbor.Tag         `cbor:"0,keyasint"`
	Path    encodedPathValue `cbor:"1,keyasint"`
}

func (e *Encoder) prepareCapabilityValue(v CapabilityValue) interface{} {
	return encodedCapabilityValue{
		Address: e.prepareAddressValue(v.Address),
		Path:    e.preparePathValue(v.Path),
	}
}

func (e *Encoder) prepareLocation(l ast.Location) (interface{}, error) {
	switch l := l.(type) {
	case ast.AddressLocation:
		return cbor.Tag{
			Number:  cborTagAddressLocation,
			Content: l.ToAddress().Bytes(),
		}, nil

	case ast.StringLocation:
		return cbor.Tag{
			Number:  cborTagStringLocation,
			Content: string(l),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported location: %T", l)
	}
}

type encodedLinkValue struct {
	TargetPath encodedPathValue `cbor:"0,keyasint"`
	Type       interface{}      `cbor:"1,keyasint"`
}

func (e *Encoder) prepareLinkValue(v LinkValue) (interface{}, error) {
	staticType, err := e.prepareStaticType(v.Type)
	if err != nil {
		return nil, err
	}
	return encodedLinkValue{
		TargetPath: e.preparePathValue(v.TargetPath),
		Type:       staticType,
	}, nil
}

// TODO: optimize, decode location from type ID
type encodedCompositeStaticType struct {
	Location interface{} `cbor:"0,keyasint"`
	TypeID   string      `cbor:"1,keyasint"`
}

// TODO: optimize, decode location from type ID
type encodedInterfaceStaticType struct {
	Location interface{} `cbor:"0,keyasint"`
	TypeID   string      `cbor:"1,keyasint"`
}

type encodedVariableSizedStaticType struct {
	Type interface{} `cbor:"0,keyasint"`
}

type encodedConstantSizedStaticType struct {
	Size int64       `cbor:"0,keyasint"`
	Type interface{} `cbor:"1,keyasint"`
}

type encodedDictionaryStaticType struct {
	KeyType   interface{} `cbor:"0,keyasint"`
	ValueType interface{} `cbor:"1,keyasint"`
}

type encodedRestrictedStaticType struct {
	Type         interface{}   `cbor:"0,keyasint"`
	Restrictions []interface{} `cbor:"1,keyasint"`
}

type encodedReferenceStaticType struct {
	Authorized bool        `cbor:"0,keyasint"`
	Type       interface{} `cbor:"1,keyasint"`
}

func (e *Encoder) prepareStaticType(t StaticType) (interface{}, error) {
	switch v := t.(type) {
	case PrimitiveStaticType:
		return e.preparePrimitiveStaticType(v), nil

	case OptionalStaticType:
		return e.prepareOptionalStaticType(v)

	case CompositeStaticType:
		return e.prepareCompositeStaticType(v)

	case InterfaceStaticType:
		return e.prepareInterfaceStaticType(v)

	case VariableSizedStaticType:
		return e.prepareVariableSizedStaticType(v)

	case ConstantSizedStaticType:
		return e.prepareConstantSizedStaticType(v)

	case ReferenceStaticType:
		return e.prepareReferenceStaticType(v)

	case DictionaryStaticType:
		return e.prepareDictionaryStaticType(v)

	case RestrictedStaticType:
		return e.prepareRestrictedStaticType(v)

	default:
		return nil, fmt.Errorf("unsupported static type: %T", t)
	}
}

func (e *Encoder) preparePrimitiveStaticType(v PrimitiveStaticType) cbor.Tag {
	return cbor.Tag{
		Number:  cborTagPrimitiveStaticType,
		Content: uint(v),
	}
}

func (e *Encoder) prepareOptionalStaticType(v OptionalStaticType) (interface{}, error) {
	staticType, err := e.prepareStaticType(v.Type)
	if err != nil {
		return nil, err
	}

	return cbor.Tag{
		Number:  cborTagOptionalStaticType,
		Content: staticType,
	}, nil
}

func (e *Encoder) prepareCompositeStaticType(v CompositeStaticType) (interface{}, error) {
	location, err := e.prepareLocation(v.Location)
	if err != nil {
		return nil, err
	}

	return encodedCompositeStaticType{
		Location: location,
		TypeID:   string(v.TypeID),
	}, nil
}

func (e *Encoder) prepareInterfaceStaticType(v InterfaceStaticType) (interface{}, error) {
	location, err := e.prepareLocation(v.Location)
	if err != nil {
		return nil, err
	}

	return encodedInterfaceStaticType{
		Location: location,
		TypeID:   string(v.TypeID),
	}, nil
}

func (e *Encoder) prepareVariableSizedStaticType(v VariableSizedStaticType) (interface{}, error) {
	staticType, err := e.prepareStaticType(v.Type)
	if err != nil {
		return nil, err
	}

	return cbor.Tag{
		Number:  cborTagVariableSizedStaticType,
		Content: staticType,
	}, nil
}

func (e *Encoder) prepareConstantSizedStaticType(v ConstantSizedStaticType) (interface{}, error) {
	staticType, err := e.prepareStaticType(v.Type)
	if err != nil {
		return nil, err
	}

	return encodedConstantSizedStaticType{
		Type: staticType,
		Size: v.Size,
	}, nil
}

func (e *Encoder) prepareReferenceStaticType(v ReferenceStaticType) (interface{}, error) {
	staticType, err := e.prepareStaticType(v.Type)
	if err != nil {
		return nil, err
	}

	return encodedReferenceStaticType{
		Authorized: v.Authorized,
		Type:       staticType,
	}, nil
}

func (e *Encoder) prepareDictionaryStaticType(v DictionaryStaticType) (interface{}, error) {
	keyType, err := e.prepareStaticType(v.KeyType)
	if err != nil {
		return nil, err
	}

	valueType, err := e.prepareStaticType(v.ValueType)
	if err != nil {
		return nil, err
	}

	return encodedDictionaryStaticType{
		KeyType:   keyType,
		ValueType: valueType,
	}, nil
}

func (e *Encoder) prepareRestrictedStaticType(v RestrictedStaticType) (interface{}, error) {
	restrictedType, err := e.prepareStaticType(v.Type)
	if err != nil {
		return nil, err
	}

	encodedRestrictions := make([]interface{}, len(v.Restrictions))
	for i, restriction := range v.Restrictions {
		encodedRestriction, err := e.prepareStaticType(restriction)
		if err != nil {
			return nil, err
		}

		encodedRestrictions[i] = encodedRestriction
	}

	return encodedRestrictedStaticType{
		Type:         restrictedType,
		Restrictions: encodedRestrictions,
	}, nil
}
