package migrations

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	oldInter "github.com/onflow/cadence0170/runtime/interpreter"
	newInter "github.com/onflow/cadence0180/runtime/interpreter"
)

func TestName(t *testing.T) {
	oldArray := oldInter.NewArrayValue(
		[]oldInter.Value{
			oldInter.NewStringValue("foo"),
			oldInter.NewStringValue("bar"),
			oldInter.BoolValue(true),
		},
	)

	converter := &ValueConverter{}
	newArray := converter.Convert(nil, oldArray)

	fmt.Println(newArray)
	assert.IsType(t, &newInter.ArrayValue{}, newArray)
}
