/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package xdr_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cadence "github.com/onflow/cadence-initial"
	"github.com/onflow/cadence-initial/encoding/xdr"
	"github.com/onflow/cadence-initial/runtime/common"
)

type encodeTest struct {
	name string
	typ  cadence.Type
	val  cadence.Value
}

func TestEncodeVoid(t *testing.T) {

	t.Parallel()

	testEncode(t, cadence.VoidType{}, cadence.Void{})
}

func TestEncodeString(t *testing.T) {

	t.Parallel()

	testAllEncode(t, []encodeTest{
		{
			"Empty",
			cadence.StringType{},
			cadence.NewString(""),
		},
		{
			"Non-empty",
			cadence.StringType{},
			cadence.NewString("abcdefg"),
		},
	}...)
}

func TestEncodeOptional(t *testing.T) {

	t.Parallel()

	testAllEncode(t, []encodeTest{
		{
			"Nil",
			cadence.OptionalType{Type: nil},
			cadence.NewOptional(nil),
		},
		{
			"Non-nil",
			cadence.OptionalType{Type: cadence.StringType{}},
			cadence.NewOptional(cadence.NewString("abcdefg")),
		},
	}...)
}

func TestEncodeBool(t *testing.T) {

	t.Parallel()

	testAllEncode(t, []encodeTest{
		{
			"True",
			cadence.BoolType{},
			cadence.NewBool(true),
		},
		{
			"False",
			cadence.BoolType{},
			cadence.NewBool(false),
		},
	}...)
}

func TestEncodeBytes(t *testing.T) {

	t.Parallel()

	testAllEncode(t, []encodeTest{
		{
			"Empty",
			cadence.BytesType{},
			cadence.NewBytes([]byte{}),
		},
		{
			"Non-empty",
			cadence.BytesType{},
			cadence.NewBytes([]byte{1, 2, 3, 4, 5}),
		},
	}...)
}

func TestEncodeAddress(t *testing.T) {

	t.Parallel()

	testEncode(t, cadence.AddressType{}, cadence.NewAddress([common.AddressLength]byte{1, 2, 3, 4, 5}))
}

func TestEncodeInt(t *testing.T) {

	t.Parallel()

	x := new(big.Int).SetUint64(math.MaxUint64)
	x = x.Mul(x, big.NewInt(2))

	largerThanMaxUint64 := encodeTest{
		"LargerThanMaxUint64",
		cadence.IntType{},
		cadence.NewIntFromBig(x),
	}

	testAllEncode(t, []encodeTest{
		{
			"Negative",
			cadence.IntType{},
			cadence.NewInt(-42),
		},
		{
			"Zero",
			cadence.IntType{},
			cadence.NewInt(0),
		},
		{
			"Positive",
			cadence.IntType{},
			cadence.NewInt(42),
		},
		largerThanMaxUint64,
	}...)
}

func TestEncodeInt8(t *testing.T) {

	t.Parallel()

	testAllEncode(t, []encodeTest{
		{
			"Min",
			cadence.Int8Type{},
			cadence.NewInt8(math.MinInt8),
		},
		{
			"Zero",
			cadence.Int8Type{},
			cadence.NewInt8(0),
		},
		{
			"Max",
			cadence.Int8Type{},
			cadence.NewInt8(math.MaxInt8),
		},
	}...)
}

func TestEncodeInt16(t *testing.T) {

	t.Parallel()

	testAllEncode(t, []encodeTest{
		{
			"Min",
			cadence.Int16Type{},
			cadence.NewInt16(math.MinInt16),
		},
		{
			"Zero",
			cadence.Int16Type{},
			cadence.NewInt16(0),
		},
		{
			"Max",
			cadence.Int16Type{},
			cadence.NewInt16(math.MaxInt16),
		},
	}...)
}

