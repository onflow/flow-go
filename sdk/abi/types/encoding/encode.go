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

//func (e *Encoder) EncodeVariable(v *types.Variable) {
//
//}

func (encoder *Encoder) Encode(name string, t types.Type) {
	encoder.definitions[name] = encoder.encode(t)
}

func (encoder *Encoder) Get() interface{} {
	return encoder.definitions
}

//region JSON Structures
type arrayObject struct {
	Array array `json:"array"`
}
type array struct {
	Of   interface{} `json:"of"`
	Size uint        `json:"size,omitempty"`
}

type structObject struct {
	Struct_ struct_ `json:"struct"`
}
type struct_ struct {
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

type functionBase struct{}

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

func (encoder *Encoder) encode(t types.Type) interface{} {
	switch v := (t).(type) {
	case types.Bool:
		return "Boolean"
	case types.Int:
		return "Int"
	case types.String:
		return "String"
	case types.VariableSizedArray:
		return arrayObject{array{Of: encoder.encode(v.ElementType)}}
	case types.ConstantSizedArray:
		return arrayObject{array{Of: encoder.encode(v.ElementType), Size: v.Size}}
	case types.Optional:
		return optionalObject{Optional: encoder.encode(v.Of)}
	case types.Struct:
		return structObject{
			Struct_: struct_{
				Fields:       encoder.mapFields(v.Fields),
				Initializers: encoder.mapNestedParameters(v.Initializers),
			},
		}
	case types.Pointer:
		return v.TypeName
	case types.Event:
		return eventObject{
			Event: encoder.mapParameters(v.Fields),
		}
	case types.Function:
		return functionObject{
			function{
				Parameters: encoder.mapParameters(v.Parameters),
				ReturnType: encoder.encode(v.ReturnType),
			},
		}
	case types.FunctionType:
		return functionObject{
			functionType{
				Parameters: encoder.mapTypes(v.ParameterTypes),
				ReturnType: encoder.encode(v.ReturnType),
			},
		}
	case types.Any:
		return "Any"
	}

	panic(fmt.Errorf("unknown type of %T", t))
}
