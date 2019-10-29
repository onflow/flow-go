package testnet

import (
	"context"
	"sync"

	"github.com/dapperlabs/flow-go/crypto"
)

// hashernode implements a simple node that has a "hash" function which takes the hash of any payload delivered to it and saves it.

type hasherNode struct {
	messages []string
	msgmu    sync.RWMutex
	hasher   crypto.Hasher
}

func newHasherNode() (*hasherNode, error) {
	h, err := crypto.NewHasher(crypto.SHA3_256)
	if err != nil {
		return &hasherNode{}, nil
	}
	return &hasherNode{
		messages: make([]string, 0),
		hasher:   h,
	}, nil
}

// receive method hashes the payload and stores it in the messages slice
func (hn *hasherNode) receive(ctx context.Context, payload []byte) ([]byte, error) {
	hash := hn.hasher.ComputeHash(payload)
	hn.messages = append(hn.messages, string(hash[:]))
	return []byte("received"), nil
}

// store method stores a message in the underlying message array
func (hn *hasherNode) store(payload []byte) {
	hash := hn.hasher.ComputeHash(payload)
	hn.messages = append(hn.messages, string(hash[:]))
}
