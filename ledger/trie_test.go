package ledger

import (
	"encoding/json"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
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
		p := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			[]byte{0x03, 0x04},
		)
		require.False(t, emptyPayload.Equals(p))
		require.False(t, p.Equals(emptyPayload))
	})

	t.Run("nil vs non-empty", func(t *testing.T) {
		p := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			[]byte{0x03, 0x04},
		)
		require.False(t, nilPayload.Equals(p))
		require.False(t, p.Equals(nilPayload))
	})

	t.Run("different key same value", func(t *testing.T) {
		value := []byte{0x03, 0x04}

		p := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			value,
		)
		// p1.Key.KeyParts[0].Type is different
		p1 := NewPayload(
			Key{KeyParts: []KeyPart{{2, []byte{0x01, 0x02}}}},
			value,
		)
		// p2.Key.KeyParts[0].Value is different
		p2 := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02, 0x03}}}},
			value,
		)
		// len(p3.Key.KeyParts) is different
		p3 := NewPayload(
			Key{KeyParts: []KeyPart{
				{1, []byte{0x01, 0x02}},
				{2, []byte{0x03, 0x04}}},
			},
			value,
		)
		require.False(t, p.Equals(p1))
		require.False(t, p.Equals(p2))
		require.False(t, p.Equals(p3))
	})

	t.Run("different key empty value", func(t *testing.T) {
		value := []byte{}

		p := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			value,
		)
		// p1.Key.KeyParts[0].Type is different
		p1 := NewPayload(
			Key{KeyParts: []KeyPart{{2, []byte{0x01, 0x02}}}},
			value,
		)
		// p2.Key.KeyParts[0].Value is different
		p2 := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02, 0x03}}}},
			value,
		)
		// len(p3.Key.KeyParts) is different
		p3 := NewPayload(
			Key{KeyParts: []KeyPart{
				{1, []byte{0x01, 0x02}},
				{2, []byte{0x03, 0x04}}},
			},
			value,
		)
		require.False(t, p.Equals(p1))
		require.False(t, p.Equals(p2))
		require.False(t, p.Equals(p3))
	})

	t.Run("same key different value", func(t *testing.T) {
		key := Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}}

		p := NewPayload(
			key,
			[]byte{0x03, 0x04},
		)
		// p1.Value is nil
		p1 := NewPayload(
			key,
			nil,
		)
		// p2.Value is empty
		p2 := NewPayload(
			key,
			[]byte{},
		)
		// p3.Value length is different
		p3 := NewPayload(
			key,
			[]byte{0x03},
		)
		// p4.Value data is different
		p4 := NewPayload(
			key,
			[]byte{0x03, 0x05},
		)
		require.False(t, p.Equals(p1))
		require.False(t, p.Equals(p2))
		require.False(t, p.Equals(p3))
		require.False(t, p.Equals(p4))
	})

	t.Run("same key same value", func(t *testing.T) {
		p1 := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			[]byte{0x03, 0x04},
		)
		p2 := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			[]byte{0x03, 0x04},
		)
		require.True(t, p1.Equals(p2))
		require.True(t, p2.Equals(p1))
	})
}

