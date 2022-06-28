package migrations

import (
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go-sdk"

	state "github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
)

func TestAccountStatusMigration(t *testing.T) {
	mig := AccountStatusMigration{
		Logger: zerolog.Logger{},
	}

	address1 := flow.HexToAddress("0x1")
	address2 := flow.HexToAddress("0x2")

	payloads := []ledger.Payload{
		{Key: createPayloadKeyWithLegacyController(address1, state.KeyStorageUsed, true), Value: utils.Uint64ToBinary(1)},
		{Key: createPayloadKeyWithLegacyController(address1, "other registers", true), Value: utils.Uint64ToBinary(2)},
		{Key: createPayloadKeyWithLegacyController(address2, "other registers2", true), Value: utils.Uint64ToBinary(3)},
		{Key: createPayloadKeyWithLegacyController(address1, KeyExists, true), Value: []byte{1}},
		{Key: createPayloadKeyWithLegacyController(address1, KeyAccountFrozen, true), Value: []byte{1}},
	}

	newPayloads, err := mig.Migrate(payloads)
	require.NoError(t, err)
	require.Equal(t, 4, len(newPayloads)) // no more frozen register

	require.True(t, newPayloads[0].Equals(&payloads[0]))
	require.True(t, newPayloads[1].Equals(&payloads[1]))
	require.True(t, newPayloads[2].Equals(&payloads[2]))

	expectedPayload := &ledger.Payload{
		Key:   createPayloadKeyWithLegacyController(address1, state.KeyAccountStatus, true),
		Value: state.NewAccountStatus().ToBytes(),
	}
	fmt.Println(newPayloads[3])
	require.True(t, newPayloads[3].Equals(expectedPayload))
}
