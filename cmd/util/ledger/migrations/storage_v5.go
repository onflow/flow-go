package migrations

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type storageFormatV5MigrationResult struct {
	ledger.Payload
	error
}

type StorageFormatV5Migration struct {
	Log zerolog.Logger
}

func (m StorageFormatV5Migration) Migrate(payloads []ledger.Payload) ([]ledger.Payload, error) {

	migratedPayloads := make([]ledger.Payload, 0, len(payloads))

	jobs := make(chan ledger.Payload)
	results := make(chan storageFormatV5MigrationResult)

	// TODO: runtime.NumCPU()
	workerCount := 1

	l := newView(payloads)
	st := state.NewState(l)
	sth := state.NewStateHolder(st)
	accounts := state.NewAccounts(sth)
	programs := programs.NewEmptyPrograms()
	var brokenTypeIDs sync.Map

	for i := 0; i < workerCount; i++ {
		go m.work(jobs, results, accounts, programs, &brokenTypeIDs)
	}

	go func() {
		for _, payload := range payloads {
			jobs <- payload
		}

		close(jobs)
	}()

	for result := range results {
		if result.error != nil {
			keyParts := result.Key.KeyParts

			rawOwner := keyParts[0].Value
			rawKey := keyParts[2].Value

			return nil, fmt.Errorf(
				"failed to migrate key: %q (owner: %x): %w",
				rawKey,
				rawOwner,
				result.error,
			)
		}
		migratedPayloads = append(migratedPayloads, result.Payload)
		if len(migratedPayloads) == len(payloads) {
			break
		}
	}

	return migratedPayloads, nil
}

func (m StorageFormatV5Migration) work(
	jobs <-chan ledger.Payload,
	results chan<- storageFormatV5MigrationResult,
	accounts *state.Accounts,
	programs *programs.Programs,
	brokenTypeIDs *sync.Map,
) {
	for payload := range jobs {
		migratedPayload, err := m.reencodePayload(
			payload,
			accounts,
			programs,
			brokenTypeIDs,
		)
		result := struct {
			ledger.Payload
			error
		}{
			Payload: payload,
		}
		if err != nil {
			result.error = err
		} else {
			if err := m.checkStorageFormat(migratedPayload); err != nil {
				panic(fmt.Errorf("%w: key = %s", err, payload.Key.String()))
			}
			result.Payload = migratedPayload
		}
		results <- result
	}
}

func (m StorageFormatV5Migration) checkStorageFormat(payload ledger.Payload) error {

	if !bytes.HasPrefix(payload.Value, []byte{0x0, 0xca, 0xde}) {
		return nil
	}

	_, version := interpreter.StripMagic(payload.Value)
	if version != interpreter.CurrentEncodingVersion {
		return fmt.Errorf("invalid version for key %s: %d", payload.Key.String(), version)
	}

	return nil
}

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

func (m StorageFormatV5Migration) reencodePayload(
	payload ledger.Payload,
	accounts *state.Accounts,
	programs *programs.Programs,
	brokenTypeIDs *sync.Map,
) (ledger.Payload, error) {

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
		return payload, nil
	}

	value, version := interpreter.StripMagic(payload.Value)

	if version != interpreter.CurrentEncodingVersion-1 {
		return ledger.Payload{},
			fmt.Errorf(
				"invalid storage format version for key: %s: %d",
				rawKey,
				version,
			)
	}

	err := storageMigrationV5DecMode.Valid(value)
	if err != nil {
		return payload, nil
	}

	// Extract the owner from the key and re-encode the value

	owner := common.BytesToAddress(rawOwner)

	newValue, err := m.reencodeValue(
		value,
		owner,
		string(rawKey),
		version,
		accounts,
		programs,
		brokenTypeIDs,
	)
	if err != nil {
		return ledger.Payload{},
			fmt.Errorf(
				"failed to re-encode key: %s: %w\n\nvalue:\n%s",
				rawKey, err, hex.Dump(value),
			)
	}

	payload.Value = interpreter.PrependMagic(
		newValue,
		interpreter.CurrentEncodingVersion,
	)

	return payload, nil
}

