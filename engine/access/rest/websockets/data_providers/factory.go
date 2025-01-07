package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/model/flow"
)

// Constants defining various topic names used to specify different types of
// data providers.
const (
	EventsTopic                        = "events"
	AccountStatusesTopic               = "account_statuses"
	BlocksTopic                        = "blocks"
	BlockHeadersTopic                  = "block_headers"
	BlockDigestsTopic                  = "block_digests"
	TransactionStatusesTopic           = "transaction_statuses"
	SendAndGetTransactionStatusesTopic = "send_and_get_transaction_statuses"
)

// DataProviderFactory defines an interface for creating data providers
// based on specified topics. The factory abstracts the creation process
// and ensures consistent access to required APIs.
type DataProviderFactory interface {
	// NewDataProvider creates a new data provider based on the specified topic
	// and configuration parameters.
	//
	// No errors are expected during normal operations.
	NewDataProvider(ctx context.Context, topic string, args models.Arguments, ch chan<- interface{}) (DataProvider, error)
}

var _ DataProviderFactory = (*DataProviderFactoryImpl)(nil)

// DataProviderFactoryImpl is an implementation of the DataProviderFactory interface.
// It is responsible for creating data providers based on the
// requested topic. It manages access to logging and relevant APIs needed to retrieve data.
type DataProviderFactoryImpl struct {
	logger zerolog.Logger

	stateStreamApi state_stream.API
	accessApi      access.API

	chain             flow.Chain
	eventFilterConfig state_stream.EventFilterConfig
	heartbeatInterval uint64

	linkGenerator commonmodels.LinkGenerator
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
	chain flow.Chain,
	eventFilterConfig state_stream.EventFilterConfig,
	heartbeatInterval uint64,
	linkGenerator commonmodels.LinkGenerator,
) *DataProviderFactoryImpl {
	return &DataProviderFactoryImpl{
		logger:            logger,
		stateStreamApi:    stateStreamApi,
		accessApi:         accessApi,
		chain:             chain,
		eventFilterConfig: eventFilterConfig,
		heartbeatInterval: heartbeatInterval,
		linkGenerator:     linkGenerator,
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
	arguments models.Arguments,
	ch chan<- interface{},
) (DataProvider, error) {
	switch topic {
	case BlocksTopic:
		return NewBlocksDataProvider(ctx, s.logger, s.accessApi, s.linkGenerator, topic, arguments, ch)
	case BlockHeadersTopic:
		return NewBlockHeadersDataProvider(ctx, s.logger, s.accessApi, topic, arguments, ch)
	case BlockDigestsTopic:
		return NewBlockDigestsDataProvider(ctx, s.logger, s.accessApi, topic, arguments, ch)
	case EventsTopic:
		return NewEventsDataProvider(ctx, s.logger, s.stateStreamApi, topic, arguments, ch, s.chain, s.eventFilterConfig, s.heartbeatInterval)
	case AccountStatusesTopic:
		return NewAccountStatusesDataProvider(ctx, s.logger, s.stateStreamApi, topic, arguments, ch, s.chain, s.eventFilterConfig, s.heartbeatInterval)
	case TransactionStatusesTopic:
		return NewTransactionStatusesDataProvider(ctx, s.logger, s.accessApi, s.linkGenerator, topic, arguments, ch)
	case SendAndGetTransactionStatusesTopic:
		return NewSendAndGetTransactionStatusesDataProvider(ctx, s.logger, s.accessApi, s.linkGenerator, topic, arguments, ch, s.chain)
	default:
		return nil, fmt.Errorf("unsupported topic \"%s\"", topic)
	}
}
