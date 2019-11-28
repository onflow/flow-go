package code

import (
	"fmt"
	"unicode"

	"github.com/dapperlabs/flow-go/sdk/abi/encoding"
	"github.com/dapperlabs/flow-go/sdk/abi/types"

	"github.com/dave/jennifer/jen"
)

func startLower(s string) string {
	return string(unicode.ToLower(rune(s[0]))) + s[1:]
}

func startUpper(s string) string {
	return string(unicode.ToUpper(rune(s[0]))) + s[1:]
}

func _lower(s string) string {
	return "_" + startLower(s)
}

//Wrapper for jen.Statement which adds some useful methods
type abiAwareStatement struct {
	*jen.Statement
}

func wrap(s *jen.Statement) *abiAwareStatement {
	return &abiAwareStatement{s}
}

func (a *abiAwareStatement) Type(t types.Type) *abiAwareStatement {
	switch t.(type) {
	case types.String:
		return wrap(a.String())
	}

	panic(fmt.Errorf("not supported type %v", t))
}

const (
	typesImportPath    = "github.com/dapperlabs/flow-go/sdk/abi/types"
	encodingImportPath = "github.com/dapperlabs/flow-go/sdk/abi/encoding"
	valuesImportPath   = "github.com/dapperlabs/flow-go/sdk/abi/values"
)

func (a *abiAwareStatement) SelfType(t types.Type) *abiAwareStatement {
	switch v := t.(type) {
	case types.String:
		return wrap(a.Statement.Qual(typesImportPath, "String").Values())
	case types.Composite:
		mappedFields := jen.Dict{}
		for key, field := range v.Fields {
			mappedFields[jen.Lit(key)] = Empty().SelfType(*field)
		}
		mappedInitializers := make([]jen.Code, len(v.Initializers))
		for i, initializer := range v.Initializers {
			params := make([]jen.Code, len(initializer))
			for i, param := range v.Initializers[i] {
				params[i] = Op("&").Qual(typesImportPath, "Parameter").SelfType(*param)
			}

			mappedInitializers[i] = jen.Values(params...)
		}
		return wrap(a.Statement.Qual(typesImportPath, "Composite").Values(jen.Dict{
			Id("Fields"):       jen.Map(jen.String()).Op("*").Qual(typesImportPath, "Field").Values(mappedFields),
			Id("Initializers"): jen.Index().Index().Op("*").Qual(typesImportPath, "Parameter").Values(mappedInitializers...),
		}))
	case types.Field:
		return wrap(a.Statement.Values(jen.Dict{
			Id("Type"):       Empty().SelfType(v.Type),
			Id("Identifier"): jen.Lit(v.Identifier),
		}))
	case types.Parameter:
		return wrap(a.Statement.Values(jen.Dict{
			Id("Field"): Qual(typesImportPath, "Field").SelfType(v.Field),
			Id("Label"): jen.Lit(v.Label),
		}))
	}

	panic(fmt.Errorf("not supported type %T", t))
}

func (a *abiAwareStatement) Params(params ...jen.Code) *abiAwareStatement {
	return &abiAwareStatement{a.Statement.Params(params...)}
}

func (a *abiAwareStatement) Id(name string) *abiAwareStatement {
	return &abiAwareStatement{a.Statement.Id(name)}
}

func (a *abiAwareStatement) Call(params ...jen.Code) *abiAwareStatement {
	return &abiAwareStatement{a.Statement.Call(params...)}
}

func (a *abiAwareStatement) Op(op string) *abiAwareStatement {
	return wrap(a.Statement.Op(op))
}

func (a *abiAwareStatement) Qual(path, name string) *abiAwareStatement {
	return wrap(a.Statement.Qual(path, name))
}

func Qual(path, name string) *abiAwareStatement {
	return wrap(jen.Qual(path, name))
}

func Op(op string) *abiAwareStatement {
	return wrap(jen.Op(op))
}

func Id(name string) *abiAwareStatement {
	return wrap(jen.Id(name))
}

func Empty() *abiAwareStatement {
	return wrap(jen.Empty())
}

func Func() *abiAwareStatement {
	return &abiAwareStatement{
		jen.Func(),
	}
}

func Var() *abiAwareStatement {
	return wrap(jen.Var())
}

func ValueConstructor(t types.Type) *abiAwareStatement {

	switch t.(type) {
	case types.String:
		return Qual(valuesImportPath, "String")
	}

	panic(fmt.Errorf("not supported type %T", t))

}