func (m StorageFormatV5Migration) reencodeValue(
	data []byte,
	owner common.Address,
	key string,
	version uint16,
	accounts *state.Accounts,
	programs *programs.Programs,
	brokenTypeIDs *sync.Map,
) ([]byte, error) {

	// Decode the value

	path := []string{key}

	rootValue, err := interpreter.DecodeValueV4(data, &owner, path, version, nil)
	if err != nil {
		return nil,
			fmt.Errorf(
				"failed to decode value: %w\n\nvalue:\n%s\n",
				err, hex.Dump(data),
			)
	}

	m.Log.Info().
		Str("key", key).
		Str("owner", owner.String()).
		Msgf("reencode: %T", rootValue)

	// Force decoding of all container values

	interpreter.InspectValue(
		rootValue,
		func(inspectedValue interpreter.Value) bool {
			switch inspectedValue := inspectedValue.(type) {
			case *interpreter.CompositeValue:
				_ = inspectedValue.Fields()
			case *interpreter.ArrayValue:
				_ = inspectedValue.Elements()
			case *interpreter.DictionaryValue:
				_ = inspectedValue.Entries()
			}
			return true
		},
	)

	m.addKnownContainerStaticTypes(rootValue, owner, key)

	// TODO:
	//
	//
	//addressLocation, ok := location.(common.AddressLocation)
	//if !ok {
	//	return nil, fmt.Errorf(
	//		"cannot load program for unsupported non-address location: %s",
	//		addressLocation,
	//	)
	//}
	//
	//contractCode, err := accounts.GetContract(
	//	addressLocation.Name,
	//	flow.Address(addressLocation.Address),
	//)

	// Infer the static types for array values and dictionary values

	err = m.inferContainerStaticTypes(rootValue, accounts, programs, brokenTypeIDs)
	if err != nil {
		return nil, err
	}

	// Check static types of arrays and dictionaries

	interpreter.InspectValue(
		rootValue,
		func(inspectedValue interpreter.Value) bool {
			switch inspectedValue := inspectedValue.(type) {
			case *interpreter.ArrayValue:

				if !m.arrayHasStaticType(inspectedValue) {

					err = inferArrayStaticType(inspectedValue, nil)
					if err != nil {
						return false
					}

					m.Log.Warn().Msgf(
						"inferred array static type %s from contents: %s",
						inspectedValue.StaticType(),
						inspectedValue,
					)
				}

			case *interpreter.DictionaryValue:

				if !m.dictionaryHasStaticType(inspectedValue) {

					err = inferDictionaryStaticType(inspectedValue, nil)
					if err != nil {
						return false
					}

					m.Log.Warn().Msgf(
						"inferred dictionary static type %s from contents: %s",
						inspectedValue.StaticType(),
						inspectedValue,
					)
				}
			}

			return true
		},
	)
	if err != nil {
		return nil, err
	}

	// Encode the value using the new encoder

	newData, deferrals, err := interpreter.EncodeValue(rootValue, path, true, nil)
	if err != nil {
		log.Err(err).
			Str("key", key).
			Str("owner", owner.String()).
			Str("rootValue", rootValue.String()).
			Msg("failed to encode value")
		return data, nil
	}

	// Encoding should not provide any deferred values or deferred moves

	if len(deferrals.Values) > 0 {
		return nil, fmt.Errorf(
			"re-encoding produced deferred values:\n%s\n",
			rootValue,
		)
	}

	if len(deferrals.Moves) > 0 {
		return nil, fmt.Errorf(
			"re-encoding produced deferred moves:\n%s\n",
			rootValue,
		)
	}

	// Sanity check: Decode the newly encoded data again
	// and compare it to the initially decoded value

	newRootValue, err := interpreter.DecodeValue(
		newData,
		&owner,
		path,
		interpreter.CurrentEncodingVersion,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to decode re-encoded value: %w\n%s\n",
			err, rootValue,
		)
	}

	equatableValue, ok := rootValue.(interpreter.EquatableValue)
	if !ok {
		return nil, fmt.Errorf(
			"cannot compare unequatable %[1]T\n%[1]s\n",
			rootValue,
		)
	}

	if !equatableValue.Equal(newRootValue, nil, false) {
		return nil, fmt.Errorf(
			"values are unequal:\n%s\n%s\n",
			rootValue,
			newRootValue,
		)
	}

	return newData, nil
}

