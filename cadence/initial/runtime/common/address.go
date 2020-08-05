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

package common

import (
	"fmt"
)

const AddressLength = 8

type Address [AddressLength]byte

// BytesToAddress returns Address with value b.
//
// If b is larger than len(h), b will be cropped from the left.
func BytesToAddress(b []byte) Address {
	var a Address
	a.SetBytes(b)
	return a
}

// Hex returns the hex string representation of the address.
func (a Address) Hex() string {
	return fmt.Sprintf("%x", a[:])
}

func (a Address) String() string {
	return a.Hex()
}

// SetBytes sets the address to the value of b.
//
// If b is larger than len(a) it will panic.
func (a *Address) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-AddressLength:]
	}

	copy(a[AddressLength-len(b):], b)
}

func (a Address) Bytes() []byte {
	// Trim leading zeros
	leadingZeros := 0
	for _, b := range a {
		if b != 0 {
			break
		}
		leadingZeros += 1
	}

	return a[leadingZeros:]
}
