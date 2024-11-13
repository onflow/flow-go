package events

import (
	"testing"

	"github.com/onflow/cadence"
	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestBytesToCadenceUInt8ArrayValue(t *testing.T) {
	t.Parallel()

	input := []byte{1, 2, 3, 4, 5}

	inCadence := bytesToCadenceUInt8ArrayValue(input)

	require.Equal(t,
		cadence.NewArray([]cadence.Value{
			cadence.UInt8(1),
			cadence.UInt8(2),
			cadence.UInt8(3),
			cadence.UInt8(4),
			cadence.UInt8(5),
		}).WithType(cadenceArrayTypeOfUInt8),
		inCadence,
	)
}

func TestHashToCadenceArrayValue(t *testing.T) {
	t.Parallel()

	input := gethCommon.Hash{1, 2, 3, 4, 5}

	inCadence := hashToCadenceArrayValue(input)

	require.Equal(t,
		cadence.NewArray([]cadence.Value{
			cadence.UInt8(1),
			cadence.UInt8(2),
			cadence.UInt8(3),
			cadence.UInt8(4),
			cadence.UInt8(5),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
			cadence.UInt8(0),
		}).WithType(cadenceHashType),
		inCadence,
	)
}

func TestHashToChecksumValue(t *testing.T) {
	t.Parallel()

	checksum := [4]byte{1, 2, 3, 4}
	inCadence := checksumToCadenceArrayValue(checksum)
	require.Equal(t,
		cadence.NewArray([]cadence.Value{
			cadence.UInt8(1),
			cadence.UInt8(2),
			cadence.UInt8(3),
			cadence.UInt8(4),
		}).WithType(checksumType),
		inCadence,
	)
}
