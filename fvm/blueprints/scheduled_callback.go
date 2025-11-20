package blueprints

import (
	_ "embed"
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

//go:embed scripts/issueScheduledTransactionExecutorTemplate.cdc
var issueScheduledTransactionExecutorTemplate string

// ProcessCallbacksTransaction constructs a transaction for processing callbacks, for the given callback.
//
// No error returns are expected during normal operation.
func ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	script := templates.GenerateProcessTransactionScript(sc.AsTemplateEnv())

	return flow.NewTransactionBodyBuilder().
		AddAuthorizer(sc.FlowServiceAccount.Address).
		SetScript(script).
		SetComputeLimit(callbackTransactionGasLimit).Build()
}

// ExecuteCallbacksTransactions constructs a list of transaction to execute callbacks, for the given chain.
//
// No error returns are expected during normal operation.
func ExecuteCallbacksTransactions(chain flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	txs := make([]*flow.TransactionBody, 0, len(processEvents))
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	env := sc.AsTemplateEnv()
	script := templates.GenerateSchedulerExecutorTransactionScript(env)

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

		tx, err := flow.NewTransactionBodyBuilder().
			AddAuthorizer(sc.ScheduledTransactionExecutor.Address).
			SetScript(script).
			AddArgument(id).
			SetComputeLimit(effort).
			Build()
		if err != nil {
			return nil, fmt.Errorf("failed to construct execute callback transactions: %w", err)
		}
		txs = append(txs, tx)
	}

	return txs, nil
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

// IssueScheduledTransactionExecutorTransaction creates a transaction that issues the
// executeScheduledTransactionsCapability to an account will be used for authorizing
// the execution of scheduled transactions.
func IssueScheduledTransactionExecutorTransaction(chain flow.Chain, address flow.Address) (*flow.TransactionBody, error) {
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	script := templates.ReplaceAddresses(issueScheduledTransactionExecutorTemplate, sc.AsTemplateEnv())

	return flow.NewTransactionBodyBuilder().
		AddAuthorizer(sc.FlowServiceAccount.Address).
		AddAuthorizer(address).
		SetScript([]byte(script)).
		SetComputeLimit(callbackTransactionGasLimit).Build()
}
