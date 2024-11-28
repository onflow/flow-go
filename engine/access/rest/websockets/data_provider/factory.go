package data_provider

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
)

// Constants defining various topic names used to specify different types of
// data providers.
const (
	BlocksTopic = "blocks"
)

// TODO: Temporary implementation without godoc; should be replaced once PR #6636 is merged

// DataProviderFactory defines an interface for creating data providers
// based on specified topics. The factory abstracts the creation process
// and ensures consistent access to required APIs.
type DataProviderFactory interface {
	// NewDataProvider creates a new data provider based on the specified topic
	// and configuration parameters.
	//
	// No errors are expected during normal operations.
	NewDataProvider(
		ch chan<- interface{},
		topic string) DataProvider
}

var _ DataProviderFactory = (*DataProviderFactoryImpl)(nil)

// DataProviderFactoryImpl is an implementation of the DataProviderFactory interface.
// It is responsible for creating data providers based on the
// requested topic. It manages access to logging and relevant APIs needed to retrieve data.
type DataProviderFactoryImpl struct {
	logger       zerolog.Logger
	streamApi    state_stream.API
	streamConfig backend.Config
}

func NewDataProviderFactory(
	logger zerolog.Logger,
	streamApi state_stream.API,
	streamConfig backend.Config,
) *DataProviderFactoryImpl {
	return &DataProviderFactoryImpl{
		logger:       logger,
		streamApi:    streamApi,
		streamConfig: streamConfig,
	}
}

func (s *DataProviderFactoryImpl) NewDataProvider(
	ch chan<- interface{},
	topic string,
) DataProvider {
	switch topic {
	case BlocksTopic:
		return NewMockBlockProvider(ch, topic, s.logger, s.streamApi)
	default:
		return nil
	}
}
