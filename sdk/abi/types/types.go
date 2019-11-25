package types

// revive:disable:redefines-builtin-id

import "encoding/json"

type Type interface {
	//isType()
	Name() string
}

// revive:enable

type isAType struct{}

func (isAType) isType() {}

type Annotation struct {
	IsMove bool
	Type   Type
}

type Void struct{ isAType }

func (Void) Name() string {
	return "Void"
}

type Bool struct{ isAType }

func (Bool) Name() string {
	return "Boolean"
}
func (s Bool) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Name())
}

type String struct{ isAType }

func (String) Name() string {
	return "String"
}
func (s String) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Name())
}

type Bytes struct{ isAType }

func (Bytes) Name() string {
	return "Bytes"
}
func (s Bytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Name())
}

type Any struct{ isAType }

func (Any) Name() string {
	return "Any"
}
func (a Any) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.Name())
}

type Int struct {
	isAType
}

func (Int) Name() string {
	return "Int"
}
func (t Int) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Name())
}

//func (Int) MarshalJSON() ([]byte, error) {
//	return json.Marshal("Int")
//}

type Int8 struct {
	isAType
}

func (Int8) Name() string {
	return "Int8"
}
func (s Int8) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Name())
}

type Int16 struct {
	isAType
}

func (Int16) Name() string {
	return "Int16"
}
func (s Int16) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Name())
}

type Int32 struct {
	isAType
}

func (Int32) Name() string {
	return "Int32"
}
func (s Int32) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Name())
}

type Int64 struct {
	isAType
}

func (Int64) Name() string {
	return "Int64"
}
func (s Int64) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Name())
}

type Uint8 struct {
	isAType
}

func (Uint8) Name() string {
	return "Uint8"
}
func (s Uint8) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Name())
}

type Uint16 struct {
	isAType
}

func (Uint16) Name() string {
	return "Uint16"
}
func (s Uint16) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Name())
}

type Uint32 struct {
	isAType
}

func (Uint32) Name() string {
	return "Uint32"
}
func (s Uint32) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Name())
}

type Uint64 struct {
	isAType
}

func (Uint64) Name() string {
	return "Uint64"
}
func (s Uint64) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Name())
}

type VariableSizedArray struct {
	isAType
	ElementType Type
}

func (v VariableSizedArray) Name() string {
	return "[" + v.ElementType.Name() + "]"
}
func (vsa VariableSizedArray) MarshalJSON() ([]byte, error) {

	//typeBytes, err := json.Marshal(vsa.ElementType)
	//
	//if err != nil {
	//	return nil, err
	//}

	type r struct {
		array struct {
			of interface{}
		}
	}

	return json.Marshal(r{
		array: struct {
			of interface{}
		}{
			of: nil,
		},
	})
}

type ConstantSizedArray struct {
	isAType
	Size        uint
	ElementType Type
}

func (c ConstantSizedArray) Name() string {
	return "[" + c.ElementType.Name() + "]"
}

type Parameter struct {
	Field
	Label string `json:"label,omitempty"`
}

type Composite struct {
	isAType
	Fields       map[string]*Field
	Identifier   string
	Initializers [][]*Parameter
}

func (c Composite) Name() string {
	return c.Identifier
}

type Struct struct {
	isAType
	Composite
}

func (s Struct) Name() string {
	return s.Composite.Name()
}

type Resource struct {
	isAType
	Composite
}

func (r Resource) Name() string {
	return r.Composite.Name()
}

type Dictionary struct {
	isAType
	KeyType     Type
	ElementType Type
}

func (d Dictionary) Name() string {
	return "{" + d.KeyType.Name() + ":" + d.ElementType.Name() + "}"
}

type Function struct {
	isAType
	Parameters []*Parameter
	ReturnType Type
}

func (f Function) Name() string {
	return "[function]"
}

// A type representing anonymous function (aka without named arguments
type FunctionType struct {
	isAType
	ParameterTypes []Type
	ReturnType     Type
}

func (FunctionType) Name() string {
	return "[Function]"
}

type Event struct {
	isAType
	Fields     []*Parameter `json:"event"`
	Identifier string       `json:"-"`
}

func (e Event) Name() string {
	return e.Identifier
}

type Field struct {
	isAType
	Identifier string `json:"name"`
	Type       Type   `json:"type"`
}

func (f Field) Name() string {
	return f.Identifier + ":" + f.Type.Name()
}

type Optional struct {
	isAType
	Of Type
}

func (o Optional) Name() string {
	return o.Of.Name() + "?"
}

//Pointer are simply pointers to already existing types, to prevent circular references
type Pointer struct {
	isAType
	TypeName string
}

func (p Pointer) Name() string {
	return p.TypeName
}
func (p Pointer) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.TypeName)
}

type Address struct{ isAType }

func (Address) Name() string {
	return "Address"
}
