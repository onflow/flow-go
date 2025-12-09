package bootstrap_test

import (
	"encoding/json"
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

// TestNodeInfoPubEncodingJSON verifies encoding and decoding of NodeInfoPub
func TestNodeInfoPubEncodingJSON(t *testing.T) {
	t.Run("Encoding node info (new format)", func(t *testing.T) {
		info := unittest.PublicNodeInfoFixture()
		enc, err := json.Marshal(&info)
		require.NoError(t, err)
		var dec bootstrap.NodeInfoPub
		err = json.Unmarshal(enc, &dec)
		require.NoError(t, err)
		assert.True(t, dec.Equals(&info))
	})

	// New we test decoding of NodeInfoPub with the old format - we use hard-coded JSONs
	// But first, lets do a santiy check that we have a valid JSON representing the new format:
	t.Run("santiy check decoding of hard-coded JSON struct representing the new format", func(t *testing.T) {
		enc := []byte(`{
			"Role":"consensus",
			"Address":"7a996e29e2020e0164686f7b7763ae3483bce36171247e0f0581a5798d2c4ce2@flow.com:1234",
			"NodeID":"7a996e29e2020e0164686f7b7763ae3483bce36171247e0f0581a5798d2c4ce2",
			"Weight":1011,
			"NetworkPubKey":"9e1ce27613e5c16f0201d7a87ce44e7477d83fb8e13465c2a753e0e2d35e10065a0e6016d86b06281c2f5f33576199adb85519df0ba664ab5e16de547b76fcb9",
			"StakingPubKey":"b8bb00d806172bae514a7d82a457045dc03d75451866747fc2d2a3290122f24a3348db1f0505c9128811a42eb0ae706b116cf91a88541888502b3cf2c60acafd4248406913b9a4f700f51181858087ffabec68b1aadd384dab50f500afe5d931",
			"StakingPoP":"8bcc848f14c4c572b020de7cd5af0d74d69e25ceb7ecb5d09ec07681c95e25c597399597e29a10820913e927ab2882d9"
		}`)

		var dec bootstrap.NodeInfoPub
		err := json.Unmarshal(enc, &dec)
		require.NoError(t, err)
		assert.Equal(t, uint64(1011), dec.Weight)
	})

	// old format: Stake should be auto-converted to Weight during decoding
	t.Run("should handle embedded struct with Stake field", func(t *testing.T) {
		enc := []byte(`{"Role":"consensus",` +
			`"Address":"7a996e29e2020e0164686f7b7763ae3483bce36171247e0f0581a5798d2c4ce2@flow.com:1234",` +
			`"NodeID":"7a996e29e2020e0164686f7b7763ae3483bce36171247e0f0581a5798d2c4ce2",` +
			`"Stake":117,` +
			`"NetworkPubKey":"9e1ce27613e5c16f0201d7a87ce44e7477d83fb8e13465c2a753e0e2d35e10065a0e6016d86b06281c2f5f33576199adb85519df0ba664ab5e16de547b76fcb9",` +
			`"StakingPubKey":"b8bb00d806172bae514a7d82a457045dc03d75451866747fc2d2a3290122f24a3348db1f0505c9128811a42eb0ae706b116cf91a88541888502b3cf2c60acafd4248406913b9a4f700f51181858087ffabec68b1aadd384dab50f500afe5d931",` +
			`"StakingPoP":"8bcc848f14c4c572b020de7cd5af0d74d69e25ceb7ecb5d09ec07681c95e25c597399597e29a10820913e927ab2882d9"}`)

		var dec bootstrap.NodeInfoPub
		err := json.Unmarshal(enc, &dec)
		require.NoError(t, err)
		assert.Equal(t, uint64(117), dec.Weight) // Stake should have the value
	})

	// mixture of old and format: should be rejected
	t.Run("should reject both Stake and Weight fields", func(t *testing.T) {
		enc := []byte(`{"Role":"consensus",` +
			`"Address":"7a996e29e2020e0164686f7b7763ae3483bce36171247e0f0581a5798d2c4ce2@flow.com:1234",` +
			`"NodeID":"7a996e29e2020e0164686f7b7763ae3483bce36171247e0f0581a5798d2c4ce2",` +
			`"Weight":1011,` +
			`"Stake":117,` +
			`"NetworkPubKey":"9e1ce27613e5c16f0201d7a87ce44e7477d83fb8e13465c2a753e0e2d35e10065a0e6016d86b06281c2f5f33576199adb85519df0ba664ab5e16de547b76fcb9",` +
			`"StakingPubKey":"b8bb00d806172bae514a7d82a457045dc03d75451866747fc2d2a3290122f24a3348db1f0505c9128811a42eb0ae706b116cf91a88541888502b3cf2c60acafd4248406913b9a4f700f51181858087ffabec68b1aadd384dab50f500afe5d931",` +
			`"StakingPoP":"8bcc848f14c4c572b020de7cd5af0d74d69e25ceb7ecb5d09ec07681c95e25c597399597e29a10820913e927ab2882d9"}`)

		var dec bootstrap.NodeInfoPub
		err := json.Unmarshal(enc, &dec)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "both Stake and Weight fields")
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
