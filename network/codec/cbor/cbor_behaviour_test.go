package cbor

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"

	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
)

// The CBOR network codec uses the cbor.ExtraDecErrorUnknownField option, which
// causes decoding to return an error when decoding a message which contains an
// extra field, not present in the target (struct into which we are decoding).
//
// This test validates this behaviour.
func TestBehaviour_DecodeExtraField(t *testing.T) {
	type model1 struct {
		A int
	}
	type model2 struct {
		A int
		B int
	}

	m2 := model2{
		A: 100,
		B: 200,
	}
	bz, err := cborcodec.EncMode.Marshal(m2)
	assert.NoError(t, err)

	var m1 model1
	err = defaultDecMode.Unmarshal(bz, &m1)
	assert.Error(t, err)
	target := &cbor.UnknownFieldError{}
	assert.ErrorAs(t, err, &target)
}

// The CBOR network codec uses the cbor.ExtraDecErrorUnknownField option, which
// causes decoding to return an error when decoding a message which contains an
// extra field, not present in the target (struct into which we are decoding).
//
// This test validates that, when decoding a message which omits a field present
// in the target, no error is returned.
func TestBehaviour_DecodeOmittedField(t *testing.T) {
	type model1 struct {
		A int
	}
	type model2 struct {
		A int
		B int
	}

	m1 := model1{
		A: 100,
	}
	bz, err := cborcodec.EncMode.Marshal(m1)
	assert.NoError(t, err)

	var m2 model2
	err = defaultDecMode.Unmarshal(bz, &m2)
	assert.NoError(t, err)
	assert.Equal(t, m2.A, m1.A)
	assert.Equal(t, m2.B, int(0))
}
