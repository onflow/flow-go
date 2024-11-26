package data_provider

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
)

type Factory interface {
	NewDataProvider(ch chan<- interface{}, topic string) DataProvider
}

type SimpleFactory struct {
	logger       zerolog.Logger
	streamApi    state_stream.API
	streamConfig backend.Config
}

func NewDataProviderFactory(logger zerolog.Logger, streamApi state_stream.API, streamConfig backend.Config) *SimpleFactory {
	return &SimpleFactory{
		logger:       logger,
		streamApi:    streamApi,
		streamConfig: streamConfig,
	}
}

func (f *SimpleFactory) NewDataProvider(ch chan<- interface{}, topic string) DataProvider {
	switch topic {
	case "blocks":
		return NewMockBlockProvider(ch, topic, f.logger, f.streamApi)
	default:
		return nil
	}
}
