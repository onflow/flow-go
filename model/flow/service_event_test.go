package flow_test

import (
	"encoding/json"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/onflow/crypto"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack"
	"gotest.tools/assert"

	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEncodeDecode(t *testing.T) {

	setup := unittest.EpochSetupFixture()
	commit := unittest.EpochCommitFixture()
	versionBeacon := unittest.VersionBeaconFixture()
	protocolVersionUpgrade := unittest.ProtocolStateVersionUpgradeFixture()

	comparePubKey := cmp.FilterValues(func(a, b crypto.PublicKey) bool {
		return true
	}, cmp.Comparer(func(a, b crypto.PublicKey) bool {
		if a == nil {
			return b == nil
		}
		return a.Equals(b)
	}))

	t.Run("json", func(t *testing.T) {
		t.Run("specific event types", func(t *testing.T) {
			// EpochSetup
			b, err := json.Marshal(setup)
			require.NoError(t, err)

			gotSetup := new(flow.EpochSetup)
			err = json.Unmarshal(b, gotSetup)
			require.NoError(t, err)
			assert.DeepEqual(t, setup, gotSetup, comparePubKey)

			// EpochCommit
			b, err = json.Marshal(commit)
			require.NoError(t, err)

			gotCommit := new(flow.EpochCommit)
			err = json.Unmarshal(b, gotCommit)
			require.NoError(t, err)
			assert.DeepEqual(t, commit, gotCommit, comparePubKey)

			// VersionBeacon
			b, err = json.Marshal(versionBeacon)
			require.NoError(t, err)

			gotVersionBeacon := new(flow.VersionBeacon)
			err = json.Unmarshal(b, gotVersionBeacon)
			require.NoError(t, err)
			assert.DeepEqual(t, versionBeacon, gotVersionBeacon)

			// ProtocolStateVersionUpgrade
			b, err = json.Marshal(protocolVersionUpgrade)
			require.NoError(t, err)

			gotProtocolVersionUpgrade := new(flow.ProtocolStateVersionUpgrade)
			err = json.Unmarshal(b, gotProtocolVersionUpgrade)
			require.NoError(t, err)
			assert.DeepEqual(t, protocolVersionUpgrade, gotProtocolVersionUpgrade)
		})

		t.Run("generic type", func(t *testing.T) {
			// EpochSetup
			b, err := json.Marshal(setup.ServiceEvent())
			require.NoError(t, err)

			outer := new(flow.ServiceEvent)
			err = json.Unmarshal(b, outer)
			require.NoError(t, err)
			gotSetup, ok := outer.Event.(*flow.EpochSetup)
			require.True(t, ok)
			assert.DeepEqual(t, setup, gotSetup, comparePubKey)

			// EpochCommit
			t.Logf("- debug: setup.ServiceEvent()=%+v\n", setup.ServiceEvent())
			b, err = json.Marshal(commit.ServiceEvent())
			require.NoError(t, err)

			outer = new(flow.ServiceEvent)
			t.Logf("- debug: outer=%+v <-- before .UnmarshalWrapped()\n", outer)
			err = json.Unmarshal(b, outer)
			t.Logf("- debug: outer=%+v <-- after .UnmarshalWrapped()\n", outer)
			require.NoError(t, err)
			gotCommit, ok := outer.Event.(*flow.EpochCommit)
			require.True(t, ok)
			assert.DeepEqual(t, commit, gotCommit, comparePubKey)

			// VersionBeacon
			t.Logf("- debug: versionBeacon.ServiceEvent()=%+v\n", versionBeacon.ServiceEvent())
			b, err = json.Marshal(versionBeacon.ServiceEvent())
			require.NoError(t, err)

			outer = new(flow.ServiceEvent)
			t.Logf("- debug: outer=%+v <-- before .UnmarshalWrapped()\n", outer)
			err = json.Unmarshal(b, outer)
			t.Logf("- debug: outer=%+v <-- after .UnmarshalWrapped()\n", outer)
			require.NoError(t, err)
			gotVersionTable, ok := outer.Event.(*flow.VersionBeacon)
			require.True(t, ok)
			assert.DeepEqual(t, versionBeacon, gotVersionTable)

			// ProtocolStateVersionUpgrade
			b, err = json.Marshal(protocolVersionUpgrade.ServiceEvent())
			require.NoError(t, err)

			outer = new(flow.ServiceEvent)
			err = json.Unmarshal(b, outer)
			require.NoError(t, err)
			gotProtocolVersionUpgrade, ok := outer.Event.(*flow.ProtocolStateVersionUpgrade)
			require.True(t, ok)
			assert.DeepEqual(t, protocolVersionUpgrade, gotProtocolVersionUpgrade)
		})
	})

	t.Run("msgpack", func(t *testing.T) {
		t.Run("specific event types", func(t *testing.T) {
			// EpochSetup
			b, err := msgpack.Marshal(setup)
			require.NoError(t, err)

			gotSetup := new(flow.EpochSetup)
			err = msgpack.Unmarshal(b, gotSetup)
			require.NoError(t, err)
			assert.DeepEqual(t, setup, gotSetup, comparePubKey)

			// EpochCommit
			b, err = msgpack.Marshal(commit)
			require.NoError(t, err)

			gotCommit := new(flow.EpochCommit)
			err = msgpack.Unmarshal(b, gotCommit)
			require.NoError(t, err)
			assert.DeepEqual(t, commit, gotCommit, comparePubKey)

			// VersionBeacon
			b, err = msgpack.Marshal(versionBeacon)
			require.NoError(t, err)

			gotVersionTable := new(flow.VersionBeacon)
			err = msgpack.Unmarshal(b, gotVersionTable)
			require.NoError(t, err)
			assert.DeepEqual(t, versionBeacon, gotVersionTable)

			// ProtocolStateVersionUpgrade
			b, err = msgpack.Marshal(protocolVersionUpgrade)
			require.NoError(t, err)

			gotProtocolVersionUpgrade := new(flow.ProtocolStateVersionUpgrade)
			err = msgpack.Unmarshal(b, gotProtocolVersionUpgrade)
			require.NoError(t, err)
			assert.DeepEqual(t, protocolVersionUpgrade, gotProtocolVersionUpgrade)
		})

		t.Run("generic type", func(t *testing.T) {
			b, err := msgpack.Marshal(setup.ServiceEvent())
			require.NoError(t, err)

			outer := new(flow.ServiceEvent)
			err = msgpack.Unmarshal(b, outer)
			require.NoError(t, err)
			gotSetup, ok := outer.Event.(*flow.EpochSetup)
			require.True(t, ok)
			assert.DeepEqual(t, setup, gotSetup, comparePubKey)

			// EpochCommit
			t.Logf("- debug: setup.ServiceEvent()=%+v\n", setup.ServiceEvent())
			b, err = msgpack.Marshal(commit.ServiceEvent())
			require.NoError(t, err)

			outer = new(flow.ServiceEvent)
			t.Logf("- debug: outer=%+v <-- before .UnmarshalWrapped()\n", outer)
			err = msgpack.Unmarshal(b, outer)
			t.Logf("- debug: outer=%+v <-- after .UnmarshalWrapped()\n", outer)
			require.NoError(t, err)
			gotCommit, ok := outer.Event.(*flow.EpochCommit)
			require.True(t, ok)
			assert.DeepEqual(t, commit, gotCommit, comparePubKey)

			// VersionBeacon
			t.Logf("- debug: versionTable.ServiceEvent()=%+v\n", versionBeacon.ServiceEvent())
			b, err = msgpack.Marshal(versionBeacon.ServiceEvent())
			require.NoError(t, err)

			outer = new(flow.ServiceEvent)
			t.Logf("- debug: outer=%+v <-- before .UnmarshalWrapped()\n", outer)
			err = msgpack.Unmarshal(b, outer)
			t.Logf("- debug: outer=%+v <-- after .UnmarshalWrapped()\n", outer)
			require.NoError(t, err)
			gotVersionTable, ok := outer.Event.(*flow.VersionBeacon)
			require.True(t, ok)
			assert.DeepEqual(t, versionBeacon, gotVersionTable, comparePubKey)

			// ProtocolStateVersionUpgrade
			b, err = msgpack.Marshal(protocolVersionUpgrade.ServiceEvent())
			require.NoError(t, err)

			outer = new(flow.ServiceEvent)
			err = msgpack.Unmarshal(b, outer)
			require.NoError(t, err)
			gotProtocolVersionUpgrade, ok := outer.Event.(*flow.ProtocolStateVersionUpgrade)
			require.True(t, ok)
			assert.DeepEqual(t, protocolVersionUpgrade, gotProtocolVersionUpgrade)
		})
	})

	t.Run("cbor", func(t *testing.T) {
		t.Run("specific event types", func(t *testing.T) {
			// EpochSetup
			b, err := cborcodec.EncMode.Marshal(setup)
			require.NoError(t, err)

			gotSetup := new(flow.EpochSetup)
			err = cbor.Unmarshal(b, gotSetup)
			require.NoError(t, err)
			assert.DeepEqual(t, setup, gotSetup, comparePubKey)

			// EpochCommit
			b, err = cborcodec.EncMode.Marshal(commit)
			require.NoError(t, err)

			gotCommit := new(flow.EpochCommit)
			err = cbor.Unmarshal(b, gotCommit)
			require.NoError(t, err)
			assert.DeepEqual(t, commit, gotCommit, comparePubKey)

			// VersionBeacon
			b, err = cborcodec.EncMode.Marshal(versionBeacon)
			require.NoError(t, err)

			gotVersionTable := new(flow.VersionBeacon)
			err = cbor.Unmarshal(b, gotVersionTable)
			require.NoError(t, err)
			assert.DeepEqual(t, versionBeacon, gotVersionTable)

			// ProtocolStateVersionUpgrade
			b, err = cborcodec.EncMode.Marshal(protocolVersionUpgrade)
			require.NoError(t, err)

			gotProtocolVersionUpgrade := new(flow.ProtocolStateVersionUpgrade)
			err = cbor.Unmarshal(b, gotProtocolVersionUpgrade)
			require.NoError(t, err)
			assert.DeepEqual(t, protocolVersionUpgrade, gotProtocolVersionUpgrade)
		})

		t.Run("generic type", func(t *testing.T) {
			// EpochSetup
			t.Logf("- debug: setup.ServiceEvent()=%+v\n", setup.ServiceEvent())
			b, err := cborcodec.EncMode.Marshal(setup.ServiceEvent())
			require.NoError(t, err)

			outer := new(flow.ServiceEvent)
			t.Logf("- debug: outer=%+v <-- before .UnmarshalWrapped()\n", outer)
			err = cbor.Unmarshal(b, outer)
			t.Logf("- debug: outer=%+v <-- after .UnmarshalWrapped()\n", outer)
			require.NoError(t, err)
			gotSetup, ok := outer.Event.(*flow.EpochSetup)
			require.True(t, ok)
			assert.DeepEqual(t, setup, gotSetup, comparePubKey)

			// EpochCommit
			b, err = cborcodec.EncMode.Marshal(commit.ServiceEvent())
			require.NoError(t, err)

			outer = new(flow.ServiceEvent)
			err = cbor.Unmarshal(b, outer)
			require.NoError(t, err)
			gotCommit, ok := outer.Event.(*flow.EpochCommit)
			require.True(t, ok)
			assert.DeepEqual(t, commit, gotCommit, comparePubKey)

			// VersionBeacon
			t.Logf("- debug: setup.ServiceEvent()=%+v\n", versionBeacon.ServiceEvent())
			b, err = cborcodec.EncMode.Marshal(versionBeacon.ServiceEvent())
			require.NoError(t, err)

			outer = new(flow.ServiceEvent)
			err = cbor.Unmarshal(b, outer)
			require.NoError(t, err)
			gotVersionTable, ok := outer.Event.(*flow.VersionBeacon)
			require.True(t, ok)
			assert.DeepEqual(t, versionBeacon, gotVersionTable)

			// ProtocolStateVersionUpgrade
			b, err = cborcodec.EncMode.Marshal(protocolVersionUpgrade.ServiceEvent())
			require.NoError(t, err)

			outer = new(flow.ServiceEvent)
			err = cbor.Unmarshal(b, outer)
			require.NoError(t, err)
			gotProtocolVersionUpgrade, ok := outer.Event.(*flow.ProtocolStateVersionUpgrade)
			require.True(t, ok)
			assert.DeepEqual(t, protocolVersionUpgrade, gotProtocolVersionUpgrade)
		})
	})
}
