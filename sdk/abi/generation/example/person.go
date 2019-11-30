package example

import (
	"bytes"

	values2 "github.com/dapperlabs/flow-go/sdk/abi/encoding/values"
	types2 "github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
)

type PersonView interface {
	FullName() string
}

type personView struct {
	_fullName string
	value     values.Composite
}

func (p personView) FullName() string {
	return p._fullName
}

type PersonConstructor interface {
	Encode() ([]byte, error)
}

type personConstructor struct {
	firstName string
	lastName  string
}

func (p personConstructor) Encode() ([]byte, error) {

	var w bytes.Buffer
	encoder := values2.NewEncoder(&w)

	err := encoder.EncodeConstantSizedArray(
		values.ConstantSizedArray{
			values.String(p.firstName),
			values.String(p.lastName),
		},
	)

	if err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

func NewPersonConstructor(firstName string, lastName string) (PersonConstructor, error) {
	return personConstructor{
		firstName: firstName,
		lastName:  lastName,
	}, nil
}

var personType = types2.Composite{
	Fields: map[string]*types2.Field{
		"FullName": &types2.Field{
			Type:       types2.String{},
			Identifier: "FullName",
		},
	},
	Initializers: [][]*types2.Parameter{
		{
			&types2.Parameter{
				Field: types2.Field{
					Identifier: "firstName",
					Type:       types2.String{},
				},
			},
			&types2.Parameter{
				Field: types2.Field{
					Identifier: "lastName",
					Type:       types2.String{},
				},
			},
		},
	},
}

func DecodePersonView(b []byte) (PersonView, error) {
	r := bytes.NewReader(b)
	dec := values2.NewDecoder(r)

	v, err := dec.DecodeComposite(personType)
	if err != nil {
		return nil, err
	}

	return personView{
		_fullName: string(v.Fields[0].(values.String)),
	}, nil
}
