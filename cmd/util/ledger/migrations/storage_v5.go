package migrations

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"

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
)

type storageFormatV5MigrationResult struct {
	key string
	ledger.Payload
	error
}

func StorageFormatV5Migration(payloads []ledger.Payload) ([]ledger.Payload, error) {

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

	for i := 0; i < workerCount; i++ {
		go storageFormatV5MigrationWorker(jobs, results, accounts, programs)
	}

	go func() {
		for _, payload := range payloads {
			jobs <- payload
		}

		close(jobs)
	}()

	for result := range results {
		if result.error != nil {
			return nil, fmt.Errorf("failed to migrate key: %#+v: %w", result.key, result.error)
		}
		migratedPayloads = append(migratedPayloads, result.Payload)
		if len(migratedPayloads) == len(payloads) {
			break
		}
	}

	return migratedPayloads, nil
}

func storageFormatV5MigrationWorker(
	jobs <-chan ledger.Payload,
	results chan<- storageFormatV5MigrationResult,
	accounts *state.Accounts,
	programs *programs.Programs,
) {
	for payload := range jobs {
		migratedPayload, err := reencodePayloadV5(payload, accounts, programs)
		result := struct {
			key string
			ledger.Payload
			error
		}{
			key: payload.Key.String(),
		}
		if err != nil {
			result.error = err
		} else {
			if err := checkStorageFormatV5(migratedPayload); err != nil {
				panic(fmt.Errorf("%w: key = %s", err, payload.Key.String()))
			}
			result.Payload = migratedPayload
		}
		results <- result
	}
}