func GenerateGo(pkg string, typesToGenerate map[string]*types.Composite) ([]byte, error) {

	f := jen.NewFile(pkg)

	f.ImportName(encodingImportPath, "encoding")
	f.ImportName(typesImportPath, "types")
	f.ImportName(valuesImportPath, "values")

	for name, typ := range typesToGenerate {

		//Generating view-related items
		viewStructName := startLower(name) + "View"
		viewInterfaceName := startUpper(name) + "View"
		typeName := startLower(name) + "Type"
		typeVariableName := startLower(name) + "Type"

		fieldNames := make([]string, 0, len(typ.Fields))
		for _, field := range typ.Fields {
			fieldNames = append(fieldNames, field.Identifier)
		}
		encoding.EncodingOrder(fieldNames)
		fieldEncodingOrder := map[string]uint{}
		for i, field := range fieldNames {
			fieldEncodingOrder[field] = uint(i)
		}

		interfaceMethods := make([]jen.Code, 0, len(typ.Fields))
		viewStructFields := make([]jen.Code, 0, len(typ.Fields))
		viewInterfaceMethodsImpls := make([]jen.Code, 0, len(typ.Fields))

		decodeFunctionFields := jen.Dict{}

		for _, field := range typ.Fields {
			viewInterfaceMethodName := startUpper(field.Identifier)
			viewStructFieldName := _lower(field.Identifier)

			interfaceMethods = append(interfaceMethods, Id(viewInterfaceMethodName).Params().Type(field.Type))
			viewStructFields = append(viewStructFields, Id(viewStructFieldName).Type(field.Type))

			viewInterfaceMethodsImpls = append(viewInterfaceMethodsImpls, Func().Params(Id("t").Op("*").Id(viewStructName)).
				Id(viewInterfaceMethodName).Params().Type(field.Type).Block(
				jen.Return(jen.Id("t").Dot(viewStructFieldName)),
			))

			decodeFunctionFields[Id(viewStructFieldName)] =
				Id("v").Dot("Fields").Index(jen.Lit(fieldEncodingOrder[field.Identifier])).
					Dot("ToGoValue").Call().Assert(Empty().Type(field.Type))
		}

		// Main view interface
		f.Type().Id(viewInterfaceName).Interface(interfaceMethods...)

		// struct backing main view interface
		f.Type().Id(viewStructName).Struct(viewStructFields...)

		//Main view interface methods
		f.Add(viewInterfaceMethodsImpls...)

		//Main view decoding function
		f.Func().Id("Decode"+viewInterfaceName).Params(Id("b").Index().Byte()).Params(Id(viewInterfaceName), jen.Error()).Block(
			//r := bytes.NewReader(b)
			Id("r").Op(":=").Qual("bytes", "NewReader").Call(Id("b")),

			//	dec := encoding.NewDecoder(r)
			Id("dec").Op(":=").Qual(encodingImportPath, "NewDecoder").Call(Id("r")),

			//v, err := dec.DecodeComposite(carType)
			//if err != nil {
			//	return nil, err
			// }
			jen.List(Id("v"), Id("err")).Op(":=").Id("dec").Dot("DecodeComposite").Call(Id(typeName)),
			jen.If(Id("err").Op("!=").Nil()).Block(
				jen.Return(jen.List(jen.Nil(), Id("err"))),
			),

			//return &<Object>View{_<field>>: v.Fields[uint(0x0)].ToGoValue().(string)}, nil
			jen.Return(
				jen.List(jen.Op("&").Id(viewStructName).Values(decodeFunctionFields), jen.Nil()),
			),
		)

		//Object type structure
		f.Add(Var().Id(typeVariableName).Op("=").SelfType(*typ))

		//Generating constructors
		//TODO multiple contructors supported
		initializer := typ.Initializers[0]

		constructorInterfaceName := startUpper(name) + "Constructor"
		constructorStructName := startLower(name) + "Constructor"
		newConstructorName := "New" + startUpper(name) + "Constructor"

		constructorStructFields := make([]jen.Code, len(initializer))
		encodedConstructorFields := make([]jen.Code, len(initializer))
		constructorFields := make([]jen.Code, len(initializer))
		constructorObjectParams := jen.Dict{}
		for i, param := range initializer {
			label := param.Label
			if label == "" {
				label = param.Identifier
			}
			constructorStructFields[i] = Id(param.Identifier).Type(param.Type)

			encodedConstructorFields[i] = ValueConstructor(param.Type).Call(Id("p").Dot(param.Identifier))

			constructorFields[i] = Id(label).Type(param.Type)

			constructorObjectParams[Id(param.Identifier)] = Id(label)
		}

		//Constructor interface
		constructorEncodeFunction := Id("Encode").Params().Params(jen.Index().Byte(), jen.Error())
		f.Type().Id(constructorInterfaceName).Interface(
			constructorEncodeFunction,
		)

		//Constructor struct
		f.Type().Id(constructorStructName).Struct(constructorStructFields...)

		// Constructor encoding function
		f.Func().Params(Id("p").Id(constructorStructName)).Add(constructorEncodeFunction).Block(
			//	var w bytes.Buffer
			Var().Id("w").Qual("bytes", "Buffer"),

			// encoder := encoding.NewEncoder(&w)
			Id("encoder").Op(":=").Qual(encodingImportPath, "NewEncoder").Call(Op("&").Id("w")),

			//err := encoder.EncodeConstantSizedArray(
			Id("err").Op(":=").Id("encoder").Dot("EncodeConstantSizedArray").Call(
				Qual(valuesImportPath, "ConstantSizedArray").Values(encodedConstructorFields...),
			),

			jen.If(Id("err").Op("!=").Nil()).Block(
				jen.Return(jen.List(jen.Nil(), Id("err"))),
			),

			jen.Return(Id("w").Dot("Bytes").Call(), jen.Nil()),
		)

		//Constructor creator
		f.Func().Id(newConstructorName).Params(constructorFields...).Params(Id(constructorInterfaceName), jen.Error()).Block(
			jen.Return(Id(constructorStructName).Values(constructorObjectParams), jen.Nil()),
		)
	}

	f.Save("sdk/abi/generation/example/person.generated.go")

	return nil, nil

}
