package flow_test

import (
	"encoding/json"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack"
	"gotest.tools/assert"

	"github.com/onflow/flow-go/crypto"
	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEncodeDecode(t *testing.T) {

	setup := unittest.EpochSetupFixture()
	commit := unittest.EpochCommitFixture()

	t.Run("json", func(t *testing.T) {
		t.Run("specific event types", func(t *testing.T) {
			b, err := json.Marshal(setup)
			require.NoError(t, err)

			gotSetup := new(flow.EpochSetup)
			err = json.Unmarshal(b, gotSetup)
			require.NoError(t, err)
			assert.DeepEqual(t, setup, gotSetup)

			b, err = json.Marshal(commit)
			require.NoError(t, err)

			gotCommit := new(flow.EpochCommit)
			err = json.Unmarshal(b, gotCommit)
			require.NoError(t, err)
			assert.DeepEqual(t, commit, gotCommit, cmp.FilterValues(func(a, b crypto.PublicKey) bool {
				return true
			}, cmp.Comparer(func(a, b crypto.PublicKey) bool {
				return a.Equals(b)
			})))
		})

		t.Run("generic type", func(t *testing.T) {
			b, err := json.Marshal(setup.ServiceEvent())
			require.NoError(t, err)

			outer := new(flow.ServiceEvent)
			err = json.Unmarshal(b, outer)
			require.NoError(t, err)
			gotSetup, ok := outer.Event.(*flow.EpochSetup)
			require.True(t, ok)
			assert.DeepEqual(t, setup, gotSetup)

			t.Logf("- debug: setup.ServiceEvent()=%+v\n", setup.ServiceEvent())
			b, err = json.Marshal(commit.ServiceEvent())
			require.NoError(t, err)

			outer = new(flow.ServiceEvent)
			t.Logf("- debug: outer=%+v <-- before .Unmarshal()\n", outer)
			err = json.Unmarshal(b, outer)
			t.Logf("- debug: outer=%+v <-- after .Unmarshal()\n", outer)
			require.NoError(t, err)
			gotCommit, ok := outer.Event.(*flow.EpochCommit)
			require.True(t, ok)
			assert.DeepEqual(t, commit, gotCommit, cmp.FilterValues(func(a, b crypto.PublicKey) bool {
				return true
			}, cmp.Comparer(func(a, b crypto.PublicKey) bool {
				return a.Equals(b)
			})))
		})
	})

	t.Run("msgpack", func(t *testing.T) {
		t.Run("specific event types", func(t *testing.T) {
			b, err := msgpack.Marshal(setup)
			require.NoError(t, err)

			gotSetup := new(flow.EpochSetup)
			err = msgpack.Unmarshal(b, gotSetup)
			require.NoError(t, err)
			assert.DeepEqual(t, setup, gotSetup)

			b, err = msgpack.Marshal(commit)
			require.NoError(t, err)

			gotCommit := new(flow.EpochCommit)
			err = msgpack.Unmarshal(b, gotCommit)
			require.NoError(t, err)
			assert.DeepEqual(t, commit, gotCommit, cmp.FilterValues(func(a, b crypto.PublicKey) bool {
				return true
			}, cmp.Comparer(func(a, b crypto.PublicKey) bool {
				return a.Equals(b)
			})))
		})

		t.Run("generic type", func(t *testing.T) {
			b, err := msgpack.Marshal(setup.ServiceEvent())
			require.NoError(t, err)

			outer := new(flow.ServiceEvent)
			err = msgpack.Unmarshal(b, outer)
			require.NoError(t, err)
			gotSetup, ok := outer.Event.(*flow.EpochSetup)
			require.True(t, ok)
			assert.DeepEqual(t, setup, gotSetup)

			t.Logf("- debug: setup.ServiceEvent()=%+v\n", setup.ServiceEvent())
			b, err = msgpack.Marshal(commit.ServiceEvent())
			require.NoError(t, err)

			outer = new(flow.ServiceEvent)
			t.Logf("- debug: outer=%+v <-- before .Unmarshal()\n", outer)
			err = msgpack.Unmarshal(b, outer)
			t.Logf("- debug: outer=%+v <-- after .Unmarshal()\n", outer)
			require.NoError(t, err)
			gotCommit, ok := outer.Event.(*flow.EpochCommit)
			require.True(t, ok)
			assert.DeepEqual(t, commit, gotCommit, cmp.FilterValues(func(a, b crypto.PublicKey) bool {
				return true
			}, cmp.Comparer(func(a, b crypto.PublicKey) bool {
				return a.Equals(b)
			})))
		})
	})

	t.Run("cbor", func(t *testing.T) {
		t.Run("specific event types", func(t *testing.T) {
			b, err := cborcodec.EncMode.Marshal(setup)
			require.NoError(t, err)

			gotSetup := new(flow.EpochSetup)
			err = cbor.Unmarshal(b, gotSetup)
			require.NoError(t, err)
			assert.DeepEqual(t, setup, gotSetup)

			b, err = cborcodec.EncMode.Marshal(commit)
			require.NoError(t, err)

			gotCommit := new(flow.EpochCommit)
			err = cbor.Unmarshal(b, gotCommit)
			require.NoError(t, err)
			assert.DeepEqual(t, commit, gotCommit, cmp.FilterValues(func(a, b crypto.PublicKey) bool {
				return true
			}, cmp.Comparer(func(a, b crypto.PublicKey) bool {
				return a.Equals(b)
			})))
		})

		t.Run("generic type", func(t *testing.T) {
			t.Logf("- debug: setup.ServiceEvent()=%+v\n", setup.ServiceEvent())
			b, err := cborcodec.EncMode.Marshal(setup.ServiceEvent())
			require.NoError(t, err)

			outer := new(flow.ServiceEvent)
			t.Logf("- debug: outer=%+v <-- before .Unmarshal()\n", outer)
			err = cbor.Unmarshal(b, outer)
			t.Logf("- debug: outer=%+v <-- after .Unmarshal()\n", outer)
			require.NoError(t, err)
			gotSetup, ok := outer.Event.(*flow.EpochSetup)
			require.True(t, ok)
			assert.DeepEqual(t, setup, gotSetup)

			b, err = cborcodec.EncMode.Marshal(commit.ServiceEvent())
			require.NoError(t, err)

			outer = new(flow.ServiceEvent)
			err = cbor.Unmarshal(b, outer)
			require.NoError(t, err)
			gotCommit, ok := outer.Event.(*flow.EpochCommit)
			require.True(t, ok)
			assert.DeepEqual(t, commit, gotCommit, cmp.FilterValues(func(a, b crypto.PublicKey) bool {
				return true
			}, cmp.Comparer(func(a, b crypto.PublicKey) bool {
				return a.Equals(b)
			})))
		})
	})
}
