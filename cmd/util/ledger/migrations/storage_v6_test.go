package migrations

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	newInter "github.com/onflow/cadence-latest/runtime/interpreter"
	oldInter "github.com/onflow/cadence-v0180/runtime/interpreter"
)

func TestValueConversion(t *testing.T) {
	oldArray := oldInter.NewArrayValue(
		[]oldInter.Value{
			oldInter.NewStringValue("foo"),
			oldInter.NewStringValue("bar"),
			oldInter.BoolValue(true),
		},
	)

	converter := NewValueConverter(nil, nil)
	newArray := converter.Convert(oldArray)

	fmt.Println(newArray)
	assert.IsType(t, &newInter.ArrayValue{}, newArray)
}
