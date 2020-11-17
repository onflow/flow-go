package migrations

import (
	"fmt"
	"sort"
	"strings"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/parser2"
	"github.com/rs/zerolog"

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
	MultipleContractsSpecialMigrations = make(map[string]func(ledger.Payload, zerolog.Logger) ([]ledger.Payload, error))
)

func MultipleContractMigration(payload []ledger.Payload) ([]ledger.Payload, error) {
	migratedPayloads := make([]ledger.Payload, 0)
	errors := make([]error, 0)
	// the cache keeps a record of already migrated addresses
	// it is completely ephemeral. If lost, it will only effect performance
	cache := map[string]struct{}{}
	logger := zerolog.Logger{}

	for _, p := range payload {
		results, err := migrateRegister(p, cache, logger)
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

func keyToRegisterId(key ledger.Key) (flow.RegisterID, error) {
	if len(key.KeyParts) != 3 ||
		key.KeyParts[0].Type != state.KeyPartOwner ||
		key.KeyParts[1].Type != state.KeyPartController ||
		key.KeyParts[2].Type != state.KeyPartKey {
		return flow.RegisterID{}, fmt.Errorf("key not in expected format %s", key.String())
	}

	return flow.NewRegisterID(string(key.KeyParts[0].Value), string(key.KeyParts[1].Value), string(key.KeyParts[2].Value)), nil
}

func migrateRegister(p ledger.Payload, cache map[string]struct{}, logger zerolog.Logger) ([]ledger.Payload, error) {
	registerId, err := keyToRegisterId(p.Key)
	if err != nil {
		return nil, err
	}
	// have we migrated this?
	if _, alreadyMigrated := cache[registerId.Owner]; alreadyMigrated {
		// yes, return it as is
		return []ledger.Payload{p}, nil
	}
	if !needsMigration(registerId) {
		return []ledger.Payload{p}, nil
	}

	// we are going to migrate this
	cache[registerId.Owner] = struct{}{}

	if em, hasEM := MultipleContractsSpecialMigrations[registerId.Owner]; hasEM {
		logger.Info().
			Err(err).
			Str("address", flow.BytesToAddress([]byte(registerId.Owner)).HexWithPrefix()).
			Msg("Using exceptional migration for address")
		return em(p, logger)
	}

	return migrateValue(p, logger)
}

func migrateValue(p ledger.Payload, l zerolog.Logger) ([]ledger.Payload, error) {

	// we don't need the the empty code register
	value := p.Value
	address := flow.BytesToAddress(p.Key.KeyParts[0].Value)
	if len(value) == 0 {
		return []ledger.Payload{}, nil
	}

	code := string(value)
	program, err := parser2.ParseProgram(code)
	if err != nil {
		l.Error().
			Err(err).
			Str("address", address.Hex()).
			Str("code", code).
			Msg("Cannot parse program at address")
		return nil, err
	}
	declarations := program.Declarations

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
		l.Error().
			Err(err).
			Str("address", address.Hex()).
			Str("code", code).
			Msg("Cannot parse program at address after removing declarations")
		return nil, err
	}
	declarations = program.Declarations

	switch len(declarations) {
	case 0:
		// If there is no declarations. Only comments? was this legal before?
		// error just in case
		// alternative would be removing the register
		l.Error().
			Str("address", address.Hex()).
			Str("code", code).
			Msg("No declarations at address")
		return []ledger.Payload{}, fmt.Errorf("no declarations at address %s", address.Hex())
	case 1:
		// If there is one declaration move it to the new key
		l.Debug().
			Str("address", address.Hex()).
			Msg("Single contract or interface at address moved to new key")
		p.Key = addNameToKey(p.Key, declarations[0].DeclarationIdentifier().Identifier)
		return []ledger.Payload{p}, nil
	case 2:
		// We have two declarations. Due to the current rules one of them is an interface and one is a contract.
		// the contract will need an import to the interface.
		l.Info().
			Str("address", address.Hex()).
			Msg("Two declarations on an address, splitting into two parts")

		_, oneIsInterface := declarations[0].(*ast.InterfaceDeclaration)
		_, twoIsInterface := declarations[1].(*ast.InterfaceDeclaration)
		if oneIsInterface == twoIsInterface {
			// declarations of same type! should not happen
			l.Error().
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
		return []ledger.Payload{
			{
				Key:   interfaceKey,
				Value: []byte(interfaceCode),
			}, {
				Key:   contractKey,
				Value: []byte(contractCode),
			},
		}, nil
	default:
		l.Error().
			Str("address", address.Hex()).
			Str("code", code).
			Msg("More than two declarations at address")
		return []ledger.Payload{}, fmt.Errorf("more than two declarations at address %s", address.Hex())
	}
}

// if it is a address
func needsMigration(id flow.RegisterID) bool {
	if id.Key != "code" {
		return false
	}
	return len([]byte(id.Owner)) == flow.AddressLength
}

func addNameToKey(key ledger.Key, name string) ledger.Key {
	newKey := key
	newKey.KeyParts = make([]ledger.KeyPart, 3)
	copy(newKey.KeyParts, key.KeyParts)
	newKeyString := fmt.Sprintf("code.%s", name)
	newKey.KeyParts[2].Value = []byte(newKeyString)
	return newKey
}
