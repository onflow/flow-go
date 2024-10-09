package flow_test

import (
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/google/go-cmp/cmp"
	gocmp "github.com/google/go-cmp/cmp"
	"github.com/onflow/crypto"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack"
	"gotest.tools/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEncodeDecode(t *testing.T) {
	setup := unittest.EpochSetupFixture()
	commit := unittest.EpochCommitFixture()
	versionBeacon := unittest.VersionBeaconFixture()
	protocolVersionUpgrade := unittest.ProtocolStateVersionUpgradeFixture()
	setEpochExtensionViewCount := &flow.SetEpochExtensionViewCount{Value: uint64(rand.Uint32())}
	ejectionEvent := &flow.EjectNode{NodeID: unittest.IdentifierFixture()}

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
			assertJsonConvert(t, setup, comparePubKey)

			// EpochCommit
			assertJsonConvert(t, commit, comparePubKey)

			// VersionBeacon
			assertJsonConvert(t, versionBeacon)

			// ProtocolStateVersionUpgrade
			assertJsonConvert(t, protocolVersionUpgrade)

			// SetEpochExtensionViewCount
			assertJsonConvert(t, setEpochExtensionViewCount)

			// EjectNode
			assertJsonConvert(t, ejectionEvent)
		})

		t.Run("generic type", func(t *testing.T) {
			// EpochSetup
			assertJsonGenericConvert(t, setup, comparePubKey)

			// EpochCommit
			assertJsonGenericConvert(t, commit, comparePubKey)

			// VersionBeacon
			assertJsonGenericConvert(t, versionBeacon)

			// ProtocolStateVersionUpgrade
			assertJsonGenericConvert(t, protocolVersionUpgrade)

			// SetEpochExtensionViewCount
			assertJsonGenericConvert(t, setEpochExtensionViewCount)

			// EjectNode
			assertJsonGenericConvert(t, ejectionEvent)
		})
	})

	t.Run("msgpack", func(t *testing.T) {
		t.Run("specific event types", func(t *testing.T) {
			// EpochSetup
			assertMsgPackConvert(t, setup, comparePubKey)

			// EpochCommit
			assertMsgPackConvert(t, commit, comparePubKey)

			// VersionBeacon
			assertMsgPackConvert(t, versionBeacon)

			// ProtocolStateVersionUpgrade
			assertMsgPackConvert(t, protocolVersionUpgrade)

			// SetEpochExtensionViewCount
			assertMsgPackConvert(t, setEpochExtensionViewCount)

			// EjectNode
			assertMsgPackConvert(t, ejectionEvent)
		})

		t.Run("generic type", func(t *testing.T) {
			// EpochSetup
			assertMsgPackGenericConvert(t, setup, comparePubKey)

			// EpochCommit
			assertMsgPackGenericConvert(t, commit, comparePubKey)

			// VersionBeacon
			assertMsgPackGenericConvert(t, versionBeacon, comparePubKey)

			// ProtocolStateVersionUpgrade
			assertMsgPackGenericConvert(t, protocolVersionUpgrade)

			// SetEpochExtensionViewCount
			assertMsgPackGenericConvert(t, setEpochExtensionViewCount)

			// EjectNode
			assertMsgPackGenericConvert(t, ejectionEvent)
		})
	})

	t.Run("cbor", func(t *testing.T) {
		t.Run("specific event types", func(t *testing.T) {
			// EpochSetup
			assertCborConvert(t, setup, comparePubKey)

			// EpochCommit
			assertCborConvert(t, commit, comparePubKey)

			// VersionBeacon
			assertCborConvert(t, versionBeacon)

			// ProtocolStateVersionUpgrade
			assertCborConvert(t, protocolVersionUpgrade)

			// SetEpochExtensionViewCount
			assertCborConvert(t, setEpochExtensionViewCount)

			// EjectNode
			assertCborConvert(t, ejectionEvent)
		})

		t.Run("generic type", func(t *testing.T) {
			// EpochSetup
			assertCborGenericConvert(t, setup, comparePubKey)

			// EpochCommit
			assertCborGenericConvert(t, commit, comparePubKey)

			// VersionBeacon
			assertCborGenericConvert(t, versionBeacon)

			// ProtocolStateVersionUpgrade
			assertCborGenericConvert(t, protocolVersionUpgrade)

			// SetEpochExtensionViewCount
			assertCborGenericConvert(t, setEpochExtensionViewCount)

			// EjectNode
			assertCborGenericConvert(t, ejectionEvent)
		})
	})
}

