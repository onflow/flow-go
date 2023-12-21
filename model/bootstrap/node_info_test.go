package bootstrap_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIdentityListCanonical(t *testing.T) {
	nodes := unittest.NodeInfosFixture(20)
	ids := bootstrap.ToIdentityList(nodes)
	// make sure the list is not sorted
	nodes[0].NodeID[0], nodes[1].NodeID[0] = 2, 1
	require.False(t, order.IsIdentifierCanonical(nodes[0].NodeID, nodes[1].NodeID))
	ids = bootstrap.ToIdentityList(nodes)
	assert.False(t, order.IdentityListCanonical(ids))

	nodes = bootstrap.Sort(nodes, order.Canonical)
	ids = bootstrap.ToIdentityList(nodes)
	require.True(t, order.IdentityListCanonical(ids))

	// check `IdentityListCanonical` detects order equality in a sorted list
	nodes[1] = nodes[10] // add a duplication
	nodes = bootstrap.Sort(nodes, order.Canonical)
	ids = bootstrap.ToIdentityList(nodes)
	assert.False(t, order.IdentityListCanonical(ids))
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
		conf := unittest.NodeInfoFixture().Public()
		enc, err := json.Marshal(conf)
		require.NoError(t, err)
		var dec bootstrap.NodeInfoPub
		err = json.Unmarshal(enc, &dec)
		require.NoError(t, err)
		assert.True(t, dec.Equals(&conf))
	})
	t.Run("compat: should accept old files using Stake field", func(t *testing.T) {
		conf := unittest.NodeInfoFixture().Public()
		enc, err := json.Marshal(conf)
		require.NoError(t, err)
		// emulate the old encoding by replacing the new field with old field name
		enc = []byte(strings.Replace(string(enc), "Weight", "Stake", 1))
		var dec bootstrap.NodeInfoPub
		err = json.Unmarshal(enc, &dec)
		require.NoError(t, err)
		assert.True(t, dec.Equals(&conf))
	})
}
