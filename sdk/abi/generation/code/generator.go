package code

import (
	"os"
	"text/template"
	"unicode"

	"github.com/dapperlabs/flow-go/sdk/abi/types"
)

var (
	tmpl = `
package {{.Package}}

import (
	"bytes"

	"github.com/dapperlabs/flow-go/sdk/abi/encoding"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
)

{{range $name, $type := .Data}}

type {{startUpper $name}}View interface {
{{range $type.Fields}}
	{{startUpper .Identifier}}() {{goType .Type}}
{{end}}
}

type {{startLower $name}}View struct {
{{range $type.Fields}}
	{{_startLower .Identifier}} {{goType .Type}}
{{end}}
	value     values.Composite
}

{{range $type.Fields}}
func (p {{startLower $name}}View) {{startUpper .Identifier}}() {{goType .Type}} {
	return p.{{_startLower .Identifier}}
}
{{end}}

type {{startUpper $name}}Constructor interface {
	Encode() ([]byte, error)
}

type {{startLower $name}}Constructor struct {
{{range index $type.Initializers 0}}
	{{startLower .Field.Identifier}} {{goType .Field.Type}}
{{end}}
	
}

func (p {{startLower $name}}Constructor) Encode() ([]byte, error) {

	var w bytes.Buffer
	encoder := encoding.NewEncoder(&w)

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

func New{{startUpper $name}}Constructor({{range index $type.Initializers 0}}{{startLower .Field.Identifier}} {{goType .Field.Type}},{{end}}) ({{startUpper $name}}Constructor, error) {
	return personConstructor{
{{range index $type.Initializers 0}}
	{{startLower .Field.Identifier}}: {{startLower .Field.Identifier}},
{{end}}
	}, nil
}

var {{startLower $name}}Type = types.Composite{
	Fields: map[string]*types.Field{
		"FullName": {
			Type:       types.String{},
			Identifier: "FullName",
		},
	},
	Initializers: [][]*types.Parameter{
		{
			&types.Parameter{
				Field: types.Field{
					Identifier: "firstName",
					Type:       types.String{},
				},
			},
			&types.Parameter{
				Field: types.Field{
					Identifier: "lastName",
					Type:       types.String{},
				},
			},
		},
	},
}

func Decode{{startUpper $name}}View(b []byte) ({{startUpper $name}}View, error) {
	r := bytes.NewReader(b)
	dec := encoding.NewDecoder(r)

	v, err := dec.DecodeComposite({{startLower $name}}Type)
	if err != nil {
		return nil, err
	}

	return {{startLower $name}}View{
{{range $i, $f := $type.Fields}}
{{/* CREATE MAPPING FUNCTION TO ORDER FIELDS? */}}
		{{_startLower $f.Identifier}}: {{goType $f.Type}}(v.Fields[{{$i}}].(values.String)),
{{end}}
	}, nil
}

{{end}}

`
)

type data struct {
	Package string
	Data    map[string]*types.Composite
}

func toLower(s string) string {
	return string(unicode.ToLower(rune(s[0]))) + s[1:]
}

func toUpper(s string) string {
	return string(unicode.ToUpper(rune(s[0]))) + s[1:]
}

func _lower(s string) string {
	return "_" + toLower(s)
}

func goType(t types.Type) string {
	switch t.(type) {
	case types.String:
		return "string"
	}

	return "<PANIC>"
}

func GenerateGo(pkg string, typesToGenerate map[string]*types.Composite) ([]byte, error) {

	data := data{
		Package: pkg,
		Data:    typesToGenerate,
	}

	err := template.Must(template.New("cadence_generate").
		Funcs(template.FuncMap{
			"startLower":  toLower,
			"_startLower": _lower,
			"startUpper":  toUpper,
			"goType":      goType,
		}).
		Parse(tmpl)).Execute(os.Stdout, data)
	if err != nil {
		panic(err)
	}

	return nil, nil

}
