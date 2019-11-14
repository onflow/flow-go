package encode_test

import (
	"bytes"
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/sdk/abi/encode"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
)

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

	var w bytes.Buffer
	enc := encode.NewEncoder(&w)

	err := enc.Encode(value1)
	require.Nil(t, err)

	t.Logf("Encoded: %x", w.Bytes())

	r := bytes.NewReader(w.Bytes())
	dec := encode.NewDecoder(r)

	value2, err := dec.Decode(compositeType)
	require.Nil(t, err)

	assert.Equal(t, value1, value2)
}
