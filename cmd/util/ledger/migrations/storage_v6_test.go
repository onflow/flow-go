package migrations

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	newInter "github.com/onflow/cadence/runtime/interpreter"
	oldInter "github.com/onflow/cadence/v18/runtime/interpreter"
)

func TestValueConversion(t *testing.T) {
	oldArray := oldInter.NewArrayValue(
		[]oldInter.Value{
			oldInter.NewStringValue("foo"),
			oldInter.NewStringValue("bar"),
			oldInter.BoolValue(true),
		},
	)

	inter := newInter.Interpreter{
		Storage: newInter.NewInMemoryStorage(),
	}

	converter := NewValueConverter(nil, &inter)
	newArray := converter.Convert(oldArray)

	fmt.Println(newArray)
	assert.IsType(t, &newInter.ArrayValue{}, newArray)
}
