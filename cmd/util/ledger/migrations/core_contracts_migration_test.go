package migrations_test

import (
	"testing"

	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func TestCoreContractsMigration(t *testing.T) {

	t.Run("other payloads are not modified", func(t *testing.T) {

		dir := t.TempDir()

		migration := migrations.CoreContractsMigration{
			Log:       zerolog.Logger{},
			Chain:     flow.Testnet.Chain(),
			OutputDir: dir,
		}

		input := []ledger.Payload{
			{
				Key: ledger.Key{
					KeyParts: []ledger.KeyPart{
						{Value: []byte{0x1}},
						{},
						{Value: []byte("otherKey")},
					},
				},
				Value: []byte("other"),
			},
		}

		output, err := migration.Migrate(input)
		require.NoError(t, err)

		require.Len(t, output, 1)
		require.Equal(t, ledger.Value("other"), output[0].Value)
	})

	t.Run("DKG contract is migrated", func(t *testing.T) {

		dir := t.TempDir()
		migration := migrations.CoreContractsMigration{
			Log:       zerolog.Logger{},
			Chain:     flow.Testnet.Chain(),
			OutputDir: dir,
		}

		dkgAddress := flow.HexToAddress("0x9eca2b38b18b5dfe")

		input := []ledger.Payload{
			{
				Key: ledger.Key{
					KeyParts: []ledger.KeyPart{
						{Value: dkgAddress[:]},
						{},
						{Value: []byte("code.FlowDKG")},
					},
				},
				Value: []byte("/* old code */"),
			},
		}

		output, err := migration.Migrate(input)
		require.NoError(t, err)

		require.Len(t, output, 1)
		require.Equal(t, ledger.Value(coreContracts.FlowDKG()), output[0].Value)
	})

}