// TestPayloadValueEquals tests equality of payload values.  It tests:
// - equality of empty, nil, and not-empty payloads
// - equality of payloads with different keys and same value
// - equality of payloads with same key and different values
// - and etc.
func TestPayloadValuEquals(t *testing.T) {
	nilPayload := (*Payload)(nil)
	emptyPayload := EmptyPayload()

	t.Run("nil vs empty", func(t *testing.T) {
		require.True(t, nilPayload.ValueEquals(emptyPayload))
		require.True(t, emptyPayload.ValueEquals(nilPayload))
	})

	t.Run("nil vs nil", func(t *testing.T) {
		require.True(t, nilPayload.ValueEquals(nilPayload))
	})

	t.Run("empty vs empty", func(t *testing.T) {
		require.True(t, emptyPayload.ValueEquals(emptyPayload))
	})

	t.Run("empty vs non-empty", func(t *testing.T) {
		p := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			[]byte{0x03, 0x04},
		)
		require.False(t, emptyPayload.ValueEquals(p))
		require.False(t, p.ValueEquals(emptyPayload))
	})

	t.Run("nil vs non-empty", func(t *testing.T) {
		p := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			[]byte{0x03, 0x04},
		)
		require.False(t, nilPayload.ValueEquals(p))
		require.False(t, p.ValueEquals(nilPayload))
	})

	t.Run("different key same value", func(t *testing.T) {
		value := []byte{0x03, 0x04}

		p := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			value,
		)
		// p1.Key.KeyParts[0].Type is different
		p1 := NewPayload(
			Key{KeyParts: []KeyPart{{2, []byte{0x01, 0x02}}}},
			value,
		)
		// p2.Key.KeyParts[0].Value is different
		p2 := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02, 0x03}}}},
			value,
		)
		// len(p3.Key.KeyParts) is different
		p3 := NewPayload(
			Key{KeyParts: []KeyPart{
				{1, []byte{0x01, 0x02}},
				{2, []byte{0x03, 0x04}}},
			},
			value,
		)
		require.True(t, p.ValueEquals(p1))
		require.True(t, p.ValueEquals(p2))
		require.True(t, p.ValueEquals(p3))
	})

	t.Run("different key empty value", func(t *testing.T) {
		value := []byte{}

		p := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			value,
		)
		// p1.Key.KeyParts[0].Type is different
		p1 := NewPayload(
			Key{KeyParts: []KeyPart{{2, []byte{0x01, 0x02}}}},
			value,
		)
		// p2.Key.KeyParts[0].Value is different
		p2 := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02, 0x03}}}},
			value,
		)
		// len(p3.Key.KeyParts) is different
		p3 := NewPayload(
			Key{KeyParts: []KeyPart{
				{1, []byte{0x01, 0x02}},
				{2, []byte{0x03, 0x04}}},
			},
			value,
		)
		require.True(t, p.ValueEquals(p1))
		require.True(t, p.ValueEquals(p2))
		require.True(t, p.ValueEquals(p3))
	})

	t.Run("same key different value", func(t *testing.T) {
		key := Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}}

		p := NewPayload(
			key,
			[]byte{0x03, 0x04},
		)
		// p1.Value is nil
		p1 := NewPayload(
			key,
			nil,
		)
		// p2.Value is empty
		p2 := NewPayload(
			key,
			[]byte{},
		)
		// p3.Value length is different
		p3 := NewPayload(
			key,
			[]byte{0x03},
		)
		// p4.Value data is different
		p4 := NewPayload(
			key,
			[]byte{0x03, 0x05},
		)
		require.False(t, p.ValueEquals(p1))
		require.False(t, p.ValueEquals(p2))
		require.False(t, p.ValueEquals(p3))
		require.False(t, p.ValueEquals(p4))
	})

	t.Run("same key same value", func(t *testing.T) {
		p1 := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			[]byte{0x03, 0x04},
		)
		p2 := NewPayload(
			Key{KeyParts: []KeyPart{{1, []byte{0x01, 0x02}}}},
			[]byte{0x03, 0x04},
		)
		require.True(t, p1.ValueEquals(p2))
		require.True(t, p2.ValueEquals(p1))
	})
}

func TestPayloadKey(t *testing.T) {
	t.Run("nil payload", func(t *testing.T) {
		var p *Payload
		k, err := p.Key()
		require.NoError(t, err)
		require.Equal(t, Key{}, k)

		_, err = p.Address()
		require.Error(t, err)
	})
	t.Run("empty payload", func(t *testing.T) {
		p := Payload{}
		k, err := p.Key()
		require.NoError(t, err)
		require.Equal(t, Key{}, k)

		_, err = p.Address()
		require.Error(t, err)
	})
	t.Run("empty key", func(t *testing.T) {
		p := NewPayload(Key{}, Value{})
		k, err := p.Key()
		require.NoError(t, err)
		require.Equal(t, Key{}, k)

		_, err = p.Address()
		require.Error(t, err)
	})
	t.Run("global key", func(t *testing.T) {
		key := Key{KeyParts: []KeyPart{{Type: 0, Value: []byte{}}, {Type: 1, Value: []byte("def")}}}
		value := Value([]byte{0, 1, 2})
		p := NewPayload(key, value)
		k, err := p.Key()
		require.NoError(t, err)
		require.Equal(t, key, k)

		addr, err := p.Address()
		require.NoError(t, err)
		require.Equal(t, flow.EmptyAddress, addr)
	})
	t.Run("key", func(t *testing.T) {
		address := []byte{1, 2, 3, 4, 5, 6, 7, 8}
		key := Key{KeyParts: []KeyPart{{Type: 0, Value: address}, {Type: 1, Value: []byte("def")}}}
		value := Value([]byte{0, 1, 2})
		p := NewPayload(key, value)
		k, err := p.Key()
		require.NoError(t, err)
		require.Equal(t, key, k)

		addr, err := p.Address()
		require.NoError(t, err)
		require.Equal(t, flow.Address(address), addr)
	})
}