func (m StorageFormatV5Migration) arrayHasStaticType(arrayValue *interpreter.ArrayValue) bool {
	return arrayValue.Type != nil &&
		arrayValue.Type.ElementType() != nil
}

func (m StorageFormatV5Migration) dictionaryHasStaticType(
	dictionaryValue *interpreter.DictionaryValue,
) bool {
	return m.arrayHasStaticType(dictionaryValue.Keys()) &&
		dictionaryValue.Type.KeyType != nil &&
		dictionaryValue.Type.ValueType != nil
}

func (m StorageFormatV5Migration) inferContainerStaticTypes(
	rootValue interpreter.Value,
	accounts *state.Accounts,
	programs *programs.Programs,
	brokenTypeIDs *sync.Map,
) error {
	var err error

	// Start with composite types and use fields' types to infer values' types

	interpreter.InspectValue(
		rootValue,
		func(inspectedValue interpreter.Value) bool {
			compositeValue, ok := inspectedValue.(*interpreter.CompositeValue)
			if !ok {
				// The inspected value is not a composite value,
				// continue inspecting other values
				return true
			}

			// If the inference for the composite's type failed before,
			// then ignore this composite and continue inspecting other values

			typeID := compositeValue.TypeID()
			_, isBroken := brokenTypeIDs.Load(typeID)
			if isBroken {
				return true
			}

			var program *interpreter.Program
			program, err = loadProgram(compositeValue.Location(), accounts, programs)
			if err != nil {
				var parsingCheckingError *runtime.ParsingCheckingError
				if !errors.As(err, &parsingCheckingError) {
					// If loading the program failed and it was not "just" a parsing / checking error,
					// then something else is wrong, so abort the migration
					return false
				}

				// If the program for the composite's type could not be parsed or checked,
				// report it, prevent a re-parse and re-check, and continue inspecting other values

				m.Log.Err(err).
					Str("typeID", string(typeID)).
					Msg("failed to parse and check program")

				brokenTypeIDs.Store(typeID, nil)

				err = nil
				return true
			}

			compositeType := program.Elaboration.CompositeTypes[typeID]
			if compositeType == nil {

				// If the composite type is missing,
				// report it, prevent a re-parse and re-check, and continue inspecting other values

				m.Log.Error().
					Str("typeID", string(typeID)).
					Msg("missing composite type")

				brokenTypeIDs.Store(typeID, nil)

				return true
			}

			var fieldsToDelete []string

			fields := compositeValue.Fields()
			for pair := fields.Oldest(); pair != nil; pair = pair.Next() {
				fieldName := pair.Key
				fieldValue := pair.Value

				member, ok := compositeType.Members.Get(fieldName)
				if !ok {
					// TODO: OK?
					// If the type info for the field is missing,
					// then delete the field contents

					fieldsToDelete = append(fieldsToDelete)
					continue
				}

				fieldType := interpreter.ConvertSemaToStaticType(member.TypeAnnotation.Type)

				err = inferContainerStaticType(fieldValue, fieldType)
				if err != nil {

					// If the container type cannot be inferred using the field type,
					// report it and continue inferring other fields

					m.Log.Err(err).
						Str("typeID", string(typeID)).
						Str("fieldType", fieldType.String()).
						Str("fieldValue", fieldValue.String()).
						Msg("failed to infer container type based on field type")

					err = nil
				}
			}

			for _, fieldName := range fieldsToDelete {
				m.Log.Warn().
					Str("typeID", string(typeID)).
					Msgf("removing field with missing type: %s", fieldName)
				fields.Delete(fieldName)
			}

			return true
		},
	)

	return err
}

