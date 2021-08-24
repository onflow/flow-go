package migrations

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path"
	"strings"
	"time"

	"encoding/hex"

	"github.com/fxamacker/cbor/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/atree"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"

	"github.com/onflow/cadence/runtime/common"
	newInter "github.com/onflow/cadence/runtime/interpreter"
	oldInter "github.com/onflow/cadence/v18/runtime/interpreter"
)

const cborTagStorageReference = 202

var storageReferenceEncodingStart = []byte{0xd8, cborTagStorageReference}

var storageMigrationV5DecMode = func() cbor.DecMode {
	decMode, err := cbor.DecOptions{
		IntDec:           cbor.IntDecConvertNone,
		MaxArrayElements: math.MaxInt32,
		MaxMapPairs:      math.MaxInt32,
		MaxNestedLevels:  256,
	}.DecMode()
	if err != nil {
		panic(err)
	}
	return decMode
}()

var CBOREncMode = func() cbor.EncMode {
	options := cbor.CanonicalEncOptions()
	options.BigIntConvert = cbor.BigIntConvertNone
	encMode, err := options.EncMode()
	if err != nil {
		panic(err)
	}
	return encMode
}()

type storageFormatV6MigrationResult struct {
	key     ledger.Key
	payload *ledger.Payload
	err     error
}

type brokenTypeCause int

type StorageFormatV6Migration struct {
	Log           zerolog.Logger
	OutputDir     string
	accounts      *state.Accounts
	programs      *programs.Programs
	brokenTypeIDs map[common.TypeID]brokenTypeCause
	reportFile    *os.File
	storage       newInter.Storage
}

func (m StorageFormatV6Migration) filename() string {
	return path.Join(m.OutputDir, fmt.Sprintf("migration_report_%d.csv", int32(time.Now().Unix())))
}

func (m *StorageFormatV6Migration) Migrate(payloads []ledger.Payload) ([]ledger.Payload, error) {

	filename := m.filename()
	m.Log.Info().Msgf("Running storage format V5 migration. Saving report to %s.", filename)

	reportFile, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = reportFile.Close()
		if err != nil {
			panic(err)
		}
	}()

	m.reportFile = reportFile

	m.Log.Info().Msg("Loading account contracts ...")

	m.accounts = m.getContractsOnlyAccounts(payloads)

	m.Log.Info().Msg("Loaded account contracts")

	m.programs = programs.NewEmptyPrograms()
	m.brokenTypeIDs = make(map[common.TypeID]brokenTypeCause, 0)

	m.storage = newInter.NewInMemoryStorage()

	migratedPayloads := make([]ledger.Payload, 0, len(payloads))

	for _, payload := range payloads {

		keyParts := payload.Key.KeyParts

		rawOwner := keyParts[0].Value
		rawKey := keyParts[2].Value

		result := m.migrate(payload)

		if result.err != nil {

			return nil, fmt.Errorf(
				"failed to migrate key: %q (owner: %x): %w",
				rawKey,
				rawOwner,
				result.err,
			)
		} else if result.payload != nil {
			migratedPayloads = append(migratedPayloads, *result.payload)
		} else {
			m.Log.Warn().Msgf("DELETED key %q (owner: %x)", rawKey, rawOwner)
			m.reportFile.WriteString(fmt.Sprintf("%x,%s,DELETED\n", rawOwner, string(rawKey)))
		}
	}

	return migratedPayloads, nil
}

func (m StorageFormatV6Migration) getContractsOnlyAccounts(payloads []ledger.Payload) *state.Accounts {
	var filteredPayloads []ledger.Payload

	for _, payload := range payloads {
		rawKey := string(payload.Key.KeyParts[2].Value)
		if strings.HasPrefix(rawKey, "contract_names") ||
			strings.HasPrefix(rawKey, "code.") ||
			rawKey == "exists" {

			filteredPayloads = append(filteredPayloads, payload)
		}
	}

	l := newView(filteredPayloads)
	st := state.NewState(l)
	sth := state.NewStateHolder(st)
	accounts := state.NewAccounts(sth)
	return accounts
}

