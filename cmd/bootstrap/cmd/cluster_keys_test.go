package cmd

import (
	"fmt"
	"testing"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestClusterForIndex(t *testing.T) {
	assert.Equal(t, 0, clusterForIndex(0, 3))
	assert.Equal(t, 1, clusterForIndex(1, 3))
	assert.Equal(t, 2, clusterForIndex(2, 3))
}

func TestCalcTotalCollectors(t *testing.T) {
	assert.Equal(t, 4, calcTotalCollectors(2, 2, 0))
	assert.Equal(t, 5, calcTotalCollectors(2, 2, 1))
	assert.Equal(t, 6, calcTotalCollectors(2, 2, 2))
	assert.Equal(t, 11, calcTotalCollectors(2, 2, 3))
	assert.Equal(t, 12, calcTotalCollectors(2, 2, 4))
	assert.Equal(t, 17, calcTotalCollectors(2, 2, 5))
	assert.Equal(t, 18, calcTotalCollectors(2, 2, 6))

	assert.Equal(t, 9, calcTotalCollectors(3, 3, 0))
	assert.Equal(t, 9, calcTotalCollectors(3, 3, 1))
	assert.Equal(t, 9, calcTotalCollectors(3, 3, 2))
	assert.Equal(t, 9, calcTotalCollectors(3, 3, 3))
	assert.Equal(t, 16, calcTotalCollectors(3, 3, 4))
	assert.Equal(t, 17, calcTotalCollectors(3, 3, 5))
	assert.Equal(t, 18, calcTotalCollectors(3, 3, 6))
	assert.Equal(t, 25, calcTotalCollectors(3, 3, 7))
	assert.Equal(t, 26, calcTotalCollectors(3, 3, 8))
	assert.Equal(t, 27, calcTotalCollectors(3, 3, 9))
}

func TestGenerateAdditionalInternalCollectors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping time intensive test")
	}
	res, _ := generateAdditionalInternalCollectors(3, 3, []NodeInfoPub{}, []NodeInfoPub{})
	assert.Len(t, res, 9)
	res, _ = generateAdditionalInternalCollectors(3, 3, []NodeInfoPub{}, generatePartnerCollectorNodes(3))
	assert.Len(t, res, 6)
	res, _ = generateAdditionalInternalCollectors(3, 3, []NodeInfoPub{}, generatePartnerCollectorNodes(9))
	assert.Len(t, res, 18)
}

func generatePartnerCollectorNodes(n int) []NodeInfoPub {
	res := make([]NodeInfoPub, n)

	for i := range res {
		networkKeys, err := run.GenerateNetworkingKeys(1, [][]byte{generateRandomSeed()})
		if err != nil {
			log.Fatal().Err(err).Msg("cannot generate networking key")
		}

		stakingKeys, err := run.GenerateStakingKeys(1, [][]byte{generateRandomSeed()})
		if err != nil {
			log.Fatal().Err(err).Msg("cannot generate staking key")
		}

		_, pub := assembleNodeInfo(NodeConfig{flow.RoleCollection, fmt.Sprintf("parter-collector-%v", i), 100},
			networkKeys[0], stakingKeys[0])

		res[i] = pub
	}

	return res
}
