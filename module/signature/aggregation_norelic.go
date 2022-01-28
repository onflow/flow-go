//go:build !relic
// +build !relic

package signature

import (
	"sync"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
)

type SignatureAggregatorSameMessage struct {
	message             []byte
	hasher              hash.Hasher
	n                   int
	publicKeys          []crypto.PublicKey
	indexToSignature    map[int]string
	cachedSignature     crypto.Signature
	cachedSignerIndices []int
}

func NewSignatureAggregatorSameMessage(
	_ []byte,
	_ string,
	_ []crypto.PublicKey,
) (*SignatureAggregatorSameMessage, error) {
	return &SignatureAggregatorSameMessage{}, nil
}

func (s *SignatureAggregatorSameMessage) Verify(_ int, _ crypto.Signature) (bool, error) {
	panic("SignatureAggregatorSameMessage.Verify not supported when flow-go is built without relic")
}

func (s *SignatureAggregatorSameMessage) VerifyAndAdd(_ int, _ crypto.Signature) (bool, error) {
	panic("SignatureAggregatorSameMessage.VerifyAndAdd not supported when flow-go is built without relic")
}

func (s *SignatureAggregatorSameMessage) TrustedAdd(_ int, _ crypto.Signature) error {
	panic("SignatureAggregatorSameMessage.TrustedAdd not supported when flow-go is built without relic")
}

func (s *SignatureAggregatorSameMessage) HasSignature(_ int) (bool, error) {
	panic("SignatureAggregatorSameMessage.HasSignature not supported when flow-go is built without relic")
}

func (s *SignatureAggregatorSameMessage) Aggregate() ([]int, crypto.Signature, error) {
	panic("SignatureAggregatorSameMessage.Aggregate not supported when flow-go is built without relic")
}

func (s *SignatureAggregatorSameMessage) VerifyAggregate(_ []int, _ crypto.Signature) (bool, error) {
	panic("SignatureAggregatorSameMessage.VerifyAggregate not supported when flow-go is built without relic")
}

type PublicKeyAggregator struct {
	n                 int
	publicKeys        []crypto.PublicKey
	lastSigners       map[int]struct{}
	lastAggregatedKey crypto.PublicKey
	sync.RWMutex
}

func NewPublicKeyAggregator(_ []crypto.PublicKey) (*PublicKeyAggregator, error) {
	return &PublicKeyAggregator{}, nil
}

func (p *PublicKeyAggregator) KeyAggregate(_ []int) (crypto.PublicKey, error) {
	panic("PublicKeyAggregator.KeyAggregate not supported when flow-go is built without relic")
}
