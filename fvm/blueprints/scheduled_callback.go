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
	placeholderScheduledContract     = "\"CallbackScheduler\""
	processedCallbackIDFieldName     = "ID"
	processedCallbackEffortFieldName = "executionEffort"
	processedEventTypeTemplate       = "A.%v.CallbackScheduler.CallbackProcessed"
)

func ProcessCallbacksTransaction(chain flow.Chain) *flow.TransactionBody {
	script := prepareScheduledContractTransaction(chain, processCallbacksTransaction)

	return flow.NewTransactionBody().
		SetScript(script).
		SetComputeLimit(SystemChunkTransactionGasLimit)
}

func ExecuteCallbacksTransactions(chainID flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	txs := make([]*flow.TransactionBody, 0, len(processEvents))

	for _, event := range processEvents {
		id, effort, err := callbackArgsFromEvent(event)
		if err != nil {
			return nil, fmt.Errorf("failed to get callback args from event: %w", err)
		}

		tx := executeCallbackTransaction(chainID, id, effort)
		txs = append(txs, tx)
	}

	return txs, nil
}

func executeCallbackTransaction(chain flow.Chain, id []byte, effort []byte) *flow.TransactionBody {
	script := prepareScheduledContractTransaction(chain, executeCallbacksTransaction)
	return flow.NewTransactionBody().
		SetScript(script).
		AddArgument(id).
		AddArgument(effort).
		SetComputeLimit(SystemChunkTransactionGasLimit)
}

func callbackArgsFromEvent(event flow.Event) ([]byte, []byte, error) {
	scheduledContractAddress := "0x0000000000000000" // todo use contract addr
	if string(event.Type) != fmt.Sprintf(processedEventTypeTemplate, scheduledContractAddress) {
		return nil, nil, fmt.Errorf("wrong event type is passed")
	}

	eventData, err := ccf.Decode(nil, event.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode event: %w", err)
	}

	cadenceEvent, ok := eventData.(cadence.Event)
	if !ok {
		return nil, nil, fmt.Errorf("event data is not a cadence event")
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
		return nil, nil, fmt.Errorf("id is not uint64")
	}

	effort, ok := effortValue.(cadence.UInt64)
	if !ok {
		return nil, nil, fmt.Errorf("effort is not uint64")
	}

	encodedID, err := ccf.Encode(id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode id: %w", err)
	}

	encodedEffort, err := ccf.Encode(effort)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode effort: %w", err)
	}

	return encodedID, encodedEffort, nil
}

func prepareScheduledContractTransaction(_ flow.Chain, txScript string) []byte {
	// todo use this instead of palceholder address
	// _ = systemcontracts.SystemContractsForChain(chain.ChainID())
	scheduledContractAddress := "0x0000000000000000"

	code := strings.ReplaceAll(
		txScript,
		placeholderScheduledContract,
		scheduledContractAddress,
	)

	return []byte(code)
}
