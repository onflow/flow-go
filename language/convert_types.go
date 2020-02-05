package language

import (
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
)

// ConvertType converts a runtime type to its corresponding Go representation.
func ConvertType(typ runtime.Type, prog *ast.Program, variable *sema.Variable) (Type, error) {
	switch t := typ.(type) {
	case *sema.AnyStructType:
		return wrapVariable(AnyStructType{}, variable), nil
	case *sema.VoidType:
		return wrapVariable(VoidType{}, variable), nil
	case *sema.OptionalType:
		return convertOptionalType(t, prog, variable)
	case *sema.BoolType:
		return wrapVariable(BoolType{}, variable), nil
	case *sema.StringType:
		return wrapVariable(StringType{}, variable), nil
	case *sema.IntType:
		return wrapVariable(IntType{}, variable), nil
	case *sema.Int8Type:
		return wrapVariable(Int8Type{}, variable), nil
	case *sema.Int16Type:
		return wrapVariable(Int16Type{}, variable), nil
	case *sema.Int32Type:
		return wrapVariable(Int32Type{}, variable), nil
	case *sema.Int64Type:
		return wrapVariable(Int64Type{}, variable), nil
	case *sema.Int128Type:
		return wrapVariable(Int128Type{}, variable), nil
	case *sema.Int256Type:
		return wrapVariable(Int256Type{}, variable), nil
	case *sema.UIntType:
		return wrapVariable(UIntType{}, variable), nil
	case *sema.UInt8Type:
		return wrapVariable(UInt8Type{}, variable), nil
	case *sema.UInt16Type:
		return wrapVariable(UInt16Type{}, variable), nil
	case *sema.UInt32Type:
		return wrapVariable(UInt32Type{}, variable), nil
	case *sema.UInt64Type:
		return wrapVariable(UInt64Type{}, variable), nil
	case *sema.UInt128Type:
		return wrapVariable(UInt128Type{}, variable), nil
	case *sema.UInt256Type:
		return wrapVariable(UInt256Type{}, variable), nil
	case *sema.Word8Type:
		return wrapVariable(Word8Type{}, variable), nil
	case *sema.Word16Type:
		return wrapVariable(Word16Type{}, variable), nil
	case *sema.Word32Type:
		return wrapVariable(Word32Type{}, variable), nil
	case *sema.Word64Type:
		return wrapVariable(Word64Type{}, variable), nil
	case *sema.VariableSizedType:
		return convertVariableSizedType(t, prog, variable)
	case *sema.ConstantSizedType:
		return convertConstantSizedType(t, prog, variable)
	case *sema.FunctionType:
		return convertFunctionType(t, prog, variable)
	case *sema.CompositeType:
		return convertCompositeType(t, prog, variable)
	case *sema.DictionaryType:
		return convertDictionaryType(t, prog, variable)
	}

	return nil, fmt.Errorf("cannot convert type of type %T", typ)
}

func wrapVariable(t Type, variable *sema.Variable) Type {
	if variable != nil {
		return Variable{Type: t}
	}

	return t
}

func convertOptionalType(t *sema.OptionalType, prog *ast.Program, variable *sema.Variable) (Type, error) {
	convertedType, err := ConvertType(t.Type, prog, nil)
	if err != nil {
		return nil, err
	}

	return wrapVariable(OptionalType{Type: convertedType}, variable), nil
}

func convertVariableSizedType(t *sema.VariableSizedType, prog *ast.Program, variable *sema.Variable) (Type, error) {
	convertedElement, err := ConvertType(t.Type, prog, nil)
	if err != nil {
		return nil, err
	}

	return wrapVariable(VariableSizedArrayType{ElementType: convertedElement}, variable), nil
}

func convertConstantSizedType(t *sema.ConstantSizedType, prog *ast.Program, variable *sema.Variable) (Type, error) {
	convertedElement, err := ConvertType(t.Type, prog, nil)
	if err != nil {
		return nil, err
	}

	return wrapVariable(
		ConstantSizedArrayType{
			Size:        uint(t.Size),
			ElementType: convertedElement,
		},
		variable,
	), nil
}

