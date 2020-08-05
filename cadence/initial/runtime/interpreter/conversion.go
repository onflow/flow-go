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
	"errors"
	"math"
)

func ByteArrayValueToByteSlice(value Value) ([]byte, error) {
	array, ok := value.(*ArrayValue)
	if !ok {
		return nil, errors.New("value is not an array")
	}

	result := make([]byte, len(array.Values))
	for i, element := range array.Values {

		b, err := ByteValueToByte(element)
		if err != nil {
			return nil, err
		}

		result[i] = b
	}

	return result, nil
}

func ByteValueToByte(element Value) (byte, error) {
	var b byte

	switch element := element.(type) {
	case BigNumberValue:
		bigInt := element.ToBigInt()
		if !bigInt.IsUint64() {
			return 0, errors.New("value is not in byte range (0-255)")
		}

		integer := bigInt.Uint64()

		if integer > math.MaxUint8 {
			return 0, errors.New("value is not in byte range (0-255)")
		}

		b = byte(integer)

	case NumberValue:
		integer := element.ToInt()

		if integer < 0 || integer > math.MaxUint8 {
			return 0, errors.New("value is not in byte range (0-255)")
		}

		b = byte(integer)

	default:
		return 0, errors.New("value is not an integer")
	}

	return b, nil
}

func ByteSliceToByteArrayValue(buf []byte) *ArrayValue {
	values := make([]Value, len(buf))
	for i, b := range buf {
		values[i] = UInt8Value(b)
	}

	return NewArrayValueUnownedNonCopying(values...)
}
