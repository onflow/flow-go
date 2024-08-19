package convert_test

import (
	"math/rand"
	"testing"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
)

// TestConvertCompatibleRange tests that converting a compatible range to a protobuf message
func TestConvertCompatibleRange(t *testing.T) {
	t.Parallel()

	startHeight := uint64(rand.Uint32())
	endHeight := uint64(rand.Uint32())

	comparableRange := &flow.CompatibleRange{
		StartHeight: startHeight,
		EndHeight:   endHeight,
	}
	expected := &entities.CompatibleRange{
		StartHeight: startHeight,
		EndHeight:   endHeight,
	}

	msg := convert.CompatibleRangeToMessage(comparableRange)
	assert.Equal(t, msg, expected)
}
