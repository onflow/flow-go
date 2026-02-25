package extended

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/error_messages"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/model/access/systemcollection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// Config holds configuration for the extended API backend.
type Config struct {
	DefaultPageSize uint32 // Page size used when limit is 0.
	MaxPageSize     uint32 // Maximum allowed page size.
}

// DefaultConfig returns the default configuration for the extended API backend.
func DefaultConfig() Config {
	return Config{
		DefaultPageSize: 50,
		MaxPageSize:     200,
	}
}

// Backend implements the extended API for querying account transactions.
type Backend struct {
	*AccountTransactionsBackend

	log zerolog.Logger
}

var _ API = (*Backend)(nil)

// New creates a new Backend instance.
func New(
	log zerolog.Logger,
	config Config,
	chainID flow.ChainID,
	store storage.AccountTransactionsReader,
	state protocol.State,
	blocks storage.Blocks,
	headers storage.Headers,
	eventsIndex *index.EventsIndex,
	txResultsIndex *index.TransactionResultsIndex,
	txErrorMessageProvider error_messages.Provider,
	collections storage.CollectionsReader,
	transactions storage.TransactionsReader,
	scheduledTransactions storage.ScheduledTransactionsReader,
	txStatusDeriver *status.TxStatusDeriver,
) (*Backend, error) {
	log = log.With().Str("component", "extended_backend").Logger()

	systemCollections, err := systemcollection.NewVersioned(chainID.Chain(), systemcollection.Default(chainID))
	if err != nil {
		return nil, fmt.Errorf("failed to create system collection set: %w", err)
	}

	transactionsProvider := provider.NewLocalTransactionProvider(
		state,
		collections,
		blocks,
		eventsIndex,
		txResultsIndex,
		txErrorMessageProvider,
		systemCollections,
		txStatusDeriver,
		chainID,
	)

	return &Backend{
		log: log,
		AccountTransactionsBackend: NewAccountTransactionsBackend(
			log,
			config,
			store,
			headers,
			collections,
			transactions,
			scheduledTransactions,
			systemCollections,
			transactionsProvider,
		),
	}, nil
}
