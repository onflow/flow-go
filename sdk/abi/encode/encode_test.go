package encode_test

import (
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/sdk/abi/encode"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
)

// TODO: test remaining types
func TestEncode(t *testing.T) {
	compositeType := types.Composite{
		FieldTypes: []types.Type{
			types.String{},
			types.String{},
		},
	}

	value1 := values.Composite{
		Fields: []values.Value{
			values.String("foo"),
			values.String("bar"),
		},
	}

	b, err := encode.Encode(value1)
	require.NoError(t, err)

	t.Logf("Encoded value: %x", b)

	value2, err := encode.Decode(compositeType, b)
	require.NoError(t, err)

	assert.Equal(t, value1, value2)
}
