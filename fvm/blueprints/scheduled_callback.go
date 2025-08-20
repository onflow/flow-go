package blueprints

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"

	"github.com/onflow/flow-go/model/flow"
)

// processScheduledCallbacksTransaction calls scheduled callback contract
// and process new callbacks that should be executed.
//
//go:embed scripts/processScheduledCallbacksTransaction.cdc
var processCallbacksTransaction string

// executeCallbacksTransaction calls scheduled callback contract
// to execute the provided callback by ID.
//
//go:embed scripts/executeScheduledCallbackTransaction.cdc
var executeCallbacksTransaction string

const (
	placeholderScheduledContract     = "import \"CallbackScheduler\""
	processedCallbackIDFieldName     = "ID"
	processedCallbackEffortFieldName = "executionEffort"
	processedEventTypeTemplate       = "A.%v.CallbackScheduler.CallbackProcessed"
	callbackTransactionGasLimit      = flow.DefaultMaxTransactionGasLimit
)

// ProcessCallbacksTransaction constructs a transaction for processing callbacks, for the given callback.
// No errors are expected during normal operation.
func ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	script := prepareScheduledContractTransaction(chain, processCallbacksTransaction)

	return flow.NewTransactionBodyBuilder().
		SetScript(script).
		SetComputeLimit(callbackTransactionGasLimit).Build()
}

// ExecuteCallbacksTransactions constructs a list of transaction to execute callbacks, for the given chain.
// No errors are expected during normal operation.
func ExecuteCallbacksTransactions(chainID flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	txs := make([]*flow.TransactionBody, 0, len(processEvents))

	for _, event := range processEvents {
		id, effort, err := callbackArgsFromEvent(event)
		if err != nil {
			return nil, fmt.Errorf("failed to get callback args from event: %w", err)
		}

		tx, err := executeCallbackTransaction(chainID, id, effort)
		if err != nil {
			return nil, fmt.Errorf("failed to construct execute callback transactions: %w", err)
		}
		txs = append(txs, tx)
	}

	return txs, nil
}

func executeCallbackTransaction(chain flow.Chain, id []byte, effort uint64) (*flow.TransactionBody, error) {
	script := prepareScheduledContractTransaction(chain, executeCallbacksTransaction)
	return flow.NewTransactionBodyBuilder().
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
func callbackArgsFromEvent(event flow.Event) ([]byte, uint64, error) {
	scheduledContractAddress := "0x0000000000000000" // todo use contract addr
	if string(event.Type) != fmt.Sprintf(processedEventTypeTemplate, scheduledContractAddress) {
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

	effort, ok := effortValue.(cadence.UInt64)
	if !ok {
		return nil, 0, fmt.Errorf("effort is not uint64")
	}

	encodedID, err := ccf.Encode(id)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to encode id: %w", err)
	}

	return encodedID, uint64(effort), nil
}

func prepareScheduledContractTransaction(_ flow.Chain, txScript string) []byte {
	// todo use this instead of palceholder address
	// _ = systemcontracts.SystemContractsForChain(chain.ChainID())
	scheduledContractAddress := "0x0000000000000000"

	code := strings.ReplaceAll(
		txScript,
		placeholderScheduledContract,
		fmt.Sprintf("%s from %s", placeholderScheduledContract, scheduledContractAddress),
	)

	return []byte(code)
}
