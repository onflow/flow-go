package data_provider

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/state_stream"
)

type MockBlockProvider struct {
	id               uuid.UUID
	topicChan        chan<- interface{} // provider is not the one who is responsible to close this channel
	topic            string
	logger           zerolog.Logger
	stopProviderFunc context.CancelFunc
	streamApi        state_stream.API
}

func NewMockBlockProvider(
	ch chan<- interface{},
	topic string,
	logger zerolog.Logger,
	streamApi state_stream.API,
) *MockBlockProvider {
	return &MockBlockProvider{
		id:               uuid.New(),
		topicChan:        ch,
		topic:            topic,
		logger:           logger.With().Str("component", "block-provider").Logger(),
		stopProviderFunc: nil,
		streamApi:        streamApi,
	}
}

func (p *MockBlockProvider) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	p.stopProviderFunc = cancel

	for {
		select {
		case <-ctx.Done():
			return
		case p.topicChan <- "block{height: 42}":
			return
		}
	}
}

func (p *MockBlockProvider) ID() uuid.UUID {
	return p.id
}

func (p *MockBlockProvider) Topic() string {
	return p.topic
}

func (p *MockBlockProvider) Close() error {
	p.stopProviderFunc()
	return nil
}
