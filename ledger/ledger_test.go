package ledger

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// this benchmark can run with this command:
//  go test -run=CanonicalForm -bench=.

//nolint
func BenchmarkCanonicalForm(b *testing.B) {

	constant := 10

	keyParts := make([]KeyPart, 0, 200)

	for i := 0; i < 16; i++ {
		keyParts = append(keyParts, KeyPart{})
		keyParts[i].Value = []byte("somedomain1")
		keyParts[i].Type = 1234
	}

	requiredLen := constant * len(keyParts)
	for _, kp := range keyParts {
		requiredLen += len(kp.Value)
	}

	retval := make([]byte, 0, requiredLen)

	for _, kp := range keyParts {
		typeNumber := strconv.Itoa(int(kp.Type))

		retval = append(retval, byte('/'))
		retval = append(retval, []byte(typeNumber)...)
		retval = append(retval, byte('/'))
		retval = append(retval, kp.Value...)
	}
}

func BenchmarkOriginalCanonicalForm(b *testing.B) {
	keyParts := make([]KeyPart, 0, 200)

	for i := 0; i < 16; i++ {
		keyParts = append(keyParts, KeyPart{})
		keyParts[i].Value = []byte("somedomain1")
		keyParts[i].Type = 1234
	}

	ret := ""

	for _, kp := range keyParts {
		ret += fmt.Sprintf("/%d/%v", kp.Type, string(kp.Value))
	}
}

// TestPayloadKeyEquals tests whether keys are equal.
// It tests equality of empty, nil, and not-empty keys.
// Empty key and nil key should be equal.
func TestPayloadKeyEquals(t *testing.T) {

	nilKey := (*Key)(nil)
	emptyKey := &Key{}

	t.Run("nil vs empty", func(t *testing.T) {
		require.True(t, nilKey.Equals(emptyKey))
		require.True(t, emptyKey.Equals(nilKey))
	})

	t.Run("nil vs nil", func(t *testing.T) {
		require.True(t, nilKey.Equals(nilKey))
	})

	t.Run("empty vs empty", func(t *testing.T) {
		require.True(t, emptyKey.Equals(emptyKey))
	})

	t.Run("empty vs not-empty", func(t *testing.T) {
		k := &Key{
			KeyParts: []KeyPart{
				{0, []byte{0x01, 0x02}},
			},
		}
		require.False(t, emptyKey.Equals(k))
		require.False(t, k.Equals(emptyKey))
	})

	t.Run("nil vs not-empty", func(t *testing.T) {
		k := &Key{
			KeyParts: []KeyPart{
				{0, []byte{0x01, 0x02}},
			},
		}
		require.False(t, nilKey.Equals(k))
		require.False(t, k.Equals(nilKey))
	})

	t.Run("num of KeyParts different", func(t *testing.T) {
		k1 := &Key{
			KeyParts: []KeyPart{
				{0, []byte{0x01, 0x02}},
			},
		}
		k2 := &Key{
			KeyParts: []KeyPart{
				{0, []byte{0x01, 0x02}},
				{1, []byte{0x03, 0x04}},
			},
		}
		require.False(t, k1.Equals(k2))
		require.False(t, k2.Equals(k1))
	})

	t.Run("KeyPart different", func(t *testing.T) {
		k := &Key{
			KeyParts: []KeyPart{
				{0, []byte{0x01, 0x02}},
				{1, []byte{0x03, 0x04}},
			},
		}
		// k1.KeyParts[1].Type is different.
		k1 := &Key{
			KeyParts: []KeyPart{
				{0, []byte{0x01, 0x02}},
				{2, []byte{0x03, 0x04}},
			},
		}
		// k2.KeyParts[1].Value is different (same length).
		k2 := &Key{
			KeyParts: []KeyPart{
				{0, []byte{0x01, 0x02}},
				{1, []byte{0x03, 0x05}},
			},
		}
		// k3.KeyParts[1].Value is different (different length).
		k3 := &Key{
			KeyParts: []KeyPart{
				{0, []byte{0x01, 0x02}},
				{1, []byte{0x03}},
			},
		}
		// k4.KeyParts has different order.
		k4 := &Key{
			KeyParts: []KeyPart{
				{1, []byte{0x03, 0x04}},
				{0, []byte{0x01, 0x02}},
			},
		}
		require.False(t, k.Equals(k1))
		require.False(t, k.Equals(k2))
		require.False(t, k.Equals(k3))
		require.False(t, k.Equals(k4))
	})

	t.Run("same", func(t *testing.T) {
		k1 := &Key{
			KeyParts: []KeyPart{
				{0, []byte{0x01, 0x02}},
			},
		}
		k2 := &Key{
			KeyParts: []KeyPart{
				{0, []byte{0x01, 0x02}},
			},
		}
		require.True(t, k1.Equals(k2))
		require.True(t, k2.Equals(k1))

		k3 := &Key{
			KeyParts: []KeyPart{
				{0, []byte{0x01, 0x02}},
				{1, []byte{0x03, 0x04}},
			},
		}
		k4 := &Key{
			KeyParts: []KeyPart{
				{0, []byte{0x01, 0x02}},
				{1, []byte{0x03, 0x04}},
			},
		}
		require.True(t, k3.Equals(k4))
		require.True(t, k4.Equals(k3))
	})
}

