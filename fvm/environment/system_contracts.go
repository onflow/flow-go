package environment

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// ContractFunctionSpec specify all the information, except the function's
// address and arguments, needed to invoke the contract function.
type ContractFunctionSpec struct {
	AddressFromChain func(flow.Chain) flow.Address
	LocationName     string
	FunctionName     string
	ArgumentTypes    []sema.Type
}

// SystemContracts provides methods for invoking system contract functions as
// service account.
type SystemContracts struct {
	chain flow.Chain

	tracer  tracing.TracerSpan
	logger  *ProgramLogger
	runtime *Runtime
}

func NewSystemContracts(
	chain flow.Chain,
	tracer tracing.TracerSpan,
	logger *ProgramLogger,
	runtime *Runtime,
) *SystemContracts {
	return &SystemContracts{
		chain:   chain,
		tracer:  tracer,
		logger:  logger,
		runtime: runtime,
	}
}

func (sys *SystemContracts) Invoke(
	spec ContractFunctionSpec,
	arguments []cadence.Value,
) (
	cadence.Value,
	error,
) {
	contractLocation := common.AddressLocation{
		Address: common.MustBytesToAddress(
			spec.AddressFromChain(sys.chain).Bytes()),
		Name: spec.LocationName,
	}

	span := sys.tracer.StartChildSpan(trace.FVMInvokeContractFunction)
	span.SetAttributes(
		attribute.String(
			"transaction.ContractFunctionCall",
			contractLocation.String()+"."+spec.FunctionName))
	defer span.End()

	runtime := sys.runtime.BorrowCadenceRuntime()
	defer sys.runtime.ReturnCadenceRuntime(runtime)

	value, err := runtime.InvokeContractFunction(
		contractLocation,
		spec.FunctionName,
		arguments,
		spec.ArgumentTypes,
	)
	if err != nil {
		log := sys.logger.Logger()
		log.Info().
			Err(err).
			Str("contract", contractLocation.String()).
			Str("function", spec.FunctionName).
			Msg("Contract function call executed with error")
	}
	return value, err
}

func FlowFeesAddress(chain flow.Chain) flow.Address {
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	return sc.FlowFees.Address
}

func ServiceAddress(chain flow.Chain) flow.Address {
	return chain.ServiceAddress()
}

var verifyPayersBalanceForTransactionExecutionSpec = ContractFunctionSpec{
	AddressFromChain: FlowFeesAddress,
	LocationName:     systemcontracts.ContractNameFlowFees,
	FunctionName:     systemcontracts.ContractServiceAccountFunction_verifyPayersBalanceForTransactionExecution,
	ArgumentTypes: []sema.Type{
		sema.NewReferenceType(
			nil,
			sema.NewEntitlementSetAccess(
				[]*sema.EntitlementType{
					sema.BorrowValueType,
				},
				sema.Conjunction,
			),
			sema.AccountType,
		),
		sema.UInt64Type,
		sema.UInt64Type,
	},
}

// CheckPayerBalanceAndGetMaxTxFees executes the verifyPayersBalanceForTransactionExecution
// on the FlowFees account.
// It checks whether the given payer has enough balance to cover inclusion fee and max execution
// fee.
// It returns (maxTransactionFee, ErrCodeInsufficientPayerBalance) if the payer doesn't have enough balance
// It returns (maxTransactionFee, nil) if the payer has enough balance
func (sys *SystemContracts) CheckPayerBalanceAndGetMaxTxFees(
	payer flow.Address,
	inclusionEffort uint64,
	maxExecutionEffort uint64,
) (cadence.Value, error) {
	return sys.Invoke(
		verifyPayersBalanceForTransactionExecutionSpec,
		[]cadence.Value{
			cadence.BytesToAddress(payer.Bytes()),
			cadence.UFix64(inclusionEffort),
			cadence.UFix64(maxExecutionEffort),
		},
	)
}

var deductTransactionFeeSpec = ContractFunctionSpec{
	AddressFromChain: FlowFeesAddress,
	LocationName:     systemcontracts.ContractNameFlowFees,
	FunctionName:     systemcontracts.ContractServiceAccountFunction_deductTransactionFee,
	ArgumentTypes: []sema.Type{
		sema.NewReferenceType(
			nil,
			sema.NewEntitlementSetAccess(
				[]*sema.EntitlementType{
					sema.BorrowValueType,
				},
				sema.Conjunction,
			),
			sema.AccountType,
		),
		sema.UInt64Type,
		sema.UInt64Type,
	},
}

// DeductTransactionFees executes the fee deduction function
// on the FlowFees account.
func (sys *SystemContracts) DeductTransactionFees(
	payer flow.Address,
	inclusionEffort uint64,
	executionEffort uint64,
) (cadence.Value, error) {
	return sys.Invoke(
		deductTransactionFeeSpec,
		[]cadence.Value{
			cadence.BytesToAddress(payer.Bytes()),
			cadence.UFix64(inclusionEffort),
			cadence.UFix64(executionEffort),
		},
	)
}

