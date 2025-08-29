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

// ProcessCallbacksTransaction constructs a transaction for processing callbacks, for the given callback.
// No errors are expected during normal operation.
func ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	script := templates.GenerateProcessCallbackScript(sc.AsTemplateEnv())

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
		if !isPendingExecutionEvent(env, event) {
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
	script := templates.GenerateExecuteCallbackScript(env)

	return flow.NewTransactionBodyBuilder().
		AddAuthorizer(sc.FlowServiceAccount.Address).
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
	const (
		processedCallbackIDFieldName     = "id"
		processedCallbackEffortFieldName = "executionEffort"
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
		processedCallbackIDFieldName,
	)

	effortValue := cadence.SearchFieldByName(
		cadenceEvent,
		processedCallbackEffortFieldName,
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
	const processedEventTypeTemplate = "A.%v.FlowCallbackScheduler.PendingExecution"

	scheduledContractAddress := env.FlowCallbackSchedulerAddress
	processedEventType := flow.EventType(fmt.Sprintf(processedEventTypeTemplate, scheduledContractAddress))

	return event.Type == processedEventType
}
