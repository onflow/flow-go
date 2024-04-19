package load

import (
	_ "embed"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/integration/benchmark/account"
	"github.com/onflow/flow-go/integration/benchmark/common"
	"github.com/onflow/flow-go/integration/benchmark/scripts"
	"github.com/onflow/flow-go/model/flow"
)

type LoadType string

const (
	CompHeavyLoadType     LoadType = "computation-heavy"
	EventHeavyLoadType    LoadType = "event-heavy"
	ExecDataHeavyLoadType LoadType = "exec-data-heavy"
	LedgerHeavyLoadType   LoadType = "ledger-heavy"

	// TODO: port this load type from old code
	// ConstExecCostLoadType LoadType = "const-exec" // for an empty transactions with various tx arguments

	TokenTransferLoadType LoadType = "token-transfer"
	AddKeysLoadType       LoadType = "add-keys"
	EVMTransferLoadType   LoadType = "evm-transfer"
	CreateAccount         LoadType = "create-account"
)

type LoadContext struct {
	ChainID flow.ChainID
	WorkerContext
	account.AccountProvider
	common.TransactionSender
	common.ReferenceBlockProvider
	Proposer *account.FlowAccount
}

type WorkerContext struct {
	WorkerID int
}

type Load interface {
	Type() LoadType
	// Setup is called once before the load starts.
	Setup(log zerolog.Logger, lc LoadContext) error
	// Load is called repeatedly from multiple goroutines.
	Load(log zerolog.Logger, lc LoadContext) error
}

var CompHeavyLoad = NewSimpleLoadType(
	CompHeavyLoadType,
	"ComputationHeavy",
	scripts.ComputationHeavyContractTemplate,
	scripts.ComputationHeavyScriptTemplate)

var EventHeavyLoad = NewSimpleLoadType(
	EventHeavyLoadType,
	"EventHeavy",
	scripts.EventHeavyContractTemplate,
	scripts.EventHeavyScriptTemplate)

var LedgerHeavyLoad = NewSimpleLoadType(
	LedgerHeavyLoadType,
	"LedgerHeavy",
	scripts.LedgerHeavyContractTemplate,
	scripts.LedgerHeavyScriptTemplate)

var ExecDataHeavyLoad = NewSimpleLoadType(
	ExecDataHeavyLoadType,
	"DataHeavy",
	scripts.DataHeavyContractTemplate,
	scripts.DataHeavyScriptTemplate)

func CreateLoadType(log zerolog.Logger, t LoadType) Load {
	switch t {
	case CompHeavyLoadType:
		return CompHeavyLoad
	case EventHeavyLoadType:
		return EventHeavyLoad
	case LedgerHeavyLoadType:
		return LedgerHeavyLoad
	case ExecDataHeavyLoadType:
		return ExecDataHeavyLoad
	case TokenTransferLoadType:
		return NewTokenTransferLoad()
	case AddKeysLoadType:
		return NewAddKeysLoad()
	case EVMTransferLoadType:
		return NewEVMTransferLoad(log)
	case CreateAccount:
		return NewCreateAccountLoad()
	default:
		panic("unknown load type")
	}
}