func (m StorageFormatV6Migration) migrate(payload ledger.Payload) storageFormatV6MigrationResult {
	// TODO: skip inner values, if they are already encoded

	migratedPayload, err := m.reencodePayload(payload)

	result := storageFormatV6MigrationResult{
		key: payload.Key,
	}

	if err != nil {
		result.err = err
	} else if migratedPayload != nil {
		if err := m.checkStorageFormat(*migratedPayload); err != nil {
			panic(fmt.Errorf("%w: key = %s", err, payload.Key.String()))
		}
		result.payload = migratedPayload
	}

	return result
}

func (m StorageFormatV6Migration) checkStorageFormat(payload ledger.Payload) error {

	if !bytes.HasPrefix(payload.Value, []byte{0x0, 0xca, 0xde}) {
		return nil
	}

	_, version := newInter.StripMagic(payload.Value)
	if version != newInter.CurrentEncodingVersion {
		return fmt.Errorf("invalid version for key %s: %d", payload.Key.String(), version)
	}

	return nil
}

func (m StorageFormatV6Migration) reencodePayload(payload ledger.Payload) (*ledger.Payload, error) {

	keyParts := payload.Key.KeyParts

	rawOwner := keyParts[0].Value
	rawController := keyParts[1].Value
	rawKey := keyParts[2].Value

	// Ignore known payload keys that are not Cadence values

	if state.IsFVMStateKey(
		string(rawOwner),
		string(rawController),
		string(rawKey),
	) {
		return &payload, nil
	}

	value, version := oldInter.StripMagic(payload.Value)

	if version != oldInter.CurrentEncodingVersion {
		return nil,
			fmt.Errorf(
				"invalid storage format version for key: %s: %d",
				rawKey,
				version,
			)
	}

	err := storageMigrationV5DecMode.Valid(value)
	if err != nil {
		return &payload, nil
	}

	// Delete known dead or orphaned contract value child keys

	//if m.isOrphanContactValueChildKey(
	//	rawKey,
	//	flow.BytesToAddress(rawOwner),
	//) {
	//	m.Log.Warn().Msgf(
	//		"DELETING orphaned contract value child key: %s (owner: %x)",
	//		string(rawKey), rawOwner,
	//	)
	//
	//	return nil, nil
	//}

	// Extract the owner from the key

	newValue, keep, err := m.reencodeValue(
		value,
		common.BytesToAddress(rawOwner),
		string(rawKey),
		version,
	)

	if err != nil {
		return nil,
			fmt.Errorf(
				"failed to re-encode key: %s: %w\n\nvalue:\n%s\n\n%s",
				rawKey, err,
				hex.Dump(value),
				cborMeLink(value),
			)
	}

	if !keep {
		return nil, nil
	}

	payload.Value = newInter.PrependMagic(
		newValue,
		newInter.CurrentEncodingVersion,
	)

	return &payload, nil
}