// ServiceEventCapable is an interface to convert a specific event type to a generic ServiceEvent type.
type ServiceEventCapable interface {
	ServiceEvent() flow.ServiceEvent
}

// assertJsonConvert asserts that value `v` can be marshaled and unmarshaled to/from JSON.
func assertJsonConvert[T any](t *testing.T, v *T, opts ...gocmp.Option) {
	b, err := json.Marshal(v)
	require.NoError(t, err)

	got := new(T)
	err = json.Unmarshal(b, got)
	require.NoError(t, err)
	assert.DeepEqual(t, v, got, opts...)
}

// assertJsonGenericConvert asserts that value `v` can be marshaled and unmarshaled to/from JSON as a generic ServiceEvent.
func assertJsonGenericConvert[T ServiceEventCapable](t *testing.T, v T, opts ...gocmp.Option) {
	b, err := json.Marshal(v.ServiceEvent())
	require.NoError(t, err)

	outer := new(flow.ServiceEvent)
	err = json.Unmarshal(b, outer)
	require.NoError(t, err)
	got, ok := outer.Event.(T)
	require.True(t, ok)
	assert.DeepEqual(t, v, got, opts...)
}

// assertMsgPackConvert asserts that value `v` can be marshaled and unmarshaled to/from MessagePack.
func assertMsgPackConvert[T any](t *testing.T, v *T, opts ...gocmp.Option) {
	b, err := msgpack.Marshal(v)
	require.NoError(t, err)

	got := new(T)
	err = msgpack.Unmarshal(b, got)
	require.NoError(t, err)
	assert.DeepEqual(t, v, got, opts...)
}

// assertMsgPackGenericConvert asserts that value `v` can be marshaled and unmarshaled to/from MessagePack as a generic ServiceEvent.
func assertMsgPackGenericConvert[T ServiceEventCapable](t *testing.T, v T, opts ...gocmp.Option) {
	b, err := msgpack.Marshal(v.ServiceEvent())
	require.NoError(t, err)

	outer := new(flow.ServiceEvent)
	err = msgpack.Unmarshal(b, outer)
	require.NoError(t, err)
	got, ok := outer.Event.(T)
	require.True(t, ok)
	assert.DeepEqual(t, v, got, opts...)
}

// assertCborConvert asserts that value `v` can be marshaled and unmarshaled to/from CBOR.
func assertCborConvert[T any](t *testing.T, v *T, opts ...gocmp.Option) {
	b, err := cbor.Marshal(v)
	require.NoError(t, err)

	got := new(T)
	err = cbor.Unmarshal(b, got)
	require.NoError(t, err)
	assert.DeepEqual(t, v, got, opts...)
}

// assertCborGenericConvert asserts that value `v` can be marshaled and unmarshaled to/from CBOR as a generic ServiceEvent.
func assertCborGenericConvert[T ServiceEventCapable](t *testing.T, v T, opts ...gocmp.Option) {
	b, err := cbor.Marshal(v.ServiceEvent())
	require.NoError(t, err)

	outer := new(flow.ServiceEvent)
	err = cbor.Unmarshal(b, outer)
	require.NoError(t, err)
	got, ok := outer.Event.(T)
	require.True(t, ok)
	assert.DeepEqual(t, v, got, opts...)
}
