package runtime

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/backends"
	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/impl"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

type ReusableCadenceRuntime struct {
	runtime.Runtime

	chain flow.Chain

	TxRuntimeEnv     runtime.Environment
	ScriptRuntimeEnv runtime.Environment

	fvmEnv     environment.Environment
	evmBackend *backends.WrappedEnvironment
}

var _ environment.ReusableCadenceRuntime = (*ReusableCadenceRuntime)(nil)

func NewReusableCadenceRuntime(
	rt runtime.Runtime,
	chain flow.Chain,
	config runtime.Config,
) *ReusableCadenceRuntime {
	reusable := &ReusableCadenceRuntime{
		Runtime:          rt,
		chain:            chain,
		TxRuntimeEnv:     runtime.NewBaseInterpreterEnvironment(config),
		ScriptRuntimeEnv: runtime.NewScriptInterpreterEnvironment(config),
	}

	reusable.declareStandardLibraryFunctions()

	return reusable
}

func (reusable *ReusableCadenceRuntime) declareStandardLibraryFunctions() {
	// random source for transactions
	reusable.TxRuntimeEnv.DeclareValue(blockRandomSourceDeclaration(reusable), nil)

	// transaction index
	declaration := transactionIndexDeclaration(reusable)
	reusable.TxRuntimeEnv.DeclareValue(declaration, nil)
	reusable.ScriptRuntimeEnv.DeclareValue(declaration, nil)

	reusable.declareEVM()
}

func (reusable *ReusableCadenceRuntime) declareEVM() {
	chainID := reusable.chain.ChainID()
	sc := systemcontracts.SystemContractsForChain(chainID)
	randomBeaconAddress := sc.RandomBeaconHistory.Address
	flowTokenAddress := sc.FlowToken.Address

	reusable.evmBackend = backends.NewWrappedEnvironment(reusable.fvmEnv)
	evmEmulator := emulator.NewEmulator(reusable.evmBackend, evm.StorageAccountAddress(chainID))
	blockStore := handler.NewBlockStore(chainID, reusable.evmBackend, evm.StorageAccountAddress(chainID))
	addressAllocator := handler.NewAddressAllocator()

	evmContractAddress := evm.ContractAccountAddress(chainID)

	contractHandler := handler.NewContractHandler(
		chainID,
		evmContractAddress,
		common.Address(flowTokenAddress),
		randomBeaconAddress,
		blockStore,
		addressAllocator,
		reusable.evmBackend,
		evmEmulator,
	)

	internalEVMContractValue := impl.NewInternalEVMContractValue(
		nil,
		contractHandler,
		evmContractAddress,
	)

	stdlib.SetupEnvironment(
		reusable.TxRuntimeEnv,
		internalEVMContractValue,
		evmContractAddress,
	)

	stdlib.SetupEnvironment(
		reusable.ScriptRuntimeEnv,
		internalEVMContractValue,
		evmContractAddress,
	)
}

func (reusable *ReusableCadenceRuntime) SetFvmEnvironment(fvmEnv environment.Environment) {
	reusable.fvmEnv = fvmEnv
	reusable.evmBackend.SetEnv(fvmEnv)
}

func (reusable *ReusableCadenceRuntime) CadenceTXEnv() runtime.Environment {
	return reusable.TxRuntimeEnv
}

func (reusable *ReusableCadenceRuntime) CadenceScriptEnv() runtime.Environment {
	return reusable.ScriptRuntimeEnv
}

func (reusable *ReusableCadenceRuntime) ReadStored(
	address common.Address,
	path cadence.Path,
) (
	cadence.Value,
	error,
) {
	return reusable.Runtime.ReadStored(
		address,
		path,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Environment:      reusable.TxRuntimeEnv,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

func (reusable *ReusableCadenceRuntime) InvokeContractFunction(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
) (
	cadence.Value,
	error,
) {
	return reusable.Runtime.InvokeContractFunction(
		contractLocation,
		functionName,
		arguments,
		argumentTypes,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Environment:      reusable.TxRuntimeEnv,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

func (reusable *ReusableCadenceRuntime) NewTransactionExecutor(
	script runtime.Script,
	location common.Location,
) runtime.Executor {
	return reusable.Runtime.NewTransactionExecutor(
		script,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Location:         location,
			Environment:      reusable.TxRuntimeEnv,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

func (reusable *ReusableCadenceRuntime) ExecuteScript(
	script runtime.Script,
	location common.Location,
) (
	cadence.Value,
	error,
) {
	return reusable.Runtime.ExecuteScript(
		script,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Location:         location,
			Environment:      reusable.ScriptRuntimeEnv,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}
