package migrations_test

import (
	"testing"

	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/ledger"
)

func TestStorageFormatV4Migration(t *testing.T) {

	const cborTagCompositeValue = 132
	const cborTagStringLocation = 193

	payloads := []ledger.Payload{
		{
			Key: ledger.Key{
				KeyParts: []ledger.KeyPart{
					ledger.NewKeyPart(state.KeyPartOwner, []byte{0x42}),
					ledger.NewKeyPart(state.KeyPartController, []byte{0x42}),
					ledger.NewKeyPart(state.KeyPartKey, []byte("test")),
				},
			},
			Value: interpreter.PrependMagic(
				[]byte{
					// tag
					0xd8, cborTagCompositeValue,
					// map, 4 pairs of items follow
					0xa4,
					// key 0
					0x0,
					// tag
					0xd8, cborTagStringLocation,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
					// key 2
					0x2,
					// positive integer 1
					0x1,
					// key 3
					0x3,
					// map, 0 pairs of items follow
					0xa0,
					// key 4
					0x4,
					// UTF-8 string, length 10
					0x6a,
					0x54, 0x65, 0x73, 0x74, 0x53, 0x74, 0x72, 0x75, 0x63, 0x74,
				},
				2,
			),
		},
	}

	results, err := migrations.StorageFormatV4Migration(payloads)
	require.NoError(t, err)

	require.Equal(t,
		ledger.Value{
			// magic prefix
			0x0, 0xca, 0xde, 0x0, 0x4,
			// tag
			0xd8, cborTagCompositeValue,
			// array, 5 items follow
			0x85,

			// tag
			0xd8, cborTagStringLocation,
			// UTF-8 string, length 4
			0x64,
			// t, e, s, t
			0x74, 0x65, 0x73, 0x74,

			// nil
			0xf6,

			// positive integer 1
			0x1,

			// array, 0 items follow
			0x80,

			// UTF-8 string, length 10
			0x6a,
			0x54, 0x65, 0x73, 0x74, 0x53, 0x74, 0x72, 0x75, 0x63, 0x74,
		},
		results[0].Value,
	)
}
