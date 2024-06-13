package migrations

import (
	"crypto/rand"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func TestCheckDomainPayloads(t *testing.T) {
	var address flow.Address
	_, err := rand.Read(address[:])
	require.NoError(t, err)

	owner := flow.AddressToRegisterOwner(address)

	t.Run("no storage domain payload", func(t *testing.T) {
		var payloads []*ledger.Payload

		// Add internal payload
		payloads = append(payloads, getPayload(t, owner, "a.s", 16))

		// Add atree paylaod
		var index [9]byte
		binary.BigEndian.PutUint64(index[:], 0)
		index[0] = '$'

		payloads = append(payloads, getPayload(t, owner, string(index[:]), 16))

		accountRegisters, err := registers.NewAccountRegistersFromPayloads(owner, payloads)
		require.NoError(t, err)

		err = CheckDomainPayloads(accountRegisters)
		require.NoError(t, err)
	})

	t.Run("some storage domain payloads", func(t *testing.T) {
		var payloads []*ledger.Payload

		// Add internal payload
		payloads = append(payloads, getPayload(t, owner, "a.s", 16))

		// Add atree paylaod
		var index [9]byte
		binary.BigEndian.PutUint64(index[:], 0)
		index[0] = '$'

		payloads = append(payloads, getPayload(t, owner, string(index[:]), 16))

		// Add domain payload
		payloads = append(payloads, getPayload(t, owner, allStorageMapDomains[0], 8))

		accountRegisters, err := registers.NewAccountRegistersFromPayloads(owner, payloads)
		require.NoError(t, err)

		err = CheckDomainPayloads(accountRegisters)
		require.NoError(t, err)
	})

	t.Run("unexpected storage domain payloads", func(t *testing.T) {
		var payloads []*ledger.Payload

		// Add internal payload
		payloads = append(payloads, getPayload(t, owner, "a.s", 16))

		// Add atree paylaod
		var index [9]byte
		binary.BigEndian.PutUint64(index[:], 0)
		index[0] = '$'

		payloads = append(payloads, getPayload(t, owner, string(index[:]), 16))

		// Add domain payload
		payloads = append(payloads, getPayload(t, owner, allStorageMapDomains[0], 8))

		// Add unexpected domain payload
		payloads = append(payloads, getPayload(t, owner, "a", 8))

		// Add domain payload with unexpected value
		payloads = append(payloads, getPayload(t, owner, allStorageMapDomains[1], 16))

		accountRegisters, err := registers.NewAccountRegistersFromPayloads(owner, payloads)
		require.NoError(t, err)

		err = CheckDomainPayloads(accountRegisters)
		errStrings := strings.Split(err.Error(), "\n")
		require.Equal(t, 2, len(errStrings))
		for _, es := range errStrings {
			require.True(t, ("found unexpected domain: a" == es) ||
				strings.HasPrefix(es, "domain payload contains unexpected value"))
		}
	})
}

func getPayload(t *testing.T, owner, key string, valueLength int) *ledger.Payload {
	ledgerKey := ledger.Key{KeyParts: []ledger.KeyPart{
		{Type: uint16(0), Value: []byte(owner)},
		{Type: uint16(2), Value: []byte(key)},
	}}

	value := make([]byte, valueLength)
	_, err := rand.Read(value)
	require.NoError(t, err)

	return ledger.NewPayload(ledgerKey, value)
}
