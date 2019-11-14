package values

type Value interface {
	// TODO: remove this function after encoding/decoding is implemented
	ToGoValue() interface{}
}

type Void struct{}

func (v Void) ToGoValue() interface{} { return nil }

type Nil struct{}

func (v Nil) ToGoValue() interface{} { return nil }

type Bool bool

func (v Bool) ToGoValue() interface{} { return bool(v) }

type String string

func (v String) ToGoValue() interface{} { return string(v) }

// TODO: use big.Int to represent Int value
type Int int

func (v Int) ToGoValue() interface{} { return int(v) }

type Int8 int8

func (v Int8) ToGoValue() interface{} { return int8(v) }

type Int16 int16

func (v Int16) ToGoValue() interface{} { return int16(v) }

type Int32 int32

func (v Int32) ToGoValue() interface{} { return int32(v) }

type Int64 int64

func (v Int64) ToGoValue() interface{} { return int64(v) }

type Uint8 uint8

func (v Uint8) ToGoValue() interface{} { return uint8(v) }

type Uint16 uint16

func (v Uint16) ToGoValue() interface{} { return uint16(v) }

type Uint32 uint32

func (v Uint32) ToGoValue() interface{} { return uint32(v) }

type Uint64 uint64

func (v Uint64) ToGoValue() interface{} { return uint64(v) }

type Array []Value

func (v Array) ToGoValue() interface{} { panic("not implemented") }

type Composite struct {
	Fields []Value
}

func (v Composite) ToGoValue() interface{} { panic("not implemented") }

type Dictionary map[Value]Value

func (v Dictionary) ToGoValue() interface{} { panic("not implemented") }
