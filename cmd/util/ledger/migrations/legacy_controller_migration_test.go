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

func createPayloadKeyWithLegacyController(a flow.Address, key string) ledger.Key {
	return ledger.Key{
		KeyParts: []ledger.KeyPart{
			ledger.NewKeyPart(0, a.Bytes()),
			ledger.NewKeyPart(1, a.Bytes()),
			ledger.NewKeyPart(2, []byte(key)),
		},
	}
}

func TestLegacyControllerMigration(t *testing.T) {
	mig := LegacyControllerMigration{
		Logger: zerolog.Logger{},
	}

	address1 := flow.HexToAddress("0x1")
	address2 := flow.HexToAddress("0x2")

	payloads := []ledger.Payload{
		{Key: createAccountPayloadKey(address1, state.KeyStorageUsed), Value: utils.Uint64ToBinary(1)},
		{Key: createPayloadKeyWithLegacyController(address1, state.ContractKey("CoreContract")), Value: utils.Uint64ToBinary(2)},
		{Key: createPayloadKeyWithLegacyController(address1, state.KeyContractNames), Value: utils.Uint64ToBinary(3)},
		{Key: createPayloadKeyWithLegacyController(address2, state.KeyPublicKey(1)), Value: utils.Uint64ToBinary(4)},
		{Key: createPayloadKeyWithLegacyController(address2, state.KeyPublicKeyCount), Value: utils.Uint64ToBinary(4)},
	}

	newPayloads, err := mig.Migrate(payloads)
	require.NoError(t, err)
	require.Equal(t, 5, len(newPayloads))

	for _, p := range newPayloads {
		require.Equal(t, 0, len(p.Key.KeyParts[1].Value))
	}
}
