package p2ptest

import (
	"testing"

	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/utils/unittest"
)

// WithFrom is a test helper that returns a function that sets the from field of a pubsub message to the given peer id.
func WithFrom(from peer.ID) func(*pb.Message) {
	return func(m *pb.Message) {
		m.From = []byte(from)
	}
}

// WithTopic is a test helper that returns a function that sets the topic of a pubsub message to the given topic.
func WithTopic(topic string) func(*pb.Message) {
	return func(m *pb.Message) {
		m.Topic = &topic
	}
}

// WithoutSignature is a test helper that returns a function that sets the signature of a pubsub message to nil, effectively removing the signature.
func WithoutSignature() func(*pb.Message) {
	return func(m *pb.Message) {
		m.Signature = nil
	}
}

// WithoutSignerId is a test helper that returns a function that sets the from field of a pubsub message to nil, effectively removing the signer id.
func WithoutSignerId() func(*pb.Message) {
	return func(m *pb.Message) {
		m.From = nil
	}
}

// PubsubMessageFixture is a test helper that returns a random pubsub message with the given options applied.
// If no options are provided, the message will be random.
// Args:
//
//	t: testing.T
//
// opt: variadic list of options to apply to the message
// Returns:
// *pb.Message: pubsub message
func PubsubMessageFixture(t *testing.T, opts ...func(*pb.Message)) *pb.Message {
	topic := unittest.RandomStringFixture(t, 10)

	m := &pb.Message{
		Data:      unittest.RandomByteSlice(t, 100),
		Topic:     &topic,
		Signature: unittest.RandomByteSlice(t, 100),
		From:      unittest.RandomByteSlice(t, 100),
		Seqno:     unittest.RandomByteSlice(t, 100),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}