// Re-encodes the value to the new storage format, using following steps:
//   - Decode to old value
//   - Covert value from old to new
//   - Encode the new value.
func (m StorageFormatV6Migration) reencodeValue(
	data []byte,
	owner common.Address,
	key string,
	version uint16,
) (
	newData []byte,
	keep bool,
	err error,
) {

	// Decode the value

	storagePath := []string{key}

	rootValue, err := oldInter.DecodeValue(data, &owner, storagePath, version, nil)
	if err != nil {
		if tagErr, ok := err.(oldInter.UnsupportedTagDecodingError); ok &&
			tagErr.Tag == cborTagStorageReference &&
			bytes.Compare(data[:2], storageReferenceEncodingStart) == 0 {

			m.Log.Warn().
				Str("key", key).
				Str("owner", owner.String()).
				Msgf("DELETING unsupported storage reference")

			return nil, false, nil

		} else {
			return nil, false, fmt.Errorf(
				"failed to decode value: %w\n\nvalue:\n%s\n",
				err, hex.Dump(data),
			)
		}
	}

	// Force decoding of all inner values

	oldInter.InspectValue(
		rootValue,
		func(inspectedValue oldInter.Value) bool {
			switch inspectedValue := inspectedValue.(type) {
			case *oldInter.CompositeValue:
				_ = inspectedValue.Fields()
			case *oldInter.ArrayValue:
				_ = inspectedValue.Elements()
			case *oldInter.DictionaryValue:
				_ = inspectedValue.Entries()
			}
			return true
		},
	)

	// Convert old value to new value

	converter := NewValueConverter(m.storage)
	newValue := converter.Convert(rootValue)

	// Encode the new value

	storable, ok := interface{}(newValue).(atree.Storable)
	if !ok {
		// TODO: encode
		// Return the original value as is for now.
		return data, true, nil

		//return nil, false,
		//	fmt.Errorf("non-storable value at key: %s\nvalue type: %s\n",
		//		key,
		//		reflect.TypeOf(newValue),
		//	)
	}

	var buf bytes.Buffer
	enc := atree.NewEncoder(&buf, CBOREncMode)

	err = storable.Encode(enc)
	if err != nil {
		return nil, false, err
	}

	err = enc.CBOR.Flush()
	if err != nil {
		return nil, false, err
	}

	return buf.Bytes(), true, nil
}

func cborMeLink(value []byte) string {
	return fmt.Sprintf("http://cbor.me/?bytes=%x", value)
}

// Value Converter
//
type ValueConverter struct {
	result  newInter.Value
	storage newInter.Storage
}

var _ oldInter.Visitor = &ValueConverter{}

func NewValueConverter(storage newInter.Storage) *ValueConverter {
	return &ValueConverter{
		storage: storage,
	}
}

func (c *ValueConverter) Convert(value oldInter.Value) newInter.Value {
	prevResult := c.result
	c.result = nil

	defer func() {
		c.result = prevResult
	}()

	// Interpreter is never used. So safe to pass nil here.
	value.Accept(nil, c)

	return c.result
}

func (c *ValueConverter) VisitValue(_ *oldInter.Interpreter, _ oldInter.Value) {
	panic("implement me")
}

func (c *ValueConverter) VisitTypeValue(_ *oldInter.Interpreter, value oldInter.TypeValue) {
	c.result = newInter.TypeValue{
		Type: ConvertStaticType(value.Type),
	}
}

func (c *ValueConverter) VisitVoidValue(_ *oldInter.Interpreter, _ oldInter.VoidValue) {
	c.result = newInter.VoidValue{}
}

func (c *ValueConverter) VisitBoolValue(_ *oldInter.Interpreter, value oldInter.BoolValue) {
	c.result = newInter.BoolValue(value)
}

func (c *ValueConverter) VisitStringValue(_ *oldInter.Interpreter, value *oldInter.StringValue) {
	c.result = newInter.NewStringValue(value.Str)
}

func (c *ValueConverter) VisitArrayValue(_ *oldInter.Interpreter, value *oldInter.ArrayValue) bool {
	newElements := make([]newInter.Value, value.Count())

	for index, element := range value.Elements() {
		newElements[index] = c.Convert(element)
	}

	arrayStaticType := ConvertStaticType(value.StaticType()).(newInter.ArrayStaticType)

	c.result = newInter.NewArrayValue(arrayStaticType, c.storage, newElements...)

	// Do not descent. We already visited children here.
	return false
}

func (c *ValueConverter) VisitIntValue(_ *oldInter.Interpreter, value oldInter.IntValue) {
	c.result = newInter.NewIntValueFromBigInt(value.BigInt)
}

func (c *ValueConverter) VisitInt8Value(_ *oldInter.Interpreter, value oldInter.Int8Value) {
	c.result = newInter.Int8Value(value)
}

