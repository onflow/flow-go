package migrations

import (
	"crypto/rand"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
)

func TestCheckDomainPayloads(t *testing.T) {
	t.Run("no storage domain payload", func(t *testing.T) {
		var payloads []*ledger.Payload

		// Add internal payload
		payloads = append(payloads, getPayload("a.s", 16))

		// Add atree paylaod
		var index [9]byte
		binary.BigEndian.PutUint64(index[:], 0)
		index[0] = '$'

		payloads = append(payloads, getPayload(string(index[:]), 16))

		err := CheckDomainPayloads(payloads)
		require.NoError(t, err)
	})

	t.Run("some storage domain payloads", func(t *testing.T) {
		var payloads []*ledger.Payload

		// Add internal payload
		payloads = append(payloads, getPayload("a.s", 16))

		// Add atree paylaod
		var index [9]byte
		binary.BigEndian.PutUint64(index[:], 0)
		index[0] = '$'

		payloads = append(payloads, getPayload(string(index[:]), 16))

		// Add domain payload
		payloads = append(payloads, getPayload(allStorageMapDomains[0], 8))

		err := CheckDomainPayloads(payloads)
		require.NoError(t, err)
	})

	t.Run("unexpected storage domain payloads", func(t *testing.T) {
		var payloads []*ledger.Payload

		// Add internal payload
		payloads = append(payloads, getPayload("a.s", 16))

		// Add atree paylaod
		var index [9]byte
		binary.BigEndian.PutUint64(index[:], 0)
		index[0] = '$'

		payloads = append(payloads, getPayload(string(index[:]), 16))

		// Add domain payload
		payloads = append(payloads, getPayload(allStorageMapDomains[0], 8))

		// Add unexpected domain payload
		payloads = append(payloads, getPayload("a", 8))

		// Add domain payload with unexpected value
		payloads = append(payloads, getPayload(allStorageMapDomains[1], 16))

		err := CheckDomainPayloads(payloads)
		errStrings := strings.Split(err.Error(), "\n")
		require.Equal(t, 2, len(errStrings))
		require.Equal(t, "found unexpected domain: a", errStrings[0])
		require.True(t, strings.HasPrefix(errStrings[1], "domain payload contains unexpected value"))
	})
}

func getPayload(key string, valueLength int) *ledger.Payload {
	address := make([]byte, 8)
	_, err := rand.Read(address)
	if err != nil {
		panic(err)
	}
	ledgerKey := ledger.Key{KeyParts: []ledger.KeyPart{
		{Type: uint16(0), Value: address},
		{Type: uint16(2), Value: []byte(key)},
	}}

	value := make([]byte, valueLength)
	_, err = rand.Read(value)
	if err != nil {
		panic(err)
	}

	return ledger.NewPayload(ledgerKey, value)
}
