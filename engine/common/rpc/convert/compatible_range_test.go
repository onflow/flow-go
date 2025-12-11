package convert_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
)

// TestConvertCompatibleRange tests that converting a compatible range to a protobuf message
func TestConvertCompatibleRange(t *testing.T) {
	t.Parallel()

	t.Run("nil range is nil", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, convert.CompatibleRangeToMessage(nil))
		assert.Nil(t, convert.MessageToCompatibleRange(nil))
	})

	t.Run("roundtrip conversion", func(t *testing.T) {
		t.Parallel()

		startHeight := uint64(rand.Uint32())
		endHeight := uint64(rand.Uint32())

		original := &accessmodel.CompatibleRange{
			StartHeight: startHeight,
			EndHeight:   endHeight,
		}

		msg := convert.CompatibleRangeToMessage(original)
		converted := convert.MessageToCompatibleRange(msg)

		assert.Equal(t, original, converted)
	})
}
