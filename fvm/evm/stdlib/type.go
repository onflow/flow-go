package stdlib

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func newContractType(chainID flow.ChainID) *sema.CompositeType {

	contracts := systemcontracts.SystemContractsForChain(chainID)

	evmCode := ContractCode(
		contracts.NonFungibleToken.Address,
		contracts.FungibleToken.Address,
		contracts.FlowToken.Address,
	)

	evmContractAddress := contracts.EVMContract.Address

	evmContractLocation := common.AddressLocation{
		Address: common.Address(evmContractAddress),
		Name:    ContractName,
	}

	templatesEnv := contracts.AsTemplateEnv()

	runtimeInterface := &checkingInterface{
		SystemContractCodes: map[common.AddressLocation][]byte{
			contracts.ViewResolver.Location():               coreContracts.ViewResolver(),
			contracts.Burner.Location():                     coreContracts.Burner(),
			contracts.FungibleToken.Location():              coreContracts.FungibleToken(templatesEnv),
			contracts.NonFungibleToken.Location():           coreContracts.NonFungibleToken(templatesEnv),
			contracts.MetadataViews.Location():              coreContracts.MetadataViews(templatesEnv),
			contracts.FlowToken.Location():                  coreContracts.FlowToken(templatesEnv),
			contracts.FungibleTokenMetadataViews.Location(): coreContracts.FungibleTokenMetadataViews(templatesEnv),
		},
	}

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{})
	env.Configure(
		runtimeInterface,
		runtime.NewCodesAndPrograms(),
		nil,
		nil,
	)

	SetupEnvironment(env, nil, evmContractAddress)

	program, err := env.ParseAndCheckProgram(evmCode, evmContractLocation, false)
	if err != nil {
		panic(err)
	}

	evmContractTypeID := evmContractLocation.TypeID(nil, ContractName)

	return program.Elaboration.CompositeType(evmContractTypeID)
}

var contractTypes = map[flow.ChainID]*sema.CompositeType{}

type CadenceTypes struct {
	TransactionExecuted *cadence.EventType
	BlockExecuted       *cadence.EventType
}

var cadenceTypes = map[flow.ChainID]CadenceTypes{}

func exportCadenceEventType(contractType *sema.CompositeType, name string) (*cadence.EventType, error) {
	transactionEventType, ok := contractType.GetNestedTypes().Get(name)
	if !ok {
		return nil, fmt.Errorf("missing %s type", name)
	}
	exportedType := runtime.ExportType(
		transactionEventType,
		map[sema.TypeID]cadence.Type{},
	)

	eventType, ok := exportedType.(*cadence.EventType)
	if !ok {
		return nil, fmt.Errorf("type %s is not an event", name)
	}

	return eventType, nil
}

func init() {
	for _, chain := range flow.AllChainIDs() {
		contractType := newContractType(chain)
		contractTypes[chain] = contractType

		transactionExecutedEvent, err := exportCadenceEventType(contractType, "TransactionExecuted")
		if err != nil {
			panic(err)
		}

		blockExecutedEvent, err := exportCadenceEventType(contractType, "BlockExecuted")
		if err != nil {
			panic(err)
		}

		cadenceTypes[chain] = CadenceTypes{
			TransactionExecuted: transactionExecutedEvent,
			BlockExecuted:       blockExecutedEvent,
		}
	}
}

func ContractTypeForChain(chainID flow.ChainID) *sema.CompositeType {
	contractType, ok := contractTypes[chainID]
	if !ok {
		// this is a panic, since it can only happen if the code is wrong
		panic(fmt.Sprintf("unknown chain: %s", chainID))
	}
	return contractType
}

func CadenceTypesForChain(chainID flow.ChainID) CadenceTypes {
	cadenceTypes, ok := cadenceTypes[chainID]
	if !ok {
		// this is a panic, since it can only happen if the code is wrong
		panic(fmt.Sprintf("unknown chain: %s", chainID))
	}
	return cadenceTypes
}
