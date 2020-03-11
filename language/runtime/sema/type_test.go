package sema

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
)

func TestConstantSizedType_String(t *testing.T) {

	ty := &ConstantSizedType{
		Type: &VariableSizedType{Type: &IntType{}},
		Size: 2,
	}

	assert.Equal(t, ty.String(), "[[Int]; 2]")
}

func TestConstantSizedType_String_OfFunctionType(t *testing.T) {

	ty := &ConstantSizedType{
		Type: &FunctionType{
			Parameters: []*Parameter{
				{
					TypeAnnotation: NewTypeAnnotation(&Int8Type{}),
				},
			},
			ReturnTypeAnnotation: NewTypeAnnotation(
				&Int16Type{},
			),
		},
		Size: 2,
	}

	assert.Equal(t, ty.String(), "[((Int8): Int16); 2]")
}

func TestVariableSizedType_String(t *testing.T) {

	ty := &VariableSizedType{
		Type: &ConstantSizedType{
			Type: &IntType{},
			Size: 2,
		},
	}

	assert.Equal(t, ty.String(), "[[Int; 2]]")
}

func TestVariableSizedType_String_OfFunctionType(t *testing.T) {

	ty := &VariableSizedType{
		Type: &FunctionType{
			Parameters: []*Parameter{
				{
					TypeAnnotation: NewTypeAnnotation(&Int8Type{}),
				},
			},
			ReturnTypeAnnotation: NewTypeAnnotation(
				&Int16Type{},
			),
		},
	}

	assert.Equal(t, ty.String(), "[((Int8): Int16)]")
}

func TestIsResourceType_AnyStructNestedInArray(t *testing.T) {

	ty := &VariableSizedType{
		Type: &AnyStructType{},
	}

	assert.False(t, ty.IsResourceType())
}

func TestIsResourceType_AnyResourceNestedInArray(t *testing.T) {

	ty := &VariableSizedType{
		Type: &AnyResourceType{},
	}

	assert.True(t, ty.IsResourceType())
}

func TestIsResourceType_ResourceNestedInArray(t *testing.T) {

	ty := &VariableSizedType{
		Type: &CompositeType{
			Kind: common.CompositeKindResource,
		},
	}

	assert.True(t, ty.IsResourceType())
}

func TestIsResourceType_ResourceNestedInDictionary(t *testing.T) {

	ty := &DictionaryType{
		KeyType: &StringType{},
		ValueType: &VariableSizedType{
			Type: &CompositeType{
				Kind: common.CompositeKindResource,
			},
		},
	}

	assert.True(t, ty.IsResourceType())
}

func TestIsResourceType_StructNestedInDictionary(t *testing.T) {

	ty := &DictionaryType{
		KeyType: &StringType{},
		ValueType: &VariableSizedType{
			Type: &CompositeType{
				Kind: common.CompositeKindStructure,
			},
		},
	}

	assert.False(t, ty.IsResourceType())
}

func TestRestrictedResourceType_StringAndID(t *testing.T) {

	t.Run("base type and restriction", func(t *testing.T) {
		interfaceType := &InterfaceType{
			CompositeKind: common.CompositeKindResource,
			Identifier:    "I",
			Location:      ast.StringLocation("b"),
		}

		ty := &RestrictedResourceType{
			Type: &CompositeType{
				Kind:       common.CompositeKindResource,
				Identifier: "R",
				Location:   ast.StringLocation("a"),
			},
			Restrictions: []*InterfaceType{interfaceType},
		}

		assert.Equal(t, ty.String(), "R{I}")
		assert.Equal(t, ty.ID(), TypeID("a.R{b.I}"))
	})

	t.Run("base type and restrictions", func(t *testing.T) {
		i1 := &InterfaceType{
			CompositeKind: common.CompositeKindResource,
			Identifier:    "I1",
			Location:      ast.StringLocation("b"),
		}

		i2 := &InterfaceType{
			CompositeKind: common.CompositeKindResource,
			Identifier:    "I2",
			Location:      ast.StringLocation("c"),
		}

		ty := &RestrictedResourceType{
			Type: &CompositeType{
				Kind:       common.CompositeKindResource,
				Identifier: "R",
				Location:   ast.StringLocation("a"),
			},
			Restrictions: []*InterfaceType{i1, i2},
		}

		assert.Equal(t, ty.String(), "R{I1, I2}")
		assert.Equal(t, ty.ID(), TypeID("a.R{b.I1,c.I2}"))
	})

	t.Run("no restrictions", func(t *testing.T) {
		ty := &RestrictedResourceType{
			Type: &CompositeType{
				Kind:       common.CompositeKindResource,
				Identifier: "R",
				Location:   ast.StringLocation("a"),
			},
		}

		assert.Equal(t, ty.String(), "R{}")
		assert.Equal(t, ty.ID(), TypeID("a.R{}"))
	})

	t.Run("no base type", func(t *testing.T) {

		interfaceType := &InterfaceType{
			CompositeKind: common.CompositeKindResource,
			Identifier:    "I",
			Location:      ast.StringLocation("b"),
		}

		ty := &RestrictedResourceType{
			Restrictions: []*InterfaceType{interfaceType},
		}

		assert.Equal(t, ty.String(), "{I}")
		assert.Equal(t, ty.ID(), TypeID("{b.I}"))
	})

	t.Run("no restrictions, no base type", func(t *testing.T) {
		ty := &RestrictedResourceType{}

		assert.Equal(t, ty.String(), "{}")
		assert.Equal(t, ty.ID(), TypeID("{}"))
	})
}

