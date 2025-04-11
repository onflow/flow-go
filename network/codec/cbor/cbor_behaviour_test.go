package cbor

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"

	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
)

// The CBOR network codec uses the [cbor.ExtraDecErrorUnknownField] option, which
// causes decoding to return an error when decoding a message which contains an
// extra field, not present in the target (struct into which we are decoding).
//
// This test validates this behaviour.
func TestBehaviour_DecodeExtraField(t *testing.T) {
	t.Run("decoding NON-ZERO VALUE of extra field not present in the target struct is forbidden", func(t *testing.T) {
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
		err = cborcodec.DefaultDecMode.Unmarshal(bz, &m1)
		assert.Error(t, err)
		target := &cbor.UnknownFieldError{}
		assert.ErrorAs(t, err, &target)
	})

	t.Run("decoding ZERO VALUE of extra field not present in the target struct is forbidden", func(t *testing.T) {
		type model1 struct {
			A *int
		}
		type model2 struct {
			A *int
			B *int
		}

		a := 100
		m2 := model2{
			A: &a,
			// B has zero-value
		}
		bz, err := cborcodec.EncMode.Marshal(m2)
		assert.NoError(t, err)

		var m1 model1
		err = cborcodec.DefaultDecMode.Unmarshal(bz, &m1)
		assert.Error(t, err)
		target := &cbor.UnknownFieldError{}
		assert.ErrorAs(t, err, &target)
	})
}

// The CBOR network codec uses the [cbor.ExtraDecErrorUnknownField] option, which
// causes decoding to return an error when decoding a message which contains an
// extra field, not present in the target (struct into which we are decoding).
//
// This test validates that, when decoding a message which OMITS a field present
// in the target, no error is returned.
//
// This behaviour is very useful for downwards compatibility: for example if we add
// a new field B to a struct, nodes running the updated software can still decode
// messages emitted by the old software - with the convention that in the decoded
// message, field B has the zero-value.
// However, note that the reverse (i.e. downwards compatibility) is not true *by default*
// Specifically the old software cannot decode the new struct, even if field B has the
// zero value, as demonstrated by the test [TestBehaviour_DecodeExtraField] above.
//
// Nevertheless, downwards compatibility can be improved with suitable conventions
// as demonstrated in the test [TestBehaviour_OmittingNewFieldForDownwardsCompatibility] below
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
	err = cborcodec.DefaultDecMode.Unmarshal(bz, &m2)
	assert.NoError(t, err)
	assert.Equal(t, m2.A, m1.A)
	assert.Equal(t, m2.B, int(0))
}

// This test demonstrates a possible convention for improving downwards compatibility,
// when we want to add a new field to an existing struct. Let's say that the struct
// `model1` describes the old data structure, to which we want to add a new integer
// field `B`.
// Note that the following pattern only works out of the box, if field `B` is required
// according to the new protocol convention. In other words, the new software can
// differentiate between the old and the new data model based on whether field `B`
// is present.
// The important aspects are
//  1. define field `B` as a pointer variable. Thereby, the new software can represent
//     an old data model with `B` being nil, while the new data model always has `B` ≠ nil.
//  2. In the new software, provide the cbor directive `cbor:",omitempty"`, which instructs
//     cbor to omit the field entirely during the encoding step. Thereby the new software
//     reproduces the encoding of the old software when dealing with the old data model.
func TestBehaviour_OmittingNewFieldForDownwardsCompatibility(t *testing.T) {
	type model1 struct {
		A int
	}
	type model2 struct {
		A int
		B *int `cbor:",omitempty"`
	}

	a := 100
	m2 := model2{
		A: a,
	}
	// m2.B is `nil`, which cbor will omit in the encoding step according to our directive "omitempty"
	bz, err := cborcodec.EncMode.Marshal(m2)
	assert.NoError(t, err)

	var m1 model1
	err = cborcodec.DefaultDecMode.Unmarshal(bz, &m1)
	assert.NoError(t, err)
}
