// Code generated by mockery v2.53.3. DO NOT EDIT.

package mockp2p

import (
	p2p "github.com/onflow/flow-go/network/p2p"
	mock "github.com/stretchr/testify/mock"

	routing "github.com/libp2p/go-libp2p/core/routing"

	time "time"
)

// PubSubAdapterConfig is an autogenerated mock type for the PubSubAdapterConfig type
type PubSubAdapterConfig struct {
	mock.Mock
}

// WithMessageIdFunction provides a mock function with given fields: f
func (_m *PubSubAdapterConfig) WithMessageIdFunction(f func([]byte) string) {
	_m.Called(f)
}

// WithPeerGater provides a mock function with given fields: topicDeliveryWeights, sourceDecay
func (_m *PubSubAdapterConfig) WithPeerGater(topicDeliveryWeights map[string]float64, sourceDecay time.Duration) {
	_m.Called(topicDeliveryWeights, sourceDecay)
}

// WithRoutingDiscovery provides a mock function with given fields: _a0
func (_m *PubSubAdapterConfig) WithRoutingDiscovery(_a0 routing.ContentRouting) {
	_m.Called(_a0)
}

// WithRpcInspector provides a mock function with given fields: _a0
func (_m *PubSubAdapterConfig) WithRpcInspector(_a0 p2p.GossipSubRPCInspector) {
	_m.Called(_a0)
}

// WithScoreOption provides a mock function with given fields: _a0
func (_m *PubSubAdapterConfig) WithScoreOption(_a0 p2p.ScoreOptionBuilder) {
	_m.Called(_a0)
}

// WithScoreTracer provides a mock function with given fields: tracer
func (_m *PubSubAdapterConfig) WithScoreTracer(tracer p2p.PeerScoreTracer) {
	_m.Called(tracer)
}

// WithSubscriptionFilter provides a mock function with given fields: _a0
func (_m *PubSubAdapterConfig) WithSubscriptionFilter(_a0 p2p.SubscriptionFilter) {
	_m.Called(_a0)
}

// WithTracer provides a mock function with given fields: t
func (_m *PubSubAdapterConfig) WithTracer(t p2p.PubSubTracer) {
	_m.Called(t)
}

// WithValidateQueueSize provides a mock function with given fields: _a0
func (_m *PubSubAdapterConfig) WithValidateQueueSize(_a0 int) {
	_m.Called(_a0)
}

// NewPubSubAdapterConfig creates a new instance of PubSubAdapterConfig. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewPubSubAdapterConfig(t interface {
	mock.TestingT
	Cleanup(func())
}) *PubSubAdapterConfig {
	mock := &PubSubAdapterConfig{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