// uses `FlowServiceAccount.setupNewAccount` from https://github.com/onflow/flow-core-contracts/blob/master/contracts/FlowServiceAccount.cdc
var setupNewAccountSpec = ContractFunctionSpec{
	AddressFromChain: ServiceAddress,
	LocationName:     systemcontracts.ContractNameServiceAccount,
	FunctionName:     systemcontracts.ContractServiceAccountFunction_setupNewAccount,
	ArgumentTypes: []sema.Type{
		sema.NewReferenceType(
			nil,
			sema.NewEntitlementSetAccess(
				[]*sema.EntitlementType{
					sema.SaveValueType,
					sema.BorrowValueType,
					sema.CapabilitiesType,
				},
				sema.Conjunction,
			),
			sema.AccountType,
		),
		sema.NewReferenceType(
			nil,
			sema.NewEntitlementSetAccess(
				[]*sema.EntitlementType{
					sema.BorrowValueType,
				},
				sema.Conjunction,
			),
			sema.AccountType,
		),
	},
}

// SetupNewAccount executes the new account setup contract on the service
// account.
func (sys *SystemContracts) SetupNewAccount(
	flowAddress flow.Address,
	payer flow.Address,
) (cadence.Value, error) {
	return sys.Invoke(
		setupNewAccountSpec,
		[]cadence.Value{
			cadence.BytesToAddress(flowAddress.Bytes()),
			cadence.BytesToAddress(payer.Bytes()),
		},
	)
}

var accountAvailableBalanceSpec = ContractFunctionSpec{
	AddressFromChain: ServiceAddress,
	LocationName:     systemcontracts.ContractNameStorageFees,
	FunctionName:     systemcontracts.ContractStorageFeesFunction_defaultTokenAvailableBalance,
	ArgumentTypes: []sema.Type{
		&sema.AddressType{},
	},
}

// AccountAvailableBalance executes the get available balance contract on the
// storage fees contract.
func (sys *SystemContracts) AccountAvailableBalance(
	address flow.Address,
) (cadence.Value, error) {
	return sys.Invoke(
		accountAvailableBalanceSpec,
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
		},
	)
}

var accountBalanceInvocationSpec = ContractFunctionSpec{
	AddressFromChain: ServiceAddress,
	LocationName:     systemcontracts.ContractNameServiceAccount,
	FunctionName:     systemcontracts.ContractServiceAccountFunction_defaultTokenBalance,
	ArgumentTypes: []sema.Type{
		sema.NewReferenceType(
			nil,
			sema.UnauthorizedAccess,
			sema.AccountType,
		),
	},
}

// AccountBalance executes the get available balance contract on the service
// account.
func (sys *SystemContracts) AccountBalance(
	address flow.Address,
) (cadence.Value, error) {
	return sys.Invoke(
		accountBalanceInvocationSpec,
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
		},
	)
}

var accountStorageCapacitySpec = ContractFunctionSpec{
	AddressFromChain: ServiceAddress,
	LocationName:     systemcontracts.ContractNameStorageFees,
	FunctionName:     systemcontracts.ContractStorageFeesFunction_calculateAccountCapacity,
	ArgumentTypes: []sema.Type{
		&sema.AddressType{},
	},
}

// AccountStorageCapacity executes the get storage capacity contract on the
// service account.
func (sys *SystemContracts) AccountStorageCapacity(
	address flow.Address,
) (cadence.Value, error) {
	return sys.Invoke(
		accountStorageCapacitySpec,
		[]cadence.Value{
			cadence.BytesToAddress(address.Bytes()),
		},
	)
}

// AccountsStorageCapacity gets storage capacity for multiple accounts at once.
func (sys *SystemContracts) AccountsStorageCapacity(
	addresses []flow.Address,
	payer flow.Address,
	maxTxFees uint64,
) (cadence.Value, error) {
	arrayValues := make([]cadence.Value, len(addresses))
	for i, address := range addresses {
		arrayValues[i] = cadence.BytesToAddress(address.Bytes())
	}

	return sys.Invoke(
		ContractFunctionSpec{
			AddressFromChain: ServiceAddress,
			LocationName:     systemcontracts.ContractNameStorageFees,
			FunctionName:     systemcontracts.ContractStorageFeesFunction_getAccountsCapacityForTransactionStorageCheck,
			ArgumentTypes: []sema.Type{
				sema.NewConstantSizedType(
					nil,
					&sema.AddressType{},
					int64(len(arrayValues)),
				),
				&sema.AddressType{},
				sema.UFix64Type,
			},
		},
		[]cadence.Value{
			cadence.NewArray(arrayValues),
			cadence.BytesToAddress(payer.Bytes()),
			cadence.UFix64(maxTxFees),
		},
	)
}
