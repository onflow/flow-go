package migrations

import (
	"fmt"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/parser2"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"strings"
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

type MultipleContractMigration struct {
	// the cache keeps a record of already migrated addresses
	// it is completely ephemeral. If lost, it will only effect performance
	cache                 map[string]struct{}
	logger                zerolog.Logger
	ExceptionalMigrations map[string]func(ledger.Payload, zerolog.Logger) ([]ledger.Payload, error)
}

func NewMultipleContractMigration(logger zerolog.Logger) MultipleContractMigration {
	return MultipleContractMigration{
		cache:                 make(map[string]struct{}, 0),
		logger:                logger,
		ExceptionalMigrations: make(map[string]func(ledger.Payload, zerolog.Logger) ([]ledger.Payload, error), 0),
	}
}

func (m *MultipleContractMigration) Migrate(payload []ledger.Payload) ([]ledger.Payload, error) {
	migratedPayloads := make([]ledger.Payload, 0)
	errors := make([]error, 0)

	for _, p := range payload {
		results, err := m.migrateRegister(p)
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

func (m *MultipleContractMigration) migrateRegister(p ledger.Payload) ([]ledger.Payload, error) {
	registerId, err := keyToRegisterId(p.Key)
	if err != nil {
		return nil, err
	}
	// have we migrated this?
	if _, alreadyMigrated := m.cache[registerId.Owner]; alreadyMigrated {
		// yes, return it as is
		return []ledger.Payload{p}, nil
	}
	if !needsMigration(registerId) {
		return []ledger.Payload{p}, nil
	}

	// we are going to migrate this
	m.cache[registerId.Owner] = struct{}{}

	if em, hasEM := m.ExceptionalMigrations[registerId.Owner]; hasEM {
		m.logger.Info().
			Err(err).
			Str("address", flow.BytesToAddress([]byte(registerId.Owner)).HexWithPrefix()).
			Msg("Using exceptional migration for address")
		return em(p, m.logger)
	}

	return m.migrateValue(p)
}

func (m *MultipleContractMigration) migrateValue(p ledger.Payload) ([]ledger.Payload, error) {

	// we don't need the the empty code register
	value := p.Value
	address := flow.BytesToAddress(p.Key.KeyParts[0].Value)
	if len(value) == 0 {
		return []ledger.Payload{}, nil
	}

	code := string(value)
	program, err := parser2.ParseProgram(code)
	if err != nil {
		m.logger.Error().
			Err(err).
			Str("address", address.Hex()).
			Str("code", code).
			Msg("Cannot parse program at address")
		return []ledger.Payload{}, err
	}

	if len(program.Declarations) == 0 {
		// If there is no declarations. Only comments? was this legal before?
		// error just in case
		// alternative would be removing the register
		m.logger.Error().
			Str("address", address.Hex()).
			Str("code", code).
			Msg("No declarations at address")
		return []ledger.Payload{}, fmt.Errorf("no declarations at address %s", address.Hex())
	}

	if len(program.Declarations) > 2 {
		m.logger.Error().
			Str("address", address.Hex()).
			Str("code", code).
			Msg("More than two declarations at address")
		return []ledger.Payload{}, fmt.Errorf("more than two declarations at address %s", address.Hex())
	}

	if len(program.Declarations) == 1 {
		// If there is one declaration move it to the new key
		m.logger.Debug().
			Str("address", address.Hex()).
			Msg("Single contract or interface at address moved to new key")
		p.Key = addNameToKey(p.Key, program.Declarations[0].DeclarationIdentifier().Identifier)
		return []ledger.Payload{p}, nil
	}

	// We have two declarations. Due to the current rules one of them is an interface and one is a contract.
	// the contract will need an import to the interface.
	m.logger.Info().
		Str("address", address.Hex()).
		Msg("Two declarations on an address, splitting into two parts")

	_, oneIsInterface := program.Declarations[0].(*ast.InterfaceDeclaration)
	_, twoIsInterface := program.Declarations[1].(*ast.InterfaceDeclaration)
	if oneIsInterface == twoIsInterface {
		// declarations of same type! should not happen
		m.logger.Error().
			Str("address", address.Hex()).
			Str("code", code).
			Msg("Two declarations of the same type at address")
		return []ledger.Payload{}, fmt.Errorf("wwo declarations of the same type at address %s", address.Hex())
	}

	var interfaceDeclaration ast.Declaration
	var contractDeclaration ast.Declaration
	if oneIsInterface {
		interfaceDeclaration = program.Declarations[0]
		contractDeclaration = program.Declarations[1]
	} else {
		interfaceDeclaration = program.Declarations[1]
		contractDeclaration = program.Declarations[0]
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

	// add import to contract code
	contractCode = fmt.Sprintf("import %s from %s\n%s", interfaceDeclaration.DeclarationIdentifier().Identifier, address.HexWithPrefix(), contractCode)

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