func TestEncodeInt32(t *testing.T) {

	t.Parallel()

	testAllEncode(t, []encodeTest{
		{
			"Min",
			cadence.Int32Type{},
			cadence.NewInt32(math.MinInt32),
		},
		{
			"Zero",
			cadence.Int32Type{},
			cadence.NewInt32(0),
		},
		{
			"Max",
			cadence.Int32Type{},
			cadence.NewInt32(math.MaxInt32),
		},
	}...)
}

func TestEncodeInt64(t *testing.T) {

	t.Parallel()

	testAllEncode(t, []encodeTest{
		{
			"Min",
			cadence.Int64Type{},
			cadence.NewInt64(math.MinInt64),
		},
		{
			"Zero",
			cadence.Int64Type{},
			cadence.NewInt64(0),
		},
		{
			"Max",
			cadence.Int64Type{},
			cadence.NewInt64(math.MaxInt64),
		},
	}...)
}

func TestEncodeUInt8(t *testing.T) {

	t.Parallel()

	testAllEncode(t, []encodeTest{
		{
			"Zero",
			cadence.UInt8Type{},
			cadence.NewUInt8(0),
		},
		{
			"Max",
			cadence.UInt8Type{},
			cadence.NewUInt8(math.MaxUint8),
		},
	}...)
}

func TestEncodeUInt16(t *testing.T) {

	t.Parallel()

	testAllEncode(t, []encodeTest{
		{
			"Zero",
			cadence.UInt16Type{},
			cadence.NewUInt16(0),
		},
		{
			"Max",
			cadence.UInt16Type{},
			cadence.NewUInt16(math.MaxUint8),
		},
	}...)
}

func TestEncodeUInt32(t *testing.T) {

	t.Parallel()

	testAllEncode(t, []encodeTest{
		{
			"Zero",
			cadence.UInt32Type{},
			cadence.NewUInt32(0),
		},
		{
			"Max",
			cadence.UInt32Type{},
			cadence.NewUInt32(math.MaxUint32),
		},
	}...)
}

func TestEncodeUInt64(t *testing.T) {

	t.Parallel()

	testAllEncode(t, []encodeTest{
		{
			"Zero",
			cadence.UInt64Type{},
			cadence.NewUInt64(0),
		},
		{
			"Max",
			cadence.UInt64Type{},
			cadence.NewUInt64(math.MaxUint64),
		},
	}...)
}

func TestEncodeVariableSizedArray(t *testing.T) {

	t.Parallel()

	emptyArray := encodeTest{
		"Empty",
		cadence.VariableSizedArrayType{
			ElementType: cadence.IntType{},
		},
		cadence.NewArray([]cadence.Value{}),
	}

	intArray := encodeTest{
		"Integers",
		cadence.VariableSizedArrayType{
			ElementType: cadence.IntType{},
		},
		cadence.NewArray([]cadence.Value{
			cadence.NewInt(1),
			cadence.NewInt(2),
			cadence.NewInt(3),
		}),
	}

	resourceArray := encodeTest{
		"Resources",
		cadence.VariableSizedArrayType{
			ElementType: fooResourceType,
		},
		cadence.NewArray([]cadence.Value{
			cadence.NewResource([]cadence.Value{
				cadence.NewString("a"),
				cadence.NewInt(1),
			}).WithType(fooResourceType),
			cadence.NewResource([]cadence.Value{
				cadence.NewString("b"),
				cadence.NewInt(1),
			}).WithType(fooResourceType),
			cadence.NewResource([]cadence.Value{
				cadence.NewString("c"),
				cadence.NewInt(1),
			}).WithType(fooResourceType),
		}),
	}

	testAllEncode(t,
		emptyArray,
		intArray,
		resourceArray,
	)
}