var testnetNFTLocation = func() common.Location {
	address, err := common.HexToAddress("631e88ae7f1d7c20")
	if err != nil {
		panic(err)
	}
	return common.AddressLocation{
		Address: address,
		Name:    "NonFungibleToken",
	}
}()

func (m StorageFormatV5Migration) addKnownContainerStaticTypes(
	value interpreter.Value,
	owner common.Address,
	key string,
) {
	switch value := value.(type) {
	case *interpreter.CompositeValue:
		switch value.QualifiedIdentifier() {
		case "FlowIDTableStaking.NodeRecord":

			if !hasAnyLocationAddress(
				value,
				"ecda6c5746d5bdf0",
				"f1a43bfd1354c9b8",
				"e94f751ba094ef6a",
				"76d9ea44cef09e20",
			) {
				return
			}

			m.addDictionaryFieldType(
				value,
				"delegators",
				interpreter.PrimitiveStaticTypeUInt32,
				interpreter.CompositeStaticType{
					Location:            value.Location(),
					QualifiedIdentifier: "FlowIDTableStaking.DelegatorRecord",
				},
				owner,
				key,
			)

		case "Versus.DropCollection":
			if !hasAnyLocationAddress(value, "1ff7e32d71183db0") {
				return
			}

			m.addDictionaryFieldType(
				value,
				"drops",
				interpreter.PrimitiveStaticTypeUInt64,
				interpreter.CompositeStaticType{
					Location:            value.Location(),
					QualifiedIdentifier: "Versus.Drop",
				},
				owner,
				key,
			)

		case "KittyItemsMarket.Collection":

			if !hasAnyLocationAddress(value,
				"fcceff21d9532b58",
				"172be932e9cd0a8f",
			) {
				return
			}

			m.addDictionaryFieldType(
				value,
				"saleOffers",
				interpreter.PrimitiveStaticTypeUInt64,
				interpreter.CompositeStaticType{
					Location:            value.Location(),
					QualifiedIdentifier: "KittyItemsMarket.SaleOffer",
				},
				owner,
				key,
			)

		case "RecordShop.Collection":

			// Probably based on KittyItemsMarket

			if !hasAnyLocationAddress(value, "7352d990d2addd95") {
				return
			}

			m.addDictionaryFieldType(
				value,
				"saleOffers",
				interpreter.PrimitiveStaticTypeUInt64,
				interpreter.CompositeStaticType{
					Location:            value.Location(),
					QualifiedIdentifier: "RecordShop.SaleOffer",
				},
				owner,
				key,
			)

		case "LikeNastyaItemsMarket.Collection":

			// Probably based on KittyItemsMarket

			if !hasAnyLocationAddress(value, "9f3e19cda04154fc") {
				return
			}

			m.addDictionaryFieldType(
				value,
				"saleOffers",
				interpreter.PrimitiveStaticTypeUInt64,
				interpreter.CompositeStaticType{
					Location:            value.Location(),
					QualifiedIdentifier: "LikeNastyaItemsMarket.SaleOffer",
				},
				owner,
				key,
			)

		case "KittyItems.Collection":

			if !hasAnyLocationAddress(value, "f79ee844bfa76528", "fcceff21d9532b58") {
				return
			}

			m.addDictionaryFieldType(
				value,
				"ownedNFTs",
				interpreter.PrimitiveStaticTypeUInt64,
				interpreter.InterfaceStaticType{
					Location:            testnetNFTLocation,
					QualifiedIdentifier: "NonFungibleToken.NFT",
				},
				owner,
				key,
			)

		case "LikeNastyaItems.Collection":

			// Likely https://medium.com/pinata/how-to-create-nfts-like-nba-top-shot-with-flow-and-ipfs-701296944bf

			if !hasAnyLocationAddress(value, "9F3E19CDA04154FC") {
				return
			}

			m.addDictionaryFieldType(
				value,
				"ownedNFTs",
				interpreter.PrimitiveStaticTypeUInt64,
				interpreter.InterfaceStaticType{
					Location:            testnetNFTLocation,
					QualifiedIdentifier: "NonFungibleToken.NFT",
				},
				owner,
				key,
			)

			m.addDictionaryFieldType(
				value,
				"metadataObjs",
				interpreter.PrimitiveStaticTypeUInt64,
				interpreter.DictionaryStaticType{
					KeyType:   interpreter.PrimitiveStaticTypeString,
					ValueType: interpreter.PrimitiveStaticTypeString,
				},
				owner,
				key,
			)

		}
	}
}

