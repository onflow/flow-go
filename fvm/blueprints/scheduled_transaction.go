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

const scheduledTransactionGasLimit = flow.DefaultMaxTransactionGasLimit

// SystemCollection returns the re-created system collection after it has been already executed
// using the events from the process scheduled transaction.
func SystemCollection(chain flow.Chain, processEvents flow.EventsList) (*flow.Collection, error) {
	process, err := ProcessScheduledTransactions(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to construct process scheduled transactions transaction: %w", err)
	}

	executes, err := ExecuteScheduledTransactions(chain, processEvents)
	if err != nil {
		return nil, fmt.Errorf("failed to construct execute scheduled transactions: %w", err)
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

// ProcessScheduledTransactions constructs a transaction for processing scheduled transactions.
// No errors are expected during normal operation.
func ProcessScheduledTransactions(chain flow.Chain) (*flow.TransactionBody, error) {
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	script := templates.GenerateProcessTransactionScript(sc.AsTemplateEnv())

	return flow.NewTransactionBodyBuilder().
		AddAuthorizer(sc.FlowServiceAccount.Address).
		SetScript(script).
		SetComputeLimit(scheduledTransactionGasLimit).Build()
}

// ExecuteScheduledTransactions constructs a list of transactions to execute scheduled transactions, for the given chain.
// No errors are expected during normal operation.
func ExecuteScheduledTransactions(chainID flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	txs := make([]*flow.TransactionBody, 0, len(processEvents))
	env := systemcontracts.SystemContractsForChain(chainID.ChainID()).AsTemplateEnv()
	sc := systemcontracts.SystemContractsForChain(chainID.ChainID())

	for _, event := range processEvents {
		// todo make sure to check event index to ensure order is indeed correct
		// event.EventIndex

		// skip any fee events or other events that are not pending execution events
		if !isPendingExecutionEvent(env, event) {
			continue
		}

		id, effort, err := scheduledTransactionArgsFromEvent(event)
		if err != nil {
			return nil, fmt.Errorf("failed to get scheduled transaction args from event: %w", err)
		}

		tx, err := executeScheduledTransaction(sc, env, id, effort)
		if err != nil {
			return nil, fmt.Errorf("failed to construct execute scheduled transactions: %w", err)
		}
		txs = append(txs, tx)
	}

	return txs, nil
}

func executeScheduledTransaction(
	sc *systemcontracts.SystemContracts,
	env templates.Environment,
	id []byte,
	effort uint64,
) (*flow.TransactionBody, error) {
	script := templates.GenerateExecuteTransactionScript(env)

	return flow.NewTransactionBodyBuilder().
		AddAuthorizer(sc.FlowServiceAccount.Address).
		SetScript(script).
		AddArgument(id).
		SetComputeLimit(effort).
		Build()
}

// scheduledTransactionArgsFromEvent decodes the event payload and returns the scheduled transaction ID and effort.
//
// The event for processed scheduled transaction event is emitted by the process scheduled transaction from
// transaction scheduler contract and has the following signature:
// event PendingExecution(id: UInt64, priority: UInt8, executionEffort: UInt64, fees: UFix64, owner: Address)
func scheduledTransactionArgsFromEvent(event flow.Event) ([]byte, uint64, error) {
	const (
		processedTransactionIDFieldName     = "id"
		processedTransactionEffortFieldName = "executionEffort"
	)

	eventData, err := ccf.Decode(nil, event.Payload)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to decode event: %w", err)
	}

	cadenceEvent, ok := eventData.(cadence.Event)
	if !ok {
		return nil, 0, fmt.Errorf("event data is not a cadence event")
	}

	idValue := cadence.SearchFieldByName(
		cadenceEvent,
		processedTransactionIDFieldName,
	)

	effortValue := cadence.SearchFieldByName(
		cadenceEvent,
		processedTransactionEffortFieldName,
	)

	id, ok := idValue.(cadence.UInt64)
	if !ok {
		return nil, 0, fmt.Errorf("id is not uint64")
	}

	cadenceEffort, ok := effortValue.(cadence.UInt64)
	if !ok {
		return nil, 0, fmt.Errorf("effort is not uint64")
	}

	effort := uint64(cadenceEffort)

	if effort > flow.DefaultMaxTransactionGasLimit {
		log.Warn().Uint64("effort", effort).Msg("effort is greater than max transaction gas limit, setting to max")
		effort = flow.DefaultMaxTransactionGasLimit
	}

	encID, err := jsoncdc.Encode(id)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to encode id: %w", err)
	}

	return encID, uint64(effort), nil
}

func isPendingExecutionEvent(env templates.Environment, event flow.Event) bool {
	processedEventType := PendingExecutionEventType(env)
	return event.Type == processedEventType
}

// PendingExecutionEventType returns the event type for FlowTransactionScheduler PendingExecution event
// for the provided environment.
func PendingExecutionEventType(env templates.Environment) flow.EventType {
	const processedEventTypeTemplate = "A.%v.FlowTransactionScheduler.PendingExecution"

	scheduledContractAddress := env.FlowTransactionSchedulerAddress
	return flow.EventType(fmt.Sprintf(processedEventTypeTemplate, scheduledContractAddress))
}
