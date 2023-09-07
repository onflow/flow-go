package mocks

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type finalizedHeaderCache struct {
	t     *testing.T
	state protocol.State
}

func NewFinalizedHeaderCache(t *testing.T, state protocol.State) *finalizedHeaderCache {
	return &finalizedHeaderCache{
		t:     t,
		state: state,
	}
}

func (cache *finalizedHeaderCache) Get() *flow.Header {
	head, err := cache.state.Final().Head()
	require.NoError(cache.t, err)
	return head
}
