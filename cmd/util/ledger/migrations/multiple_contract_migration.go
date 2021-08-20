package migrations

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/parser2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type MultipleContractMigrationError struct {
	Errors []error
}

func (e *MultipleContractMigrationError) Error() string {
	var b strings.Builder
	b.WriteString("multiple contract migration errors:")
	for i, err := range e.Errors {
		_, err := fmt.Fprintf(&b, "\nerr %d: %s", i, err)
		if err != nil {
			panic(err)
		}
	}
	return b.String()
}

var (
	MultipleContractsSpecialMigrations = make(map[string]func(ledger.Payload) ([]ledger.Payload, error))
)

func MultipleContractMigration(payloads []ledger.Payload) ([]ledger.Payload, error) {

	migratedPayloads, contractValueMappings, errors := migrateContractValues(payloads)

	for _, p := range payloads {
		results, err := migrateNonContractValue(p, contractValueMappings)
		// dont fail fast... try to collect errors so multiple errors can be addressed at once
		if err != nil {
			errors = append(errors, err)
			continue
		}
		migratedPayloads = append(migratedPayloads, results...)
	}

	if len(errors) != 0 {
		return nil, &MultipleContractMigrationError{
			Errors: errors,
		}
	}
	return migratedPayloads, nil
}

func createContractNamesKey(originalKey ledger.Key) ledger.Key {
	return ledger.Key{
		KeyParts: []ledger.KeyPart{
			originalKey.KeyParts[0],
			originalKey.KeyParts[1],
			{
				Type:  state.KeyPartKey,
				Value: []byte("contract_names"),
			},
		},
	}
}

func encodeContractNames(contractNames []string) ([]byte, error) {
	sort.Strings(contractNames)
	var buf bytes.Buffer
	cborEncoder := cbor.NewEncoder(&buf)
	err := cborEncoder.Encode(contractNames)
	if err != nil {
		return nil, fmt.Errorf("cannot encode contract names")
	}
	return buf.Bytes(), nil
}

func contractsRegister(contractsKey ledger.Key, contractNames []string) (ledger.Payload, error) {
	encodedContract, err := encodeContractNames(contractNames)
	if err != nil {
		return ledger.Payload{}, err
	}
	return ledger.Payload{
		Key:   createContractNamesKey(contractsKey),
		Value: encodedContract,
	}, nil
}

const deferredValueOfContractValuePrefix = "contract\x1f"

func migrateNonContractValue(p ledger.Payload, contractValueMappings map[string]string) ([]ledger.Payload, error) {
	registerID, err := keyToRegisterID(p.Key)
	if err != nil {
		return nil, err
	}

	// ignore contract value registers, they have already been migrated
	if registerID.Key == "contract" {
		return nil, nil
	}

	if !isAddress(registerID) {
		return []ledger.Payload{p}, nil
	}

	// migrate contract code register
	if registerID.Key == "code" {
		if em, hasEM := MultipleContractsSpecialMigrations[registerID.Owner]; hasEM {
			log.Info().
				Err(err).
				Str("address", flow.BytesToAddress([]byte(registerID.Owner)).HexWithPrefix()).
				Msg("Using exceptional migration for address")
			return em(p)
		}
		return migrateContractCode(p)
	}

	// migrate deferred value of contract value
	if strings.HasPrefix(registerID.Key, deferredValueOfContractValuePrefix) {
		return migrateDeferredValueOfContractValue(p, registerID, contractValueMappings)
	}

	return []ledger.Payload{p}, nil
}

func migrateDeferredValueOfContractValue(
	payload ledger.Payload,
	registerID flow.RegisterID,
	mappings map[string]string,
) (
	[]ledger.Payload,
	error,
) {
	registerKeySuffix := registerID.Key[len(deferredValueOfContractValuePrefix):]

	rawAddress := string(payloadKeyAddress(payload))
	address := common.BytesToAddress(flow.BytesToAddress([]byte(rawAddress)).Bytes())

	contractName, ok := mappings[rawAddress]
	if !ok {
		return nil, fmt.Errorf("missing contract name for address: %s", address)
	}

	newRegisterKey := strings.Join([]string{
		deferredValueOfContractValuePrefix,
		contractName,
		"\x1f",
		registerKeySuffix,
	}, "")

	newPayload := ledger.Payload{
		Key:   changeKey(payload.Key, newRegisterKey),
		Value: payload.Value,
	}

	logKeyChange(address.Hex(), payload.Key, newPayload.Key)

	return []ledger.Payload{newPayload}, nil
}