func (c *ValueConverter) VisitInt16Value(_ *oldInter.Interpreter, value oldInter.Int16Value) {
	c.result = newInter.Int16Value(value)
}

func (c *ValueConverter) VisitInt32Value(_ *oldInter.Interpreter, value oldInter.Int32Value) {
	c.result = newInter.Int32Value(value)
}

func (c *ValueConverter) VisitInt64Value(_ *oldInter.Interpreter, value oldInter.Int64Value) {
	c.result = newInter.Int64Value(value)
}

func (c *ValueConverter) VisitInt128Value(_ *oldInter.Interpreter, value oldInter.Int128Value) {
	c.result = newInter.NewInt128ValueFromBigInt(value.BigInt)
}

func (c *ValueConverter) VisitInt256Value(_ *oldInter.Interpreter, value oldInter.Int256Value) {
	c.result = newInter.NewInt256ValueFromBigInt(value.BigInt)
}

func (c *ValueConverter) VisitUIntValue(_ *oldInter.Interpreter, value oldInter.UIntValue) {
	c.result = newInter.NewUIntValueFromBigInt(value.BigInt)
}

func (c *ValueConverter) VisitUInt8Value(_ *oldInter.Interpreter, value oldInter.UInt8Value) {
	c.result = newInter.UInt8Value(value)
}

func (c *ValueConverter) VisitUInt16Value(_ *oldInter.Interpreter, value oldInter.UInt16Value) {
	c.result = newInter.UInt16Value(value)
}

func (c *ValueConverter) VisitUInt32Value(_ *oldInter.Interpreter, value oldInter.UInt32Value) {
	c.result = newInter.UInt32Value(value)
}

func (c *ValueConverter) VisitUInt64Value(_ *oldInter.Interpreter, value oldInter.UInt64Value) {
	c.result = newInter.UInt64Value(value)
}

func (c *ValueConverter) VisitUInt128Value(_ *oldInter.Interpreter, value oldInter.UInt128Value) {
	c.result = newInter.NewUInt128ValueFromBigInt(value.BigInt)
}

func (c *ValueConverter) VisitUInt256Value(_ *oldInter.Interpreter, value oldInter.UInt256Value) {
	c.result = newInter.NewUInt256ValueFromBigInt(value.BigInt)
}

func (c *ValueConverter) VisitWord8Value(_ *oldInter.Interpreter, value oldInter.Word8Value) {
	c.result = newInter.Word8Value(value)
}

func (c *ValueConverter) VisitWord16Value(_ *oldInter.Interpreter, value oldInter.Word16Value) {
	c.result = newInter.Word16Value(value)
}

func (c *ValueConverter) VisitWord32Value(_ *oldInter.Interpreter, value oldInter.Word32Value) {
	c.result = newInter.Word32Value(value)
}

func (c *ValueConverter) VisitWord64Value(_ *oldInter.Interpreter, value oldInter.Word64Value) {
	c.result = newInter.Word64Value(value)
}

func (c *ValueConverter) VisitFix64Value(_ *oldInter.Interpreter, value oldInter.Fix64Value) {
	c.result = newInter.NewFix64ValueWithInteger(int64(value.ToInt()))
}

func (c *ValueConverter) VisitUFix64Value(_ *oldInter.Interpreter, value oldInter.UFix64Value) {
	c.result = newInter.NewUFix64ValueWithInteger(uint64(value.ToInt()))
}

func (c *ValueConverter) VisitCompositeValue(_ *oldInter.Interpreter, value *oldInter.CompositeValue) bool {
	fields := newInter.NewStringValueOrderedMap()

	value.Fields().Foreach(func(key string, fieldVal oldInter.Value) {
		fields.Set(key, c.Convert(fieldVal))
	})

	c.result = &newInter.CompositeValue{
		// TODO: Convert location and kind to new package?
		Location:            value.Location(),
		QualifiedIdentifier: value.QualifiedIdentifier(),
		Kind:                value.Kind(),
		Fields:              fields,

		// TODO
		StorageID: atree.StorageID{},
	}

	// Do not descent
	return false
}

