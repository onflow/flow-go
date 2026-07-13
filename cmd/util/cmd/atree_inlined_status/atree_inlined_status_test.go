package atree_inlined_status

import (
	crand "crypto/rand"
	"math/rand"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func TestCheckAtreeInlinedStatus(t *testing.T) {
	const nWorkers = 10

	t.Run("no payloads", func(t *testing.T) {
		var payloads []*ledger.Payload
		atreeInlinedPayloadCount, atreeNonInlinedPayloadCount, err := checkAtreeInlinedStatus(payloads, nWorkers)
		require.NoError(t, err)
		require.Equal(t, 0, atreeInlinedPayloadCount)
		require.Equal(t, 0, atreeNonInlinedPayloadCount)
	})

	t.Run("payloads no goroutine", func(t *testing.T) {
		payloadCount := rand.Intn(50)
		testCheckAtreeInlinedStatus(t, payloadCount, nWorkers)
	})

	t.Run("payloads using goroutine", func(t *testing.T) {
		payloadCount := rand.Intn(numOfPayloadPerJob) + numOfPayloadPerJob
		testCheckAtreeInlinedStatus(t, payloadCount, nWorkers)
	})
}

func testCheckAtreeInlinedStatus(t *testing.T, payloadCount int, nWorkers int) {
	atreeNoninlinedPayloadCount := rand.Intn(payloadCount + 1)
	atreeInlinedPayloadCount := payloadCount - atreeNoninlinedPayloadCount

	payloads := make([]*ledger.Payload, 0, payloadCount)
	for range atreeInlinedPayloadCount {
		key := getRandomKey()
		value := getAtreeInlinedPayload(t)
		payloads = append(payloads, ledger.NewPayload(key, value))
	}
	for range atreeNoninlinedPayloadCount {
		key := getRandomKey()
		value := getAtreeNoninlinedPayload(t)
		payloads = append(payloads, ledger.NewPayload(key, value))
	}

	rand.Shuffle(len(payloads), func(i, j int) {
		payloads[i], payloads[j] = payloads[j], payloads[i]
	})

	gotAtreeInlinedPayloadCount, gotAtreeNoninlinedPayloadCount, err := checkAtreeInlinedStatus(payloads, nWorkers)
	require.NoError(t, err)
	require.Equal(t, atreeNoninlinedPayloadCount, gotAtreeNoninlinedPayloadCount)
	require.Equal(t, atreeInlinedPayloadCount, gotAtreeInlinedPayloadCount)
}

func getAtreeNoninlinedPayload(t *testing.T) []byte {
	num := rand.Uint64()
	encodedNum, err := cbor.Marshal(num)
	require.NoError(t, err)

	data := []byte{
		// extra data
		// version
		0x00,
		// extra data flag
		0x80,
		// array of extra data
		0x81,
		// type info
		0x18, 0x2a,

		// version
		0x00,
		// array data slab flag
		0x80,
		// CBOR encoded array head (fixed size 3 byte)
		0x99, 0x00, 0x01,
	}

	return append(data, encodedNum...)
}

func getAtreeInlinedPayload(t *testing.T) []byte {
	num := rand.Uint64()
	encodedNum, err := cbor.Marshal(num)
	require.NoError(t, err)

	data := []byte{
		// version
		0x10,
		// flag
		0x80,

		// extra data
		// array of extra data
		0x81,
		// type info
		0x18, 0x2a,

		// CBOR encoded array head (fixed size 3 byte)
		0x99, 0x00, 0x01,
	}

	return append(data, encodedNum...)
}

func getRandomKey() ledger.Key {
	var address [8]byte
	_, err := crand.Read(address[:])
	if err != nil {
		panic(err)
	}

	var key [9]byte
	key[0] = flow.SlabIndexPrefix
	_, err = crand.Read(key[1:])
	if err != nil {
		panic(err)
	}

	return ledger.Key{
		KeyParts: []ledger.KeyPart{
			{Type: uint16(0), Value: address[:]},
			{Type: uint16(2), Value: key[:]},
		}}
}
