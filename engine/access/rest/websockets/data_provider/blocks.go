package data_provider

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/state_stream"
)

type MockBlockProvider struct {
	id               uuid.UUID
	ch               chan<- interface{}
	topic            string
	logger           zerolog.Logger
	ctx              context.Context
	stopProviderFunc context.CancelFunc
	streamApi        state_stream.API
}

func NewMockBlockProvider(
	ctx context.Context,
	ch chan<- interface{},
	topic string,
	logger zerolog.Logger,
	streamApi state_stream.API,
) *MockBlockProvider {
	ctx, cancel := context.WithCancel(ctx)
	return &MockBlockProvider{
		id:               uuid.New(),
		ch:               ch,
		topic:            topic,
		logger:           logger.With().Str("component", "block-provider").Logger(),
		ctx:              ctx,
		stopProviderFunc: cancel,
		streamApi:        streamApi,
	}
}

func (p *MockBlockProvider) Run() {
	select {
	case <-p.ctx.Done():
		return
	default:
		p.ch <- "hello"
		p.ch <- "world"
	}
}

func (p *MockBlockProvider) ID() uuid.UUID {
	return p.id
}

func (p *MockBlockProvider) Topic() string {
	return p.topic
}

func (p *MockBlockProvider) Close() {
	p.stopProviderFunc()
}
