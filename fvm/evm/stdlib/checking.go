package stdlib

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
)

// checkingInterface is a runtime.Interface implementation
// that can be used for ParseAndCheckProgram.
// It is not suitable for execution.
type checkingInterface struct {
	runtime.EmptyRuntimeInterface
	SystemContractCodes map[common.AddressLocation][]byte
	Programs            map[runtime.Location]*interpreter.Program
}

var _ runtime.Interface = &checkingInterface{}

func (*checkingInterface) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) (
	[]runtime.ResolvedLocation,
	error,
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

	if len(identifiers) == 0 {
		return nil, fmt.Errorf("no identifiers provided")
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

func (r *checkingInterface) GetOrLoadProgram(
	location runtime.Location,
	load func() (*interpreter.Program, error),
) (
	program *interpreter.Program,
	err error,
) {
	if r.Programs == nil {
		r.Programs = map[runtime.Location]*interpreter.Program{}
	}

	var ok bool
	program, ok = r.Programs[location]
	if ok {
		return
	}

	program, err = load()

	// NOTE: important: still set empty program,
	// even if error occurred

	r.Programs[location] = program

	return
}

func (r *checkingInterface) GetAccountContractCode(location common.AddressLocation) (code []byte, err error) {
	return r.SystemContractCodes[location], nil
}
