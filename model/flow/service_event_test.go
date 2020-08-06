package flow_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestEncodeDecode(t *testing.T) {

	setup := unittest.EpochSetupFixture(3)
	commit := unittest.EpochCommitFixture(3)

	t.Run("specific event types", func(t *testing.T) {
		b, err := json.Marshal(setup)
		require.Nil(t, err)

		gotSetup := new(flow.EpochSetup)
		err = json.Unmarshal(b, gotSetup)
		require.Nil(t, err)
		assert.DeepEqual(t, setup, gotSetup)

		b, err = json.Marshal(commit)
		require.Nil(t, err)

		gotCommit := new(flow.EpochCommit)
		err = json.Unmarshal(b, gotCommit)
		require.Nil(t, err)
		assert.DeepEqual(t, commit, gotCommit, cmp.FilterValues(func(a, b crypto.PublicKey) bool {
			return true
		}, cmp.Comparer(func(a, b crypto.PublicKey) bool {
			return a.Equals(b)
		})))
	})

	t.Run("generic type", func(t *testing.T) {
		b, err := json.Marshal(setup.ServiceEvent())
		require.Nil(t, err)

		outer := new(flow.ServiceEvent)
		err = json.Unmarshal(b, outer)
		require.Nil(t, err)
		gotSetup, ok := outer.Event.(*flow.EpochSetup)
		require.True(t, ok)
		assert.DeepEqual(t, setup, gotSetup)

		b, err = json.Marshal(commit.ServiceEvent())
		require.Nil(t, err)

		outer = new(flow.ServiceEvent)
		err = json.Unmarshal(b, outer)
		require.Nil(t, err)
		gotCommit, ok := outer.Event.(*flow.EpochCommit)
		require.True(t, ok)
		assert.DeepEqual(t, commit, gotCommit, cmp.FilterValues(func(a, b crypto.PublicKey) bool {
			return true
		}, cmp.Comparer(func(a, b crypto.PublicKey) bool {
			return a.Equals(b)
		})))
	})
}
