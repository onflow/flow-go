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
	case reflect.Ptr:
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem())) // Initialize if nil
		}
		generateRandomReflectValue(t, field.Elem()) // Modify underlying value
	case reflect.Struct:
		// Modify a single random field in the struct
		numFields := field.NumField()
		if numFields > 0 {
			randomField := field.Field(rand.Intn(numFields))
			if randomField.CanSet() {
				generateRandomReflectValue(t, randomField)
			}
		}
	default:
		require.FailNowf(t, "unsupported type: %s", field.Kind().String())
	}
}

// RequireEntityNotMalleable ensures that the entity is not malleable, i.e. changing any field should change the ID.
// Non-malleability is a required property for any entity that implements the [flow.IDEntity] interface. This is especially
// important for entities that contain signatures and are transmitted over the network.
// We would like to ensure that we can create a cryptographic commitment to the content of the entity that covers all fields and
// prevent any tampering with the content of the entity.
// This function consumes the entity and modifies its fields randomly to ensure that the ID changes after each modification.
// Generally speaking each type that implements [flow.IDEntity] method should be tested with this function.
func RequireEntityNotMalleable(t *testing.T, entity flow.IDEntity) {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		// in case of a struct we would like to ensure that implementation of ID() method covers all fields
		// to do this we will generate random value for each field and check if ID has changed at each step
		tType := v.Type()
		origID := entity.ID()
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			require.Truef(t, field.CanSet(), "field %s is not settable", tType.Field(i).Name)

			// modify only this field
			generateRandomReflectValue(t, field)
			newID := entity.ID()

			// ensure ID has changed
			require.NotEqualf(t, origID, newID, "ID did not change after modifying field: %s", tType.Field(i).Name)
			origID = newID
		}
	} else {
		// if we are dealing with not composite type then we can just generate random value for inner type
		// and check if ID has changed
		origID := entity.ID()
		generateRandomReflectValue(t, v)

		// Ensure ID has changed
		require.NotEqual(t, origID, entity.ID(), "ID did not change after modifying value")
	}
}

func requireEntityNotMalleableHelper(t *testing.T, entity reflect.Value, idFunc func() flow.Identifier) {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		// in case of a struct we would like to ensure that implementation of ID() method covers all fields
		// to do this we will generate random value for each field and check if ID has changed at each step
		tType := v.Type()
		origID := idFunc()
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			require.Truef(t, field.CanSet(), "field %s is not settable", tType.Field(i).Name)

			// modify only this field
			generateRandomReflectValue(t, field)
			newID := idFunc()

			// ensure ID has changed
			require.NotEqualf(t, origID, newID, "ID did not change after modifying field: %s", tType.Field(i).Name)
			origID = newID
		}
	} else {
		// if we are dealing with not composite type then we can just generate random value for inner type
		// and check if ID has changed
		origID := idFunc()
		generateRandomReflectValue(t, v)

		// Ensure ID has changed
		require.NotEqual(t, origID, idFunc(), "ID did not change after modifying value")
	}
}
