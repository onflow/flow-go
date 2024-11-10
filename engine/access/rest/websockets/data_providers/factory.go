package data_providers

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/state_stream"
)

const (
	EventsTopic              = "events"
	AccountStatusesTopic     = "account_statuses"
	BlockHeadersTopic        = "block_headers"
	BlockDigestsTopic        = "block_digests"
	TransactionStatusesTopic = "transaction_statuses"

	BlocksFromStartBlockIDTopic     = "blocks_from_start_block_id"
	BlocksFromStartBlockHeightTopic = "blocks_from_start_block_height"
	BlocksFromLatestTopic           = "blocks_from_latest"
)

type DataProviderFactory struct {
	logger            zerolog.Logger
	eventFilterConfig state_stream.EventFilterConfig

	stateStreamApi state_stream.API
	accessApi      access.API
}

func NewDataProviderFactory(
	logger zerolog.Logger,
	eventFilterConfig state_stream.EventFilterConfig,
	stateStreamApi state_stream.API,
	accessApi access.API,
) *DataProviderFactory {
	return &DataProviderFactory{
		logger:            logger,
		eventFilterConfig: eventFilterConfig,
		stateStreamApi:    stateStreamApi,
		accessApi:         accessApi,
	}
}

func (s *DataProviderFactory) NewDataProvider(
	ctx context.Context,
	topic string,
	arguments map[string]string,
	ch chan<- interface{},
) (DataProvider, error) {
	switch topic {
	case BlocksFromStartBlockIDTopic:
		return NewBlocksFromStartBlockIDProvider(ctx, s.logger, s.accessApi, topic, arguments, ch)
	case BlocksFromStartBlockHeightTopic:
		return NewBlocksFromStartBlockHeightProvider(ctx, s.logger, s.accessApi, topic, arguments, ch)
	case BlocksFromLatestTopic:
		return NewBlocksFromLatestProvider(ctx, s.logger, s.accessApi, topic, arguments, ch)
	// TODO: Implemented handlers for each topic should be added in respective case
	case EventsTopic,
		AccountStatusesTopic,
		BlockHeadersTopic,
		BlockDigestsTopic,
		TransactionStatusesTopic:
		return nil, fmt.Errorf("topic \"%s\" not implemented yet", topic)
	default:
		return nil, fmt.Errorf("unsupported topic \"%s\"", topic)
	}
}
