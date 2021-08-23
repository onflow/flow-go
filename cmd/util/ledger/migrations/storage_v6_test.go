package migrations

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	newInter "github.com/onflow/cadence/runtime/interpreter"
	oldInter "github.com/onflow/cadence/v18/runtime/interpreter"
)

func TestValueConversion(t *testing.T) {
	oldArray := oldInter.NewArrayValueUnownedNonCopying(
		oldInter.VariableSizedStaticType{
			Type: oldInter.PrimitiveStaticTypeAnyStruct,
		},
		oldInter.NewStringValue("foo"),
		oldInter.NewStringValue("bar"),
		oldInter.BoolValue(true),
	)

	storage := newInter.NewInMemoryStorage()
	converter := NewValueConverter(storage)

	newArray := converter.Convert(oldArray)

	fmt.Println(newArray)
	assert.IsType(t, &newInter.ArrayValue{}, newArray)
}
