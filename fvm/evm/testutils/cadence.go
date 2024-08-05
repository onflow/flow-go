package testutils

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/stretchr/testify/require"
)

// LocationResolver is a location Cadence runtime interface location resolver
// very similar to ContractReader.ResolveLocation,
// but it does not look up available contract names
func LocationResolver(
	identifiers []ast.Identifier,
	location common.Location,
) (
	result []sema.ResolvedLocation,
	err error,
) {
	addressLocation, isAddress := location.(common.AddressLocation)

	// if the location is not an address location, e.g. an identifier location
	// (`import Crypto`), then return a single resolved location which declares
	// all identifiers.
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
	// then assume the imported identifier is the address location's identifier (the contract)
	if len(identifiers) == 0 {
		identifiers = []ast.Identifier{
			{Identifier: addressLocation.Name},
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

func EncodeArgs(argValues []cadence.Value) [][]byte {
	args := make([][]byte, len(argValues))
	for i, arg := range argValues {
		var err error
		args[i], err = json.Encode(arg)
		if err != nil {
			panic(fmt.Errorf("broken test: invalid argument: %w", err))
		}
	}
	return args
}

func CheckCadenceEventTypes(t testing.TB, events []cadence.Event, expectedTypes []string) {
	require.Equal(t, len(events), len(expectedTypes))
	for i, ev := range events {
		require.Equal(t, expectedTypes[i], ev.EventType.QualifiedIdentifier)
	}
}