func (c *ValueConverter) VisitDictionaryValue(_ *oldInter.Interpreter, value *oldInter.DictionaryValue) bool {
	staticType := ConvertStaticType(value.StaticType()).(newInter.DictionaryStaticType)

	keysAndValues := make([]newInter.Value, value.Count()*2)

	for index, key := range value.Keys().Elements() {
		keysAndValues[index*2] = c.Convert(key)
	}

	index := 0
	value.Entries().Foreach(func(_ string, value oldInter.Value) {
		keysAndValues[index*2+1] = c.Convert(value)
		index++
	})

	// TODO: pass address as a parameter?
	c.result = newInter.NewDictionaryValue(
		staticType,
		c.storage,
		keysAndValues...,
	)

	// Do not descent
	return false
}

func (c *ValueConverter) VisitNilValue(_ *oldInter.Interpreter, _ oldInter.NilValue) {
	c.result = newInter.NilValue{}
}

func (c *ValueConverter) VisitSomeValue(_ *oldInter.Interpreter, value *oldInter.SomeValue) bool {
	innerValue := c.Convert(value.Value)
	c.result = newInter.NewSomeValueNonCopying(innerValue)

	// Do not descent
	return false
}

func (c *ValueConverter) VisitStorageReferenceValue(_ *oldInter.Interpreter, _ *oldInter.StorageReferenceValue) {
	panic("value not storable")
}

func (c *ValueConverter) VisitEphemeralReferenceValue(_ *oldInter.Interpreter, _ *oldInter.EphemeralReferenceValue) {
	panic("value not storable")
}

func (c *ValueConverter) VisitAddressValue(_ *oldInter.Interpreter, value oldInter.AddressValue) {
	c.result = newInter.AddressValue(value)
}

func (c *ValueConverter) VisitPathValue(_ *oldInter.Interpreter, value oldInter.PathValue) {
	c.result = newInter.PathValue{
		Domain:     value.Domain,
		Identifier: value.Identifier,
	}
}

func (c *ValueConverter) VisitCapabilityValue(_ *oldInter.Interpreter, value oldInter.CapabilityValue) {
	address := c.Convert(value).(newInter.AddressValue)
	pathValue := c.Convert(value).(newInter.PathValue)
	burrowType := ConvertStaticType(value.BorrowType)

	c.result = &newInter.CapabilityValue{
		Address:    address,
		Path:       pathValue,
		BorrowType: burrowType,
	}
}

func (c *ValueConverter) VisitLinkValue(_ *oldInter.Interpreter, value oldInter.LinkValue) {
	targetPath := c.Convert(value.TargetPath).(newInter.PathValue)
	c.result = newInter.LinkValue{
		TargetPath: targetPath,
		Type:       ConvertStaticType(value.Type),
	}
}

func (c *ValueConverter) VisitInterpretedFunctionValue(_ *oldInter.Interpreter, _ oldInter.InterpretedFunctionValue) {
	panic("value not storable")
}

func (c *ValueConverter) VisitHostFunctionValue(_ *oldInter.Interpreter, _ oldInter.HostFunctionValue) {
	panic("value not storable")
}

func (c *ValueConverter) VisitBoundFunctionValue(_ *oldInter.Interpreter, _ oldInter.BoundFunctionValue) {
	panic("value not storable")
}

func (c *ValueConverter) VisitDeployedContractValue(_ *oldInter.Interpreter, _ oldInter.DeployedContractValue) {
	panic("value not storable")
}

// Type conversions

