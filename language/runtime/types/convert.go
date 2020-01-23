package types

import (
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
)

// Convert converts a runtime type to its corresponding Go representation.
func Convert(typ runtime.Type, prog *ast.Program, variable *sema.Variable) (Type, error) {
	switch t := typ.(type) {
	case *sema.AnyStructType:
		return wrapVariable(AnyStruct{}, variable), nil
	case *sema.VoidType:
		return wrapVariable(Void{}, variable), nil
	case *sema.OptionalType:
		return convertOptionalType(t, prog, variable)
	case *sema.BoolType:
		return wrapVariable(Bool{}, variable), nil
	case *sema.StringType:
		return wrapVariable(String{}, variable), nil
	case *sema.IntType:
		return wrapVariable(Int{}, variable), nil
	case *sema.Int8Type:
		return wrapVariable(Int8{}, variable), nil
	case *sema.Int16Type:
		return wrapVariable(Int16{}, variable), nil
	case *sema.Int32Type:
		return wrapVariable(Int32{}, variable), nil
	case *sema.Int64Type:
		return wrapVariable(Int64{}, variable), nil
	case *sema.UInt8Type:
		return wrapVariable(UInt8{}, variable), nil
	case *sema.UInt16Type:
		return wrapVariable(UInt16{}, variable), nil
	case *sema.UInt32Type:
		return wrapVariable(UInt32{}, variable), nil
	case *sema.UInt64Type:
		return wrapVariable(UInt64{}, variable), nil
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
	} else {
		return t
	}
}

func convertOptionalType(t *sema.OptionalType, prog *ast.Program, variable *sema.Variable) (Type, error) {
	convertedType, err := Convert(t.Type, prog, nil)
	if err != nil {
		return nil, err
	}

	return wrapVariable(Optional{Type: convertedType}, variable), nil
}

func convertVariableSizedType(t *sema.VariableSizedType, prog *ast.Program, variable *sema.Variable) (Type, error) {
	convertedElement, err := Convert(t.Type, prog, nil)
	if err != nil {
		return nil, err
	}

	return wrapVariable(VariableSizedArray{ElementType: convertedElement}, variable), nil
}

func convertConstantSizedType(t *sema.ConstantSizedType, prog *ast.Program, variable *sema.Variable) (Type, error) {
	convertedElement, err := Convert(t.Type, prog, nil)
	if err != nil {
		return nil, err
	}

	return wrapVariable(
		ConstantSizedArray{
			Size:        uint(t.Size),
			ElementType: convertedElement,
		},
		variable,
	), nil
}

func convertFunctionType(t *sema.FunctionType, prog *ast.Program, variable *sema.Variable) (Type, error) {
	convertedReturnType, err := Convert(t.ReturnTypeAnnotation.Type, prog, nil)
	if err != nil {
		return nil, err
	}

	// TODO: return
	// we have function type rather than named functions with params
	if variable == nil {
		parameterTypes := make([]Type, len(t.Parameters))

		for i, parameter := range t.Parameters {
			convertedParameterType, err := Convert(parameter.TypeAnnotation.Type, prog, nil)
			if err != nil {
				return nil, err
			}

			parameterTypes[i] = convertedParameterType
		}

		return FunctionType{
			ParameterTypes: parameterTypes,
			ReturnType:     convertedReturnType,
		}, nil

	} else {
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

			convertedParameterType, err := Convert(parameter.TypeAnnotation.Type, prog, nil)
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

	convert := func() (Composite, error) {

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
		for identifer, _ := range t.Members {
			fieldNames = append(fieldNames, identifer)
		}

		// sort field names in lexicographical order
		sort.Strings(fieldNames)

		for _, identifer := range fieldNames {
			field := t.Members[identifer]

			convertedFieldType, err := Convert(field.TypeAnnotation.Type, prog, nil)
			if err != nil {
				return Composite{}, err
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
			convertedType, err := Convert(semaType, prog, nil)
			if err != nil {
				return Composite{}, err
			}

			parameters[i] = Parameter{
				Label:      parameter.Label,
				Identifier: parameter.Identifier.Identifier,
				Type:       convertedType,
			}
		}

		return Composite{
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
		return Struct{
			Composite: composite,
		}, nil

	case common.CompositeKindResource:
		return Resource{
			Composite: composite,
		}, nil

	case common.CompositeKindEvent:
		return Event{
			Composite: composite,
		}, nil
	}

	panic(fmt.Sprintf("cannot convert type %v of unknown kind %v", t, t.Kind))
}

func convertDictionaryType(t *sema.DictionaryType, prog *ast.Program, variable *sema.Variable) (Type, error) {
	convertedKeyType, err := Convert(t.KeyType, prog, nil)
	if err != nil {
		return nil, err
	}

	convertedElementType, err := Convert(t.ValueType, prog, nil)
	if err != nil {
		return nil, err
	}

	return wrapVariable(Dictionary{
		KeyType:     convertedKeyType,
		ElementType: convertedElementType,
	}, variable), nil
}