func (m StorageFormatV5Migration) addDictionaryFieldType(
	value *interpreter.CompositeValue,
	fieldName string,
	keyType interpreter.StaticType,
	valueType interpreter.StaticType,
	owner common.Address,
	key string,
) {
	fieldValue, ok := value.Fields().Get(fieldName)
	if !ok {
		return
	}

	dictionaryValue, ok := fieldValue.(*interpreter.DictionaryValue)
	if !ok {
		return
	}

	dictionaryValue.Keys().Type = interpreter.VariableSizedStaticType{
		Type: keyType,
	}

	dictionaryValue.Type = interpreter.DictionaryStaticType{
		KeyType:   keyType,
		ValueType: valueType,
	}

	m.Log.Info().
		Str("owner", owner.String()).
		Str("key", key).
		Msgf(
			"added known static type %s to dictionary: %s",
			dictionaryValue.Type,
			dictionaryValue.String(),
		)
}

func hasAnyLocationAddress(value *interpreter.CompositeValue, hexAddresses ...string) bool {
	location := value.Location()
	addressLocation, ok := location.(common.AddressLocation)
	if !ok {
		return false
	}

	for _, hexAddress := range hexAddresses {
		address, err := common.HexToAddress(hexAddress)
		if err != nil {
			return false
		}
		if addressLocation.Address == address {
			return true
		}
	}

	return false
}

func inferContainerStaticType(value interpreter.Value, t interpreter.StaticType) error {

	// Only infer static type for arrays and dictionaries

	switch value := value.(type) {
	case *interpreter.ArrayValue:
		err := inferArrayStaticType(value, t)
		if err != nil {
			return err
		}

	case *interpreter.DictionaryValue:
		err := inferDictionaryStaticType(value, t)
		if err != nil {
			return err
		}
	}

	return nil
}

func inferArrayStaticType(value *interpreter.ArrayValue, t interpreter.StaticType) error {

	if t == nil {
		if value.Count() == 0 {
			return fmt.Errorf("cannot infer static type for empty array value")
		}

		var elementType interpreter.StaticType

		for _, element := range value.Elements() {
			if elementType == nil {
				elementType = element.StaticType()
			} else if !element.StaticType().Equal(elementType) {
				return fmt.Errorf("cannot infer static type for array with mixed elements")
			}
		}

		// TODO: infer element type to AnyStruct or AnyResource based on kinds of elements instead?
		value.Type = interpreter.VariableSizedStaticType{
			Type: elementType,
		}
	} else {

		switch arrayType := t.(type) {
		case interpreter.VariableSizedStaticType:
			value.Type = arrayType

		case interpreter.ConstantSizedStaticType:
			value.Type = arrayType

		default:
			switch t {
			case interpreter.PrimitiveStaticTypeAnyStruct,
				interpreter.PrimitiveStaticTypeAnyResource:

				value.Type = interpreter.VariableSizedStaticType{
					Type: t,
				}

			default:
				return fmt.Errorf(
					"failed to infer static type for array value and given type %s: %s",
					t, value,
				)
			}
		}
	}

	// Recursively infer type for array elements

	elementType := value.Type.ElementType()

	for _, element := range value.Elements() {
		err := inferContainerStaticType(element, elementType)
		if err != nil {
			return err
		}
	}

	return nil
}