func ConvertStaticType(staticType oldInter.StaticType) newInter.StaticType {
	switch typ := staticType.(type) {
	case oldInter.CompositeStaticType:
		return newInter.CompositeStaticType{
			Location:            typ.Location,
			QualifiedIdentifier: typ.QualifiedIdentifier,
		}
	case oldInter.InterfaceStaticType:
		return newInter.InterfaceStaticType{
			Location:            typ.Location,
			QualifiedIdentifier: typ.QualifiedIdentifier,
		}
	case oldInter.VariableSizedStaticType:
		return newInter.VariableSizedStaticType{
			Type: ConvertStaticType(typ.Type),
		}
	case oldInter.ConstantSizedStaticType:
		return newInter.ConstantSizedStaticType{
			Type: ConvertStaticType(typ.Type),
			Size: typ.Size,
		}
	case oldInter.DictionaryStaticType:
		return newInter.DictionaryStaticType{
			KeyType:   ConvertStaticType(typ.KeyType),
			ValueType: ConvertStaticType(typ.ValueType),
		}
	case oldInter.OptionalStaticType:
		return newInter.OptionalStaticType{
			Type: ConvertStaticType(typ.Type),
		}
	case *oldInter.RestrictedStaticType:
		restrictions := make([]newInter.InterfaceStaticType, 0, len(typ.Restrictions))
		for _, oldInterfaceType := range typ.Restrictions {
			newInterfaceType := ConvertStaticType(oldInterfaceType).(newInter.InterfaceStaticType)
			restrictions = append(restrictions, newInterfaceType)
		}

		return &newInter.RestrictedStaticType{
			Type:         ConvertStaticType(typ.Type),
			Restrictions: restrictions,
		}
	case oldInter.ReferenceStaticType:
		return newInter.ReferenceStaticType{
			Authorized: typ.Authorized,
			Type:       ConvertStaticType(typ.Type),
		}
	case oldInter.CapabilityStaticType:
		return newInter.CapabilityStaticType{
			BorrowType: ConvertStaticType(typ.BorrowType),
		}
	case oldInter.PrimitiveStaticType:
		return ConvertPrimitiveStaticType(typ)
	default:
		panic(fmt.Errorf("cannot covert static type: %s", staticType))
	}
}