func migrateContractValues(payloads []ledger.Payload) ([]ledger.Payload, map[string]string, []error) {
	migratedPayloads := make([]ledger.Payload, 0, len(payloads))
	contractValueMappings := make(map[string]string)
	errors := make([]error, 0)

	for _, p := range payloads {
		registerID, err := keyToRegisterID(p.Key)
		if err != nil {
			// dont fail fast... try to collect errors so multiple errors can be addressed at once
			errors = append(errors, err)
			continue
		}

		if !isAddress(registerID) {
			continue
		}

		if registerID.Key != "contract" {
			continue
		}

		results, mapping, err := migrateContractValue(p)
		if err != nil {
			// dont fail fast... try to collect errors so multiple errors can be addressed at once
			errors = append(errors, err)
			continue
		}
		migratedPayloads = append(migratedPayloads, results...)

		if mapping != nil {
			if _, ok := contractValueMappings[string(mapping.address)]; ok {
				err = fmt.Errorf(
					"contract value mapping for address %v already exists",
					mapping.address,
				)
				errors = append(errors, err)
				continue
			}
			contractValueMappings[string(mapping.address)] = mapping.contractName
		}
	}

	return migratedPayloads, contractValueMappings, errors
}

type contractValueMapping struct {
	address      []byte
	contractName string
}

func migrateContractValue(p ledger.Payload) ([]ledger.Payload, *contractValueMapping, error) {
	rawAddress := payloadKeyAddress(p)
	address := common.BytesToAddress(flow.BytesToAddress(rawAddress).Bytes())
	storedData, version := interpreter.StripMagic(p.Value)
	if len(storedData) == 0 {
		return []ledger.Payload{}, nil, nil
	}

	storedValue, err := decode(storedData, version, address)
	if err != nil {
		log.Error().
			Err(err).
			Str("address", address.Hex()).
			Msg("Cannot decode contract at address")
		return nil, nil, err
	}

	value := interpreter.NewSomeValueNonCopying(storedValue).Value.(*interpreter.CompositeValue)
	pieces := strings.Split(string(value.TypeID()), ".")
	if len(pieces) != 3 {
		log.Error().
			Str("TypeId", string(value.TypeID())).
			Str("address", address.Hex()).
			Msg("contract TypeId not in correct format")
		return nil, nil, fmt.Errorf("contract TypeId not in correct format")
	}

	contractName := pieces[2]

	newKey := changeKey(p.Key, fmt.Sprintf("contract\x1F%s", contractName))

	logKeyChange(address.Hex(), p.Key, newKey)

	newPayloads := []ledger.Payload{
		{
			Key:   newKey,
			Value: p.Value,
		},
	}

	mapping := &contractValueMapping{
		address:      rawAddress,
		contractName: contractName,
	}

	return newPayloads, mapping, nil
}

func payloadKeyAddress(p ledger.Payload) []byte {
	return p.Key.KeyParts[0].Value
}

