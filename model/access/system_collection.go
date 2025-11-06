package access

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
	"github.com/onflow/flow-go/model/flow"
)

// VersionedSystemCollections is a map of all versions of the system collection.
var VersionedSystemCollections = map[Version]SystemCollection{
	VersionV0: &SystemCollectionV0{},
	VersionV1: &SystemCollectionV1{},
	VersionV2: &SystemCollectionV2{},
}

// SystemCollection defines the builders for system collection and their transactions.
type SystemCollection interface {
	ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error)
	ExecuteCallbacksTransactions(chain flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error)
	SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error)
	SystemCollection(chain flow.Chain, processEvents flow.EventsList) (*flow.Collection, error)
}

// SystemCollections is a collection of all versions of system collections.
//
// This is useful when we want to obtain a system collection by ID but we don't
// have the block height, so we check all versions of system collection if it matches by ID.
type SystemCollections struct {
	transactions []*flow.TransactionBody
}

func NewSystemCollections(chain flow.Chain, versionedSystemCollection Versioned[SystemCollection]) (*SystemCollections, error) {
	transactions := make([]*flow.TransactionBody, 0)

	for _, collection := range versionedSystemCollection.all() {
		collection, err := collection.SystemCollection(chain, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to construct system collection: %w", err)
		}

		transactions = append(transactions, collection.Transactions...)
	}

	return &SystemCollections{
		transactions: transactions,
	}, nil
}

func (s *SystemCollections) ByID(id flow.Identifier) (*flow.TransactionBody, bool) {
	for _, tx := range s.transactions {
		if tx.ID() == id {
			return tx, true
		}
	}
	return nil, false
}

// We copy implementations from the blueprints package to freeze each versioned system
// collection at a specific point in time. This prevents future changes to blueprints
// from automatically affecting historical versions, ensuring version stability.
//
// When creating a new version, create a new versioned type and copy the implementation
// from blueprints at that point in time.

// SystemCollectionV2 is the latest and current version of the system collection.
type SystemCollectionV2 struct{}

//go:embed scripts/systemChunkTransactionTemplateV2.cdc
var systemChunkTransactionTemplateV2 string

const (
	systemChunkTransactionGasLimitV2 = 100_000_000
	callbackTransactionGasLimitV2    = flow.DefaultMaxTransactionGasLimit
	placeholderMigrationAddressV2    = "\"Migration\""
)

func (s *SystemCollectionV2) ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	script := templates.GenerateProcessTransactionScript(sc.AsTemplateEnv())

	return flow.NewTransactionBodyBuilder().
		AddAuthorizer(sc.FlowServiceAccount.Address).
		SetScript(script).
		SetComputeLimit(callbackTransactionGasLimitV2).Build()
}

func (s *SystemCollectionV2) ExecuteCallbacksTransactions(chain flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	txs := make([]*flow.TransactionBody, 0, len(processEvents))
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	env := sc.AsTemplateEnv()
	slices.SortFunc(processEvents, func(a, b flow.Event) int {
		return int(a.EventIndex - b.EventIndex)
	})

	for _, event := range processEvents {
		// skip any fee events or other events that are not pending execution events
		if !blueprints.IsPendingExecutionEvent(env, event) {
			continue
		}

		id, effort, err := callbackArgsFromEventV2(event)
		if err != nil {
			return nil, fmt.Errorf("failed to get callback args from event: %w", err)
		}

		tx, err := executeCallbackTransactionV2(sc, env, id, effort)
		if err != nil {
			return nil, fmt.Errorf("failed to construct execute callback transactions: %w", err)
		}
		txs = append(txs, tx)
	}

	return txs, nil
}

func executeCallbackTransactionV2(
	sc *systemcontracts.SystemContracts,
	env templates.Environment,
	id []byte,
	effort uint64,
) (*flow.TransactionBody, error) {
	script := templates.GenerateSchedulerExecutorTransactionScript(env)

	return flow.NewTransactionBodyBuilder().
		AddAuthorizer(sc.ScheduledTransactionExecutor.Address).
		SetScript(script).
		AddArgument(id).
		SetComputeLimit(effort).
		Build()
}

func callbackArgsFromEventV2(event flow.Event) ([]byte, uint64, error) {
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

func (s *SystemCollectionV2) SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	systemTxBody, err := flow.NewTransactionBodyBuilder().
		SetScript([]byte(prepareSystemContractCodeV2(chain.ChainID()))).
		SetComputeLimit(systemChunkTransactionGasLimitV2).
		AddAuthorizer(chain.ServiceAddress()).
		Build()
	if err != nil {
		return nil, fmt.Errorf("could not build system chunk transaction: %w", err)
	}

	return systemTxBody, nil
}

