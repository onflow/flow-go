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
		return a.Id(viewInterfaceName(v.TypeName))
	case *types.Optional:
		return a.Op("*").Type(v.Of)
	case *types.UInt8:
		return wrap(a.Uint8())
	case *types.UInt16:
		return wrap(a.Uint16())
	}

	panic(fmt.Errorf("not supported type %T", t))
}

var ifErrorBlock = jen.If(id("err").Op("!=").Nil()).Block(
	jen.Return(jen.List(jen.Nil(), id("err"))),
)

func FieldToGo(t types.Type, fieldName string, fieldAccessor *jen.Statement) (*jen.Statement, *abiAwareStatement) {
	switch v := t.(type) {
	case *types.StructPointer:
		return jen.List(id(fieldName), id("err")).Op(":=").Id(viewInterfaceFromComposite(v.TypeName)).Call(fieldAccessor.Assert(qual(valuesImportPath, "Composite"))).Line().Add(ifErrorBlock), id(fieldName)
	case *types.VariableSizedArray:
		id(fieldName).Op(":=").Make(wrap(jen.Index()).Type(t), jen.Lit(0))

	}
	return nil, wrap(fieldAccessor.Dot("ToGoValue").Call().Assert(empty().Type(t)))
}

const (
	typesImportPath          = "github.com/dapperlabs/flow-go/sdk/abi/types"
	typesEncodingImportPath  = "github.com/dapperlabs/flow-go/sdk/abi/encoding/types"
	valuesImportPath         = "github.com/dapperlabs/flow-go/sdk/abi/values"
	valuesEncodingImportPath = "github.com/dapperlabs/flow-go/sdk/abi/encoding/values"
)

// SelfType write t as itself in Go
func (a *abiAwareStatement) SelfType(t types.Type, allTypesMap map[string]*types.Composite) *abiAwareStatement {
	switch v := t.(type) {
	case *types.String:
		return wrap(a.Statement.Qual(typesImportPath, "String").Values())
	case *types.Composite:
		mappedFields := jen.Dict{}
		for key, field := range v.Fields {
			mappedFields[jen.Lit(key)] = empty().SelfType(field, allTypesMap)
		}
		mappedInitializers := make([]jen.Code, len(v.Initializers))
		for i, initializer := range v.Initializers {
			params := make([]jen.Code, len(initializer))
			for i, param := range v.Initializers[i] {
				params[i] = op("&").Qual(typesImportPath, "Parameter").SelfType(param, allTypesMap)
			}

			mappedInitializers[i] = jen.Values(params...)
		}
		return wrap(a.Statement.Qual(typesImportPath, "Composite").Values(jen.Dict{
			id("Fields"):       jen.Map(jen.String()).Op("*").Qual(typesImportPath, "Field").Values(mappedFields),
			id("Initializers"): jen.Index().Index().Op("*").Qual(typesImportPath, "Parameter").Values(mappedInitializers...),
			id("Identifier"):   jen.Lit(v.Identifier),
		}))
	case *types.Field:
		return wrap(a.Statement.Values(jen.Dict{
			id("Type"):       empty().SelfType(v.Type, allTypesMap),
			id("Identifier"): jen.Lit(v.Identifier),
		}))
	case *types.Parameter:
		return wrap(a.Statement.Values(jen.Dict{
			id("Field"): qual(typesImportPath, "Field").SelfType(&v.Field, allTypesMap),
			id("Label"): jen.Lit(v.Label),
		}))
	case *types.VariableSizedArray:
		return wrap(a.Statement.Qual(typesImportPath, "VariableSizedArray").Values(jen.Dict{
			id("ElementType"): empty().SelfType(v.ElementType, allTypesMap),
		}))
	case *types.StructPointer: //Here we attach real type object rather then re-print pointer
		if _, ok := allTypesMap[v.TypeName]; ok {
			return a.Id(typeVariableName(v.TypeName))
		}
		panic(fmt.Errorf("StructPointer to unknown type name %s", v))
	case *types.Optional:
		return wrap(a.Statement.Qual(typesImportPath, "Optional").Values(jen.Dict{
			id("Of"): empty().SelfType(v.Of, allTypesMap),
		}))
	case *types.UInt8:
		return wrap(a.Statement.Qual(typesImportPath, "UInt8").Values())
	case *types.UInt16:
		return wrap(a.Statement.Qual(typesImportPath, "UInt16").Values())
	}

	panic(fmt.Errorf("not supported type %T", t))
}

func (a *abiAwareStatement) Params(params ...jen.Code) *abiAwareStatement {
	return &abiAwareStatement{a.Statement.Params(params...)}
}

//revive:disable:var-naming We want to keep it called like this
func (a *abiAwareStatement) Id(name string) *abiAwareStatement {
	return &abiAwareStatement{a.Statement.Id(name)}
}

//revive:enable:var-naming

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
	return qual(valuesImportPath, "NewValueOrPanic").Call(fieldExpr)
}

func viewInterfaceName(name string) string {
	return startUpper(name) + "View"
}

func constructorStructName(name string) string {
	return startLower(name) + "Constructor"
}

func typeVariableName(name string) string {
	return startLower(name) + "Type"
}

func viewInterfaceFromComposite(name string) string {
	return viewInterfaceName(name) + "fromComposite"
}

