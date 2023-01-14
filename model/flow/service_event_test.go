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
	versionBeacon := unittest.VersionBeaconFixture(3, 21)

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

			// VersionTable
			b, err = json.Marshal(versionBeacon)
			require.NoError(t, err)

			gotVersionBeacon := new(flow.VersionBeacon)
			err = json.Unmarshal(b, gotVersionBeacon)
			require.NoError(t, err)
			assert.DeepEqual(t, versionBeacon, gotVersionBeacon)
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
			t.Logf("- debug: outer=%+v <-- before .Unmarshal()\n", outer)
			err = json.Unmarshal(b, outer)
			t.Logf("- debug: outer=%+v <-- after .Unmarshal()\n", outer)
			require.NoError(t, err)
			gotCommit, ok := outer.Event.(*flow.EpochCommit)
			require.True(t, ok)
			assert.DeepEqual(t, commit, gotCommit, comparePubKey)

			// VersionTable
			t.Logf("- debug: versionBeacon.ServiceEvent()=%+v\n", versionBeacon.ServiceEvent())
			b, err = json.Marshal(versionBeacon.ServiceEvent())
			require.NoError(t, err)

			outer = new(flow.ServiceEvent)
			t.Logf("- debug: outer=%+v <-- before .Unmarshal()\n", outer)
			err = json.Unmarshal(b, outer)
			t.Logf("- debug: outer=%+v <-- after .Unmarshal()\n", outer)
			require.NoError(t, err)
			gotVersionTable, ok := outer.Event.(*flow.VersionBeacon)
			require.True(t, ok)
			assert.DeepEqual(t, versionBeacon, gotVersionTable, comparePubKey)
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
			assert.DeepEqual(t, versionBeacon, gotVersionTable, comparePubKey)
		})

		t.Run("generic type", func(t *testing.T) {
			// EpochSetup
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
			t.Logf("- debug: outer=%+v <-- before .Unmarshal()\n", outer)
			err = msgpack.Unmarshal(b, outer)
			t.Logf("- debug: outer=%+v <-- after .Unmarshal()\n", outer)
			require.NoError(t, err)
			gotCommit, ok := outer.Event.(*flow.EpochCommit)
			require.True(t, ok)
			assert.DeepEqual(t, commit, gotCommit, comparePubKey)

			// VersionTable
			t.Logf("- debug: versionTable.ServiceEvent()=%+v\n", versionBeacon.ServiceEvent())
			b, err = msgpack.Marshal(versionBeacon.ServiceEvent())
			require.NoError(t, err)

			outer = new(flow.ServiceEvent)
			t.Logf("- debug: outer=%+v <-- before .Unmarshal()\n", outer)
			err = msgpack.Unmarshal(b, outer)
			t.Logf("- debug: outer=%+v <-- after .Unmarshal()\n", outer)
			require.NoError(t, err)
			gotVersionTable, ok := outer.Event.(*flow.VersionBeacon)
			require.True(t, ok)
			assert.DeepEqual(t, versionBeacon, gotVersionTable, comparePubKey)
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

			// VersionTable
			b, err = cborcodec.EncMode.Marshal(versionBeacon)
			require.NoError(t, err)

			gotVersionTable := new(flow.VersionBeacon)
			err = cbor.Unmarshal(b, gotVersionTable)
			require.NoError(t, err)
			assert.DeepEqual(t, versionBeacon, gotVersionTable, comparePubKey)
		})

		t.Run("generic type", func(t *testing.T) {
			// EpochSetup
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

			// VersionTable
			t.Logf("- debug: setup.ServiceEvent()=%+v\n", versionBeacon.ServiceEvent())
			b, err = cborcodec.EncMode.Marshal(versionBeacon.ServiceEvent())
			require.NoError(t, err)

			outer = new(flow.ServiceEvent)
			err = cbor.Unmarshal(b, outer)
			require.NoError(t, err)
			gotVersionTable, ok := outer.Event.(*flow.VersionBeacon)
			require.True(t, ok)
			assert.DeepEqual(t, versionBeacon, gotVersionTable, comparePubKey)
		})
	})
}

func TestEquality(t *testing.T) {

	setupA := unittest.EpochSetupFixture().ServiceEvent()
	setupBevent := unittest.EpochSetupFixture()
	commitA := unittest.EpochCommitFixture().ServiceEvent()
	commitBevent := unittest.EpochCommitFixture()
	versionTableA := unittest.VersionBeaconFixture(2, 21).ServiceEvent()
	versionTableBevent := unittest.VersionBeaconFixture(2, 22)

	// modify B versions
	setupBevent.Counter += 1
	commitBevent.Counter += 1
	versionTableBevent.Sequence += 1

	setupB := setupBevent.ServiceEvent()
	commitB := commitBevent.ServiceEvent()
	versionTableB := versionTableBevent.ServiceEvent()

	t.Run("epoch setup", func(t *testing.T) {
		t.Parallel()

		setupAvsSetupA, err := setupA.EqualTo(&setupA)
		require.NoError(t, err)
		require.True(t, setupAvsSetupA)

		setupAvsSetupB, err := setupA.EqualTo(&setupB)
		require.NoError(t, err)
		require.False(t, setupAvsSetupB)

		setupBvsSetupA, err := setupB.EqualTo(&setupA)
		require.NoError(t, err)
		require.False(t, setupBvsSetupA)
	})

	t.Run("epoch commit", func(t *testing.T) {
		t.Parallel()

		commitAvsCommitA, err := commitA.EqualTo(&commitA)
		require.NoError(t, err)
		require.True(t, commitAvsCommitA)

		commitAvsCommitB, err := commitA.EqualTo(&commitB)
		require.NoError(t, err)
		require.False(t, commitAvsCommitB)

		commitBvsCommitA, err := commitB.EqualTo(&commitA)
		require.NoError(t, err)
		require.False(t, commitBvsCommitA)
	})

	t.Run("version beacon", func(t *testing.T) {

		t.Parallel()

		versionAvsVersionA, err := versionTableA.EqualTo(&versionTableA)
		require.NoError(t, err)
		require.True(t, versionAvsVersionA)

		versionAvsVersionB, err := versionTableA.EqualTo(&versionTableB)
		require.NoError(t, err)
		require.False(t, versionAvsVersionB)

		versionBvsVersionA, err := versionTableB.EqualTo(&versionTableA)
		require.NoError(t, err)
		require.False(t, versionBvsVersionA)
	})

	t.Run("mixed versions never match", func(t *testing.T) {

		t.Parallel()

		versionAvsCommitA, err := versionTableA.EqualTo(&commitA)
		require.NoError(t, err)
		require.False(t, versionAvsCommitA)

		versionAvsEpochB, err := versionTableA.EqualTo(&commitB)
		require.NoError(t, err)
		require.False(t, versionAvsEpochB)

		setupBvsCommitA, err := setupB.EqualTo(&commitA)
		require.NoError(t, err)
		require.False(t, setupBvsCommitA)
	})
}