func TestEncodeConstantSizedArray(t *testing.T) {

	t.Parallel()

	testAllEncode(t, []encodeTest{
		{
			"Empty",
			cadence.ConstantSizedArrayType{
				Size:        0,
				ElementType: cadence.IntType{},
			},
			cadence.NewArray([]cadence.Value{}),
		},
		{
			"Integers",
			cadence.ConstantSizedArrayType{
				Size:        3,
				ElementType: cadence.IntType{},
			},
			cadence.NewArray([]cadence.Value{
				cadence.NewInt(1),
				cadence.NewInt(2),
				cadence.NewInt(3),
			}),
		},
	}...)
}

func TestEncodeDictionary(t *testing.T) {

	t.Parallel()

	simpleDict := encodeTest{
		"Simple",
		cadence.DictionaryType{
			KeyType:     cadence.StringType{},
			ElementType: cadence.IntType{},
		},
		cadence.NewDictionary([]cadence.KeyValuePair{
			{
				Key:   cadence.NewString("a"),
				Value: cadence.NewInt(1),
			},
			{
				Key:   cadence.NewString("b"),
				Value: cadence.NewInt(2),
			},
			{
				Key:   cadence.NewString("c"),
				Value: cadence.NewInt(3),
			},
		}),
	}

	nestedDict := encodeTest{
		"Nested",
		cadence.DictionaryType{
			KeyType: cadence.StringType{},
			ElementType: cadence.DictionaryType{
				KeyType:     cadence.StringType{},
				ElementType: cadence.IntType{},
			},
		},
		cadence.NewDictionary([]cadence.KeyValuePair{
			{
				Key: cadence.NewString("a"),
				Value: cadence.NewDictionary([]cadence.KeyValuePair{
					{
						Key:   cadence.NewString("1"),
						Value: cadence.NewInt(1),
					},
				}),
			},
			{
				Key: cadence.NewString("b"),
				Value: cadence.NewDictionary([]cadence.KeyValuePair{
					{
						Key:   cadence.NewString("2"),
						Value: cadence.NewInt(2),
					},
				}),
			},
			{
				Key: cadence.NewString("c"),
				Value: cadence.NewDictionary([]cadence.KeyValuePair{
					{
						Key:   cadence.NewString("3"),
						Value: cadence.NewInt(3),
					},
				}),
			},
		}),
	}

	resourceDict := encodeTest{
		"Resources",
		cadence.DictionaryType{
			KeyType:     cadence.StringType{},
			ElementType: fooResourceType,
		},
		cadence.NewDictionary([]cadence.KeyValuePair{
			{
				Key: cadence.NewString("a"),
				Value: cadence.NewResource([]cadence.Value{
					cadence.NewString("a"),
					cadence.NewInt(1),
				}).WithType(fooResourceType),
			},
			{
				Key: cadence.NewString("b"),
				Value: cadence.NewResource([]cadence.Value{
					cadence.NewString("b"),
					cadence.NewInt(2),
				}).WithType(fooResourceType),
			},
			{
				Key: cadence.NewString("c"),
				Value: cadence.NewResource([]cadence.Value{
					cadence.NewString("c"),
					cadence.NewInt(3),
				}).WithType(fooResourceType),
			},
		}),
	}

	testAllEncode(t,
		simpleDict,
		nestedDict,
		resourceDict,
	)
}

