package corruptlibp2p

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetGossipSubParams(t *testing.T) {
	gossipSubParams := getGossipSubParams()

	require.Equal(t, gossipSubParams, gossipSubParams)
}