func inferDictionaryStaticType(value *interpreter.DictionaryValue, t interpreter.StaticType) error {
	entries := value.Entries()

	if t == nil {
		if entries.Len() == 0 {
			//if value.Count() == 0 {
			return fmt.Errorf("cannot infer static type for empty dictionary value: %s", value.String())
			//}
			//
			//// The dictionary has deferred values,
			//// which is only the case when the values are resources
			//
			//var keyType interpreter.StaticType
			//for _, key := range value.Keys().Elements() {
			//	if keyType == nil {
			//		keyType = key.StaticType()
			//	} else if !key.StaticType().Equal(keyType) {
			//		return fmt.Errorf("cannot infer key static type for dictionary with mixed type keys")
			//	}
			//}
			//
			//value.Type = interpreter.DictionaryStaticType{
			//	KeyType: keyType,
			//	// NOTE: can only infer AnyResource as values are not available
			//	ValueType: interpreter.PrimitiveStaticTypeAnyResource,
			//}

		} else {

			var keyType interpreter.StaticType
			for _, key := range value.Keys().Elements() {
				if keyType == nil {
					keyType = key.StaticType()
				} else if !key.StaticType().Equal(keyType) {
					return fmt.Errorf("cannot infer key static type for dictionary with mixed type keys")
				}
			}

			var valueType interpreter.StaticType
			for pair := entries.Oldest(); pair != nil; pair = pair.Next() {
				if valueType == nil {
					valueType = pair.Value.StaticType()
				} else if !pair.Value.StaticType().Equal(valueType) {
					return fmt.Errorf("cannot infer value static type for dictionary with mixed type values")
				}
			}

			// TODO: infer value type to AnyStruct or AnyResource based on kinds of values instead?
			value.Type = interpreter.DictionaryStaticType{
				KeyType:   keyType,
				ValueType: valueType,
			}
		}
	} else {

		if dictionaryType, ok := t.(interpreter.DictionaryStaticType); ok {
			value.Type = dictionaryType
		} else {
			switch t {
			case interpreter.PrimitiveStaticTypeAnyStruct,
				interpreter.PrimitiveStaticTypeAnyResource:

				value.Type = interpreter.DictionaryStaticType{
					KeyType:   interpreter.PrimitiveStaticTypeAnyStruct,
					ValueType: t,
				}

			default:
				return fmt.Errorf(
					"failed to infer static type for dictionary value and given type %s: %s",
					t, value,
				)
			}
		}
	}

	// Recursively infer type for dictionary keys and values

	err := inferContainerStaticType(
		value.Keys(),
		interpreter.VariableSizedStaticType{
			Type: value.Type.KeyType,
		},
	)
	if err != nil {
		return err
	}

	for pair := entries.Oldest(); pair != nil; pair = pair.Next() {
		err := inferContainerStaticType(
			pair.Value,
			value.Type.ValueType,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func loadProgram(
	location common.Location,
	accounts *state.Accounts,
	programs *programs.Programs,
) (
	*interpreter.Program,
	error,
) {
	program, _, ok := programs.Get(location)
	if ok {
		return program, nil
	}

	addressLocation, ok := location.(common.AddressLocation)
	if !ok {
		return nil, fmt.Errorf(
			"cannot load program for unsupported non-address location: %s",
			addressLocation,
		)
	}

	contractCode, err := accounts.GetContract(
		addressLocation.Name,
		flow.Address(addressLocation.Address),
	)
	if err != nil {
		return nil, err
	}

	rt := runtime.NewInterpreterRuntime()
	program, err = rt.ParseAndCheckProgram(
		contractCode,
		runtime.Context{
			Interface: migrationRuntimeInterface{accounts, programs},
			Location:  location,
		},
	)
	if err != nil {
		return nil, err
	}

	programs.Set(location, program, nil)

	return program, nil
}

type migrationRuntimeInterface struct {
	accounts *state.Accounts
	programs *programs.Programs
}

func (m migrationRuntimeInterface) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) ([]runtime.ResolvedLocation, error) {

	addressLocation, isAddress := location.(common.AddressLocation)

	// if the location is not an address location, e.g. an identifier location (`import Crypto`),
	// then return a single resolved location which declares all identifiers.
	if !isAddress {
		return []runtime.ResolvedLocation{
			{
				Location:    location,
				Identifiers: identifiers,
			},
		}, nil
	}

	// if the location is an address,
	// and no specific identifiers where requested in the import statement,
	// then fetch all identifiers at this address
	if len(identifiers) == 0 {
		address := flow.Address(addressLocation.Address)

		contractNames, err := m.accounts.GetContractNames(address)
		if err != nil {
			return nil, fmt.Errorf("ResolveLocation failed: %w", err)
		}

		// if there are no contractNames deployed,
		// then return no resolved locations
		if len(contractNames) == 0 {
			return nil, nil
		}

		identifiers = make([]ast.Identifier, len(contractNames))

		for i := range identifiers {
			identifiers[i] = runtime.Identifier{
				Identifier: contractNames[i],
			}
		}
	}

	// return one resolved location per identifier.
	// each resolved location is an address contract location
	resolvedLocations := make([]runtime.ResolvedLocation, len(identifiers))
	for i := range resolvedLocations {
		identifier := identifiers[i]
		resolvedLocations[i] = runtime.ResolvedLocation{
			Location: common.AddressLocation{
				Address: addressLocation.Address,
				Name:    identifier.Identifier,
			},
			Identifiers: []runtime.Identifier{identifier},
		}
	}

	return resolvedLocations, nil
}

func (m migrationRuntimeInterface) GetCode(location runtime.Location) ([]byte, error) {
	contractLocation, ok := location.(common.AddressLocation)
	if !ok {
		return nil, fmt.Errorf("GetCode failed: expected AddressLocation")
	}

	add, err := m.accounts.GetContract(contractLocation.Name, flow.Address(contractLocation.Address))
	if err != nil {
		return nil, fmt.Errorf("GetCode failed: %w", err)
	}

	return add, nil
}

func (m migrationRuntimeInterface) GetProgram(location runtime.Location) (*interpreter.Program, error) {
	program, _, ok := m.programs.Get(location)
	if ok {
		return program, nil
	}

	return nil, nil
}

func (m migrationRuntimeInterface) SetProgram(location runtime.Location, program *interpreter.Program) error {
	m.programs.Set(location, program, nil)
	return nil
}

func (m migrationRuntimeInterface) GetValue(_, _ []byte) (value []byte, err error) {
	panic("unexpected GetValue call")
}

func (m migrationRuntimeInterface) SetValue(_, _, _ []byte) (err error) {
	panic("unexpected SetValue call")
}

func (m migrationRuntimeInterface) CreateAccount(_ runtime.Address) (address runtime.Address, err error) {
	panic("unexpected CreateAccount call")
}

func (m migrationRuntimeInterface) AddEncodedAccountKey(_ runtime.Address, _ []byte) error {
	panic("unexpected AddEncodedAccountKey call")
}

func (m migrationRuntimeInterface) RevokeEncodedAccountKey(_ runtime.Address, _ int) (publicKey []byte, err error) {
	panic("unexpected RevokeEncodedAccountKey call")
}

func (m migrationRuntimeInterface) AddAccountKey(
	_ runtime.Address,
	_ *runtime.PublicKey,
	_ runtime.HashAlgorithm,
	_ int,
) (*runtime.AccountKey, error) {
	panic("unexpected AddAccountKey call")
}

func (m migrationRuntimeInterface) GetAccountKey(_ runtime.Address, _ int) (*runtime.AccountKey, error) {
	panic("unexpected GetAccountKey call")
}

func (m migrationRuntimeInterface) RevokeAccountKey(_ runtime.Address, _ int) (*runtime.AccountKey, error) {
	panic("unexpected RevokeAccountKey call")
}

func (m migrationRuntimeInterface) UpdateAccountContractCode(_ runtime.Address, _ string, _ []byte) (err error) {
	panic("unexpected UpdateAccountContractCode call")
}

func (m migrationRuntimeInterface) GetAccountContractCode(
	address runtime.Address,
	name string,
) (code []byte, err error) {
	return m.accounts.GetContract(name, flow.Address(address))
}

func (m migrationRuntimeInterface) RemoveAccountContractCode(_ runtime.Address, _ string) (err error) {
	panic("unexpected RemoveAccountContractCode call")
}

func (m migrationRuntimeInterface) GetSigningAccounts() ([]runtime.Address, error) {
	panic("unexpected GetSigningAccounts call")
}

func (m migrationRuntimeInterface) ProgramLog(_ string) error {
	panic("unexpected ProgramLog call")
}

func (m migrationRuntimeInterface) EmitEvent(_ cadence.Event) error {
	panic("unexpected EmitEvent call")
}

func (m migrationRuntimeInterface) ValueExists(_, _ []byte) (exists bool, err error) {
	panic("unexpected ValueExists call")
}

func (m migrationRuntimeInterface) GenerateUUID() (uint64, error) {
	panic("unexpected GenerateUUID call")
}

func (m migrationRuntimeInterface) GetComputationLimit() uint64 {
	panic("unexpected GetComputationLimit call")
}

func (m migrationRuntimeInterface) SetComputationUsed(_ uint64) error {
	panic("unexpected SetComputationUsed call")
}

func (m migrationRuntimeInterface) DecodeArgument(_ []byte, _ cadence.Type) (cadence.Value, error) {
	panic("unexpected DecodeArgument call")
}

func (m migrationRuntimeInterface) GetCurrentBlockHeight() (uint64, error) {
	panic("unexpected GetCurrentBlockHeight call")
}

func (m migrationRuntimeInterface) GetBlockAtHeight(_ uint64) (block runtime.Block, exists bool, err error) {
	panic("unexpected GetBlockAtHeight call")
}

func (m migrationRuntimeInterface) UnsafeRandom() (uint64, error) {
	panic("unexpected UnsafeRandom call")
}

func (m migrationRuntimeInterface) VerifySignature(
	_ []byte,
	_ string,
	_ []byte,
	_ []byte,
	_ runtime.SignatureAlgorithm,
	_ runtime.HashAlgorithm,
) (bool, error) {
	panic("unexpected VerifySignature call")
}

func (m migrationRuntimeInterface) Hash(_ []byte, _ string, _ runtime.HashAlgorithm) ([]byte, error) {
	panic("unexpected Hash call")
}

func (m migrationRuntimeInterface) GetAccountBalance(_ common.Address) (value uint64, err error) {
	panic("unexpected GetAccountBalance call")
}

func (m migrationRuntimeInterface) GetAccountAvailableBalance(_ common.Address) (value uint64, err error) {
	panic("unexpected GetAccountAvailableBalance call")
}

func (m migrationRuntimeInterface) GetStorageUsed(_ runtime.Address) (value uint64, err error) {
	panic("unexpected GetStorageUsed call")
}

func (m migrationRuntimeInterface) GetStorageCapacity(_ runtime.Address) (value uint64, err error) {
	panic("unexpected GetStorageCapacity call")
}

func (m migrationRuntimeInterface) ImplementationDebugLog(_ string) error {
	panic("unexpected ImplementationDebugLog call")
}

func (m migrationRuntimeInterface) ValidatePublicKey(_ *runtime.PublicKey) (bool, error) {
	panic("unexpected ValidatePublicKey call")
}
