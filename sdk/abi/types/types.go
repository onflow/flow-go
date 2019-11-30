package types

import (
	"encoding/json"
	"errors"
	"fmt"
)

// revive:disable:redefines-builtin-id

type Type interface {
	isType()
	//UnmarshalJSON(data []byte) error
}

// revive:enable

func GetOnlyEntry(m map[string]json.RawMessage) (string, json.RawMessage, error) {
	if len(m) > 1 {
		return "", nil, errors.New(fmt.Sprintf("more then one entry in %v", m))

	}
	for k, v := range m {
		return k, v, nil
	}
	return "", nil, errors.New(fmt.Sprintf("no entires, but one required in %v", m))
}

type IsAType struct {
	Type
}

func (IsAType) isType() {}

//
//func (t *IsAType) UnmarshalJSON(data []byte) error {
//	var typeCheck interface{}
//	err := json.Unmarshal(data, &typeCheck)
//	if err != nil {
//		return err
//	}
//
//	switch v := typeCheck.(type) {
//	case string:
//		var newType Type
//		switch v {
//		case "Void": newType = &Void{}
//		case "Int": newType = &Int{}
//		case "String": newType = &String{}
//		}
//
//		err := json.Unmarshal(data, &t)
//		if err != nil {
//			return err
//		}
//		*t = IsAType{newType}
//
//	case map[string]interface{}:
//
//		var d map[string]json.RawMessage
//
//		err := json.Unmarshal(data, &d)
//		if err != nil {
//			return err
//		}
//
//		key, value, err := GetOnlyEntry(d)
//		if err != nil {
//			return err
//		}
//		var newType Type
//
//		var typeCheck interface{}
//
//		err = json.Unmarshal(value, &typeCheck)
//		if err != nil {
//			return err
//		}
//
//		if s, ok := typeCheck.(string); ok {
//			switch key {
//			case "struct": newType = &StructPointer{TypeName:s}
//			}
//
//		} else {
//			switch key {
//			case "struct":
//				newType = &Struct{}
//			}
//			err = json.Unmarshal(value, &newType)
//			if err != nil {
//				return err
//			}
//		}
//
//		*t = IsAType{newType}
//	}
//
//	return nil
//}

type Annotation struct {
	IsMove bool
	Type   Type
}

type Void struct{ IsAType }

type Bool struct{ IsAType }

type String struct{ IsAType }

type Bytes struct{ IsAType }

type Any struct{ IsAType }

type Int struct{ IsAType }

type Int8 struct{ IsAType }

type Int16 struct{ IsAType }

type Int32 struct{ IsAType }

type Int64 struct{ IsAType }

type UInt8 struct{ IsAType }

type UInt16 struct{ IsAType }

type UInt32 struct{ IsAType }

type UInt64 struct{ IsAType }

type Variable struct {
	IsAType
	Type Type
}

type VariableSizedArray struct {
	IsAType
	ElementType Type
}

type ConstantSizedArray struct {
	IsAType
	Size        uint
	ElementType Type
}

type Parameter struct {
	Field
	Label string
}

type Composite struct {
	IsAType
	Fields       map[string]*Field
	Identifier   string
	Initializers [][]*Parameter
}

//func (c *Composite) UnmarshalJSON(data []byte) error {
//
//}

type Struct struct {
	IsAType
	Composite
}

//func (c *Struct) UnmarshalJSON(data []byte) error {
//	return json.Unmarshal(data, &c)
//}

type Resource struct {
	IsAType
	Composite
}

type Dictionary struct {
	IsAType
	KeyType     Type
	ElementType Type
}

type Function struct {
	IsAType
	Parameters []*Parameter
	ReturnType Type
}

// A type representing anonymous function (aka without named arguments)
type FunctionType struct {
	IsAType
	ParameterTypes []Type
	ReturnType     Type
}

type Event struct {
	IsAType
	Fields     []*Parameter
	Identifier string
}

type Field struct {
	IsAType
	Identifier string
	Type       Type
}

//func (c *Field) UnmarshalJSON(data []byte) error {
//	type fieldRaw struct {
//		Identifier string
//		Type IsAType
//	}
//	var raw fieldRaw
//	err := json.Unmarshal(data, &raw)
//	if err != nil {
//		return err
//	}
//
//	*c = Field{
//		IsAType:    IsAType{},
//		Identifier: raw.Identifier,
//		Type:       raw.Type.Type,
//	}
//	return nil
//}

type Optional struct {
	IsAType
	Of Type
}

//Pointers are simply pointers to already existing types, to prevent circular references
type ResourcePointer struct {
	IsAType
	TypeName string
}

type StructPointer struct {
	IsAType
	TypeName string
}

type Address struct{ IsAType }
