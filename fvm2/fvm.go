package fvm

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/model/flow"
)

type StorageKey struct {
	owner      string
	controller string
	key        string
}

type StorageValue []byte

type Storage interface {
	Set(StorageKey, StorageValue) error
	Get(StorageKey) (StorageValue, error)
	Touch(StorageKey) error
	Delete(StorageKey) error
}

// flow.Tx probably should be part of this package and not the models

type VirtualMachine interface {

	// validates and verifies a transaction (signature, sequence number validation), construct a new result handler, runs the transaction using runner,
	// collect fees and updates sequence number
	ExecuteTransaction(tx *flow.TransactionBody) Res

	// construct a new runnable with restricted permissions and limied result handler, runs the script
	ExecuteScript()

	// constructs a new runnable with full permissions and runs the script through the runnable
	ExecuteSystem()
}

type FlowVirtualMachine struct {
	Runtime runtime.Runtime
}

func NewFlowVirtualMachine(rt runtime.Runtime, context FVMContext) *FlowVirtualMachine {
	// Logger:  logger,
	return &FlowVirtualMachine{Runtime: rt}
}

type Context interface {
	AccountHandler()
	MetricHandler()
	CryptoHandler()
	ProtocolHandler()
	RandHandler()
	ResultHandler() // provides reset and export result functionality
}

// maybe to include tx validator as input or just part of the logic in the fvm
// storge -> accountHandler and all other stateful handlers ->
// constructs a new runner under the hood

// // this should be called for each collection
// ChangeContext(Context)
// context can include the storage

// TODO from here figure out the FVM Context  (can it be shared between emulator and others)

// ProtocolHandler

// Chain:                            flow.Mainnet.Chain(),
// GasLimit:                         DefaultGasLimit,
// MaxStateKeySize:                  state.DefaultMaxKeySize,
// MaxStateValueSize:                state.DefaultMaxValueSize,
// MaxStateInteractionSize:          state.DefaultMaxInteractionSize,
// EventCollectionByteSizeLimit:     DefaultEventCollectionByteSizeLimit,
// ServiceAccountEnabled:            true,
// RestrictedAccountCreationEnabled: true,
// RestrictedDeploymentEnabled:      true,
// CadenceLoggingEnabled:            false,

// Result includes all the artifacts generated as an outcome of running a runnable
type Res struct {
	Logs                   []string
	Events                 []cadence.Event
	ComputationSpent       uint64
	LedgerInteractionSpent uint64
	NewAccounts            []cadence.Address
}
