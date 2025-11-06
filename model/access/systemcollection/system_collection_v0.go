package systemcollection

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"

	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// builderV0 is the system collection which included a scheduled
// transaction execution with the service account authorizer.
type builderV0 struct{}

//go:embed scripts/systemChunkTransactionTemplateV0.cdc
var systemChunkTransactionTemplateV1 string

//go:embed scripts/processScheduledTransactionsTemplateV0.cdc
var processScheduledTransactionsTemplateV1 string

//go:embed scripts/executeScheduledTransactionTemplateV0.cdc
var executeScheduledTransactionTemplateV1 string

const (
	systemChunkTransactionGasLimitV1 = 100_000_000
	callbackTransactionGasLimitV1    = flow.DefaultMaxTransactionGasLimit
	placeholderMigrationAddressV1    = "\"Migration\""
)

func (s *builderV0) ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	script := templates.ReplaceAddresses(processScheduledTransactionsTemplateV1, sc.AsTemplateEnv())

	return flow.NewTransactionBodyBuilder().
		AddAuthorizer(sc.FlowServiceAccount.Address).
		SetScript([]byte(script)).
		SetComputeLimit(callbackTransactionGasLimitV1).Build()
}

func (s *builderV0) ExecuteCallbacksTransactions(chain flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	txs := make([]*flow.TransactionBody, 0, len(processEvents))
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	env := sc.AsTemplateEnv()
	script := prepareExecuteScheduledTransactionTemplateV1(chain.ChainID())

	slices.SortFunc(processEvents, func(a, b flow.Event) int {
		return int(a.EventIndex - b.EventIndex)
	})

	for _, event := range processEvents {
		// skip any fee events or other events that are not pending execution events
		if !blueprints.IsPendingExecutionEvent(env, event) {
			continue
		}

		id, effort, err := callbackArgsFromEventV1(event)
		if err != nil {
			return nil, fmt.Errorf("failed to get callback args from event: %w", err)
		}

		tx, err := flow.NewTransactionBodyBuilder().
			AddAuthorizer(sc.FlowServiceAccount.Address).
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

func callbackArgsFromEventV1(event flow.Event) ([]byte, uint64, error) {
	cadenceId, cadenceEffort, err := blueprints.ParsePendingExecutionEvent(event)
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

func (s *builderV0) SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	script := prepareSystemContractCodeV1(chain.ChainID())
	systemTxBody, err := flow.NewTransactionBodyBuilder().
		SetScript(script).
		SetComputeLimit(systemChunkTransactionGasLimitV1).
		AddAuthorizer(chain.ServiceAddress()).
		Build()
	if err != nil {
		return nil, fmt.Errorf("could not build system chunk transaction: %w", err)
	}

	return systemTxBody, nil
}

func (s *builderV0) SystemCollection(chain flow.Chain, providerFn access.EventProvider) (*flow.Collection, error) {
	process, err := s.ProcessCallbacksTransaction(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to construct process callbacks transaction: %w", err)
	}

	processEvents, err := providerFn()
	if err != nil {
		return nil, fmt.Errorf("failed to get process transactions events: %w", err)
	}

	executes, err := s.ExecuteCallbacksTransactions(chain, processEvents)
	if err != nil {
		return nil, fmt.Errorf("failed to construct execute callbacks transactions: %w", err)
	}

	systemTx, err := s.SystemChunkTransaction(chain)
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

func prepareSystemContractCodeV1(chainID flow.ChainID) []byte {
	sc := systemcontracts.SystemContractsForChain(chainID)
	code := templates.ReplaceAddresses(systemChunkTransactionTemplateV1, sc.AsTemplateEnv())
	code = strings.ReplaceAll(
		code,
		placeholderMigrationAddressV1,
		sc.Migration.Address.HexWithPrefix(),
	)
	return []byte(code)
}

func prepareProcessScheduledTransactionsTemplateV1(chainID flow.ChainID) []byte {
	sc := systemcontracts.SystemContractsForChain(chainID)
	code := templates.ReplaceAddresses(processScheduledTransactionsTemplateV1, sc.AsTemplateEnv())
	return []byte(code)
}

func prepareExecuteScheduledTransactionTemplateV1(chainID flow.ChainID) []byte {
	sc := systemcontracts.SystemContractsForChain(chainID)
	code := templates.ReplaceAddresses(executeScheduledTransactionTemplateV1, sc.AsTemplateEnv())
	return []byte(code)
}