func GenerateGo(pkg string, typesToGenerate map[string]*types.Composite, writer io.Writer) error {

	f := jen.NewFile(pkg)

	f.HeaderComment("Code generated by Flow Go SDK. DO NOT EDIT.")

	f.ImportName(typesEncodingImportPath, "types")
	f.ImportName(valuesEncodingImportPath, "values")
	f.ImportName(typesImportPath, "types")
	f.ImportName(valuesImportPath, "values")

	for name, typ := range typesToGenerate {

		//Generating view-related items
		viewStructName := startLower(name) + "View"
		viewInterfaceName := viewInterfaceName(name)
		typeName := startLower(name) + "Type"
		typeVariableName := typeVariableName(name)
		viewInterfaceFromComposite := viewInterfaceFromComposite(name)

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
		decodeFunctionPrepareStatement := make([]jen.Code, 0)

		for _, field := range typ.Fields {
			viewInterfaceMethodName := startUpper(field.Identifier)
			viewStructFieldName := _lower(field.Identifier)

			interfaceMethods = append(interfaceMethods, id(viewInterfaceMethodName).Params().Type(field.Type))
			viewStructFields = append(viewStructFields, id(viewStructFieldName).Type(field.Type))

			viewInterfaceMethodsImpls = append(viewInterfaceMethodsImpls, function().Params(id("t").Op("*").Id(viewStructName)).
				Id(viewInterfaceMethodName).Params().Type(field.Type).Block(
				jen.Return(jen.Id("t").Dot(viewStructFieldName)),
			))

			preparation, value := FieldToGo(field.Type, viewStructFieldName, id("composite").Dot("Fields").Index(jen.Lit(fieldEncodingOrder[field.Identifier])))

			decodeFunctionPrepareStatement = append(decodeFunctionPrepareStatement, preparation)

			decodeFunctionFields[id(viewStructFieldName)] = value
		}

		// Main view interface
		f.Type().Id(viewInterfaceName).Interface(interfaceMethods...)

		// struct backing main view interface
		f.Type().Id(viewStructName).Struct(viewStructFields...)

		//Main view interface methods
		for _, viewInterfaceMethodImpl := range viewInterfaceMethodsImpls {
			f.Add(viewInterfaceMethodImpl)
		}

		f.Func().Id(viewInterfaceFromComposite).Params(id("composite").Qual(valuesImportPath, "Composite")).Params(id(viewInterfaceName), jen.Error()).Block(
			//return &<Object>View{_<field>>: v.Fields[uint(0x0)].ToGoValue().(string)}, nil

			empty().Add(decodeFunctionPrepareStatement...),

			jen.Return(
				jen.List(jen.Op("&").Id(viewStructName).Values(decodeFunctionFields), jen.Nil()),
			),
		)

		//Main view decoding function

		f.Func().Id("Decode"+viewInterfaceName).Params(id("b").Index().Byte()).Params(id(viewInterfaceName), jen.Error()).Block(
			//r := bytes.NewReader(b)
			id("r").Op(":=").Qual("bytes", "NewReader").Call(id("b")),

			//dec := encoding.NewDecoder(r)
			id("dec").Op(":=").Qual(valuesEncodingImportPath, "NewDecoder").Call(id("r")),

			//v, err := dec.DecodeComposite(carType)
			//if err != nil {
			//	return nil, err
			// }
			jen.List(id("v"), id("err")).Op(":=").Id("dec").Dot("DecodeComposite").Call(id(typeName)),
			ifErrorBlock,

			jen.Return(id(viewInterfaceFromComposite).Call(id("v"))),
		)

		// Variable size array of main views decoding function
		f.Func().Id("Decode"+viewInterfaceName+"VariableSizedArray").Params(id("b").Index().Byte()).Params(jen.Index().Id(viewInterfaceName), jen.Error()).Block(
			//r := bytes.NewReader(b)
			id("r").Op(":=").Qual("bytes", "NewReader").Call(id("b")),

			//dec := encoding.NewDecoder(r)
			id("dec").Op(":=").Qual(valuesEncodingImportPath, "NewDecoder").Call(id("r")),

			//v, err := dec.DecodeVariableSizedArray(carType)
			//if err != nil {
			//	return nil, err
			// }
			jen.List(id("v"), id("err")).Op(":=").Id("dec").Dot("DecodeVariableSizedArray").Call(qual(typesImportPath, "VariableSizedArray").Values(jen.Dict{id("ElementType"): id(typeName)})),
			ifErrorBlock,

			//array := make([]<viewInterface>, len(v))
			id("array").Op(":=").Make(jen.Index().Id(viewInterfaceName), jen.Len(id("v"))),

			//for i, t := range v {
			//  array[i], err =  <View>FromComposite(t.(<type>))
			//  if err != nil {
			//    return nil, err
			//  }
			//}
			jen.For(jen.List(id("i"), id("t"))).Op(":=").Range().Id("v").Block(
				jen.List(id("array").Index(id("i")), id("err")).Op("=").Id(viewInterfaceFromComposite).Call(id("t").Assert(qual(valuesImportPath, "Composite"))),
				ifErrorBlock,
			),

			//return array, nil
			jen.Return(id("array"), jen.Nil()),
		)

		//Object type structure
		f.Add(variable().Id(typeVariableName).Op("=").SelfType(typ, typesToGenerate))

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
				//p.toValue()
				id("p").Dot("toValue").Call(),
			),

			//if err != nil {
			//  return nil, err
			//}
			ifErrorBlock,

			//return w.Bytes(), nil
			jen.Return(id("w").Dot("Bytes").Call(), jen.Nil()),
		)

		//Constructor creator
		f.Func().Id(newConstructorName).Params(constructorFields...).Params(id(constructorInterfaceName), jen.Error()).Block(
			jen.Return(id(constructorStructName).Values(constructorObjectParams), jen.Nil()),
		)
	}

	return f.Render(writer)
}