func TestPayloadValue(t *testing.T) {
	t.Run("nil payload", func(t *testing.T) {
		var p *Payload
		require.Equal(t, 0, p.Value().Size())
	})
	t.Run("empty payload", func(t *testing.T) {
		p := Payload{}
		require.Equal(t, 0, p.Value().Size())
	})
	t.Run("empty value", func(t *testing.T) {
		key := Key{KeyParts: []KeyPart{{Type: 0, Value: []byte("abc")}, {Type: 1, Value: []byte("def")}}}
		p := NewPayload(key, Value{})
		require.Equal(t, 0, p.Value().Size())
	})
	t.Run("value", func(t *testing.T) {
		key := Key{KeyParts: []KeyPart{{Type: 0, Value: []byte("abc")}, {Type: 1, Value: []byte("def")}}}
		value := Value([]byte{0, 1, 2})
		p := NewPayload(key, value)
		require.Equal(t, value, p.Value())
	})
}

func TestPayloadJSONSerialization(t *testing.T) {
	t.Run("nil payload", func(t *testing.T) {
		encoded := []byte("null")

		var p *Payload
		b, err := json.Marshal(p)
		require.NoError(t, err)
		require.Equal(t, encoded, b)

		var p2 *Payload
		err = json.Unmarshal(b, &p2)
		require.NoError(t, err)
		require.Equal(t, p, p2)
	})

	t.Run("empty payload", func(t *testing.T) {
		encoded := []byte(`{"Key":{"KeyParts":null},"Value":""}`)

		var p Payload
		b, err := json.Marshal(p)
		require.NoError(t, err)
		require.Equal(t, encoded, b)

		var p2 Payload
		err = json.Unmarshal(b, &p2)
		require.NoError(t, err)

		// Reset b to make sure that p2 doesn't share underlying data with JSON-encoded data.
		for i := range b {
			b[i] = 0
		}

		require.True(t, p2.IsEmpty())

		k2, err := p2.Key()
		require.NoError(t, err)
		require.True(t, k2.Equals(&Key{}))

		v2 := p2.Value()
		require.True(t, v2.Equals(Value{}))

		// after decoding, the value will be normalized to []byte{}
		// which will be different from the original format
		require.True(t, p.value == nil)
		require.True(t, p2.value != nil)
	})

	t.Run("empty key", func(t *testing.T) {
		encoded := []byte(`{"Key":{"KeyParts":null},"Value":"000102"}`)

		k := Key{}
		v := []byte{0, 1, 2}
		p := NewPayload(k, v)
		b, err := json.Marshal(p)
		require.NoError(t, err)
		require.Equal(t, encoded, b)

		var p2 Payload
		err = json.Unmarshal(b, &p2)
		require.NoError(t, err)

		// Reset b to make sure that p2 doesn't share underlying data with JSON-encoded data.
		for i := range b {
			b[i] = 0
		}

		require.True(t, p.Equals(&p2))

		k2, err := p2.Key()
		require.NoError(t, err)
		require.True(t, k2.Equals(&k))

		v2 := p2.Value()
		require.True(t, v2.Equals(v))
	})

	t.Run("empty value", func(t *testing.T) {
		encoded := []byte(`{"Key":{"KeyParts":[{"Type":1,"Value":"0102"}]},"Value":""}`)

		k := Key{KeyParts: []KeyPart{{1, []byte{1, 2}}}}
		var v Value
		p := NewPayload(k, v)
		b, err := json.Marshal(p)
		require.NoError(t, err)
		require.Equal(t, encoded, b)

		var p2 Payload
		err = json.Unmarshal(b, &p2)
		require.NoError(t, err)

		// Reset b to make sure that p2 doesn't share underlying data with JSON-encoded data.
		for i := range b {
			b[i] = 0
		}

		require.True(t, p.Equals(&p2))

		k2, err := p2.Key()
		require.NoError(t, err)
		require.True(t, k2.Equals(&k))

		v2 := p2.Value()
		require.True(t, v2.Equals(v))
	})

	t.Run("payload", func(t *testing.T) {
		encoded := []byte(`{"Key":{"KeyParts":[{"Type":1,"Value":"0102"}]},"Value":"030405"}`)

		k := Key{KeyParts: []KeyPart{{1, []byte{1, 2}}}}
		v := Value{3, 4, 5}
		p := NewPayload(k, v)
		b, err := json.Marshal(p)
		require.NoError(t, err)
		require.Equal(t, []byte(encoded), b)

		var p2 Payload
		err = json.Unmarshal(b, &p2)
		require.NoError(t, err)

		// Reset b to make sure that p2 doesn't share underlying data with JSON-encoded data.
		for i := range b {
			b[i] = 0
		}

		require.True(t, p.Equals(&p2))

		k2, err := p2.Key()
		require.NoError(t, err)
		require.True(t, k2.Equals(&k))

		v2 := p2.Value()
		require.True(t, v2.Equals(v))
	})
}

