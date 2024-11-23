package data_provider

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
)

type Factory struct {
	logger       zerolog.Logger
	streamApi    state_stream.API
	streamConfig backend.Config
}

func NewDataProviderFactory(logger zerolog.Logger, streamApi state_stream.API, streamConfig backend.Config) *Factory {
	return &Factory{
		logger:       logger,
		streamApi:    streamApi,
		streamConfig: streamConfig,
	}
}

func (f *Factory) NewDataProvider(ch chan<- interface{}, topic string) DataProvider {
	switch topic {
	case "blocks":
		return NewMockBlockProvider(ch, topic, f.logger, f.streamApi)
	default:
		return nil
	}
}
