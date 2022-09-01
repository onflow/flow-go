package migrations

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
)

func TestAccountStatusMigration(t *testing.T) {
	mig := NewAccountStatusMigration(zerolog.Logger{})

	address1 := flow.HexToAddress("0x1")
	address2 := flow.HexToAddress("0x2")

	payloads := []ledger.Payload{
		*ledger.NewPayload(
			createPayloadKeyWithLegacyController(address1, KeyStorageUsed, true),
			utils.Uint64ToBinary(12)),
		*ledger.NewPayload(
			createPayloadKeyWithLegacyController(address1, "other registers", true),
			utils.Uint64ToBinary(2)),
		*ledger.NewPayload(
			createPayloadKeyWithLegacyController(address2, "other registers2", true),
			utils.Uint64ToBinary(3)),
		*ledger.NewPayload(
			createPayloadKeyWithLegacyController(address1, KeyExists, true),
			[]byte{1}),
		*ledger.NewPayload(
			createPayloadKeyWithLegacyController(address1, KeyAccountFrozen, true),
			[]byte{1}),
		*ledger.NewPayload(
			createPayloadKeyWithLegacyController(address1, KeyPublicKeyCount, true),
			utils.Uint64ToBinary(2)),
		*ledger.NewPayload(
			createPayloadKeyWithLegacyController(address1, KeyPrefixPublicKey+"0", true),
			[]byte{1}),
		*ledger.NewPayload(
			createPayloadKeyWithLegacyController(address1, KeyPrefixPublicKey+"1", true),
			[]byte{2}),
		*ledger.NewPayload(
			createPayloadKeyWithLegacyController(address1, KeyStorageIndex, true),
			[]byte{1, 0, 0, 0, 0, 0, 0, 0}),
	}

	newPayloads, err := mig.Migrate(payloads)
	require.NoError(t, err)

	require.Equal(t, 5, len(newPayloads))
	require.True(t, newPayloads[0].Equals(&payloads[1]))
	require.True(t, newPayloads[1].Equals(&payloads[2]))
	require.True(t, newPayloads[2].Equals(&payloads[6]))
	require.True(t, newPayloads[3].Equals(&payloads[7]))

	// check address one status
	expectedStatus := environment.NewAccountStatus()
	expectedStatus.SetFrozenFlag(true)
	expectedStatus.SetPublicKeyCount(2)
	expectedStatus.SetStorageUsed(12)
	expectedStatus.SetStorageIndex([8]byte{1, 0, 0, 0, 0, 0, 0, 0})
	expectedPayload := ledger.NewPayload(
		createPayloadKeyWithLegacyController(address1, state.KeyAccountStatus, true),
		expectedStatus.ToBytes(),
	)

	// check address two status
	require.True(t, newPayloads[4].Equals(expectedPayload))
}
