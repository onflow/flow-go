package code

import (
	"fmt"
	"io"
	"unicode"

	"github.com/dapperlabs/flow-go/sdk/abi/encoding/values"
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

// write type information as a part of statement
func (a *abiAwareStatement) Type(t types.Type) *abiAwareStatement {
	switch v := t.(type) {
	case *types.String:
		return wrap(a.String())
	case *types.VariableSizedArray:
		return wrap(a.Index()).Type(v.ElementType)
	case *types.StructPointer:
		return a.Id(constructorStructName(v.TypeName))
	case *types.Optional:
		return a.Op("*").Type(v.Of)
	}

	panic(fmt.Errorf("not supported type %T", t))
}

const (
	typesImportPath          = "github.com/dapperlabs/flow-go/sdk/abi/types"
	typesEncodingImportPath  = "github.com/dapperlabs/flow-go/sdk/abi/encoding/types"
	valuesImportPath         = "github.com/dapperlabs/flow-go/sdk/abi/values"
	valuesEncodingImportPath = "github.com/dapperlabs/flow-go/sdk/abi/encoding/values"
)

// SelfType write t as itself in Go
func (a *abiAwareStatement) SelfType(t types.Type) *abiAwareStatement {
	switch v := t.(type) {
	case *types.String:
		return wrap(a.Statement.Qual(typesImportPath, "String").Values())
	case *types.Composite:
		mappedFields := jen.Dict{}
		for key, field := range v.Fields {
			mappedFields[jen.Lit(key)] = empty().SelfType(field)
		}
		mappedInitializers := make([]jen.Code, len(v.Initializers))
		for i, initializer := range v.Initializers {
			params := make([]jen.Code, len(initializer))
			for i, param := range v.Initializers[i] {
				params[i] = op("&").Qual(typesImportPath, "Parameter").SelfType(param)
			}

			mappedInitializers[i] = jen.Values(params...)
		}
		return wrap(a.Statement.Qual(typesImportPath, "Composite").Values(jen.Dict{
			id("Fields"):       jen.Map(jen.String()).Op("*").Qual(typesImportPath, "Field").Values(mappedFields),
			id("Initializers"): jen.Index().Index().Op("*").Qual(typesImportPath, "Parameter").Values(mappedInitializers...),
		}))
	case *types.Field:
		return wrap(a.Statement.Values(jen.Dict{
			id("Type"):       empty().SelfType(v.Type),
			id("Identifier"): jen.Lit(v.Identifier),
		}))
	case *types.Parameter:
		return wrap(a.Statement.Values(jen.Dict{
			id("Field"): qual(typesImportPath, "Field").SelfType(&v.Field),
			id("Label"): jen.Lit(v.Label),
		}))
	case *types.VariableSizedArray:
		return wrap(a.Statement.Qual(typesImportPath, "VariableSizedArray").Values(jen.Dict{
			id("ElementType"): empty().SelfType(v.ElementType),
		}))
	case *types.StructPointer:
		return wrap(a.Statement.Qual(typesImportPath, "StructPointer").Values(jen.Dict{
			id("TypeName"): jen.Lit(v.TypeName),
		}))
	case *types.Optional:
		return wrap(a.Statement.Qual(typesImportPath, "Optional").Values(jen.Dict{
			id("Of"): empty().SelfType(v.Of),
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

func qual(path, name string) *abiAwareStatement {
	return wrap(jen.Qual(path, name))
}

func op(op string) *abiAwareStatement {
	return wrap(jen.Op(op))
}

func id(name string) *abiAwareStatement {
	return wrap(jen.Id(name))
}

func empty() *abiAwareStatement {
	return wrap(jen.Empty())
}

func function() *abiAwareStatement {
	return &abiAwareStatement{
		jen.Func(),
	}
}

func variable() *abiAwareStatement {
	return wrap(jen.Var())
}

func valueConstructor(t types.Type, fieldExpr *jen.Statement) *abiAwareStatement {

	switch v := t.(type) {
	case *types.String:
		return qual(valuesImportPath, "String").Call(fieldExpr)
	case *types.StructPointer:
		return wrap(fieldExpr.Dot("toValue").Call())
	case *types.Optional:
		return valueConstructor(v.Of, fieldExpr)
	}

	panic(fmt.Errorf("not supported type %T", t))

}

func viewInterfaceName(name string) string {
	return startUpper(name) + "View"
}

func constructorStructName(name string) string {
	return startLower(name) + "Constructor"
}

func GenerateGo(pkg string, typesToGenerate map[string]*types.Composite, writer io.Writer) error {

	f := jen.NewFile(pkg)

	f.HeaderComment("Code generated by Flow Go SDK. DO NOT EDIT")

	f.ImportName(typesEncodingImportPath, "types")
	f.ImportName(valuesEncodingImportPath, "values")
	f.ImportName(typesImportPath, "types")
	f.ImportName(valuesImportPath, "values")

	for name, typ := range typesToGenerate {

		//Generating view-related items
		viewStructName := startLower(name) + "View"
		viewInterfaceName := viewInterfaceName(name)
		typeName := startLower(name) + "Type"
		typeVariableName := startLower(name) + "Type"

		fieldNames := make([]string, 0, len(typ.Fields))
		for _, field := range typ.Fields {
			fieldNames = append(fieldNames, field.Identifier)
		}
		values.SortInEncodingOrder(fieldNames)
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

			interfaceMethods = append(interfaceMethods, id(viewInterfaceMethodName).Params().Type(field.Type))
			viewStructFields = append(viewStructFields, id(viewStructFieldName).Type(field.Type))

			viewInterfaceMethodsImpls = append(viewInterfaceMethodsImpls, function().Params(id("t").Op("*").Id(viewStructName)).
				Id(viewInterfaceMethodName).Params().Type(field.Type).Block(
				jen.Return(jen.Id("t").Dot(viewStructFieldName)),
			))

			decodeFunctionFields[id(viewStructFieldName)] =
				id("v").Dot("Fields").Index(jen.Lit(fieldEncodingOrder[field.Identifier])).
					Dot("ToGoValue").Call().Assert(empty().Type(field.Type))
		}

		// Main view interface
		f.Type().Id(viewInterfaceName).Interface(interfaceMethods...)

		// struct backing main view interface
		f.Type().Id(viewStructName).Struct(viewStructFields...)

		//Main view interface methods
		for _, viewInterfaceMethodImpl := range viewInterfaceMethodsImpls {
			f.Add(viewInterfaceMethodImpl)
		}
		//f.Add(viewInterfaceMethodsImpls...)

		//Main view decoding function
		f.Func().Id("Decode"+viewInterfaceName).Params(id("b").Index().Byte()).Params(id(viewInterfaceName), jen.Error()).Block(
			//r := bytes.NewReader(b)
			id("r").Op(":=").Qual("bytes", "NewReader").Call(id("b")),

			//	dec := encoding.NewDecoder(r)
			id("dec").Op(":=").Qual(valuesEncodingImportPath, "NewDecoder").Call(id("r")),

			//v, err := dec.DecodeComposite(carType)
			//if err != nil {
			//	return nil, err
			// }
			jen.List(id("v"), id("err")).Op(":=").Id("dec").Dot("DecodeComposite").Call(id(typeName)),
			jen.If(id("err").Op("!=").Nil()).Block(
				jen.Return(jen.List(jen.Nil(), id("err"))),
			),

			//return &<Object>View{_<field>>: v.Fields[uint(0x0)].ToGoValue().(string)}, nil
			jen.Return(
				jen.List(jen.Op("&").Id(viewStructName).Values(decodeFunctionFields), jen.Nil()),
			),
		)

		//Object type structure
		f.Add(variable().Id(typeVariableName).Op("=").SelfType(typ))

		//Generating constructors
		//TODO multiple contructors supported
		initializer := typ.Initializers[0]

		constructorInterfaceName := startUpper(name) + "Constructor"
		constructorStructName := constructorStructName(name)
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
			constructorStructFields[i] = id(param.Identifier).Type(param.Type)

			encodedConstructorFields[i] = valueConstructor(param.Type, id("p").Dot(param.Identifier))

			constructorFields[i] = id(label).Type(param.Type)

			constructorObjectParams[id(param.Identifier)] = id(label)
		}

		//Constructor interface
		constructorEncodeFunction := id("Encode").Params().Params(jen.Index().Byte(), jen.Error())
		f.Type().Id(constructorInterfaceName).Interface(
			constructorEncodeFunction,
		)

		//Constructor struct
		f.Type().Id(constructorStructName).Struct(constructorStructFields...)

		f.Func().Params(id("p").Id(constructorStructName)).Id("toValue").Params().Qual(valuesImportPath, "ConstantSizedArray").Block(
			jen.Return(qual(valuesImportPath, "ConstantSizedArray").Values(encodedConstructorFields...)),
		)

		// Constructor encoding function
		f.Func().Params(id("p").Id(constructorStructName)).Add(constructorEncodeFunction).Block(
			//	var w bytes.Buffer
			variable().Id("w").Qual("bytes", "Buffer"),

			// encoder := encoding.NewEncoder(&w)
			id("encoder").Op(":=").Qual(valuesEncodingImportPath, "NewEncoder").Call(op("&").Id("w")),

			//err := encoder.EncodeConstantSizedArray(
			id("err").Op(":=").Id("encoder").Dot("EncodeConstantSizedArray").Call(
				id("p").Dot("toValue").Call(),
			),

			jen.If(id("err").Op("!=").Nil()).Block(
				jen.Return(jen.List(jen.Nil(), id("err"))),
			),

			jen.Return(id("w").Dot("Bytes").Call(), jen.Nil()),
		)

		//Constructor creator
		f.Func().Id(newConstructorName).Params(constructorFields...).Params(id(constructorInterfaceName), jen.Error()).Block(
			jen.Return(id(constructorStructName).Values(constructorObjectParams), jen.Nil()),
		)
	}

	return f.Render(writer)
}
