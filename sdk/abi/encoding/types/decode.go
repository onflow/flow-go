package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/dapperlabs/flow-go/sdk/abi/types"
)

type DefinitionEntry struct {
	types.Type
}

//func unmarshalStruct(data json.RawMessage) types.Struct {
//
//}

//func (de *DefinitionEntry) decodeDefinition(data []byte) error {
//	var decoded map[string]json.RawMessage
//	err := json.Unmarshal(data, &decoded)
//	if err != nil {
//		return err
//	}
//
//	key, value, err := GetOnlyEntry(decoded)
//	if err != nil {
//		return err
//	}
//	switch key {
//	case "struct":
//		var typeCheck interface{}
//		err := json.Unmarshal(value, &typeCheck)
//		if err != nil {
//			return err
//		}
//
//		switch v := typeCheck.(type) {
//		case string:
//			*de = DefinitionEntry{Type: &types.StructPointer{
//				TypeName: v,
//			}}
//			return nil
//		case map[string]interface{}:
//			var d types.Struct
//			err := json.Unmarshal(value, &d)
//			if err != nil {
//				return err
//			}
//			*de = DefinitionEntry{&d}
//
//		}
//	}
//
//	//GetOnlyEntry(de)
//
//	return nil
//}

//func (de DefinitionEntry) ToType() (types.Type, error) {
//
//}

//func (t types.isAType) decodeDefinition(data []byte) error {
//
//}

//func (f field) toField() (types.Field, error) {
//	toType, err := ToType(f.Type)
//	if err != nil {
//		return types.Field{}, err
//	}
//	return types.Field{
//		Identifier: f.Name,
//		Type:       toType,
//	}, nil
//}

type structRaw struct {
	Fields map[string]field
}

//func (de DefinitionEntry) Decode() (types.Type, error) {
//	for typ, data := range de {
//		switch typ {
//		case Struct:
//		}
//	}
//}

func decodeByStringType(t string, data interface{}) types.Type {
	switch t {
	case "struct":

	}

	return nil
}

const (
	Struct   StructType = "struct"
	Variable StructType = "variable"
	Function StructType = "function"
	Resource StructType = "resource"
	Event    StructType = "event"
)

type StructType string

type TypedStruct map[string]interface{}

func GetOnlyEntry(m map[string]interface{}) (string, interface{}, error) {
	if len(m) > 1 {
		return "", nil, errors.New(fmt.Sprintf("more then one entry in %v", m))

	}
	for k, v := range m {
		return k, v, nil
	}
	return "", nil, errors.New(fmt.Sprintf("no entires, but one required in %v", m))
}

//func decodeComposite(m map[string]interface{}) (types.Composite, error) {
//	composite := types.Composite{}
//	if fields, ok := m["Fields"]; ok {
//
//	}else {
//		return composite, errors.New(fmt.Sprintf("required field Fields doesn't exist in: %v", m))
//	}
//}

//func ToType(t interface{}) (types.Type, error) {
//	switch v := t.(type) {
//	case map[string]interface{}:
//		key, value, err := GetOnlyEntry(v)
//		if err != nil {
//			return nil, err
//		}
//		switch key {
//		case "struct":
//			if v, ok := value.(map[string]interface{}); ok {
//
//
//				return &types.Struct{
//					Composite: types.Composite{
//						Fields:       v["Fields"],
//						Identifier:   v["Identifier"],
//						Initializers: v["Initializers"],
//					},
//				}, nil
//			} else if v, ok := value.(string); ok {
//				return &types.StructPointer{
//					TypeName: v,
//				}, nil
//			} else {
//				return nil, errors.New(fmt.Sprintf("unknown data format under struct key: %v", v))
//			}
//		}
//	case string:
//
//	}
//}

//func DecodeType(name string, val interface{}) (types.Type, error) {
//	switch name {
//	case ""
//	}
//}

//func DecodeStruct(b json.RawMessage) (types.Struct, error) {
//	var s StructData
//	err := json.Unmarshal(b, &s)
//
//	if err != nil {
//		return types.Struct{}, err
//	}
//
//	for name, field := range s.Fields {
//		s.Fields[name] = DecodeField(field)
//	}
//}

//func toStruct(m map[string]interface{}) {
//
//
//
//	return types.Struct{
//		Composite: types.Composite{
//			IsAType:      types.IsAType{},
//			Fields:       nil,
//			Identifier:   "",
//			Initializers: nil,
//		},
//	}
//}

func getString(m map[string]interface{}, key string) (string, error) {

	value, err := getObject(m, key)
	if err != nil {
		return "", nil
	}

	if s, ok := value.(string); ok {
		return s, nil
	}

	return "", errors.New(fmt.Sprintf("Value for key  %s it is not a string in %v", key, m))
}

func getArray(m map[string]interface{}, key string) ([]interface{}, error) {
	value, err := getObject(m, key)
	if err != nil {
		return nil, nil
	}
	if s, ok := value.([]interface{}); ok {
		return s, nil
	}

	return nil, errors.New(fmt.Sprintf("Value for key  %s it is not an array in %v", key, m))
}

func getMap(m map[string]interface{}, key string) (map[string]interface{}, error) {
	value, err := getObject(m, key)
	if err != nil {
		return nil, nil
	}
	if s, ok := value.(map[string]interface{}); ok {
		return s, nil
	}

	return nil, errors.New(fmt.Sprintf("Value for key  %s it is not a map in %v", key, m))

}

