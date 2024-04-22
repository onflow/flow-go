package migrations

import (
	"bytes"
	"context"
	"encoding/hex"
	"runtime"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/flow-go/ledger"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixSlabsWithBrokenReferences(t *testing.T) {

	rawAddress := mustDecodeHex("5e3448b3cffb97f2")

	address := common.MustBytesToAddress(rawAddress)

	ownerKey := ledger.KeyPart{Type: 0, Value: rawAddress}

	oldPayloads := []*ledger.Payload{
		// account status "a.s" register
		ledger.NewPayload(
			ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: mustDecodeHex("612e73")}}),
			ledger.Value(mustDecodeHex("00000000000000083900000000000000090000000000000001")),
		),

		// storage domain register
		ledger.NewPayload(
			ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: mustDecodeHex("73746f72616765")}}),
			ledger.Value(mustDecodeHex("0000000000000008")),
		),

		// public domain register
		ledger.NewPayload(
			ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: mustDecodeHex("7075626c6963")}}),
			ledger.Value(mustDecodeHex("0000000000000007")),
		),

		// MapDataSlab [balance:1000.00089000 uuid:13797744]
		ledger.NewPayload(
			ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: mustDecodeHex("240000000000000001")}}),
			ledger.Value(mustDecodeHex("008883d88483d8c082487e60df042a9c086869466c6f77546f6b656e6f466c6f77546f6b656e2e5661756c7402021b146e6a6a4c5eee08008883005b00000000000000100887f9d0544c60cbefe0afc51d7f46609b0000000000000002826762616c616e6365d8bc1b00000017487843a8826475756964d8a41a00d28970")),
		),

		// MapDataSlab [uuid:13799884 roles:StorageIDStorable({[94 52 72 179 207 251 151 242] [0 0 0 0 0 0 0 3]}) recipient:0x5e3448b3cffb97f2]
		ledger.NewPayload(
			ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: mustDecodeHex("240000000000000002")}}),
			ledger.Value(mustDecodeHex("00c883d88483d8c0824848602d8056ff9d937046616e546f705065726d697373696f6e7746616e546f705065726d697373696f6e2e486f6c64657202031bb9d0e9f36650574100c883005b000000000000001820d6c23f2e85e694b0070dbc21a9822de5725916c4a005e99b0000000000000003826475756964d8a41a00d291cc8265726f6c6573d8ff505e3448b3cffb97f200000000000000038269726563697069656e74d883485e3448b3cffb97f2")),
		),

		// This slab contains broken references.
		// MapDataSlab [StorageIDStorable({[0 0 0 0 0 0 0 0] [0 0 0 0 0 0 0 45]}):Capability<&A.48602d8056ff9d93.FanTopPermission.Admin>(address: 0x48602d8056ff9d93, path: /private/FanTopAdmin)]
		ledger.NewPayload(
			ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: mustDecodeHex("240000000000000003")}}),
			ledger.Value(mustDecodeHex("00c883d8d982d8d582d8c0824848602d8056ff9d937046616e546f705065726d697373696f6e7546616e546f705065726d697373696f6e2e526f6c65d8ddf6011b535c9de83a38cab000c883005b000000000000000856c1dcdf34d761b79b000000000000000182d8ff500000000000000000000000000000002dd8c983d8834848602d8056ff9d93d8c882026b46616e546f7041646d696ed8db82f4d8d582d8c0824848602d8056ff9d937046616e546f705065726d697373696f6e7646616e546f705065726d697373696f6e2e41646d696e")),
		),

		// MapDataSlab [resources:StorageIDStorable({[94 52 72 179 207 251 151 242] [0 0 0 0 0 0 0 5]}) uuid:15735719 address:0x5e3448b3cffb97f2]
		ledger.NewPayload(
			ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: mustDecodeHex("240000000000000004")}}),
			ledger.Value(mustDecodeHex("00c883d88483d8c0824848602d8056ff9d937346616e546f705065726d697373696f6e563261781a46616e546f705065726d697373696f6e5632612e486f6c64657202031b5a99ef3adb06d40600c883005b00000000000000185c9fead93697b692967de568f789d3c2d5e974502c8b12e99b000000000000000382697265736f7572636573d8ff505e3448b3cffb97f20000000000000005826475756964d8a41a00f01ba7826761646472657373d883485e3448b3cffb97f2")),
		),

		// MapDataSlab ["admin":StorageIDStorable({[94 52 72 179 207 251 151 242] [0 0 0 0 0 0 0 6]})]
		ledger.NewPayload(
			ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: mustDecodeHex("240000000000000005")}}),
			ledger.Value(mustDecodeHex("00c883d8d982d8d408d8dc82d8d40581d8d682d8c0824848602d8056ff9d937346616e546f705065726d697373696f6e563261781846616e546f705065726d697373696f6e5632612e526f6c65011b8059ccce9aa48cfb00c883005b00000000000000087a89c005baa53d9a9b000000000000000182d8876561646d696ed8ff505e3448b3cffb97f20000000000000006")),
		),

		// MapDataSlab [role:"admin" uuid:15735727]
		ledger.NewPayload(
			ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: mustDecodeHex("240000000000000006")}}),
			ledger.Value(mustDecodeHex("008883d88483d8c0824848602d8056ff9d937346616e546f705065726d697373696f6e563261781946616e546f705065726d697373696f6e5632612e41646d696e02021b4fc212cd0f233183008883005b0000000000000010858862f5e3e45e48d2bf75097a8aaf819b00000000000000028264726f6c65d8876561646d696e826475756964d8a41a00f01baf")),
		),

		// MapDataSlab [
		//	FanTopPermissionV2a:PathLink<&{A.48602d8056ff9d93.FanTopPermissionV2a.Receiver}>(/storage/FanTopPermissionV2a)
		//  flowTokenReceiver:PathLink<&{A.9a0766d93b6608b7.FungibleToken.Receiver}>(/storage/flowTokenVault)
		//  flowTokenBalance:PathLink<&{A.9a0766d93b6608b7.FungibleToken.Balance}>(/storage/flowTokenVault)
		//  FanTopPermission:PathLink<&{A.48602d8056ff9d93.FanTopPermission.Receiver}>(/storage/FanTopPermission)]
		ledger.NewPayload(
			ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: mustDecodeHex("240000000000000007")}}),
			ledger.Value(mustDecodeHex("008883f6041bc576c5f201b94974008883005b00000000000000207971082fb163397089dbafb546246f429beff1dc622768dcb916d25455dc0be39b0000000000000004827346616e546f705065726d697373696f6e563261d8cb82d8c882017346616e546f705065726d697373696f6e563261d8db82f4d8dc82d8d40581d8d682d8c0824848602d8056ff9d937346616e546f705065726d697373696f6e563261781c46616e546f705065726d697373696f6e5632612e52656365697665728271666c6f77546f6b656e5265636569766572d8cb82d8c882016e666c6f77546f6b656e5661756c74d8db82f4d8dc82d8d582d8c082487e60df042a9c086869466c6f77546f6b656e6f466c6f77546f6b656e2e5661756c7481d8d682d8c082489a0766d93b6608b76d46756e6769626c65546f6b656e7646756e6769626c65546f6b656e2e52656365697665728270666c6f77546f6b656e42616c616e6365d8cb82d8c882016e666c6f77546f6b656e5661756c74d8db82f4d8dc82d8d582d8c082487e60df042a9c086869466c6f77546f6b656e6f466c6f77546f6b656e2e5661756c7481d8d682d8c082489a0766d93b6608b76d46756e6769626c65546f6b656e7546756e6769626c65546f6b656e2e42616c616e6365827046616e546f705065726d697373696f6ed8cb82d8c882017046616e546f705065726d697373696f6ed8db82f4d8dc82d8d40581d8d682d8c0824848602d8056ff9d937046616e546f705065726d697373696f6e781946616e546f705065726d697373696f6e2e5265636569766572")),
		),

		// MapDataSlab [
		//	FanTopPermission:StorageIDStorable({[94 52 72 179 207 251 151 242] [0 0 0 0 0 0 0 2]})
		//  FanTopPermissionV2a:StorageIDStorable({[94 52 72 179 207 251 151 242] [0 0 0 0 0 0 0 4]})
		//  flowTokenVault:StorageIDStorable({[94 52 72 179 207 251 151 242] [0 0 0 0 0 0 0 1]})]
		ledger.NewPayload(
			ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: mustDecodeHex("240000000000000008")}}),
			ledger.Value(mustDecodeHex("008883f6031b7d303e276f3b803f008883005b00000000000000180a613a86f5856a480b3a715aa29b9876e5d7742a5a1df8e09b0000000000000003827046616e546f705065726d697373696f6ed8ff505e3448b3cffb97f20000000000000002827346616e546f705065726d697373696f6e563261d8ff505e3448b3cffb97f20000000000000004826e666c6f77546f6b656e5661756c74d8ff505e3448b3cffb97f20000000000000001")),
		),
	}

	slabIndexWithBrokenReferences := mustDecodeHex("240000000000000003")
	fixedSlabWithBrokenReferences := ledger.NewPayload(
		ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: slabIndexWithBrokenReferences}}),
		ledger.Value(mustDecodeHex("008883d8d982d8d582d8c0824848602d8056ff9d937046616e546f705065726d697373696f6e7546616e546f705065726d697373696f6e2e526f6c65d8ddf6001b535c9de83a38cab0008883005b00000000000000009b0000000000000000")),
	)

	// Account status register is updated to include address ID counter and new storage used.
	accountStatusRegisterID := mustDecodeHex("612e73")
	updatedAccountStatusRegister := ledger.NewPayload(
		ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: accountStatusRegisterID}}),
		ledger.Value([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x7, 0xcc, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x9, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}),
	)

	expectedNewPayloads := make([]*ledger.Payload, len(oldPayloads))
	copy(expectedNewPayloads, oldPayloads)

	for i, payload := range expectedNewPayloads {
		key, err := payload.Key()
		require.NoError(t, err)

		if bytes.Equal(key.KeyParts[1].Value, slabIndexWithBrokenReferences) {
			expectedNewPayloads[i] = fixedSlabWithBrokenReferences
		} else if bytes.Equal(key.KeyParts[1].Value, accountStatusRegisterID) {
			expectedNewPayloads[i] = updatedAccountStatusRegister
		}
	}

	rwf := &testReportWriterFactory{}

	log := zerolog.New(zerolog.NewTestWriter(t))

	accountsToFix := map[common.Address]string{
		address: "Broken contract FanTopPermission",
	}

	migration := NewFixBrokenReferencesInSlabsMigration(rwf, accountsToFix)

	err := migration.InitMigration(log, nil, runtime.NumCPU())
	require.NoError(t, err)

	defer migration.Close()

	newPayloads, err := migration.MigrateAccount(
		context.Background(),
		address,
		oldPayloads,
	)
	require.NoError(t, err)
	require.Equal(t, len(expectedNewPayloads), len(newPayloads))

	for _, expected := range expectedNewPayloads {
		k, _ := expected.Key()
		rawExpectedKey := expected.EncodedKey()

		var found bool
		for _, p := range newPayloads {
			if bytes.Equal(rawExpectedKey, p.EncodedKey()) {
				found = true
				require.Equal(t, expected.Value(), p.Value(), k.String())
				break
			}
		}
		require.True(t, found)
	}

	writer := rwf.reportWriters[fixSlabsWithBrokenReferencesName]
	assert.Equal(t,
		[]any{
			fixedSlabsWithBrokenReferences{
				Account:  address,
				Payloads: []*ledger.Payload{fixedSlabWithBrokenReferences},
				Msg:      accountsToFix[address],
			},
		},
		writer.entries,
	)
}

func mustDecodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
