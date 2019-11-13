package example

import (
	"github.com/dapperlabs/flow-go/language/runtime/types"
	"github.com/dapperlabs/flow-go/language/runtime/values"
	"github.com/dapperlabs/flow-go/sdk/abi/encode"
)

type Person interface {
	FullName() string
	Type() types.Type
	Value() values.Value
}

var personType types.Type = types.Composite{
	FieldTypes: []types.Type{types.String{}},
}

func EncodePerson(p Person) ([]byte, error) {
	return encode.Encode(p.Value())
}

func DecodePerson(b []byte) (Person, error) {
	v, err := encode.Decode(personType, b)
	if err != nil {
		return nil, err
	}

	return newPersonFromValue(v), nil
}

type person struct {
	values.Composite
}

func newPersonFromValue(v values.Value) Person {
	value := v.(values.Composite)
	return person{value}
}

func (p person) FullName() string {
	return string(p.Fields[0].(values.String))
}

func (p person) Type() types.Type {
	return personType
}

func (p person) Value() values.Value {
	return p
}