func TestEncodeResource(t *testing.T) {

	t.Parallel()

	simpleResource := encodeTest{
		"Simple",
		fooResourceType,
		cadence.NewResource([]cadence.Value{
			cadence.NewString("foo"),
			cadence.NewInt(42),
		}).WithType(fooResourceType),
	}

	multiTypeResourceType := cadence.ResourceType{
		Fields: []cadence.Field{
			{
				Identifier: "a",
				Type:       cadence.StringType{},
			},
			{
				Identifier: "b",
				Type:       cadence.IntType{},
			},
			{
				Identifier: "c",
				Type:       cadence.BoolType{},
			},
		},
	}

	multiTypeResource := encodeTest{
		"MultipleTypes",
		multiTypeResourceType,
		cadence.NewResource([]cadence.Value{
			cadence.NewString("foo"),
			cadence.NewInt(42),
			cadence.NewBool(true),
		}).WithType(multiTypeResourceType),
	}

	arrayResourceType := cadence.ResourceType{
		Fields: []cadence.Field{
			{
				Identifier: "a",
				Type: cadence.VariableSizedArrayType{
					ElementType: cadence.IntType{},
				},
			},
		},
	}

	arrayResource := encodeTest{
		"ArrayField",
		arrayResourceType,
		cadence.NewResource([]cadence.Value{
			cadence.NewArray([]cadence.Value{
				cadence.NewInt(1),
				cadence.NewInt(2),
				cadence.NewInt(3),
				cadence.NewInt(4),
				cadence.NewInt(5),
			}),
		}).WithType(arrayResourceType),
	}

	innerResourceType := cadence.ResourceType{
		Fields: []cadence.Field{
			{
				Identifier: "c",
				Type:       cadence.IntType{},
			},
		},
	}

	outerResourceType := cadence.ResourceType{
		Fields: []cadence.Field{
			{
				Identifier: "a",
				Type:       cadence.StringType{},
			},
			{
				Identifier: "b",
				Type:       innerResourceType,
			},
		},
	}

	nestedResource := encodeTest{
		"Nested",
		outerResourceType,
		cadence.NewResource([]cadence.Value{
			cadence.NewString("foo"),
			cadence.NewResource([]cadence.Value{
				cadence.NewInt(42),
			}).WithType(innerResourceType),
		}).WithType(outerResourceType),
	}

	testAllEncode(t,
		simpleResource,
		multiTypeResource,
		arrayResource,
		nestedResource,
	)
}

func TestEncodeEvent(t *testing.T) {

	t.Parallel()

	simpleEventType := cadence.EventType{
		Fields: []cadence.Field{
			{
				Identifier: "a",
				Type:       cadence.IntType{},
			},
			{
				Identifier: "b",
				Type:       cadence.StringType{},
			},
		},
	}

	simpleEvent := encodeTest{
		"Simple",
		simpleEventType,
		cadence.NewEvent(
			[]cadence.Value{
				cadence.NewInt(1),
				cadence.NewString("foo"),
			},
		).WithType(simpleEventType),
	}

	resourceEventType := cadence.EventType{
		Fields: []cadence.Field{
			{
				Identifier: "a",
				Type:       cadence.StringType{},
			},
			{
				Identifier: "b",
				Type:       fooResourceType,
			},
		},
	}

	resourceEvent := encodeTest{
		"Resources",
		resourceEventType,
		cadence.NewEvent(
			[]cadence.Value{
				cadence.NewString("foo"),
				cadence.NewResource(
					[]cadence.Value{
						cadence.NewString("bar"),
						cadence.NewInt(42),
					},
				).WithType(fooResourceType),
			},
		).WithType(resourceEventType),
	}

	testAllEncode(t, simpleEvent, resourceEvent)
}

func testAllEncode(t *testing.T, tests ...encodeTest) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testEncode(t, test.typ, test.val)
		})
	}
}

const numTrials = 250

func testEncode(t *testing.T, typ cadence.Type, val cadence.Value) {
	b1, err := xdr.Encode(val)
	require.NoError(t, err)

	t.Logf("Encoded value: %x", b1)

	// encoding should be deterministic, repeat to confirm
	for i := 0; i < numTrials; i++ {
		b2, err := xdr.Encode(val)
		require.NoError(t, err)
		assert.Equal(t, b1, b2)
	}

	decodedVal, err := xdr.Decode(typ, b1)
	require.NoError(t, err)

	assert.Equal(t, val, decodedVal)
}

var fooResourceType = cadence.ResourceType{
	TypeID:     "test.Foo",
	Identifier: "Foo",
	Fields: []cadence.Field{
		{
			Identifier: "a",
			Type:       cadence.StringType{},
		},
		{
			Identifier: "b",
			Type:       cadence.IntType{},
		},
	},
}
