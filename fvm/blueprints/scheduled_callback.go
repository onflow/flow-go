package blueprints

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/rs/zerolog/log"

	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

const callbackTransactionGasLimit = flow.DefaultMaxTransactionGasLimit

// SystemCollection returns the re-created system collection after it has been already executed
// using the events from the process callback transaction.
func SystemCollection(chain flow.Chain, processEvents flow.EventsList) (*flow.Collection, error) {
	process, err := ProcessCallbacksTransaction(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to construct process callbacks transaction: %w", err)
	}

	executes, err := ExecuteCallbacksTransactions(chain, processEvents)
	if err != nil {
		return nil, fmt.Errorf("failed to construct execute callbacks transactions: %w", err)
	}

	systemTx, err := SystemChunkTransaction(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to construct system chunk transaction: %w", err)
	}

	transactions := make([]*flow.TransactionBody, 0, len(executes)+2) // +2 process and system tx
	transactions = append(transactions, process)
	transactions = append(transactions, executes...)
	transactions = append(transactions, systemTx)

	collection, err := flow.NewCollection(flow.UntrustedCollection{
		Transactions: transactions,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to construct system collection: %w", err)
	}

	return collection, nil
}

// ProcessCallbacksTransaction constructs a transaction for processing callbacks, for the given callback.
// No errors are expected during normal operation.
func ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	script := templates.GenerateProcessTransactionScript(sc.AsTemplateEnv())

	return flow.NewTransactionBodyBuilder().
		AddAuthorizer(sc.FlowServiceAccount.Address).
		SetScript(script).
		SetComputeLimit(callbackTransactionGasLimit).Build()
}

// ExecuteCallbacksTransactions constructs a list of transaction to execute callbacks, for the given chain.
// No errors are expected during normal operation.
func ExecuteCallbacksTransactions(chainID flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	txs := make([]*flow.TransactionBody, 0, len(processEvents))
	env := systemcontracts.SystemContractsForChain(chainID.ChainID()).AsTemplateEnv()
	sc := systemcontracts.SystemContractsForChain(chainID.ChainID())

	for _, event := range processEvents {
		// todo make sure to check event index to ensure order is indeed correct
		// event.EventIndex

		// skip any fee events or other events that are not pending execution events
		if !IsPendingExecutionEvent(env, event) {
			continue
		}

		id, effort, err := callbackArgsFromEvent(event)
		if err != nil {
			return nil, fmt.Errorf("failed to get callback args from event: %w", err)
		}

		tx, err := executeCallbackTransaction(sc, env, id, effort)
		if err != nil {
			return nil, fmt.Errorf("failed to construct execute callback transactions: %w", err)
		}
		txs = append(txs, tx)
	}

	return txs, nil
}

func executeCallbackTransaction(
	sc *systemcontracts.SystemContracts,
	env templates.Environment,
	id []byte,
	effort uint64,
) (*flow.TransactionBody, error) {
	script := templates.GenerateExecuteTransactionScript(env)

	return flow.NewTransactionBodyBuilder().
		AddAuthorizer(sc.ScheduledTransactionExecutor.Address).
		SetScript(script).
		AddArgument(id).
		SetComputeLimit(effort).
		Build()
}

// callbackArgsFromEvent decodes the event payload and returns the callback ID and effort.
//
// The event for processed callback event is emitted by the process callback transaction from
// callback scheduler contract and has the following signature:
// event PendingExecution(id: UInt64, priority: UInt8, executionEffort: UInt64, fees: UFix64, callbackOwner: Address)
func callbackArgsFromEvent(event flow.Event) ([]byte, uint64, error) {
	cadenceId, cadenceEffort, err := ParsePendingExecutionEvent(event)
	if err != nil {
		return nil, 0, err
	}

	effort := uint64(cadenceEffort)

	if effort > flow.DefaultMaxTransactionGasLimit {
		log.Warn().Uint64("effort", effort).Msg("effort is greater than max transaction gas limit, setting to max")
		effort = flow.DefaultMaxTransactionGasLimit
	}

	encID, err := jsoncdc.Encode(cadenceId)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to encode id: %w", err)
	}

	return encID, uint64(effort), nil
}

// ParsePendingExecutionEvent decodes the PendingExecution event payload and returns the scheduled
// transaction's id and effort.
func ParsePendingExecutionEvent(event flow.Event) (cadence.UInt64, cadence.UInt64, error) {
	const (
		processedCallbackIDFieldName     = "id"
		processedCallbackEffortFieldName = "executionEffort"
	)

	eventData, err := ccf.Decode(nil, event.Payload)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to decode event: %w", err)
	}

	cadenceEvent, ok := eventData.(cadence.Event)
	if !ok {
		return 0, 0, fmt.Errorf("event data is not a cadence event")
	}

	idValue := cadence.SearchFieldByName(
		cadenceEvent,
		processedCallbackIDFieldName,
	)

	effortValue := cadence.SearchFieldByName(
		cadenceEvent,
		processedCallbackEffortFieldName,
	)

	cadenceId, ok := idValue.(cadence.UInt64)
	if !ok {
		return 0, 0, fmt.Errorf("id is not uint64")
	}

	cadenceEffort, ok := effortValue.(cadence.UInt64)
	if !ok {
		return 0, 0, fmt.Errorf("effort is not uint64")
	}

	return cadenceId, cadenceEffort, nil
}

// IsPendingExecutionEvent returns true if the event is a pending execution event.
func IsPendingExecutionEvent(env templates.Environment, event flow.Event) bool {
	processedEventType := PendingExecutionEventType(env)
	return event.Type == processedEventType
}

// PendingExecutionEventType returns the event type for FlowCallbackScheduler PendingExecution event
// for the provided environment.
func PendingExecutionEventType(env templates.Environment) flow.EventType {
	const processedEventTypeTemplate = "A.%v.FlowTransactionScheduler.PendingExecution"

	scheduledContractAddress := env.FlowTransactionSchedulerAddress
	return flow.EventType(fmt.Sprintf(processedEventTypeTemplate, scheduledContractAddress))
}
