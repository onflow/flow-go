package corruptlibp2p

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGetGossipSubParams(t *testing.T) {
	gossipSubParams := getGossipSubParams()

	require.Equal(t, gossipSubParams, gossipSubParams)
}
