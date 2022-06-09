package migrations_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func TestCoreContractsMigration(t *testing.T) {

	t.Run("other payloads are not modified", func(t *testing.T) {

		migration := migrations.CoreContractsMigration{
			Log:   zerolog.Logger{},
			Chain: flow.Testnet,
		}

		input := []ledger.Payload{
			{
				Key: ledger.Key{
					KeyParts: []ledger.KeyPart{
						{Value: []byte{}},
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

}
