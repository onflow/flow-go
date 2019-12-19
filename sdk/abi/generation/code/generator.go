package code

import (
	"fmt"
	"io"
	"strconv"
	"unicode"

	"github.com/dave/jennifer/jen"

	"github.com/dapperlabs/flow-go/sdk/abi/encoding/values"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
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

// Wrapper for jen.Statement which adds some useful methods
type abiAwareStatement struct {
	*jen.Statement
}

func wrap(s *jen.Statement) *abiAwareStatement {
	return &abiAwareStatement{s}
}

// write type information as a part of statement
func (a *abiAwareStatement) Type(t types.Type) *abiAwareStatement {
	switch v := t.(type) {
	case types.String:
		return wrap(a.String())
	case types.VariableSizedArray:
		return a.Index().Type(v.ElementType)
	case types.StructPointer:
		return a.Id(viewInterfaceName(v.TypeName))
	case types.Optional:
		return a.Op("*").Type(v.Type)
	case types.Int:
		return wrap(a.Int())
	case types.UInt8:
		return wrap(a.Uint8())
	case types.UInt16:
		return wrap(a.Uint16())
	}

	panic(fmt.Errorf("not supported type %T", t))
}

var ifErrorBlock = jen.If(id("err").Op("!=").Nil()).Block(
	jen.Return(jen.List(jen.Nil(), id("err"))),
).Line()

var converterFunctions = map[string]*abiAwareStatement{}
var converterTypesCache = map[types.Type]string{}
var converterCounter = 0

// converterFor generates a converter function which converts
// interface{} to desired type.
// Due to Go limitation with casting and arbitrary nesting
// of structures in Cadence, generating this seems like a best
// approach
func converterFor(t types.Type) *abiAwareStatement {

	paramName := "p"

	// Caches/creates a converter function for a type
	funcWrapper := func(statement *jen.Statement) *abiAwareStatement {

		if name, ok := converterTypesCache[t]; ok {
			return id(name)
		}

		name := "__converter" + strconv.Itoa(converterCounter)
		f := wrap(jen.Func().Id(name).Params(id(paramName).Qual(valuesImportPath, "Value")).
			Params(empty().Type(t), jen.Error()).Block(statement))

		converterFunctions[name] = f
		converterTypesCache[t] = name

		converterCounter++

		return id(name)
	}

	var convert func(t types.Type, depth int, writeTo *abiAwareStatement, param *abiAwareStatement) *abiAwareStatement

	param := id(paramName)

	// Builds "intelligent" nil check
	ifNil := func(value *abiAwareStatement, writeTo *abiAwareStatement, code []jen.Code) *abiAwareStatement {
		if writeTo == nil {
			return wrap(jen.If(value.Op("==").Nil()).Block(
				jen.Return(jen.Nil(), jen.Nil()),
			).Else().Block(code...).Line())
		}
		return wrap(jen.If(value.Op("==").Nil()).Block(
			writeTo.Clone().Op("=").Nil(),
		).Else().Block(code...).Line())
	}

	convert = func(t types.Type, depth int, writeTo *abiAwareStatement, param *abiAwareStatement) *abiAwareStatement {

		retVariable := "ret" + strconv.Itoa(depth)
		goVariable := "go" + strconv.Itoa(depth)
		castVariable := "cast" + strconv.Itoa(depth)

		// casts and converts to given types
		goCast := func(target *abiAwareStatement, value *abiAwareStatement, typ jen.Code, errorZero *abiAwareStatement) *abiAwareStatement {
			return wrap(&jen.Statement{
				jen.List(id(castVariable), id("ok")).Op(":=").Add(value.Clone()).Assert(typ).Line(),
				jen.If(jen.Op("!").Id("ok")).Block(
					jen.Return(errorZero, qual("fmt", "Errorf").Call(jen.Lit("cannot cast %T"), value)).Line(),
				).Line(),
				jen.Do(func(statement *jen.Statement) {
					if target != nil {
						statement.Add(target.Clone().Op("=").Id(castVariable).Dot("ToGoValue").Call().Line())
					}
				}),
			})
		}

		switch v := t.(type) {
		case types.Optional:
			if writeTo == nil {
				return funcWrapper(&jen.Statement{
					variable().Id(retVariable).Type(v.Type).Line(),
					variable().Id(goVariable).Interface().Line(),
					goCast(id(goVariable), param, qual(valuesImportPath, "Optional"), nilStatement()).Line(),
					variable().Err().Error().Line(),
					ifNil(id(goVariable), writeTo, []jen.Code{
						convert(v.Type, depth+1, id(retVariable), id(castVariable).Dot("Value")).Line(),
					}),
					jen.Return(op("&").Id(retVariable), jen.Nil()).Line(),
				})
			}
			return wrap(&jen.Statement{
				variable().Id(retVariable).Type(v.Type).Line(),
				variable().Id(goVariable).Interface().Line(),
				goCast(id(goVariable), param, qual(valuesImportPath, "Optional"), nilStatement()).Line(),
				ifNil(id(goVariable), writeTo, []jen.Code{
					convert(v.Type, depth+1, id(retVariable), id(castVariable).Dot("Value")).Line(),
					writeTo.Clone().Op("=").Op("&").Id(retVariable).Line(),
				}),
			})

		case types.Int:
			if writeTo == nil {
				return qual(valuesImportPath, "CastToInt")
			}
			return wrap(&jen.Statement{
				goCast(nil, param, qual(valuesImportPath, "Int"), id(retVariable)).Line(),
				jen.List(writeTo, id("err")).Op("=").Qual(valuesImportPath, "CastToInt").Call(id(castVariable)).Line(),
				ifErrorBlock,
			})
		case types.String:
			if writeTo == nil {
				return qual(valuesImportPath, "CastToString")
			}
			return wrap(&jen.Statement{
				goCast(nil, param, qual(valuesImportPath, "String"), nilStatement()).Line(),
				jen.List(writeTo, id("err")).Op("=").Qual(valuesImportPath, "CastToString").Call(id(castVariable)).Line(),
				ifErrorBlock,
			})
		case types.UInt8:
			if writeTo == nil {
				return qual(valuesImportPath, "CastToUInt8")
			}
			return wrap(&jen.Statement{
				goCast(nil, param, qual(valuesImportPath, "UInt8"), nilStatement()).Line(),
				jen.List(writeTo, id("err")).Op("=").Qual(valuesImportPath, "CastToUInt8").Call(id(castVariable)).Line(),
				ifErrorBlock,
			})
		case types.UInt16:
			if writeTo == nil {
				return qual(valuesImportPath, "CastToUInt16")
			}
			return wrap(&jen.Statement{
				goCast(nil, param, qual(valuesImportPath, "UInt16"), nilStatement()).Line(),
				jen.List(writeTo, id("err")).Op("=").Qual(valuesImportPath, "CastToUInt16").Call(id(castVariable)).Line(),
				ifErrorBlock,
			})
		case types.StructPointer:
			if writeTo == nil {
				return id(viewInterfaceFromValue(v.TypeName))
			}
			return wrap(&jen.Statement{
				jen.List(writeTo, id("err")).Op("=").Id(viewInterfaceFromValue(v.TypeName)).Call(param).Line(),
				ifErrorBlock,
			})
		case types.VariableSizedArray:
			elemVariable := "elem" + strconv.Itoa(depth)
			iterVariable := "i" + strconv.Itoa(depth)
			if writeTo == nil {
				return funcWrapper(&jen.Statement{
					variable().Id(retVariable).Index().Type(v.ElementType).Line(),
					goCast(nil, param, qual(valuesImportPath, "VariableSizedArray"), nilStatement()).Line(),
					variable().Err().Error().Line(),
					ifErrorBlock,
					id(retVariable).Op("=").Make(index().Type(v.ElementType), jen.Len(id(castVariable).Dot("Values"))).Line(),
					jen.For().List(id(iterVariable), id(elemVariable)).Op(":=").Range().Id(castVariable).Dot("Values").Block(
						convert(v.ElementType, depth+1, id(retVariable).Index(id(iterVariable)), id(elemVariable).Clone()).Line(),
					).Line(),
					jen.Return(id(retVariable), jen.Nil()).Line(),
				})
			}
			return wrap(&jen.Statement{
				variable().Id(retVariable).Index().Type(v.ElementType).Line(),
				goCast(nil, param, qual(valuesImportPath, "VariableSizedArray"), nilStatement()).Line(),
				ifErrorBlock,
				id(retVariable).Op("=").Make(index().Type(v.ElementType), jen.Len(id(castVariable).Dot("Values"))).Line(),
				jen.For().List(id(iterVariable), id(elemVariable)).Op(":=").Range().Id(castVariable).Dot("Values").Block(
					convert(v.ElementType, depth+1, id(retVariable).Index(id(iterVariable)), id(elemVariable).Clone()).Line(),
				).Line(),
				writeTo.Clone().Op("=").Id(retVariable).Line(),
			})

		}

		panic(fmt.Errorf("unsupoprted type %T cor converter generation", t))
	}

	return convert(t, 0, nil, param)
}

const (
	typesImportPath          = "github.com/dapperlabs/flow-go/sdk/abi/types"
	typesEncodingImportPath  = "github.com/dapperlabs/flow-go/sdk/abi/encoding/types"
	valuesImportPath         = "github.com/dapperlabs/flow-go/sdk/abi/values"
	valuesEncodingImportPath = "github.com/dapperlabs/flow-go/sdk/abi/encoding/values"
)

// SelfType writes t as itself in Go
func (a *abiAwareStatement) SelfType(t types.Type, allTypesMap map[string]types.Composite) *abiAwareStatement {
	switch v := t.(type) {
	case types.String:
		return wrap(a.Statement.Qual(typesImportPath, "String").Values())
	case types.Composite:
		mappedFields := make([]jen.Code, len(v.Fields))

		for i, field := range v.Fields {
			mappedFields[i] = qual(typesImportPath, "Field").Values(
				jen.Dict{
					id("Identifier"): jen.Lit(field.Identifier),
					id("Type"):       empty().SelfType(field.Type, allTypesMap),
				},
			)
		}

		mappedInitializers := make([]jen.Code, len(v.Initializers))

		for i, initializer := range v.Initializers {
			params := make([]jen.Code, len(initializer))
			for i, param := range v.Initializers[i] {
				params[i] = qual(typesImportPath, "Parameter").Values(
					jen.Dict{
						id("Label"):      jen.Lit(param.Label),
						id("Identifier"): jen.Lit(param.Identifier),
						id("Type"):       empty().SelfType(param.Type, allTypesMap),
					},
				)
			}

			mappedInitializers[i] = jen.Values(params...)
		}

		return wrap(a.Statement.Qual(typesImportPath, "Composite").Values(jen.Dict{
			id("Identifier"):   jen.Lit(v.Identifier),
			id("Fields"):       jen.Index().Qual(typesImportPath, "Field").Values(mappedFields...),
			id("Initializers"): jen.Index().Index().Qual(typesImportPath, "Parameter").Values(mappedInitializers...),
		}))
	case types.VariableSizedArray:
		return wrap(a.Statement.Qual(typesImportPath, "VariableSizedArray").Values(jen.Dict{
			id("ElementType"): empty().SelfType(v.ElementType, allTypesMap),
		}))
	case types.StructPointer: //Here we attach real type object rather then re-print pointer
		if _, ok := allTypesMap[v.TypeName]; ok {
			return a.Id(typeVariableName(v.TypeName))
		}
		panic(fmt.Errorf("StructPointer to unknown type name %s", v))
	case types.Optional:
		return wrap(a.Statement.Qual(typesImportPath, "Optional").Values(jen.Dict{
			id("Type"): empty().SelfType(v.Type, allTypesMap),
		}))
	case types.UInt8:
		return wrap(a.Statement.Qual(typesImportPath, "UInt8").Values())
	case types.UInt16:
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

func (a *abiAwareStatement) Dot(name string) *abiAwareStatement {
	return wrap(a.Statement.Dot(name))
}

func (a *abiAwareStatement) Qual(path, name string) *abiAwareStatement {
	return wrap(a.Statement.Qual(path, name))
}

func (a *abiAwareStatement) Block(statements ...jen.Code) *abiAwareStatement {
	return wrap(a.Statement.Block(statements...))
}

func (a *abiAwareStatement) Clone() *abiAwareStatement {
	return wrap(a.Statement.Clone())
}

func (a *abiAwareStatement) Assert(typ jen.Code) *abiAwareStatement {
	return wrap(a.Statement.Assert(typ))
}

func (a *abiAwareStatement) Index(items ...jen.Code) *abiAwareStatement {
	return wrap(a.Statement.Index(items...))
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

func nilStatement() *abiAwareStatement {
	return wrap(jen.Nil())
}

func index() *abiAwareStatement {
	return wrap(jen.Index())
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
	return qual(valuesImportPath, "MustConvertValue").Call(fieldExpr)
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

func viewInterfaceFromValue(name string) string {
	return viewInterfaceName(name) + "fromValue"
}

func GenerateGo(pkg string, typesToGenerate map[string]types.Composite, writer io.Writer) error {

	f := jen.NewFile(pkg)

	f.HeaderComment("Code generated by Flow Go SDK. DO NOT EDIT.")

	f.ImportName(typesEncodingImportPath, "types")
	f.ImportName(valuesEncodingImportPath, "values")
	f.ImportName(typesImportPath, "types")
	f.ImportName(valuesImportPath, "values")

	names := make([]string, 0, len(typesToGenerate))
	for name, _ := range typesToGenerate {
		names = append(names, name)
	}
	values.SortInEncodingOrder(names)

	for _, name := range names {
		typ := typesToGenerate[name]

		// Generating view-related items
		viewStructName := startLower(name) + "View"
		viewInterfaceName := viewInterfaceName(name)
		typeName := startLower(name) + "Type"
		typeVariableName := typeVariableName(name)
		viewInterfaceFromValue := viewInterfaceFromValue(name)

		interfaceMethods := make([]jen.Code, 0, len(typ.Fields))
		viewStructFields := make([]jen.Code, 0, len(typ.Fields))
		viewInterfaceMethodsImpls := make([]jen.Code, 0, len(typ.Fields))

		decodeFunctionFields := jen.Dict{}
		decodeFunctionPrepareStatement := make([]jen.Code, 0)

		for i, field := range typ.Fields {
			viewInterfaceMethodName := startUpper(field.Identifier)
			viewStructFieldName := _lower(field.Identifier)

			interfaceMethods = append(interfaceMethods, id(viewInterfaceMethodName).Params().Type(field.Type))
			viewStructFields = append(viewStructFields, id(viewStructFieldName).Type(field.Type))

			viewInterfaceMethodsImpls = append(viewInterfaceMethodsImpls, function().Params(id("t").Op("*").Id(viewStructName)).
				Id(viewInterfaceMethodName).Params().Type(field.Type).Block(
				jen.Return(jen.Id("t").Dot(viewStructFieldName)),
			))

			fieldAccessor := id("composite").Dot("Fields").Index(jen.Lit(i))

			preparation := jen.List(id(viewStructFieldName), id("err")).Op(":=").Add(converterFor(field.Type)).Call(fieldAccessor)

			decodeFunctionPrepareStatement = append(decodeFunctionPrepareStatement, preparation.Line().Add(ifErrorBlock).Line())

			decodeFunctionFields[id(viewStructFieldName)] = id(viewStructFieldName)
		}

		// Main view interface
		f.Type().Id(viewInterfaceName).Interface(interfaceMethods...)

		// struct backing main view interface
		f.Type().Id(viewStructName).Struct(viewStructFields...)

		//Main view interface methods
		for _, viewInterfaceMethodImpl := range viewInterfaceMethodsImpls {
			f.Add(viewInterfaceMethodImpl)
		}

		f.Func().Id(viewInterfaceFromValue).Params(id("value").Qual(valuesImportPath, "Value")).Params(id(viewInterfaceName), jen.Error()).Block(
			jen.List(id("composite"), jen.Err()).Op(":=").Qual(valuesImportPath, "CastToComposite").Call(id("value")),
			ifErrorBlock,

			empty().Add(decodeFunctionPrepareStatement...),

			// return &<Object>View{_<field>>: v.Fields[uint(0x0)].ToGoValue().(string)}, nil
			jen.Return(
				jen.List(jen.Op("&").Id(viewStructName).Values(decodeFunctionFields), jen.Nil()),
			),
		)

		//Main view decoding function
		f.Func().Id("Decode"+viewInterfaceName).Params(id("b").Index().Byte()).Params(id(viewInterfaceName), jen.Error()).Block(
			// r := bytes.NewReader(b)
			id("r").Op(":=").Qual("bytes", "NewReader").Call(id("b")),

			// dec := encoding.NewDecoder(r)
			id("dec").Op(":=").Qual(valuesEncodingImportPath, "NewDecoder").Call(id("r")),

			// v, err := dec.DecodeComposite(carType)
			// if err != nil {
			//   return nil, err
			// }
			jen.List(id("v"), id("err")).Op(":=").Id("dec").Dot("DecodeComposite").Call(id(typeName)),
			ifErrorBlock,

			jen.Return(id(viewInterfaceFromValue).Call(id("v"))),
		)

		// Variable size array of main views decoding function
		f.Func().Id("Decode"+viewInterfaceName+"VariableSizedArray").Params(id("b").Index().Byte()).Params(jen.Index().Id(viewInterfaceName), jen.Error()).Block(
			// r := bytes.NewReader(b)
			id("r").Op(":=").Qual("bytes", "NewReader").Call(id("b")),

			// dec := encoding.NewDecoder(r)
			id("dec").Op(":=").Qual(valuesEncodingImportPath, "NewDecoder").Call(id("r")),

			// v, err := dec.DecodeVariableSizedArray(carType)
			// if err != nil {
			//   return nil, err
			// }
			jen.List(id("v"), id("err")).Op(":=").Id("dec").Dot("DecodeVariableSizedArray").Call(qual(typesImportPath, "VariableSizedArray").Values(jen.Dict{id("ElementType"): id(typeName)})),
			ifErrorBlock,

			//  array := make([]<viewInterface>, len(v.Values))
			id("array").Op(":=").Make(jen.Index().Id(viewInterfaceName), jen.Len(id("v").Dot("Values"))),

			// for i, t := range v {
			//   array[i], err =  <View>FromComposite(t.(<type>))
			//   if err != nil {
			//     return nil, err
			//   }
			// }
			jen.For(jen.List(id("i"), id("t"))).Op(":=").Range().Id("v").Dot("Values").Block(
				jen.List(id("array").Index(id("i")), id("err")).Op("=").Id(viewInterfaceFromValue).Call(id("t").Assert(qual(valuesImportPath, "Composite"))),
				ifErrorBlock,
			),

			// return array, nil
			jen.Return(id("array"), jen.Nil()),
		)

		// Object type structure
		f.Add(variable().Id(typeVariableName).Op("=").SelfType(typ, typesToGenerate))

		// Generating constructors
		// TODO: support multiple constructors
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

			encodedConstructorFields[i] = valueConstructor(param.Type, id("p").Dot(param.Identifier).Statement)

			constructorFields[i] = id(label).Type(param.Type)

			constructorObjectParams[id(param.Identifier)] = id(label)
		}

		// Constructor interface
		constructorEncodeFunction := id("Encode").Params().Params(jen.Index().Byte(), jen.Error())
		f.Type().Id(constructorInterfaceName).Interface(
			constructorEncodeFunction,
		)

		// Constructor struct
		f.Type().Id(constructorStructName).Struct(constructorStructFields...)

		f.Func().Params(id("p").Id(constructorStructName)).Id("toValue").Params().Qual(valuesImportPath, "ConstantSizedArray").Block(
			jen.Return(
				qual(valuesImportPath, "NewConstantSizedArray").Call(
					jen.Index().Qual(valuesImportPath, "Value").Values(encodedConstructorFields...),
				),
			),
		)

		// Constructor encoding function
		f.Func().Params(id("p").Id(constructorStructName)).Add(constructorEncodeFunction).Block(
			// var w bytes.Buffer
			variable().Id("w").Qual("bytes", "Buffer"),

			// encoder := encoding.NewEncoder(&w)
			id("encoder").Op(":=").Qual(valuesEncodingImportPath, "NewEncoder").Call(op("&").Id("w")),

			// err := encoder.EncodeConstantSizedArray(
			id("err").Op(":=").Id("encoder").Dot("EncodeConstantSizedArray").Call(
				// p.toValue()
				id("p").Dot("toValue").Call(),
			),

			// if err != nil {
			//   return nil, err
			// }
			ifErrorBlock,

			// return w.Bytes(), nil
			jen.Return(id("w").Dot("Bytes").Call(), jen.Nil()),
		)

		// Constructor creator
		f.Func().Id(newConstructorName).Params(constructorFields...).Params(id(constructorInterfaceName), jen.Error()).Block(
			jen.Return(id(constructorStructName).Values(constructorObjectParams), jen.Nil()),
		)
	}

	convertersNames := make([]string, 0, len(converterFunctions))
	for key, _ := range converterFunctions {
		convertersNames = append(convertersNames, key)
	}
	values.SortInEncodingOrder(convertersNames)

	for _, name := range convertersNames {
		f.Add(converterFunctions[name])
	}

	return f.Render(writer)
}
