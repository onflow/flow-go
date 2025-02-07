package unittest

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// MockEntity implements a bare minimum entity for sake of test.
type MockEntity struct {
	Identifier flow.Identifier
	Nonce      uint64
}

func (m MockEntity) ID() flow.Identifier {
	return m.Identifier
}

func (m MockEntity) Checksum() flow.Identifier {
	return m.Identifier
}

func EntityListFixture(n uint) []*MockEntity {
	list := make([]*MockEntity, 0, n)

	for i := uint(0); i < n; i++ {
		list = append(list, &MockEntity{
			Identifier: IdentifierFixture(),
		})
	}

	return list
}

func MockEntityFixture() *MockEntity {
	return &MockEntity{Identifier: IdentifierFixture()}
}

func MockEntityListFixture(count int) []*MockEntity {
	entities := make([]*MockEntity, 0, count)
	for i := 0; i < count; i++ {
		entities = append(entities, MockEntityFixture())
	}
	return entities
}

// generateRandomReflectValue uses reflection to switch on the field type and generate a random value for it.
func generateRandomReflectValue(t *testing.T, field reflect.Value) {
	switch field.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		field.SetUint(field.Uint() + uint64(rand.Intn(100)+1))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		field.SetInt(field.Int() + int64(rand.Intn(100)+1))
	case reflect.String:
		field.SetString(fmt.Sprintf("random_%d", rand.Intn(100000)))
	case reflect.Float64, reflect.Float32:
		field.SetFloat(field.Float() + rand.Float64()*10)
	case reflect.Bool:
		field.SetBool(!field.Bool())
	case reflect.Slice:
		if field.Len() > 0 {
			index := rand.Intn(field.Len())
			generateRandomReflectValue(t, field.Index(index))
		} else {
			newElem := reflect.New(field.Type().Elem()).Elem()
			generateRandomReflectValue(t, newElem)
			field.Set(reflect.Append(field, newElem))
		}
	case reflect.Array:
		index := rand.Intn(field.Len())
		generateRandomReflectValue(t, field.Index(index))
	case reflect.Map:
		if field.Len() > 0 {
			for _, key := range field.MapKeys() {
				oldVal := field.MapIndex(key)
				newVal := reflect.New(oldVal.Type()).Elem()
				generateRandomReflectValue(t, newVal)
				field.SetMapIndex(key, newVal)
				break
			}
		} else {
			key := reflect.New(field.Type().Key()).Elem()
			generateRandomReflectValue(t, key)
			val := reflect.New(field.Type().Elem()).Elem()
			generateRandomReflectValue(t, val)
			field.SetMapIndex(key, val)
		}
	default:
		require.FailNowf(t, "cannot generate random value", "unsupported type: %s", field.Kind().String())
	}
}

// RequireEntityNotMalleable ensures that the entity is not malleable, i.e. changing any field should change the ID.
// Non-malleability is a required property for any entity that implements the [flow.IDEntity] interface. This is especially
// important for entities that contain signatures and are transmitted over the network.
// We would like to ensure that we can create a cryptographic commitment to the content of the entity that covers all fields and
// prevent any tampering with the content of the entity.
// This function consumes the entity and modifies its fields randomly to ensure that the ID changes after each modification.
// Generally speaking each type that implements [flow.IDEntity] method should be tested with this function.
// ATTENTION: We put only one requirement for data types, that is all fields have to be exported so we can modify them.
func RequireEntityNotMalleable(t *testing.T, entity flow.IDEntity) {
	v := reflect.ValueOf(entity)
	isMalleable := isEntityMalleable(t, v, entity.ID)
	require.False(t, isMalleable, "entity is malleable")
}

// isEntityMalleable is a helper function to recursively check fields of the entity. Every time we change a field we check if ID of the entity has changed.
// If ID has not changed then entity is malleable.
// This function returns boolean value so we can add extra traces to the test output in case of recursive calls to structure fields.
func isEntityMalleable(t *testing.T, v reflect.Value, idFunc func() flow.Identifier) bool {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			// if pointer is nil we need to initialize it, it will get zero values
			// but later on we will generate random values for it.
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		// in case of a struct we would like to ensure that changing any field will change the ID of the entity.
		// we will recursively check all fields of the struct and if any of the fields is malleable then the whole entity is malleable.
		tType := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			require.Truef(t, field.CanSet(), "field %s is not settable", tType.Field(i).Name)
			require.False(t, isEntityMalleable(t, field, idFunc), "field %s is malleable", tType.Field(i).Name)
		}
		return false
	} else {
		// when dealing with non-composite type we can generate random values for it and check if ID has changed.
		origID := idFunc()
		generateRandomReflectValue(t, v)
		return origID == idFunc()
	}
}