func convertFunctionType(t *sema.FunctionType, prog *ast.Program, variable *sema.Variable) (Type, error) {
	convertedReturnType, err := ConvertType(t.ReturnTypeAnnotation.Type, prog, nil)
	if err != nil {
		return nil, err
	}

	// TODO: return
	// we have function type rather than named functions with params
	if variable == nil {
		parameterTypes := make([]Type, len(t.Parameters))

		for i, parameter := range t.Parameters {
			convertedParameterType, err := ConvertType(parameter.TypeAnnotation.Type, prog, nil)
			if err != nil {
				return nil, err
			}

			parameterTypes[i] = convertedParameterType
		}

		return FunctionType{
			ParameterTypes: parameterTypes,
			ReturnType:     convertedReturnType,
		}, nil

	}

	functionDeclaration := func() *ast.FunctionDeclaration {
		for _, fn := range prog.FunctionDeclarations() {
			if fn.Identifier.Identifier == variable.Identifier && fn.Identifier.Pos == *variable.Pos {
				return fn
			}
		}

		panic(fmt.Sprintf("cannot find type %v declaration in AST tree", t))
	}()

	parameters := make([]Parameter, len(t.Parameters))

	for i, parameter := range t.Parameters {
		astParam := functionDeclaration.ParameterList.Parameters[i]

		convertedParameterType, err := ConvertType(parameter.TypeAnnotation.Type, prog, nil)
		if err != nil {
			return nil, err
		}

		parameters[i] = Parameter{
			Label:      astParam.Label,
			Identifier: astParam.Identifier.Identifier,
			Type:       convertedParameterType,
		}
	}

	return Function{
		Parameters: parameters,
		ReturnType: convertedReturnType,
	}.WithID(string(t.ID())), nil
}

func convertCompositeType(t *sema.CompositeType, prog *ast.Program, variable *sema.Variable) (Type, error) {
	// this type is exported as a field or parameter type, not main definition
	if variable == nil {
		switch t.Kind {
		case common.CompositeKindStructure:
			return StructPointer{TypeName: t.Identifier}, nil
		case common.CompositeKindResource:
			return ResourcePointer{TypeName: t.Identifier}, nil
		case common.CompositeKindEvent:
			return EventPointer{TypeName: t.Identifier}, nil
		}

		panic(fmt.Sprintf("cannot convert type %v of unknown kind %v", t, t.Kind))
	}

	convert := func() (CompositeType, error) {

		compositeDeclaration := func() *ast.CompositeDeclaration {
			for _, cd := range prog.CompositeDeclarations() {
				if cd.Identifier.Identifier == variable.Identifier &&
					cd.Identifier.Pos == *variable.Pos {
					return cd
				}
			}
			panic(fmt.Sprintf("cannot find type %v declaration in AST tree", t))
		}()

		fields := make([]Field, 0, len(t.Members))

		// TODO: do not sort fields before export, store in order declared
		fieldNames := make([]string, 0, len(t.Members))
		for identifer := range t.Members {
			fieldNames = append(fieldNames, identifer)
		}

		// sort field names in lexicographical order
		sort.Strings(fieldNames)

		for _, identifer := range fieldNames {
			field := t.Members[identifer]

			convertedFieldType, err := ConvertType(field.TypeAnnotation.Type, prog, nil)
			if err != nil {
				return CompositeType{}, err
			}

			fields = append(fields, Field{
				Identifier: identifer,
				Type:       convertedFieldType,
			})
		}

		parameters := make([]Parameter, len(t.ConstructorParameters))

		// TODO: For now we have only one initializer, so we just assume this here
		// as this is post SEMA we really hope AST list of params matches SEMA type one
		for i, parameter := range compositeDeclaration.Members.Initializers()[0].ParameterList.Parameters {
			semaType := t.ConstructorParameters[i].TypeAnnotation.Type
			convertedType, err := ConvertType(semaType, prog, nil)
			if err != nil {
				return CompositeType{}, err
			}

			parameters[i] = Parameter{
				Label:      parameter.Label,
				Identifier: parameter.Identifier.Identifier,
				Type:       convertedType,
			}
		}

		return CompositeType{
			Identifier:   t.Identifier,
			Fields:       fields,
			Initializers: [][]Parameter{parameters},
		}.WithID(string(t.ID())), nil
	}

	composite, err := convert()
	if err != nil {
		return nil, err
	}

	switch t.Kind {
	case common.CompositeKindStructure:
		return StructType{
			CompositeType: composite,
		}, nil

	case common.CompositeKindResource:
		return ResourceType{
			CompositeType: composite,
		}, nil

	case common.CompositeKindEvent:
		return EventType{
			CompositeType: composite,
		}, nil
	}

	panic(fmt.Sprintf("cannot convert type %v of unknown kind %v", t, t.Kind))
}

func convertDictionaryType(t *sema.DictionaryType, prog *ast.Program, variable *sema.Variable) (Type, error) {
	convertedKeyType, err := ConvertType(t.KeyType, prog, nil)
	if err != nil {
		return nil, err
	}

	convertedElementType, err := ConvertType(t.ValueType, prog, nil)
	if err != nil {
		return nil, err
	}

	return wrapVariable(DictionaryType{
		KeyType:     convertedKeyType,
		ElementType: convertedElementType,
	}, variable), nil
}
