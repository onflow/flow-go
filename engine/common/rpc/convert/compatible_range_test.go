package convert_test

import (
	"math/rand"
	"testing"

	"github.com/onflow/flow/protobuf/go/flow/entities"
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

	t.Run("convert range to message", func(t *testing.T) {
		startHeight := uint64(rand.Uint32())
		endHeight := uint64(rand.Uint32())

		comparableRange := &accessmodel.CompatibleRange{
			StartHeight: startHeight,
			EndHeight:   endHeight,
		}
		expected := &entities.CompatibleRange{
			StartHeight: startHeight,
			EndHeight:   endHeight,
		}

		msg := convert.CompatibleRangeToMessage(comparableRange)
		assert.Equal(t, msg, expected)
	})
}