func TestRestrictedResourceType_Equals(t *testing.T) {

	t.Run("same base type and more restrictions", func(t *testing.T) {

		i1 := &InterfaceType{
			CompositeKind: common.CompositeKindResource,
			Identifier:    "I1",
			Location:      ast.StringLocation("b"),
		}

		i2 := &InterfaceType{
			CompositeKind: common.CompositeKindResource,
			Identifier:    "I2",
			Location:      ast.StringLocation("b"),
		}

		a := &RestrictedResourceType{
			Type: &CompositeType{
				Kind:       common.CompositeKindResource,
				Identifier: "R",
				Location:   ast.StringLocation("a"),
			},
			Restrictions: []*InterfaceType{i1},
		}

		b := &RestrictedResourceType{
			Type: &CompositeType{
				Kind:       common.CompositeKindResource,
				Identifier: "R",
				Location:   ast.StringLocation("a"),
			},
			Restrictions: []*InterfaceType{i1, i2},
		}

		assert.False(t, a.Equal(b))
	})

	t.Run("same base type and fewer restrictions", func(t *testing.T) {

		i1 := &InterfaceType{
			CompositeKind: common.CompositeKindResource,
			Identifier:    "I1",
			Location:      ast.StringLocation("b"),
		}

		i2 := &InterfaceType{
			CompositeKind: common.CompositeKindResource,
			Identifier:    "I2",
			Location:      ast.StringLocation("b"),
		}

		a := &RestrictedResourceType{
			Type: &CompositeType{
				Kind:       common.CompositeKindResource,
				Identifier: "R",
				Location:   ast.StringLocation("a"),
			},
			Restrictions: []*InterfaceType{i1, i2},
		}

		b := &RestrictedResourceType{
			Type: &CompositeType{
				Kind:       common.CompositeKindResource,
				Identifier: "R",
				Location:   ast.StringLocation("a"),
			},
			Restrictions: []*InterfaceType{i1},
		}

		assert.False(t, a.Equal(b))
	})

	t.Run("same base type and same restrictions", func(t *testing.T) {
		i1 := &InterfaceType{
			CompositeKind: common.CompositeKindResource,
			Identifier:    "I1",
			Location:      ast.StringLocation("b"),
		}

		i2 := &InterfaceType{
			CompositeKind: common.CompositeKindResource,
			Identifier:    "I2",
			Location:      ast.StringLocation("b"),
		}

		a := &RestrictedResourceType{
			Type: &CompositeType{
				Kind:       common.CompositeKindResource,
				Identifier: "R",
				Location:   ast.StringLocation("a"),
			},
			Restrictions: []*InterfaceType{i1, i2},
		}

		b := &RestrictedResourceType{
			Type: &CompositeType{
				Kind:       common.CompositeKindResource,
				Identifier: "R",
				Location:   ast.StringLocation("a"),
			},
			Restrictions: []*InterfaceType{i1, i2},
		}

		assert.True(t, a.Equal(b))
	})

	t.Run("different base type and same restrictions", func(t *testing.T) {

		i1 := &InterfaceType{
			CompositeKind: common.CompositeKindResource,
			Identifier:    "I1",
			Location:      ast.StringLocation("b"),
		}

		i2 := &InterfaceType{
			CompositeKind: common.CompositeKindResource,
			Identifier:    "I2",
			Location:      ast.StringLocation("b"),
		}

		a := &RestrictedResourceType{
			Type: &CompositeType{
				Kind:       common.CompositeKindResource,
				Identifier: "R1",
				Location:   ast.StringLocation("a"),
			},
			Restrictions: []*InterfaceType{i1, i2},
		}

		b := &RestrictedResourceType{
			Type: &CompositeType{
				Kind:       common.CompositeKindResource,
				Identifier: "R2",
				Location:   ast.StringLocation("a"),
			},
			Restrictions: []*InterfaceType{i1, i2},
		}

		assert.False(t, a.Equal(b))
	})
}

func TestRestrictedResourceType_GetMember(t *testing.T) {

	t.Run("forbid undeclared members", func(t *testing.T) {
		resourceType := &CompositeType{
			Kind:       common.CompositeKindResource,
			Identifier: "R",
			Location:   ast.StringLocation("a"),
			Members:    map[string]*Member{},
		}
		ty := &RestrictedResourceType{
			Type:         resourceType,
			Restrictions: []*InterfaceType{},
		}

		fieldName := "s"
		resourceType.Members[fieldName] = NewPublicConstantFieldMember(ty.Type, fieldName, &IntType{})

		var reportedError error
		member := ty.GetMember(fieldName, ast.Range{}, func(err error) {
			reportedError = err
		})

		assert.IsType(t, &InvalidRestrictedTypeMemberAccessError{}, reportedError)
		assert.NotNil(t, member)
	})

	t.Run("allow declared members", func(t *testing.T) {
		interfaceType := &InterfaceType{
			CompositeKind: common.CompositeKindResource,
			Identifier:    "I",
			Members:       map[string]*Member{},
		}

		resourceType := &CompositeType{
			Kind:       common.CompositeKindResource,
			Identifier: "R",
			Location:   ast.StringLocation("a"),
			Members:    map[string]*Member{},
		}
		restrictedType := &RestrictedResourceType{
			Type: resourceType,
			Restrictions: []*InterfaceType{
				interfaceType,
			},
		}

		fieldName := "s"

		resourceMember := NewPublicConstantFieldMember(restrictedType.Type, fieldName, &IntType{})
		resourceType.Members[fieldName] = resourceMember

		interfaceMember := NewPublicConstantFieldMember(restrictedType.Type, fieldName, &IntType{})
		interfaceType.Members[fieldName] = interfaceMember

		actualMember := restrictedType.GetMember(fieldName, ast.Range{}, nil)
		assert.Same(t, interfaceMember, actualMember)
	})

}
