package values

type Value interface {
	isValue()
	// TODO: remove this function after encoding/decoding is implemented
	ToGoValue() interface{}
}

type Void struct{}

func (Void) isValue()                 {}
func (v Void) ToGoValue() interface{} { return nil }

type Nil struct{}

func (Nil) isValue()                 {}
func (v Nil) ToGoValue() interface{} { return nil }

type Bool bool

func (Bool) isValue()                 {}
func (v Bool) ToGoValue() interface{} { return bool(v) }

type String string

func (String) isValue()                 {}
func (v String) ToGoValue() interface{} { return string(v) }

// TODO: use big.Int to represent Int value
type Int int

func (Int) isValue()                 {}
func (v Int) ToGoValue() interface{} { return int(v) }

type Int8 int8

func (Int8) isValue()                 {}
func (v Int8) ToGoValue() interface{} { return int8(v) }

type Int16 int16

func (Int16) isValue()                 {}
func (v Int16) ToGoValue() interface{} { return int16(v) }

type Int32 int32

func (Int32) isValue()                 {}
func (v Int32) ToGoValue() interface{} { return int32(v) }

type Int64 int64

func (Int64) isValue()                 {}
func (v Int64) ToGoValue() interface{} { return int64(v) }

type Uint8 uint8

func (Uint8) isValue()                 {}
func (v Uint8) ToGoValue() interface{} { return uint8(v) }

type Uint16 uint16

func (Uint16) isValue()                 {}
func (v Uint16) ToGoValue() interface{} { return uint16(v) }

type Uint32 uint32

func (Uint32) isValue()                 {}
func (v Uint32) ToGoValue() interface{} { return uint32(v) }

type Uint64 uint64

func (Uint64) isValue()                 {}
func (v Uint64) ToGoValue() interface{} { return uint64(v) }

type Array []Value

func (Array) isValue()                 {}
func (v Array) ToGoValue() interface{} { panic("not implemented") }

type Composite struct {
	Fields []Value
}

func (Composite) isValue()                 {}
func (v Composite) ToGoValue() interface{} { panic("not implemented") }

type Dictionary map[Value]Value

func (Dictionary) isValue()                 {}
func (v Dictionary) ToGoValue() interface{} { panic("not implemented") }

type Event struct {
	Identifier string
	Fields     []EventField
}

func (Event) isValue()                 {}
func (v Event) ToGoValue() interface{} { panic("not implemented") }

type EventField struct {
	Identifier string
	Value      Value
}

type Address [20]byte

func (Address) isValue()                 {}
func (v Address) ToGoValue() interface{} { return [20]byte(v) }

func BytesToAddress(b []byte) Address {
	var a Address
	copy(a[:], b)
	return a
}

type Bytes []byte

func (Bytes) isValue()                 {}
func (v Bytes) ToGoValue() interface{} { return []byte(v) }
