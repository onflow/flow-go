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

package interpreter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"example.com/cadence-initial/runtime/common"
	"example.com/cadence-initial/runtime/tests/utils"
)

func newTestCompositeValue(owner common.Address) *CompositeValue {
	return NewCompositeValue(
		utils.TestLocation,
		"Test",
		common.CompositeKindStructure,
		map[string]Value{},
		&owner,
	)
}

func TestOwnerNewArray(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}

	value := newTestCompositeValue(oldOwner)

	assert.Equal(t, &oldOwner, value.GetOwner())

	array := NewArrayValueUnownedNonCopying(value)

	assert.Nil(t, array.GetOwner())
	assert.Nil(t, value.GetOwner())
}

func TestSetOwnerArray(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)

	array := NewArrayValueUnownedNonCopying(value)

	array.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, array.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerArrayCopy(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)

	array := NewArrayValueUnownedNonCopying(value)

	array.SetOwner(&newOwner)

	arrayCopy := array.Copy().(*ArrayValue)
	valueCopy := arrayCopy.Values[0]

	assert.Nil(t, arrayCopy.GetOwner())
	assert.Nil(t, valueCopy.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerArraySetIndex(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value1 := newTestCompositeValue(oldOwner)
	value2 := newTestCompositeValue(oldOwner)

	array := NewArrayValueUnownedNonCopying(value1)
	array.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, array.GetOwner())
	assert.Equal(t, &newOwner, value1.GetOwner())
	assert.Equal(t, &oldOwner, value2.GetOwner())

	array.Set(nil, LocationRange{}, NewIntValueFromInt64(0), value2)

	assert.Equal(t, &newOwner, array.GetOwner())
	assert.Equal(t, &newOwner, value1.GetOwner())
	assert.Equal(t, &newOwner, value2.GetOwner())
}

func TestSetOwnerArrayAppend(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)

	array := NewArrayValueUnownedNonCopying()
	array.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, array.GetOwner())
	assert.Equal(t, &oldOwner, value.GetOwner())

	array.Append(value)

	assert.Equal(t, &newOwner, array.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerArrayInsert(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)

	array := NewArrayValueUnownedNonCopying()
	array.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, array.GetOwner())
	assert.Equal(t, &oldOwner, value.GetOwner())

	array.Insert(0, value)

	assert.Equal(t, &newOwner, array.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestOwnerNewDictionary(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}

	keyValue := NewStringValue("test")
	value := newTestCompositeValue(oldOwner)

	assert.Equal(t, &oldOwner, value.GetOwner())

	dictionary := NewDictionaryValueUnownedNonCopying(keyValue, value)

	assert.Nil(t, dictionary.GetOwner())
	// NOTE: keyValue is string, has no owner
	assert.Nil(t, value.GetOwner())
}

func TestSetOwnerDictionary(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	keyValue := NewStringValue("test")
	value := newTestCompositeValue(oldOwner)

	dictionary := NewDictionaryValueUnownedNonCopying(keyValue, value)

	dictionary.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, dictionary.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerDictionaryCopy(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	keyValue := NewStringValue("test")
	value := newTestCompositeValue(oldOwner)

	dictionary := NewDictionaryValueUnownedNonCopying(keyValue, value)
	dictionary.SetOwner(&newOwner)

	dictionaryCopy := dictionary.Copy().(*DictionaryValue)
	valueCopy := dictionaryCopy.Entries[keyValue.KeyString()]

	assert.Nil(t, dictionaryCopy.GetOwner())
	assert.Nil(t, valueCopy.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerDictionarySetIndex(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	keyValue := NewStringValue("test")
	value := newTestCompositeValue(oldOwner)

	dictionary := NewDictionaryValueUnownedNonCopying()
	dictionary.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, dictionary.GetOwner())
	assert.Equal(t, &oldOwner, value.GetOwner())

	dictionary.Set(
		nil,
		LocationRange{},
		keyValue,
		NewSomeValueOwningNonCopying(value),
	)

	assert.Equal(t, &newOwner, dictionary.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerDictionaryInsert(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	keyValue := NewStringValue("test")
	value := newTestCompositeValue(oldOwner)

	dictionary := NewDictionaryValueUnownedNonCopying()
	dictionary.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, dictionary.GetOwner())
	assert.Equal(t, &oldOwner, value.GetOwner())

	dictionary.Insert(nil, LocationRange{}, keyValue, value)

	assert.Equal(t, &newOwner, dictionary.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestOwnerNewSome(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}

	value := newTestCompositeValue(oldOwner)

	assert.Equal(t, &oldOwner, value.GetOwner())

	any := NewSomeValueOwningNonCopying(value)

	assert.Equal(t, &oldOwner, any.GetOwner())
	assert.Equal(t, &oldOwner, value.GetOwner())
}

func TestSetOwnerSome(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)

	assert.Equal(t, &oldOwner, value.GetOwner())

	any := NewSomeValueOwningNonCopying(value)

	any.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, any.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerSomeCopy(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)

	assert.Equal(t, &oldOwner, value.GetOwner())

	some := NewSomeValueOwningNonCopying(value)
	some.SetOwner(&newOwner)

	someCopy := some.Copy().(*SomeValue)
	valueCopy := someCopy.Value

	assert.Nil(t, someCopy.GetOwner())
	assert.Nil(t, valueCopy.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestOwnerNewComposite(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}

	composite := newTestCompositeValue(oldOwner)

	assert.Equal(t, &oldOwner, composite.GetOwner())
}

func TestSetOwnerComposite(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)
	composite := newTestCompositeValue(oldOwner)

	const fieldName = "test"

	composite.Fields[fieldName] = value

	composite.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, composite.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerCompositeCopy(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}

	value := newTestCompositeValue(oldOwner)
	composite := newTestCompositeValue(oldOwner)

	const fieldName = "test"

	composite.Fields[fieldName] = value

	compositeCopy := composite.Copy().(*CompositeValue)
	valueCopy := compositeCopy.Fields[fieldName]

	assert.Nil(t, compositeCopy.GetOwner())
	assert.Nil(t, valueCopy.GetOwner())
	assert.Equal(t, &oldOwner, value.GetOwner())
}

func TestSetOwnerCompositeSetMember(t *testing.T) {

	t.Parallel()

	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)
	composite := newTestCompositeValue(oldOwner)

	const fieldName = "test"

	composite.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, composite.GetOwner())
	assert.Equal(t, &oldOwner, value.GetOwner())

	composite.SetMember(
		nil,
		LocationRange{},
		fieldName,
		value,
	)

	assert.Equal(t, &newOwner, composite.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}
