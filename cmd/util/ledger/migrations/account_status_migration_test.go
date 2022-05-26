package migrations

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go-sdk"

	state "github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
)

func createAccountPayloadKey(a flow.Address, key string) ledger.Key {
	return ledger.Key{
		KeyParts: []ledger.KeyPart{
			ledger.NewKeyPart(0, a.Bytes()),
			ledger.NewKeyPart(1, []byte("")),
			ledger.NewKeyPart(2, []byte(key)),
		},
	}
}

func TestAccountStatusMigration(t *testing.T) {
	mig := AccountStatusMigration{
		Logger: zerolog.Logger{},
	}

	address1 := flow.HexToAddress("0x1")
	address2 := flow.HexToAddress("0x2")

	payloads := []ledger.Payload{
		{Key: createAccountPayloadKey(address1, state.KeyStorageUsed), Value: utils.Uint64ToBinary(1)},
		{Key: createAccountPayloadKey(address1, "other registers"), Value: utils.Uint64ToBinary(2)},
		{Key: createAccountPayloadKey(address2, "other registers2"), Value: utils.Uint64ToBinary(3)},
		{Key: createAccountPayloadKey(address1, state.KeyExists), Value: []byte{1}},
		{Key: createAccountPayloadKey(address1, state.KeyAccountFrozen), Value: []byte{1}},
	}

	newPayloads, err := mig.Migrate(payloads)
	require.NoError(t, err)
	require.Equal(t, 4, len(newPayloads)) // no more frozen register

	require.True(t, newPayloads[0].Equals(&payloads[0]))
	require.True(t, newPayloads[1].Equals(&payloads[1]))
	require.True(t, newPayloads[2].Equals(&payloads[2]))

	expectedPayload := &ledger.Payload{
		Key:   createAccountPayloadKey(address1, state.KeyAccountStatus),
		Value: state.NewAccountStatus().ToBytes(),
	}
	require.True(t, newPayloads[3].Equals(expectedPayload))
}