func TestPayloadCBORSerialization(t *testing.T) {
	t.Run("nil payload", func(t *testing.T) {
		encoded := []byte{0xf6} // null

		var p *Payload
		b, err := cbor.Marshal(p)
		require.NoError(t, err)
		require.Equal(t, encoded, b)

		var p2 *Payload
		err = cbor.Unmarshal(b, &p2)
		require.NoError(t, err)
		require.Equal(t, p, p2)
	})

	t.Run("empty payload", func(t *testing.T) {
		encoded := []byte{
			0xa2,             // map(2)
			0x63,             // text(3)
			0x4b, 0x65, 0x79, // "Key"
			0xa1,                                           // map(1)
			0x68,                                           // text(8)
			0x4b, 0x65, 0x79, 0x50, 0x61, 0x72, 0x74, 0x73, // "KeyParts"
			0xf6,                         // null
			0x65,                         // text(5)
			0x56, 0x61, 0x6c, 0x75, 0x65, // "Value"
			0x40, // null will be normalized to []byte{}
		}

		var p Payload
		b, err := cbor.Marshal(p)
		require.NoError(t, err)
		require.True(t, p.value == nil)
		require.Equal(t, encoded, b)

		var p2 Payload
		err = cbor.Unmarshal(b, &p2)
		require.NoError(t, err)

		// Reset b to make sure that p2 doesn't share underlying data with CBOR-encoded data.
		for i := range b {
			b[i] = 0
		}

		require.True(t, p2.IsEmpty())

		k2, err := p2.Key()
		require.NoError(t, err)
		require.True(t, k2.Equals(&Key{}))

		v2 := p2.Value()
		require.True(t, v2.Equals(Value{}))

		// after decoding, the value will be normalized to []byte{}
		// which will be different from the original format
		require.True(t, p.value == nil)
		require.True(t, p2.value != nil)
	})

	t.Run("empty key", func(t *testing.T) {
		encoded := []byte{
			0xa2,             // map(2)
			0x63,             // text(3)
			0x4b, 0x65, 0x79, // "Key"
			0xa1,                                           // map(1)
			0x68,                                           // text(8)
			0x4b, 0x65, 0x79, 0x50, 0x61, 0x72, 0x74, 0x73, // "KeyParts"
			0xf6,                         // null
			0x65,                         // text(5)
			0x56, 0x61, 0x6c, 0x75, 0x65, // "Value"
			0x43,             // bytes(3)
			0x00, 0x01, 0x02, // "\u0000\u0001\u0002"
		}

		k := Key{}
		v := []byte{0, 1, 2}
		p := NewPayload(k, v)
		b, err := cbor.Marshal(p)
		require.NoError(t, err)
		require.Equal(t, encoded, b)

		var p2 Payload
		err = cbor.Unmarshal(b, &p2)
		require.NoError(t, err)

		// Reset b to make sure that p2 doesn't share underlying data with CBOR-encoded data.
		for i := range b {
			b[i] = 0
		}

		require.True(t, p.Equals(&p2))

		k2, err := p2.Key()
		require.NoError(t, err)
		require.True(t, k2.Equals(&k))

		v2 := p2.Value()
		require.True(t, v2.Equals(v))
	})

	t.Run("empty value", func(t *testing.T) {
		encoded := []byte{
			0xa2,             // map(2)
			0x63,             // text(3)
			0x4b, 0x65, 0x79, // "Key"
			0xa1,                                           // map(1)
			0x68,                                           // text(8)
			0x4b, 0x65, 0x79, 0x50, 0x61, 0x72, 0x74, 0x73, // "KeyParts"
			0x81,                   // array(1)
			0xa2,                   // map(2)
			0x64,                   // text(4)
			0x54, 0x79, 0x70, 0x65, // "Type"
			0x01,                         // unsigned(1)
			0x65,                         // text(5)
			0x56, 0x61, 0x6c, 0x75, 0x65, // "Value"
			0x42,       // bytes(2)
			0x01, 0x02, // "\u0001\u0002"
			0x65,                         // text(5)
			0x56, 0x61, 0x6c, 0x75, 0x65, // "Value"
			0x40, // null will be normalized to []byte{}
		}

		k := Key{KeyParts: []KeyPart{{1, []byte{1, 2}}}}
		var v Value
		p := NewPayload(k, v)
		b, err := cbor.Marshal(p)
		require.NoError(t, err)
		require.Equal(t, encoded, b)

		var p2 Payload
		err = cbor.Unmarshal(b, &p2)
		require.NoError(t, err)

		// Reset b to make sure that p2 doesn't share underlying data with CBOR-encoded data.
		for i := range b {
			b[i] = 0
		}

		require.True(t, p.Equals(&p2))

		k2, err := p2.Key()
		require.NoError(t, err)
		require.True(t, k2.Equals(&k))

		v2 := p2.Value()
		require.True(t, v2.Equals(v))
	})

	t.Run("payload", func(t *testing.T) {
		encoded := []byte{
			0xa2,             // map(2)
			0x63,             // text(3)
			0x4b, 0x65, 0x79, // "Key"
			0xa1,                                           // map(1)
			0x68,                                           // text(8)
			0x4b, 0x65, 0x79, 0x50, 0x61, 0x72, 0x74, 0x73, // "KeyParts"
			0x81,                   // array(1)
			0xa2,                   // map(2)
			0x64,                   // text(4)
			0x54, 0x79, 0x70, 0x65, // "Type"
			0x01,                         // unsigned(1)
			0x65,                         // text(5)
			0x56, 0x61, 0x6c, 0x75, 0x65, // "Value"
			0x42,       // bytes(2)
			0x01, 0x02, // "\u0001\u0002"
			0x65,                         // text(5)
			0x56, 0x61, 0x6c, 0x75, 0x65, // "Value"
			0x43,             // bytes(3)
			0x03, 0x04, 0x05, // "\u0003\u0004\u0005"
		}

		k := Key{KeyParts: []KeyPart{{1, []byte{1, 2}}}}
		v := Value{3, 4, 5}
		p := NewPayload(k, v)
		b, err := cbor.Marshal(p)
		require.NoError(t, err)
		require.Equal(t, []byte(encoded), b)

		var p2 Payload
		err = cbor.Unmarshal(b, &p2)
		require.NoError(t, err)

		// Reset b to make sure that p2 doesn't share underlying data with CBOR-encoded data.
		for i := range b {
			b[i] = 0
		}

		require.True(t, p.Equals(&p2))

		k2, err := p2.Key()
		require.NoError(t, err)
		require.True(t, k2.Equals(&k))

		v2 := p2.Value()
		require.True(t, v2.Equals(v))
	})
}
