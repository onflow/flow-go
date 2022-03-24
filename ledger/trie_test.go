package ledger

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPayloadEquals tests equality of payloads.  It tests:
// - equality of empty, nil, and not-empty payloads
// - equality of payloads with different keys and same value
// - equality of payloads with same key and different values
// - and etc.
func TestPayloadEquals(t *testing.T) {
	nilPayload := (*Payload)(nil)
	emptyPayload := EmptyPayload()

	t.Run("nil vs empty", func(t *testing.T) {
		require.True(t, nilPayload.Equals(emptyPayload))
		require.True(t, emptyPayload.Equals(nilPayload))
	})

	t.Run("nil vs nil", func(t *testing.T) {
		require.True(t, nilPayload.Equals(nilPayload))
	})

	t.Run("empty vs empty", func(t *testing.T) {
		require.True(t, emptyPayload.Equals(emptyPayload))
	})

	t.Run("empty vs non-empty", func(t *testing.T) {
		p := &Payload{
			Key:   Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			Value: []byte{0x03, 0x04},
		}
		require.False(t, emptyPayload.Equals(p))
		require.False(t, p.Equals(emptyPayload))
	})

	t.Run("nil vs non-empty", func(t *testing.T) {
		p := &Payload{
			Key:   Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			Value: []byte{0x03, 0x04},
		}
		require.False(t, nilPayload.Equals(p))
		require.False(t, p.Equals(nilPayload))
	})

	// Payloads with different keys and same value are equal, because
	// payload key is auxiliary info (it isn't included in comparison).
	t.Run("different key same value", func(t *testing.T) {
		value := []byte{0x03, 0x04}

		p := &Payload{
			Key:   Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			Value: value,
		}
		// p1.Key.KeyParts[0].Type is different
		p1 := &Payload{
			Key:   Key{KeyParts: []KeyPart{{2, []byte{0x01, 0x02}}}},
			Value: value,
		}
		// p2.Key.KeyParts[0].Value is different
		p2 := &Payload{
			Key:   Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02, 0x03}}}},
			Value: value,
		}
		// len(p3.Key.KeyParts) is different
		p3 := &Payload{
			Key: Key{KeyParts: []KeyPart{
				{1, []byte{0x01, 0x02}},
				{2, []byte{0x03, 0x04}}},
			},
			Value: value,
		}
		require.True(t, p.Equals(p1))
		require.True(t, p.Equals(p2))
		require.True(t, p.Equals(p3))
	})

	t.Run("different key empty value", func(t *testing.T) {
		value := []byte{}

		p := &Payload{
			Key:   Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			Value: value,
		}
		// p1.Key.KeyParts[0].Type is different
		p1 := &Payload{
			Key:   Key{KeyParts: []KeyPart{{2, []byte{0x01, 0x02}}}},
			Value: value,
		}
		// p2.Key.KeyParts[0].Value is different
		p2 := &Payload{
			Key:   Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02, 0x03}}}},
			Value: value,
		}
		// len(p3.Key.KeyParts) is different
		p3 := &Payload{
			Key: Key{KeyParts: []KeyPart{
				{1, []byte{0x01, 0x02}},
				{2, []byte{0x03, 0x04}}},
			},
			Value: value,
		}
		require.True(t, p.Equals(p1))
		require.True(t, p.Equals(p2))
		require.True(t, p.Equals(p3))
	})

	t.Run("same key different value", func(t *testing.T) {
		key := Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}}

		p := &Payload{
			Key:   key,
			Value: []byte{0x03, 0x04},
		}
		// p1.Value is nil
		p1 := &Payload{
			Key: key,
		}
		// p2.Value is empty
		p2 := &Payload{
			Key:   key,
			Value: []byte{},
		}
		// p3.Value length is different
		p3 := &Payload{
			Key:   key,
			Value: []byte{0x03},
		}
		// p4.Value data is different
		p4 := &Payload{
			Key:   key,
			Value: []byte{0x03, 0x05},
		}
		require.False(t, p.Equals(p1))
		require.False(t, p.Equals(p2))
		require.False(t, p.Equals(p3))
		require.False(t, p.Equals(p4))
	})

	t.Run("same key same value", func(t *testing.T) {
		p1 := &Payload{
			Key:   Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			Value: []byte{0x03, 0x04},
		}
		p2 := &Payload{
			Key:   Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			Value: []byte{0x03, 0x04},
		}
		require.True(t, p1.Equals(p2))
		require.True(t, p2.Equals(p1))
	})
}