func ConvertPrimitiveStaticType(staticType oldInter.PrimitiveStaticType) newInter.PrimitiveStaticType {
	switch staticType {
	case oldInter.PrimitiveStaticTypeVoid:
		return newInter.PrimitiveStaticTypeVoid

	case oldInter.PrimitiveStaticTypeAny:
		return newInter.PrimitiveStaticTypeAny

	case oldInter.PrimitiveStaticTypeNever:
		return newInter.PrimitiveStaticTypeNever

	case oldInter.PrimitiveStaticTypeAnyStruct:
		return newInter.PrimitiveStaticTypeAnyStruct

	case oldInter.PrimitiveStaticTypeAnyResource:
		return newInter.PrimitiveStaticTypeAnyResource

	case oldInter.PrimitiveStaticTypeBool:
		return newInter.PrimitiveStaticTypeBool

	case oldInter.PrimitiveStaticTypeAddress:
		return newInter.PrimitiveStaticTypeAddress

	case oldInter.PrimitiveStaticTypeString:
		return newInter.PrimitiveStaticTypeString

	case oldInter.PrimitiveStaticTypeCharacter:
		return newInter.PrimitiveStaticTypeCharacter

	case oldInter.PrimitiveStaticTypeMetaType:
		return newInter.PrimitiveStaticTypeMetaType

	case oldInter.PrimitiveStaticTypeBlock:
		return newInter.PrimitiveStaticTypeBlock

	// Number

	case oldInter.PrimitiveStaticTypeNumber:
		return newInter.PrimitiveStaticTypeNumber
	case oldInter.PrimitiveStaticTypeSignedNumber:
		return newInter.PrimitiveStaticTypeSignedNumber

	// Integer
	case oldInter.PrimitiveStaticTypeInteger:
		return newInter.PrimitiveStaticTypeInteger
	case oldInter.PrimitiveStaticTypeSignedInteger:
		return newInter.PrimitiveStaticTypeSignedInteger

	// FixedPoint
	case oldInter.PrimitiveStaticTypeFixedPoint:
		return newInter.PrimitiveStaticTypeFixedPoint
	case oldInter.PrimitiveStaticTypeSignedFixedPoint:
		return newInter.PrimitiveStaticTypeSignedFixedPoint

	// Int*
	case oldInter.PrimitiveStaticTypeInt:
		return newInter.PrimitiveStaticTypeInt
	case oldInter.PrimitiveStaticTypeInt8:
		return newInter.PrimitiveStaticTypeInt8
	case oldInter.PrimitiveStaticTypeInt16:
		return newInter.PrimitiveStaticTypeInt16
	case oldInter.PrimitiveStaticTypeInt32:
		return newInter.PrimitiveStaticTypeInt32
	case oldInter.PrimitiveStaticTypeInt64:
		return newInter.PrimitiveStaticTypeInt64
	case oldInter.PrimitiveStaticTypeInt128:
		return newInter.PrimitiveStaticTypeInt128
	case oldInter.PrimitiveStaticTypeInt256:
		return newInter.PrimitiveStaticTypeInt256

	// UInt*
	case oldInter.PrimitiveStaticTypeUInt:
		return newInter.PrimitiveStaticTypeUInt
	case oldInter.PrimitiveStaticTypeUInt8:
		return newInter.PrimitiveStaticTypeUInt8
	case oldInter.PrimitiveStaticTypeUInt16:
		return newInter.PrimitiveStaticTypeUInt16
	case oldInter.PrimitiveStaticTypeUInt32:
		return newInter.PrimitiveStaticTypeUInt32
	case oldInter.PrimitiveStaticTypeUInt64:
		return newInter.PrimitiveStaticTypeUInt64
	case oldInter.PrimitiveStaticTypeUInt128:
		return newInter.PrimitiveStaticTypeUInt128
	case oldInter.PrimitiveStaticTypeUInt256:
		return newInter.PrimitiveStaticTypeUInt256

	// Word *

	case oldInter.PrimitiveStaticTypeWord8:
		return newInter.PrimitiveStaticTypeWord8
	case oldInter.PrimitiveStaticTypeWord16:
		return newInter.PrimitiveStaticTypeWord16
	case oldInter.PrimitiveStaticTypeWord32:
		return newInter.PrimitiveStaticTypeWord32
	case oldInter.PrimitiveStaticTypeWord64:
		return newInter.PrimitiveStaticTypeWord64

	// Fix*
	case oldInter.PrimitiveStaticTypeFix64:
		return newInter.PrimitiveStaticTypeFix64

	// UFix*
	case oldInter.PrimitiveStaticTypeUFix64:
		return newInter.PrimitiveStaticTypeUFix64

	// Storage

	case oldInter.PrimitiveStaticTypePath:
		return newInter.PrimitiveStaticTypePath
	case oldInter.PrimitiveStaticTypeStoragePath:
		return newInter.PrimitiveStaticTypeStoragePath
	case oldInter.PrimitiveStaticTypeCapabilityPath:
		return newInter.PrimitiveStaticTypeCapabilityPath
	case oldInter.PrimitiveStaticTypePublicPath:
		return newInter.PrimitiveStaticTypePublicPath
	case oldInter.PrimitiveStaticTypePrivatePath:
		return newInter.PrimitiveStaticTypePrivatePath
	case oldInter.PrimitiveStaticTypeCapability:
		return newInter.PrimitiveStaticTypeCapability
	case oldInter.PrimitiveStaticTypeAuthAccount:
		return newInter.PrimitiveStaticTypeAuthAccount
	case oldInter.PrimitiveStaticTypePublicAccount:
		return newInter.PrimitiveStaticTypePublicAccount
	case oldInter.PrimitiveStaticTypeDeployedContract:
		return newInter.PrimitiveStaticTypeDeployedContract
	case oldInter.PrimitiveStaticTypeAuthAccountContracts:
		return newInter.PrimitiveStaticTypeAuthAccountContracts
	default:
		panic(fmt.Errorf("cannot covert static type: %s", staticType.String()))
	}
}