func checkStorageFormatV5(payload ledger.Payload) error {

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

func reencodePayloadV5(
	payload ledger.Payload,
	accounts *state.Accounts,
	programs *programs.Programs,
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

	newValue, err := reencodeValueV5(
		value,
		owner,
		string(rawKey),
		version,
		accounts,
		programs,
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

func reencodeValueV5(
	data []byte,
	owner common.Address,
	key string,
	version uint16,
	accounts *state.Accounts,
	programs *programs.Programs,
) ([]byte, error) {

	// Decode the value

	path := []string{key}

	value, err := interpreter.DecodeValueV4(data, &owner, path, version, nil)
	if err != nil {
		return nil,
			fmt.Errorf(
				"failed to decode value: %w\n\nvalue:\n%s\n",
				err, hex.Dump(data),
			)
	}

	// Force decoding of all container values

	interpreter.InspectValue(
		value,
		func(value interpreter.Value) bool {
			switch value := value.(type) {
			case *interpreter.CompositeValue:
				_ = value.Fields()
			case *interpreter.ArrayValue:
				_ = value.Elements()
			case *interpreter.DictionaryValue:
				_ = value.Entries()
			}
			return true
		},
	)

	// Infer the static types for array values and dictionary values

	ok, err := inferContainerStaticTypes(value, accounts, programs)
	if err != nil {
		return nil, err
	}
	// If the types could not be inferred,
	// then return the data as-is, unmigrated
	if !ok {
		return data, nil
	}

	// Check static types of arrays and dictionaries

	interpreter.InspectValue(
		value,
		func(value interpreter.Value) bool {
			switch value := value.(type) {
			case *interpreter.ArrayValue:

				if value.Type == nil ||
					value.Type.ElementType() == nil {

					err = fmt.Errorf("missing static type for array: %s", value)
					return false
				}

			case *interpreter.DictionaryValue:

				if value.Type.KeyType == nil ||
					value.Type.ValueType == nil {

					err = fmt.Errorf("missing static type for dictionary: %s", value)
					return false
				}
			}

			return true
		},
	)
	if err != nil {
		return nil, err
	}

	// Encode the value using the new encoder

	newData, deferrals, err := interpreter.EncodeValue(value, path, true, nil)
	if err != nil {
		fmt.Printf(
			"failed to encode value for owner=%s key=%s: %s\n%s\n",
			owner, key, err, value,
		)
		return data, nil
	}

	// Encoding should not provide any deferred values or deferred moves

	if len(deferrals.Values) > 0 {
		return nil, fmt.Errorf(
			"re-encoding produced deferred values:\n%s\n",
			value,
		)
	}

	if len(deferrals.Moves) > 0 {
		return nil, fmt.Errorf(
			"re-encoding produced deferred moves:\n%s\n",
			value,
		)
	}

	// Sanity check: Decode the newly encoded data again
	// and compare it to the initially decoded value

	newValue, err := interpreter.DecodeValue(
		newData,
		&owner,
		path,
		interpreter.CurrentEncodingVersion,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to decode re-encoded value: %w\n%s\n",
			err, value,
		)
	}

	equatableValue, ok := value.(interpreter.EquatableValue)
	if !ok {
		return nil, fmt.Errorf(
			"cannot compare unequatable %[1]T\n%[1]s\n",
			value,
		)
	}

	if !equatableValue.Equal(newValue, nil, false) {
		return nil, fmt.Errorf(
			"values are unequal:\n%s\n%s\n",
			value, newValue,
		)
	}

	return newData, nil
}

func inferContainerStaticTypes(
	value interpreter.Value,
	accounts *state.Accounts,
	programs *programs.Programs,
) (bool, error) {
	var err error
	var typeLoadFailure bool

	interpreter.InspectValue(
		value,
		func(value interpreter.Value) bool {
			compositeValue, ok := value.(*interpreter.CompositeValue)
			if !ok {
				return true
			}

			typeID := compositeValue.TypeID()

			var program *interpreter.Program
			program, err = loadProgram(compositeValue.Location(), accounts, programs)
			if err != nil {
				var parsingCheckingError *runtime.ParsingCheckingError
				if errors.As(err, &parsingCheckingError) {
					fmt.Printf(
						"Failed to parse and check program for: %s: %s\n",
						typeID, err.Error(),
					)
					typeLoadFailure = true
					err = nil
				}

				return false
			}

			compositeType := program.Elaboration.CompositeTypes[typeID]
			if compositeType == nil {
				typeLoadFailure = true
				err = nil
				return false
			}

			fields := compositeValue.Fields()
			for pair := fields.Oldest(); pair != nil; pair = pair.Next() {
				fieldName := pair.Key
				fieldValue := pair.Value

				member, ok := compositeType.Members.Get(fieldName)
				if !ok {
					err = fmt.Errorf("missing type for composite field: %s.%s", typeID, fieldName)
					return false
				}

				staticType := interpreter.ConvertSemaToStaticType(member.TypeAnnotation.Type)

				err = inferContainerStaticType(fieldValue, staticType)
				if err != nil {
					return false
				}
			}

			return true
		},
	)
	if err != nil {
		return false, err
	}
	if typeLoadFailure {
		return false, nil
	}

	return true, nil
}

func inferContainerStaticType(value interpreter.Value, t interpreter.StaticType) error {

	// Only infer static type for arrays and dictionaries
	switch value := value.(type) {
	case *interpreter.ArrayValue:

		switch arrayType := t.(type) {
		case interpreter.VariableSizedStaticType:
			value.Type = arrayType
			// TODO: elements

		case interpreter.ConstantSizedStaticType:
			value.Type = arrayType
			// TODO: elements

		default:
			fmt.Printf("??? ARRAY VALUE NON-ARRAY TYPE: %s\n", t)
		}

	case *interpreter.DictionaryValue:
		if dictionaryType, ok := t.(interpreter.DictionaryStaticType); ok {
			value.Type = dictionaryType

			err := inferContainerStaticType(
				value.Keys(),
				interpreter.VariableSizedStaticType{
					Type: dictionaryType.KeyType,
				},
			)
			if err != nil {
				return err
			}

			entries := value.Entries()
			for pair := entries.Oldest(); pair != nil; pair = pair.Next() {
				// TODO: entry value (keys already inferred above)
			}
		} else {
			fmt.Printf("??? DICT VALUE NON-DICT TYPE: %s\n", t)
		}

	default:
		return nil
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