func (s *SystemCollectionV2) SystemCollection(chain flow.Chain, processEvents flow.EventsList) (*flow.Collection, error) {
	process, err := s.ProcessCallbacksTransaction(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to construct process callbacks transaction: %w", err)
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

func prepareSystemContractCodeV2(chainID flow.ChainID) string {
	sc := systemcontracts.SystemContractsForChain(chainID)
	code := templates.ReplaceAddresses(
		systemChunkTransactionTemplateV2,
		sc.AsTemplateEnv(),
	)
	code = strings.ReplaceAll(
		code,
		placeholderMigrationAddressV2,
		sc.Migration.Address.HexWithPrefix(),
	)
	return code
}

// SystemCollectionV1 is the system collection which included a scheduled
// transaction execution with the service account authorizer.
type SystemCollectionV1 struct{}

//go:embed scripts/systemChunkTransactionTemplateV1.cdc
var systemChunkTransactionTemplateV1 string

const (
	systemChunkTransactionGasLimitV1 = 100_000_000
	callbackTransactionGasLimitV1    = flow.DefaultMaxTransactionGasLimit
	placeholderMigrationAddressV1    = "\"Migration\""
)

func (s *SystemCollectionV1) ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	script := templates.GenerateProcessTransactionScript(sc.AsTemplateEnv())

	return flow.NewTransactionBodyBuilder().
		AddAuthorizer(sc.FlowServiceAccount.Address).
		SetScript(script).
		SetComputeLimit(callbackTransactionGasLimitV1).Build()
}

func (s *SystemCollectionV1) ExecuteCallbacksTransactions(chain flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	txs := make([]*flow.TransactionBody, 0, len(processEvents))
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	env := sc.AsTemplateEnv()
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

		tx, err := executeCallbackTransactionV1(sc, env, id, effort)
		if err != nil {
			return nil, fmt.Errorf("failed to construct execute callback transactions: %w", err)
		}
		txs = append(txs, tx)
	}

	return txs, nil
}

func executeCallbackTransactionV1(
	sc *systemcontracts.SystemContracts,
	env templates.Environment,
	id []byte,
	effort uint64,
) (*flow.TransactionBody, error) {
	// todo change to previous
	// script := templates.GenerateSchedulerExecutorTransactionScript(env)

	return flow.NewTransactionBodyBuilder().
		AddAuthorizer(sc.FlowServiceAccount.Address).
		SetScript(script).
		AddArgument(id).
		SetComputeLimit(effort).
		Build()
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

func (s *SystemCollectionV1) SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	systemTxBody, err := flow.NewTransactionBodyBuilder().
		SetScript([]byte(prepareSystemContractCodeV1(chain.ChainID()))).
		SetComputeLimit(systemChunkTransactionGasLimitV1).
		AddAuthorizer(chain.ServiceAddress()).
		Build()
	if err != nil {
		return nil, fmt.Errorf("could not build system chunk transaction: %w", err)
	}

	return systemTxBody, nil
}

func (s *SystemCollectionV1) SystemCollection(chain flow.Chain, processEvents flow.EventsList) (*flow.Collection, error) {
	process, err := s.ProcessCallbacksTransaction(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to construct process callbacks transaction: %w", err)
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

func prepareSystemContractCodeV1(chainID flow.ChainID) string {
	sc := systemcontracts.SystemContractsForChain(chainID)
	code := templates.ReplaceAddresses(
		systemChunkTransactionTemplateV1,
		sc.AsTemplateEnv(),
	)
	code = strings.ReplaceAll(
		code,
		placeholderMigrationAddressV1,
		sc.Migration.Address.HexWithPrefix(),
	)
	return code
}

// SystemCollectionV0 is the system collection before scheduled transactions were introduced.
type SystemCollectionV0 struct{}

//go:embed scripts/systemChunkTransactionTemplateV0.cdc
var systemChunkTransactionTemplateV0 string

const (
	systemChunkTransactionGasLimitV0 = 100_000_000
	placeholderMigrationAddressV0    = "\"Migration\""
)

func (s *SystemCollectionV0) ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	return nil, nil
}

func (s *SystemCollectionV0) ExecuteCallbacksTransactions(chain flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	return nil, nil
}

func (s *SystemCollectionV0) SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	systemTxBody, err := flow.NewTransactionBodyBuilder().
		SetScript([]byte(prepareSystemContractCodeV0(chain.ChainID()))).
		SetComputeLimit(systemChunkTransactionGasLimitV0).
		AddAuthorizer(chain.ServiceAddress()).
		Build()
	if err != nil {
		return nil, fmt.Errorf("could not build system chunk transaction: %w", err)
	}

	return systemTxBody, nil
}

func (s *SystemCollectionV0) SystemCollection(chain flow.Chain, processEvents flow.EventsList) (*flow.Collection, error) {
	systemTx, err := s.SystemChunkTransaction(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to construct system chunk transaction: %w", err)
	}

	return flow.NewCollection(flow.UntrustedCollection{
		Transactions: []*flow.TransactionBody{systemTx},
	})
}

func prepareSystemContractCodeV0(chainID flow.ChainID) string {
	sc := systemcontracts.SystemContractsForChain(chainID)
	code := templates.ReplaceAddresses(
		systemChunkTransactionTemplateV0,
		sc.AsTemplateEnv(),
	)
	code = strings.ReplaceAll(
		code,
		placeholderMigrationAddressV0,
		sc.Migration.Address.HexWithPrefix(),
	)
	return code
}
