package values

import (
	"fmt"
	"math/big"
)

type Value interface {
	isValue()
}

type Void struct{}

func (Void) isValue() {}

type Nil struct{}

func (Nil) isValue() {}

type Bool bool

func (Bool) isValue() {}

type String string

func (String) isValue() {}

type Bytes []byte

func (Bytes) isValue() {}

type Int struct {
	Int *big.Int
}

func NewInt(v int) Int {
	return Int{big.NewInt(int64(v))}
}

func NewIntFromBig(v *big.Int) Int {
	return Int{v}
}

func (v Int) ToInt() int {
	return int(v.Int.Int64())
}

func (Int) isValue() {}

type Int8 int8

func (Int8) isValue() {}

type Int16 int16

func (Int16) isValue() {}

type Int32 int32

func (Int32) isValue() {}

type Int64 int64

func (Int64) isValue() {}

type Uint8 uint8

func (Uint8) isValue() {}

type Uint16 uint16

func (Uint16) isValue() {}

type Uint32 uint32

func (Uint32) isValue() {}

type Uint64 uint64

func (Uint64) isValue() {}

type VariableSizedArray []Value

func (VariableSizedArray) isValue() {}

type ConstantSizedArray []Value

func (ConstantSizedArray) isValue() {}

type Dictionary []KeyValuePair

func (Dictionary) isValue() {}

type KeyValuePair struct {
	Key   Value
	Value Value
}

type Event struct {
	// TODO: is the Identifier field needed here?
	Identifier string
	Fields     []Value
}

type Composite struct {
	Fields []Value
}

func (Composite) isValue() {}

func (Event) isValue() {}

const AddressLength = 20

type Address [AddressLength]byte

func (Address) isValue() {}

func (a Address) String() string {
	return a.Hex()
}

func (a Address) Hex() string {
	return fmt.Sprintf("%x", a)
}

func BytesToAddress(b []byte) Address {
	var a Address
	copy(a[AddressLength-len(b):AddressLength], b)
	return a
}