// TestPayloadValueEquals tests whether values are equal.
// It tests equality of empty, nil, and not-empty values.
// Empty value and nil value should be equal.
func TestPayloadValueEquals(t *testing.T) {

	nilValue := (Value)(nil)
	emptyValue := Value{}

	t.Run("nil vs empty", func(t *testing.T) {
		require.True(t, nilValue.Equals(emptyValue))
		require.True(t, emptyValue.Equals(nilValue))
	})

	t.Run("nil vs nil", func(t *testing.T) {
		require.True(t, nilValue.Equals(nilValue))
	})

	t.Run("empty vs empty", func(t *testing.T) {
		require.True(t, emptyValue.Equals(emptyValue))
	})

	t.Run("empty vs non-empty", func(t *testing.T) {
		v := Value{0x01, 0x02}
		require.False(t, emptyValue.Equals(v))
		require.False(t, v.Equals(emptyValue))
	})

	t.Run("nil vs non-empty", func(t *testing.T) {
		v := Value{0x01, 0x02}
		require.False(t, nilValue.Equals(v))
		require.False(t, v.Equals(nilValue))
	})

	t.Run("length different", func(t *testing.T) {
		v1 := Value{0x01, 0x02}
		v2 := Value{0x01, 0x02, 0x03}
		require.False(t, v1.Equals(v2))
		require.False(t, v2.Equals(v1))
	})

	t.Run("data different", func(t *testing.T) {
		v1 := Value{0x01, 0x02}
		v2 := Value{0x01, 0x03}
		require.False(t, v1.Equals(v2))
		require.False(t, v2.Equals(v1))
	})

	t.Run("same", func(t *testing.T) {
		v1 := Value{0x01, 0x02}
		v2 := Value{0x01, 0x02}
		require.True(t, v1.Equals(v2))
		require.True(t, v2.Equals(v1))
	})
}

// TestPayloadEquals tests whether payloads are equal.
// It tests equality of empty, nil, and not-empty payloads.
// Empty payload and nil payload should be equal.
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

	t.Run("different key", func(t *testing.T) {
		p := &Payload{
			Key:   Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			Value: []byte{0x03, 0x04},
		}
		// p1.Key.KeyParts[0].Type is different
		p1 := &Payload{
			Key:   Key{KeyParts: []KeyPart{{2, []byte{0x01, 0x02}}}},
			Value: []byte{0x03, 0x04},
		}
		// p2.Key.KeyParts[0].Value is different
		p2 := &Payload{
			Key:   Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02, 0x03}}}},
			Value: []byte{0x03, 0x04},
		}
		// len(p3.Key.KeyParts) is different
		p3 := &Payload{
			Key: Key{KeyParts: []KeyPart{
				{1, []byte{0x01, 0x02}},
				{2, []byte{0x03, 0x04}}},
			},
			Value: []byte{0x03, 0x04},
		}
		require.False(t, p.Equals(p1))
		require.False(t, p.Equals(p2))
		require.False(t, p.Equals(p3))
	})

	t.Run("different value", func(t *testing.T) {
		p := &Payload{
			Key:   Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			Value: []byte{0x03, 0x04},
		}
		// p1.Value is nil
		p1 := &Payload{
			Key: Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
		}
		// p2.Value is empty
		p2 := &Payload{
			Key:   Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			Value: []byte{},
		}
		// p3.Value length is different
		p3 := &Payload{
			Key:   Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			Value: []byte{0x03},
		}
		// p4.Value data is different
		p4 := &Payload{
			Key:   Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			Value: []byte{0x03, 0x05},
		}
		require.False(t, p.Equals(p1))
		require.False(t, p.Equals(p2))
		require.False(t, p.Equals(p3))
		require.False(t, p.Equals(p4))
	})

	t.Run("same", func(t *testing.T) {
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