func migrateContractCode(p ledger.Payload) ([]ledger.Payload, error) {

	// we don't need the the empty code register
	value := p.Value
	address := flow.BytesToAddress(payloadKeyAddress(p))
	if len(value) == 0 {
		return []ledger.Payload{}, nil
	}

	code := string(value)
	program, err := parser2.ParseProgram(code)
	if err != nil {
		log.Error().
			Err(err).
			Str("address", address.Hex()).
			Str("code", code).
			Msg("Cannot parse program at address")
		return nil, err
	}
	declarations := program.Declarations()

	// find import declarations
	importDeclarations := make([]ast.Declaration, 0)
	for _, d := range declarations {
		if _, isImport := d.(*ast.ImportDeclaration); isImport {
			importDeclarations = append(importDeclarations, d)
		}
	}

	// sort them backwards because we will be slicing the code string
	sort.SliceStable(importDeclarations, func(i, j int) bool {
		return importDeclarations[i].StartPosition().Offset > importDeclarations[j].StartPosition().Offset
	})

	imports := ""
	for _, d := range importDeclarations {
		importStart := d.StartPosition().Offset
		importEnd := d.EndPosition().Offset + 1
		// aggregate imports for later use
		imports = imports + code[importStart:importEnd] + "\n"
		// remove imports from the code
		code = code[:importStart] + code[importEnd:]
	}

	// parse the code again to get accurate locations of the remaining declarations
	program, err = parser2.ParseProgram(code)
	if err != nil {
		log.Error().
			Err(err).
			Str("address", address.Hex()).
			Str("code", code).
			Msg("Cannot parse program at address after removing declarations")
		return nil, err
	}

	declarations = program.Declarations()
	switch len(declarations) {
	case 0:
		// If there is no declarations. Only comments? was this legal before?
		// error just in case
		// alternative would be removing the register
		log.Error().
			Str("address", address.Hex()).
			Str("code", code).
			Msg("No declarations at address")
		return nil, fmt.Errorf("no declarations at address %s", address.Hex())
	case 1:
		// If there is one declaration move it to the new key
		log.Debug().
			Str("address", address.Hex()).
			Msg("Single contract or interface at address moved to new key")
		oldKey := p.Key
		p.Key = addNameToKey(p.Key, declarations[0].DeclarationIdentifier().Identifier)
		contractsRegister, err := contractsRegister(p.Key, []string{declarations[0].DeclarationIdentifier().Identifier})
		if err != nil {
			return nil, err
		}
		logKeyChange(address.Hex(), oldKey, p.Key, contractsRegister.Key)
		return []ledger.Payload{p, contractsRegister}, nil
	case 2:
		// We have two declarations. Due to the current rules one of them is an interface and one is a contract.
		// the contract will need an import to the interface.
		log.Info().
			Str("address", address.Hex()).
			Msg("Two declarations on an address, splitting into two parts")

		_, oneIsInterface := declarations[0].(*ast.InterfaceDeclaration)
		_, twoIsInterface := declarations[1].(*ast.InterfaceDeclaration)
		if oneIsInterface == twoIsInterface {
			// declarations of same type! should not happen
			log.Error().
				Str("address", address.Hex()).
				Str("code", code).
				Msg("Two declarations of the same type at address")
			return []ledger.Payload{}, fmt.Errorf("two declarations of the same type at address %s", address.Hex())
		}

		var interfaceDeclaration ast.Declaration
		var contractDeclaration ast.Declaration
		if oneIsInterface {
			interfaceDeclaration = declarations[0]
			contractDeclaration = declarations[1]
		} else {
			interfaceDeclaration = declarations[1]
			contractDeclaration = declarations[0]
		}

		interfaceStart := interfaceDeclaration.StartPosition().Offset
		interfaceEnd := interfaceDeclaration.EndPosition().Offset
		contractStart := contractDeclaration.StartPosition().Offset
		contractEnd := contractDeclaration.EndPosition().Offset

		var contractCode string
		var interfaceCode string

		if contractStart < interfaceStart {
			split := contractEnd + 1
			contractCode = code[:split]
			interfaceCode = code[split:]
		} else {
			split := interfaceEnd + 1
			contractCode = code[split:]
			interfaceCode = code[:split]
		}

		// add original imports and interface import to contract code
		contractCode = fmt.Sprintf("%simport %s from %s\n%s", imports, interfaceDeclaration.DeclarationIdentifier().Identifier, address.HexWithPrefix(), contractCode)
		// add original imports to interface code
		interfaceCode = imports + interfaceCode

		interfaceKey := addNameToKey(p.Key, interfaceDeclaration.DeclarationIdentifier().Identifier)
		contractKey := addNameToKey(p.Key, contractDeclaration.DeclarationIdentifier().Identifier)

		contractsRegister, err := contractsRegister(p.Key, []string{declarations[0].DeclarationIdentifier().Identifier, declarations[0].DeclarationIdentifier().Identifier})
		if err != nil {
			return nil, err
		}
		logKeyChange(address.Hex(), p.Key, interfaceKey, contractKey, contractsRegister.Key)
		return []ledger.Payload{
			{
				Key:   interfaceKey,
				Value: []byte(interfaceCode),
			}, {
				Key:   contractKey,
				Value: []byte(contractCode),
			},
			contractsRegister,
		}, nil
	default:
		log.Error().
			Str("address", address.Hex()).
			Str("code", code).
			Msg("More than two declarations at address")
		return []ledger.Payload{}, fmt.Errorf("more than two declarations at address %s", address.Hex())
	}
}

func isAddress(id flow.RegisterID) bool {
	return len([]byte(id.Owner)) == flow.AddressLength
}

func addNameToKey(key ledger.Key, name string) ledger.Key {
	return changeKey(key, fmt.Sprintf("code.%s", name))
}

func changeKey(key ledger.Key, value string) ledger.Key {
	newKey := key
	newKey.KeyParts = make([]ledger.KeyPart, 3)
	copy(newKey.KeyParts, key.KeyParts)
	newKey.KeyParts[2].Value = []byte(value)
	return newKey
}

func logKeyChange(address string, original ledger.Key, changed ...ledger.Key) {
	arr := zerolog.Arr()
	for i := range changed {
		arr.Str(string(changed[i].KeyParts[2].Value))
	}
	log.Info().
		Str("original", string(original.KeyParts[2].Value)).
		Str("address", address).
		Array("new", arr).
		Msg("migrated key")
}
