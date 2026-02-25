package extended

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/error_messages"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/model/access/systemcollection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
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

// Backend implements the extended API for querying account transactions and token transfers.
type Backend struct {
	*AccountTransactionsBackend
	*AccountTransfersBackend

	log zerolog.Logger
}

var _ API = (*Backend)(nil)

// New creates a new Backend instance.
func New(
	log zerolog.Logger,
	config Config,
	chainID flow.ChainID,
	store storage.AccountTransactionsReader,
	ftStore storage.FungibleTokenTransfersBootstrapper,
	nftStore storage.NonFungibleTokenTransfersBootstrapper,
	state protocol.State,
	blocks storage.Blocks,
	headers storage.Headers,
	eventsIndex *index.EventsIndex,
	txResultsIndex *index.TransactionResultsIndex,
	txErrorMessageProvider error_messages.Provider,
	collections storage.CollectionsReader,
	transactions storage.TransactionsReader,
	scheduledTransactions storage.ScheduledTransactionsReader,
	txStatusDeriver *txstatus.TxStatusDeriver,
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

	base := &backendBase{
		config:                config,
		headers:               headers,
		collections:           collections,
		transactions:          transactions,
		scheduledTransactions: scheduledTransactions,
		systemCollections:     systemCollections,
		transactionsProvider:  transactionsProvider,
	}

	chain := chainID.Chain()
	return &Backend{
		log:                        log,
		AccountTransactionsBackend: NewAccountTransactionsBackend(log, base, store, chain),
		AccountTransfersBackend:    NewAccountTransfersBackend(log, base, ftStore, nftStore, chain),
	}, nil
}

// mapReadError converts storage read errors to appropriate gRPC status errors.
func mapReadError(ctx context.Context, label string, err error) error {
	switch {
	case errors.Is(err, storage.ErrNotBootstrapped):
		return status.Errorf(codes.FailedPrecondition, "%s index not initialized: %v", label, err)
	case errors.Is(err, storage.ErrHeightNotIndexed):
		return status.Errorf(codes.OutOfRange, "requested height not indexed: %v", err)
	case errors.Is(err, storage.ErrInvalidQuery):
		return status.Errorf(codes.InvalidArgument, "invalid query: %v", err)
	case errors.Is(err, storage.ErrNotFound):
		return status.Errorf(codes.NotFound, "not found: %v", err)
	default:
		err = fmt.Errorf("failed to get %s: %w", label, err)
		irrecoverable.Throw(ctx, err)
		return err
	}
}

// collectResults iterates over the storage iterator and collects results that match the filter.
// It returns when it reaches the limit or the iterator is exhausted.
// Returns the results matching the filter and the next cursor.
//
// No error returns are expected during normal operation.
func collectResults[T any, C any](iter storage.IndexIterator[T, C], limit uint32, filter storage.IndexFilter[*T]) ([]T, *C, error) {
	var collected []T
	for item := range iter {
		// stop once we've collected `limit` results
		// go one extra iteration to check if there are more results and build the next cursor
		if uint32(len(collected)) >= limit {
			nextCursor, err := item.Cursor()
			if err != nil {
				return nil, nil, fmt.Errorf("could not get key for next cursor: %w", err)
			}
			return collected, &nextCursor, nil
		}

		tx, err := item.Value()
		if err != nil {
			return nil, nil, fmt.Errorf("could not get transaction: %w", err)
		}
		if filter != nil && !filter(&tx) {
			continue
		}
		collected = append(collected, tx)
	}
	return collected, nil, nil
}