func getIndex(a []interface{}, index int) (interface{}, error) {
	if len(a) <= index || index < 0 {
		return nil, errors.New(fmt.Sprintf("Index %d doesn't exist in array in %v", index, a))

	}
	return a[index], nil
}

func getObject(data map[string]interface{}, key string) (interface{}, error) {
	v, ok := data[key]

	if ok {
		return v, nil
	}

	return nil, errors.New(fmt.Sprintf("Key %s doesn't exist  in %v", key, data))

}

func toField(data interface{}, name string) (*types.Field, error) {

	typ, err := toType(data, name)
	if err != nil {
		return nil, err
	}
	return &types.Field{
		Identifier: name,
		Type:       typ,
	}, nil
}

func toParameter(data map[string]interface{}) (*types.Parameter, error) {

	name, err := getString(data, "name")
	if err != nil {
		return nil, err
	}
	label, err := getString(data, "label")
	if err != nil {
		label = ""
	}

	typRaw, err := getObject(data, "type")
	if err != nil {
		return nil, err
	}

	typ, err := toType(typRaw, name)
	if err != nil {
		return nil, err
	}

	return &types.Parameter{
		Field: types.Field{
			Identifier: name,
			Type:       typ,
		},
		Label: label,
	}, nil
}

func interfaceToListOfMaps(input interface{}) ([]map[string]interface{}, error) {
	array, ok := input.([]interface{})
	if !ok {
		return nil, errors.New(fmt.Sprintf("%v is not of expected type []interface{}", input))
	}

	ret := make([]map[string]interface{}, len(array))
	for i, a := range array {
		ret[i], ok = a.(map[string]interface{})
		if !ok {
			return nil, errors.New(fmt.Sprintf("%v is not of expected type map[string]interface{}", a))
		}
	}
	return ret, nil
}

func toStruct(data map[string]interface{}, name string) (*types.Struct, error) {

	fieldsRaw, err := getMap(data, "fields")
	if err != nil {
		return nil, err
	}

	fields := map[string]*types.Field{}

	for name, field := range fieldsRaw {
		fields[name], err = toField(field, name)
		if err != nil {
			return nil, err
		}
	}

	initializersRaw, err := getArray(data, "initializers")
	if err != nil {
		return nil, err
	}

	initializerRaw, err := getIndex(initializersRaw, 0)
	if err != nil {
		return nil, err
	}

	initializers, err := interfaceToListOfMaps(initializerRaw)

	if err != nil {
		return nil, err
	}

	parameters := make([]*types.Parameter, len(initializers))

	for i, raw := range initializers {
		parameters[i], err = toParameter(raw)
		if err != nil {
			return nil, err
		}
	}

	return &types.Struct{
		Composite: types.Composite{
			Fields:     fields,
			Identifier: name,
			Initializers: [][]*types.Parameter{
				parameters,
			},
		},
	}, nil
}

func toType(data interface{}, name string) (types.Type, error) {

	switch v := data.(type) {

	//Simple string cases
	case string:
		var newType types.Type
		switch v {
		case "Void":
			newType = &types.Void{}
		case "Int":
			newType = &types.Int{}
		case "String":
			newType = &types.String{}
		default:
			return nil, errors.New(fmt.Sprintf("unsupported string type key %s in %v", v, data))
		}

		return newType, nil

	//If object with key as type descriptor
	case map[string]interface{}:

		key, value, err := GetOnlyEntry(v)
		if err != nil {
			return nil, err
		}

		//when type of declaration doesn't matter as we can handle both
		switch key {
		case "variable":
			typ, err := toType(value, name)
			if err != nil {
				return nil, err
			}
			return &types.Variable{
				Type: typ,
			}, nil
		}

		//when case require more handling
		switch v := value.(type) {
		// when type inside is simple string
		case string:
			switch key {
			case "struct":
				return &types.StructPointer{TypeName: v}, nil
			}

		//when type inside is complex
		case map[string]interface{}:
			switch key {
			case "struct":
				return toStruct(v, name)
			}

		}
	}

	return nil, errors.New(fmt.Sprintf("unsupported data chunk %v", data))
}

func decodeDefinition(data map[string]interface{}, name string) (types.Type, error) {

	key, value, err := GetOnlyEntry(data)
	if err != nil {
		return nil, err
	}

	switch v := value.(type) {
	case map[string]interface{}:
		switch key {
		case "struct":
			return toStruct(v, name)

		case "variable":
			typ, err := toType(value, name)
			if err != nil {
				return nil, err
			}
			return &types.Variable{
				Type: typ,
			}, nil
		default:
			return nil, errors.New(fmt.Sprintf("unsupported definition type %v named %s", value, key))

		}

	default:
		return nil, errors.New(fmt.Sprintf("unsupported definition type %v", data))
	}

}

type JsonContainer struct {
	Definitions map[string]map[string]interface{}
}

func Decode() (map[string]types.Type, error) {

	bytes, err := ioutil.ReadFile("sdk/abi/generation/example/person.cdc.abi.json")

	if err != nil {
		panic(err)
	}

	jsonRoot := JsonContainer{}

	err = json.Unmarshal(bytes, &jsonRoot)

	if err != nil {
		panic(err)
	}

	definitions := map[string]types.Type{}

	for name, definition := range jsonRoot.Definitions {
		typ, err := toType(definition, name)
		if err != nil {
			panic(err)
		}
		definitions[name] = typ
	}
	//
	//	//toType, err := ToType(definition)
	//	//if err != nil {
	//	//	return nil, err
	//	//}
	//	//
	//	//definitions[name] = toType
	//}

	fmt.Printf("%v", definitions)
	return definitions, nil
}
