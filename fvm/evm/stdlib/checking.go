package stdlib

import (
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/environment"
)

// checkingInterface is a runtime.Interface implementation
// that can be used for ParseAndCheckProgram.
// It is not suitable for execution.
type checkingInterface struct {
	runtime.EmptyRuntimeInterface
	SystemContractCodes   map[common.Location][]byte
	Programs              map[runtime.Location]*interpreter.Program
	cryptoContractAddress common.Address
}

var _ runtime.Interface = &checkingInterface{}

func (i *checkingInterface) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) (
	[]runtime.ResolvedLocation,
	error,
) {
	return environment.ResolveLocation(
		identifiers,
		location,
		nil,
		i.cryptoContractAddress,
	)
}

func (i *checkingInterface) GetOrLoadProgram(
	location runtime.Location,
	load func() (*interpreter.Program, error),
) (
	program *interpreter.Program,
	err error,
) {
	if i.Programs == nil {
		i.Programs = map[runtime.Location]*interpreter.Program{}
	}

	var ok bool
	program, ok = i.Programs[location]
	if ok {
		return
	}

	program, err = load()

	// NOTE: important: still set empty program,
	// even if error occurred

	i.Programs[location] = program

	return
}

func (i *checkingInterface) GetCode(location common.Location) ([]byte, error) {
	return i.SystemContractCodes[location], nil
}

func (i *checkingInterface) GetAccountContractCode(location common.AddressLocation) (code []byte, err error) {
	return i.SystemContractCodes[location], nil
}
