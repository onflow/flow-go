package environment

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/flow-go/model/flow"
)

// ContractFunctionSpec specify all the information, except the function's
// address and arguments, needed to invoke the contract function.
type ContractFunctionSpec struct {
	AddressFromChain func(flow.Chain) flow.Address
	LocationName     string
	FunctionName     string
	ArgumentTypes    []sema.Type
}

// ContractFunctionInvoker invokes a contract function
type ContractFunctionInvoker interface {
	Invoke(
		spec ContractFunctionSpec,
		arguments []cadence.Value,
	) (
		cadence.Value,
		error,
	)
}
