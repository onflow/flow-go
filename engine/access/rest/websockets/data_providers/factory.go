package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/state_stream"
)

// Constants defining various topic names used to specify different types of
// data providers.
const (
	EventsTopic              = "events"
	AccountStatusesTopic     = "account_statuses"
	BlocksTopic              = "blocks"
	BlockHeadersTopic        = "block_headers"
	BlockDigestsTopic        = "block_digests"
	TransactionStatusesTopic = "transaction_statuses"
)

// DataProviderFactory is responsible for creating data providers based on the
// requested topic. It manages access to logging, state stream configuration,
// and relevant APIs needed to retrieve data.
type DataProviderFactory interface {
	// NewDataProvider creates a new data provider based on the specified topic
	// and configuration parameters.
	//
	// No errors are expected during normal operations.
	NewDataProvider(context.Context, string, map[string]string, chan<- interface{}) (DataProvider, error)
}

var _ DataProviderFactory = (*DataProviderFactoryImpl)(nil)

// DataProviderFactoryImpl is responsible for creating data providers based on the
// requested topic. It manages access to logging, state stream configuration,
// and relevant APIs needed to retrieve data.
type DataProviderFactoryImpl struct {
	logger zerolog.Logger

	stateStreamApi state_stream.API
	accessApi      access.API
}

// NewDataProviderFactory creates a new DataProviderFactory
//
// Parameters:
// - logger: Used for logging within the data providers.
// - eventFilterConfig: Configuration for filtering events from state streams.
// - stateStreamApi: API for accessing data from the Flow state stream API.
// - accessApi: API for accessing data from the Flow Access API.
func NewDataProviderFactory(
	logger zerolog.Logger,
	stateStreamApi state_stream.API,
	accessApi access.API,
) *DataProviderFactoryImpl {
	return &DataProviderFactoryImpl{
		logger:         logger,
		stateStreamApi: stateStreamApi,
		accessApi:      accessApi,
	}
}

// NewDataProvider creates a new data provider based on the specified topic
// and configuration parameters.
//
// Parameters:
// - ctx: Context for managing request lifetime and cancellation.
// - topic: The topic for which a data provider is to be created.
// - arguments: Configuration arguments for the data provider.
// - ch: Channel to which the data provider sends data.
//
// No errors are expected during normal operations.
func (s *DataProviderFactoryImpl) NewDataProvider(
	ctx context.Context,
	topic string,
	arguments map[string]string,
	ch chan<- interface{},
) (DataProvider, error) {
	switch topic {
	case BlocksTopic:
		return NewBlocksDataProvider(ctx, s.logger, s.accessApi, topic, arguments, ch)
	case BlockHeadersTopic:
		return NewBlockHeadersDataProvider(ctx, s.logger, s.accessApi, topic, arguments, ch)
	case BlockDigestsTopic:
		return NewBlockDigestsDataProvider(ctx, s.logger, s.accessApi, topic, arguments, ch)
	case EventsTopic:
		return NewEventsDataProvider(ctx, s.logger, s.stateStreamApi, topic, arguments, ch)
		// TODO: Implemented handlers for each topic should be added in respective case
	case AccountStatusesTopic,
		TransactionStatusesTopic:
		return nil, fmt.Errorf("topic \"%s\" not implemented yet", topic)
	default:
		return nil, fmt.Errorf("unsupported topic \"%s\"", topic)
	}
}
