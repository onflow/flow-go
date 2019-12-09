package encoding

import (
	"fmt"

	"github.com/dapperlabs/flow-go/sdk/abi/types"
)

func NewEncoder() Encoder {
	return Encoder{definitions: map[string]interface{}{}}
}

type Encoder struct {
	definitions map[string]interface{}
}

func (encoder *Encoder) Encode(name string, t types.Type) {
	encoder.definitions[name] = encoder.encode(t)
}

func (encoder *Encoder) Get() interface{} {
	return abiObject{
		encoder.definitions,
		"", //Once we setup schema, probably on withflow.org
	}
}

//region JSON Structures

type abiObject struct {
	Definitions map[string]interface{} `json:"definitions"`
	Schema      string                 `json:"schema,omitempty"`
}

type arrayObject struct {
	Array array `json:"array"`
}
type array struct {
	Of   interface{} `json:"of"`
	Size uint        `json:"size,omitempty"`
}

type structObject struct {
	Struct structData `json:"struct"`
}
type structData struct {
	Fields       map[string]interface{} `json:"fields"`
	Initializers [][]parameter          `json:"initializers"`
}
type field struct {
	Name string      `json:"name"`
	Type interface{} `json:"type"`
}
type parameter struct {
	field
	Label string `json:"label,omitempty"`
}

type eventObject struct {
	Event []parameter `json:"event"`
}

type optionalObject struct {
	Optional interface{} `json:"optional"`
}

type function struct {
	Parameters []parameter `json:"parameters,omitempty"`
	ReturnType interface{} `json:"returnType,omitempty"`
}

type functionType struct {
	Parameters []interface{} `json:"parameters,omitempty"`
	ReturnType interface{}   `json:"returnType,omitempty"`
}

type functionObject struct {
	Function interface{} `json:"function"`
}

type dictionary struct {
	Keys   interface{} `json:"keys"`
	Values interface{} `json:"values"`
}

type dictionaryObject struct {
	Dictionary dictionary `json:"dictionary"`
}

type resourceObject struct {
	Resource structData `json:"resource"`
}

type resourcePointer struct {
	Resource string `json:"resource"`
}

type structPointer struct {
	Struct string `json:"struct"`
}

type variableObject struct {
	Variable interface{} `json:"variable"`
}

//endregion

func (encoder *Encoder) mapFields(m map[string]*types.Field) map[string]interface{} {
	ret := map[string]interface{}{}

	for k, v := range m {
		ret[k] = encoder.encode(v.Type)
	}

	return ret
}

func (encoder *Encoder) mapParameters(p []*types.Parameter) []parameter {
	ret := make([]parameter, len(p))

	for i := range p {
		ret[i] = parameter{
			field: field{
				Name: p[i].Identifier,
				Type: encoder.encode(p[i].Type),
			},
			Label: p[i].Label,
		}
	}

	return ret
}

func (encoder *Encoder) mapNestedParameters(p [][]*types.Parameter) [][]parameter {

	ret := make([][]parameter, len(p))
	for i := range ret {
		ret[i] = encoder.mapParameters(p[i])
	}

	return ret
}

func (encoder *Encoder) mapTypes(types []types.Type) []interface{} {
	ret := make([]interface{}, len(types))

	for i, t := range types {
		ret[i] = encoder.encode(t)
	}

	return ret
}

// For function return type Void is redundant, so we remove it
func (encoder *Encoder) encodeReturnType(returnType types.Type) interface{} {
	if _, ok := returnType.(types.Void); ok == true {
		return nil
	}
	return encoder.encode(returnType)
}

func (encoder *Encoder) encode(t types.Type) interface{} {
	switch v := (t).(type) {
	case types.AnyStruct:
		return "AnyStruct"
	case types.Bool:
		return "Bool"
	case types.Void:
		return "Void"
	case types.String:
		return "String"

	case types.Int:
		return "Int"
	case types.Int8:
		return "Int8"
	case types.Int16:
		return "Int16"
	case types.Int32:
		return "Int32"
	case types.Int64:
		return "Int64"
	case types.UInt8:
		return "UInt8"
	case types.UInt16:
		return "UInt16"
	case types.UInt32:
		return "UInt16"
	case types.UInt64:
		return "UInt16"

	case types.VariableSizedArray:
		return arrayObject{array{Of: encoder.encode(v.ElementType)}}
	case types.ConstantSizedArray:
		return arrayObject{array{Of: encoder.encode(v.ElementType), Size: v.Size}}

	case types.Optional:
		return optionalObject{Optional: encoder.encode(v.Of)}

	case types.Struct:
		return structObject{
			Struct: structData{
				Fields:       encoder.mapFields(v.Fields),
				Initializers: encoder.mapNestedParameters(v.Initializers),
			},
		}
	case types.StructPointer:
		return structPointer{
			v.TypeName,
		}
	case types.ResourcePointer:
		return resourcePointer{
			v.TypeName,
		}
	case types.Event:
		return eventObject{
			Event: encoder.mapParameters(v.Fields),
		}
	case types.Function:
		return functionObject{
			function{
				Parameters: encoder.mapParameters(v.Parameters),
				ReturnType: encoder.encodeReturnType(v.ReturnType),
			},
		}
	case types.FunctionType:
		return functionObject{
			functionType{
				Parameters: encoder.mapTypes(v.ParameterTypes),
				ReturnType: encoder.encodeReturnType(v.ReturnType),
			},
		}

	case types.Dictionary:
		return dictionaryObject{
			dictionary{
				Keys:   encoder.encode(v.KeyType),
				Values: encoder.encode(v.ElementType),
			},
		}
	case types.Resource:
		return resourceObject{
			structData{
				Fields:       encoder.mapFields(v.Fields),
				Initializers: encoder.mapNestedParameters(v.Initializers),
			},
		}
	case types.Variable:
		return variableObject{
			encoder.encode(v.Type),
		}

	}

	panic(fmt.Errorf("unknown type of %T", t))
}
