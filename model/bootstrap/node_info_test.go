package bootstrap_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIdentityListCanonical(t *testing.T) {
	nodes := bootstrap.PrivToNodeInfoList(unittest.PrivateNodeInfosFixture(20))
	// make sure the list is not sorted
	if flow.IdentifierCanonical(nodes[0].NodeID(), nodes[1].NodeID()) < 0 {
		nodes[0], nodes[1] = nodes[1], nodes[0]
	}
	require.False(t, flow.IsIdentifierCanonical(nodes[0].NodeID(), nodes[1].NodeID()))
	ids := bootstrap.ToIdentityList(nodes)
	assert.False(t, flow.IsIdentityListCanonical(ids))

	// make a copy of the original list of nodes
	nodesCopy := make([]bootstrap.NodeInfo, len(nodes))
	copy(nodesCopy, nodes)

	sortedNodes := bootstrap.Sort(nodes, flow.Canonical[flow.Identity])
	sortedIds := bootstrap.ToIdentityList(sortedNodes)
	require.True(t, flow.IsIdentityListCanonical(sortedIds))
	// make sure original list didn't change
	assert.Equal(t, nodesCopy, nodes)

	// check `IsIdentityListCanonical` detects order equality in a sorted list
	nodes[1] = nodes[10] // add a duplication
	copy(nodesCopy, nodes)
	sortedNodes = bootstrap.Sort(nodes, flow.Canonical[flow.Identity])
	sortedIds = bootstrap.ToIdentityList(sortedNodes)
	assert.False(t, flow.IsIdentityListCanonical(sortedIds))
	// make sure original list didn't change
	assert.Equal(t, nodesCopy, nodes)
}

func TestNodeConfigEncodingJSON(t *testing.T) {
	t.Run("normal node config", func(t *testing.T) {
		conf := unittest.NodeConfigFixture()
		enc, err := json.Marshal(conf)
		require.NoError(t, err)
		var dec bootstrap.NodeConfig
		err = json.Unmarshal(enc, &dec)
		require.NoError(t, err)
		assert.Equal(t, conf, dec)
	})
	t.Run("compat: should accept old files using Stake field", func(t *testing.T) {
		conf := unittest.NodeConfigFixture()
		enc, err := json.Marshal(conf)
		require.NoError(t, err)
		// emulate the old encoding by replacing the new field with old field name
		enc = []byte(strings.Replace(string(enc), "Weight", "Stake", 1))
		var dec bootstrap.NodeConfig
		err = json.Unmarshal(enc, &dec)
		require.NoError(t, err)
		assert.Equal(t, conf, dec)
	})
}

func TestNodeInfoPubEncodingJSON(t *testing.T) {
	t.Run("normal node info", func(t *testing.T) {
		info := unittest.PublicNodeInfoFixture()
		enc, err := json.Marshal(&info)
		require.NoError(t, err)
		var dec bootstrap.NodeInfoPub
		err = json.Unmarshal(enc, &dec)
		require.NoError(t, err)
		assert.True(t, dec.Equals(&info))
	})
	t.Run("compat: should accept old files using Stake field", func(t *testing.T) {
		info := unittest.PublicNodeInfoFixture()
		enc, err := json.Marshal(&info)
		require.NoError(t, err)
		// emulate the old encoding by replacing the new field with old field name
		enc = []byte(strings.Replace(string(enc), "Weight", "Stake", 1))
		var dec bootstrap.NodeInfoPub
		err = json.Unmarshal(enc, &dec)
		require.NoError(t, err)
		assert.True(t, dec.Equals(&info))
		fmt.Println(info.Weight, dec.Weight)
	})
}

// Test of write and read of NodeInfoPriv files
func TestPrivNodeWriteRead(t *testing.T) {
	nodes := unittest.PrivateNodeInfosFixture(1)
	dir := unittest.TempDir(t)
	defer os.RemoveAll(dir)
	internalPrivDir, _, err := utils.WriteInternalFiles(nodes, dir)
	require.NoError(t, err)

	files, err := common.FilesInDir(internalPrivDir)
	require.NoError(t, err)

	for _, f := range files {
		// skip files that do not include node-infos
		if !strings.Contains(f, bootstrap.PathPrivNodeInfoPrefix) {
			continue
		}

		// read file and append to partners
		var p bootstrap.NodeInfoPriv
		err = common.ReadJSON(f, &p)
		require.NoError(t, err)
		require.Equal(t, p.NodeID(), nodes[0].NodeID())
		require.Equal(t, p.Role(), nodes[0].Role())
		require.Equal(t, p.Address, nodes[0].Address)
		require.True(t, p.NetworkPrivKey.PrivateKey.Equals(nodes[0].NetworkPrivKey.PrivateKey))
		require.True(t, p.StakingPrivKey.PrivateKey.Equals(nodes[0].StakingPrivKey.PrivateKey))
	}
}
