package blueprints

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

const callbackTransactionGasLimit = flow.DefaultMaxTransactionGasLimit

<<<<<<< HEAD
// ProcessCallbacksTransaction constructs a transaction for processing callbacks, for the given callback.
// No errors are expected during normal operation.
func ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	script := prepareScheduledContractTransaction(chain, processCallbacksTransaction)
=======
func ProcessCallbacksTransaction(chain flow.Chain) *flow.TransactionBody {
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	script := templates.GenerateProcessCallbackScript(sc.AsTemplateEnv())
>>>>>>> master

	return flow.NewTransactionBodyBuilder().
		SetScript(script).
		SetComputeLimit(callbackTransactionGasLimit).Build()
}

// ExecuteCallbacksTransactions constructs a list of transaction to execute callbacks, for the given chain.
// No errors are expected during normal operation.
func ExecuteCallbacksTransactions(chainID flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	txs := make([]*flow.TransactionBody, 0, len(processEvents))
	env := systemcontracts.SystemContractsForChain(chainID.ChainID()).AsTemplateEnv()

	for _, event := range processEvents {
		id, effort, err := callbackArgsFromEvent(env, event)
		if err != nil {
			return nil, fmt.Errorf("failed to get callback args from event: %w", err)
		}

<<<<<<< HEAD
		tx, err := executeCallbackTransaction(chainID, id, effort)
		if err != nil {
			return nil, fmt.Errorf("failed to construct execute callback transactions: %w", err)
		}
=======
		tx := executeCallbackTransaction(env, id, effort)
>>>>>>> master
		txs = append(txs, tx)
	}

	return txs, nil
}

<<<<<<< HEAD
func executeCallbackTransaction(chain flow.Chain, id []byte, effort uint64) (*flow.TransactionBody, error) {
	script := prepareScheduledContractTransaction(chain, executeCallbacksTransaction)
	return flow.NewTransactionBodyBuilder().
=======
func executeCallbackTransaction(env templates.Environment, id []byte, effort uint64) *flow.TransactionBody {
	script := templates.GenerateExecuteCallbackScript(env)

	return flow.NewTransactionBody().
>>>>>>> master
		SetScript(script).
		AddArgument(id).
		SetComputeLimit(effort).
		Build()
}

// callbackArgsFromEvent decodes the event payload and returns the callback ID and effort.
//
// The event for processed callback event is emitted by the process callback transaction from
// callback scheduler contract and has the following signature:
// event CallbackProcessed(ID: UInt64, executionEffort: UInt64)
func callbackArgsFromEvent(env templates.Environment, event flow.Event) ([]byte, uint64, error) {
	const (
		processedCallbackIDFieldName     = "ID"
		processedCallbackEffortFieldName = "executionEffort"
		processedEventTypeTemplate       = "A.%v.CallbackScheduler.CallbackProcessed"
	)

	scheduledContractAddress := env.FlowCallbackSchedulerAddress
	processedEventType := flow.EventType(fmt.Sprintf(processedEventTypeTemplate, scheduledContractAddress))

	if event.Type != processedEventType {
		return nil, 0, fmt.Errorf("wrong event type is passed")
	}

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

	encodedID, err := ccf.Encode(id)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to encode id: %w", err)
	}

	return encodedID, effort, nil
}
