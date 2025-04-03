package unittest

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/onflow/crypto"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// MockEntity implements a bare minimum entity for sake of test.
// TODO(malleability, #7076): This is a leftover from previous design, it is used to test mempools that are working with entity.
// As soon as that work stream is finished, this entity should be removed.
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
	for range n {
		list = append(list, MockEntityFixture())
	}
	return list
}

func MockEntityFixture() *MockEntity {
	return &MockEntity{Identifier: IdentifierFixture()}
}

// RequireEntityNonMalleable is RequireNonMalleable with the constraint that models implement [flow.IDEntity]
// and the content hash function is the ID() function.
// Non-malleability is a required property for any entity that implements the [flow.IDEntity] interface.
// This is especially important for entities that contain signatures and are transmitted over the network.
// ID is used by the protocol to insure entity integrity when transmitted over the network. ID must therefore be a binding cryptographic commitment to an entity.
// This function consumes the entity and modifies its fields randomly to ensure that the ID changes after each modification.
// Generally speaking each type that implements [flow.IDEntity] method should be tested with this function.
func RequireEntityNonMalleable(t *testing.T, entity flow.IDEntity, ops ...MalleabilityCheckerOpt) {
	err := NewMalleabilityChecker(ops...).Check(entity)
	require.NoError(t, err)
}

// RequireNonMalleable is a sanity check that the model is not malleable with respect to a content hash over the model (hashModel).
// Non-malleability in this sense means that it is computationally hard to build a different model with the same hash.
// Hence, changing *any* field of a non-malleable model should change the hash, which we check here.
// Note that this is sanity check of non-malleability and that passing this test does not guarantee non-malleability.
// ATTENTION: We put only one requirement for data types, that is all fields have to be exported so we can modify them.
func RequireNonMalleable(t *testing.T, model any, hashModel func() flow.Identifier, ops ...MalleabilityCheckerOpt) {
	err := NewMalleabilityChecker(ops...).CheckCustom(model, hashModel)
	require.NoError(t, err)
}

// MalleabilityChecker is a customizable checker to test whether an entity is malleable. If a structure implements [flow.IDEntity]
// interface, *any* change to the data structure has to change the ID of the entity as well.
// The MalleabilityChecker performs a recursive check of all fields of the entity and ensures that changing any field will change
// the ID of the entity. By default, the MalleabilityChecker uses pre-defined generators for each basic golang type, which return
// a random value, to modify the entity's field values. However, the MalleabilityChecker can be customized, by providing custom
// generators for specific types or specific fields.
//
// The caller can provide a loosely instantiated entity struct, which serves as a template for further modification.
// If checker encounters a field that is nil, it will insert a randomized instance into field and continue the check.
// If checker encounters a nil/empty slice or map, it will create a new instance of the slice/map, insert a value and continue the check.
// In rare cases, a type may have a different ID computation depending on whether a field is nil.
// In such cases, we can use the WithPinnedField option to skip malleability checks on this field.
//
// This checker heavily relies on generation of random values for the fields based on their type. All types are split into three categories:
//  1. structures, primitives, slices, arrays, maps (generateRandomReflectValue)
//  2. interfaces (generateInterfaceFlowValue)
//
// Checker knows how to deal with each of the categories and generate random values for them.
// There are two ways to handle types not natively recognized byt he MalleabilityChecker:
//  1. User can provide a custom type generator for the type using WithTypeGenerator option.
//  2. User can provide a custom generator for the field using WithFieldGenerator option.
//
// It is recommended to use the first option if type is used in multiple places and general enough.
// Matching by type (instead of field name) is less selective, by covering all fields of the given type.
// Field generator is very useful for cases where the field is context-sensitive, and we cannot generate a completely random value.
type MalleabilityChecker struct {
	typeGenerator  map[reflect.Type]func() reflect.Value
	fieldGenerator map[string]func() reflect.Value
	pinnedFields   map[string]struct{}
}

// MalleabilityCheckerOpt is a functional option for the MalleabilityChecker which allows to modify behavior of the checker.
type MalleabilityCheckerOpt func(*MalleabilityChecker)

// WithTypeGenerator allows to override the default behavior of the checker for the given type, meaning if a field of the given type
// is encountered, the MalleabilityChecker will use the provided generator instead of a random value.
// An example usage would be:
//
//	type BarType struct {
//		Baz string
//	}
//	type FooType struct {
//		Bar []BarType
//	}
//
//	...
//
//	WithTypeGenerator(func() BarType { return randomBar()})
//
// ATTENTION: In order for the MalleabilityChecker to work properly, two calls of the generator should produce two different values.
func WithTypeGenerator[T any](generator func() T) MalleabilityCheckerOpt {
	return func(mc *MalleabilityChecker) {
		mc.typeGenerator[reflect.TypeOf((*T)(nil)).Elem()] = func() reflect.Value {
			return reflect.ValueOf(generator())
		}
	}
}

// WithPinnedField allows to skip malleability checks for the given field. If a field with given path is encountered, the MalleabilityChecker
// will skip the check for this field. Pinning is mutually exclusive with field generators, meaning if a field is pinned, the checker will
// not overwrite the field with a random value, even if a field generator is provided. This is useful when the ID computation varies depending
// on specific values of some field (typically used for temporary downwards compatibility - new fields are added as temporary optional).
// An example usage would be:
//
//	type BarType struct {
//		Baz string
//	}
//	type FooType struct {
//		Bar BarType
//	}
//
//	...
//
//	WithPinnedField("Bar.Baz")
func WithPinnedField(field string) MalleabilityCheckerOpt {
	return func(mc *MalleabilityChecker) {
		mc.pinnedFields[field] = struct{}{}
	}
}

// WithFieldGenerator allows to override the default behavior of the checker for the given field, meaning if a field with given path
// is encountered, the MalleabilityChecker will use the provided generator instead of a random value.
// An example usage would be:
//
//	type BarType struct {
//		Baz string
//	}
//	type FooType struct {
//		Bar BarType
//	}
//
//	...
//
//	WithFieldGenerator("Bar.Baz", func() string { return randomString()})
func WithFieldGenerator[T any](field string, generator func() T) MalleabilityCheckerOpt {
	return func(mc *MalleabilityChecker) {
		mc.fieldGenerator[field] = func() reflect.Value {
			return reflect.ValueOf(generator())
		}
	}
}

// NewMalleabilityChecker creates a new instance of the MalleabilityChecker with the given options.
func NewMalleabilityChecker(ops ...MalleabilityCheckerOpt) *MalleabilityChecker {
	checker := &MalleabilityChecker{
		pinnedFields:   make(map[string]struct{}),
		typeGenerator:  make(map[reflect.Type]func() reflect.Value),
		fieldGenerator: make(map[string]func() reflect.Value),
	}

	for _, op := range ops {
		op(checker)
	}

	return checker
}

// Check is a method that performs the malleability check on the entity.
// The caller provides a loosely instantiated entity struct, which serves a template for further modification.
// The malleability check is recursively applied to all fields of the entity.
// If one of the fields is nil or empty slice/map, the checker will create a new instance of the field and continue the check.
// In rare cases, a type may have a different ID computation depending on whether a field is nil, in such case, we can use field pinning to
// prevent the checker from changing the field.
// It returns an error if the entity is malleable, otherwise it returns nil.
// No errors are expected during normal operations.
func (mc *MalleabilityChecker) Check(entity flow.IDEntity) error {
	v := reflect.ValueOf(entity)
	if !v.IsValid() {
		return fmt.Errorf("input is not a valid entity")
	}
	if v.Kind() != reflect.Ptr {
		// If it is not a pointer type, we may not be able to set fields to test malleability, since the entity may not be addressable
		return fmt.Errorf("entity is not a pointer type (try checking a reference to it), entity: %v %v", v.Kind(), v.Type())
	}
	if v.IsNil() {
		return fmt.Errorf("entity is nil, nothing to check")
	}
	v = v.Elem()
	if err := mc.isEntityMalleable(v, nil, "", entity.ID); err != nil {
		return err
	}
	return mc.checkExpectations()
}

func (mc *MalleabilityChecker) CheckCustom(model any, hashModel func() flow.Identifier) error {
	v := reflect.ValueOf(model)
	if !v.IsValid() {
		return fmt.Errorf("input is not a valid model")
	}
	if v.Kind() != reflect.Ptr {
		// If it is not a pointer type, we may not be able to set fields to test malleability, since the model may not be addressable
		return fmt.Errorf("model is not a pointer type (try checking a reference to it), model: %v %v", v.Kind(), v.Type())
	}
	if v.IsNil() {
		return fmt.Errorf("model is nil, nothing to check")
	}
	v = v.Elem()
	if err := mc.isEntityMalleable(v, nil, "", hashModel); err != nil {
		return err
	}
	return mc.checkExpectations()
}

// checkExpectations checks if all pre-configured options were used during the check.
// This includes checking if all pinned fields were used and if all field generators were used.
// Pins and field generators are mutually exclusive, and consumed once per field.
// An error is returned in case checker has been misconfigured.
func (mc *MalleabilityChecker) checkExpectations() error {
	for field := range mc.pinnedFields {
		return fmt.Errorf("field %s is pinned, but wasn't used, checker misconfigured", field)
	}
	for field := range mc.fieldGenerator {
		return fmt.Errorf("field %s has a generator, but wasn't used, checker misconfigured", field)
	}
	return nil
}

// isEntityMalleable is a helper function to recursively check fields of the entity.
// This function is called recursively for each field of the entity and checks if the entity is malleable by comparing ID
// before and after changing the field value.
// Arguments:
//   - v: field value to check.
//   - structField: optional metadata about the field, it is present only for values which are fields of a struct.
//   - parentFieldPath: previously accumulated field path which leads to the current field.
//   - idFunc:  function to get the ID of the whole entity.
//
// This function returns error if the entity is malleable, otherwise it returns nil.
func (mc *MalleabilityChecker) isEntityMalleable(v reflect.Value, structField *reflect.StructField, parentFieldPath string, idFunc func() flow.Identifier) error {
	var fullFieldPath string
	// if we are dealing with a field of a struct, we need to build a full field path and use that for custom options lookup.
	if structField != nil {
		fullFieldPath = buildFieldPath(parentFieldPath, structField.Name)
		// pinning has priority over field generators, if the field is pinned, we skip the check for this field.
		// if we have both pin and field generator, we will never use the field generator but will fail to meet the expectations, after running the check.
		if _, ok := mc.pinnedFields[fullFieldPath]; ok {
			// make sure we consume the pin so we can check if all pins were used.
			delete(mc.pinnedFields, fullFieldPath)
			return nil
		}
	}

	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	tType := v.Type()

	// if we have a field generator for the field, we use it to generate a random value for the field instead of using the default flow.
	if generator, ok := mc.fieldGenerator[fullFieldPath]; ok {
		// make sure we consume the field generator so we can check if all field generators were used.
		delete(mc.fieldGenerator, fullFieldPath)
		origID := idFunc()
		v.Set(generator())
		newID := idFunc()
		if origID != newID {
			return nil
		}
		return fmt.Errorf("ID did not change after changing %s value", fullFieldPath)
	}

	if v.Kind() == reflect.Struct {
		// any time we encounter a structure we need to go through all fields and check if the entity is malleable in recursive manner.
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if !field.CanSet() {
				return fmt.Errorf("field %s is not settable, try providing a field generator for field %s", tType.Field(i).Name, fullFieldPath)
			}

			nextField := tType.Field(i)
			if err := mc.isEntityMalleable(field, &nextField, fullFieldPath, idFunc); err != nil {
				return fmt.Errorf("field %s is malleable: %w", tType.Field(i).Name, err)
			}
		}
		return nil
	} else {
		// when dealing with non-composite type we can generate random values for it and check if ID has changed.
		origID := idFunc()
		err := mc.generateRandomReflectValue(v)
		if err != nil {
			return fmt.Errorf("failed to generate random value for %s: %w", fullFieldPath, err)
		}
		newID := idFunc()
		if origID != newID {
			return nil
		}
		return fmt.Errorf("ID did not change after changing %s value", fullFieldPath)
	}
}

// buildFieldPath is a helper function to build a full field path.
func buildFieldPath(fieldPath string, fieldName string) string {
	if fieldPath == "" {
		return fieldName
	}
	return fieldPath + "." + fieldName
}

// generateRandomReflectValue uses reflection to switch on the field type and generate a random value for it.
// This function mutates the input [reflect.Value]. If it cannot mutate the input, an error is returned and the malleability check should be considered failed.
// If a type generator is provided for the field type, it will be used to generate a random value for the field.
// No errors are expected during normal operations.
func (mc *MalleabilityChecker) generateRandomReflectValue(field reflect.Value) error {
	if generator, ok := mc.typeGenerator[field.Type()]; ok {
		field.Set(generator())
		return nil
	}

	switch field.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		field.SetUint(^field.Uint())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		field.SetInt(^field.Int())
	case reflect.String:
		field.SetString(fmt.Sprintf("random_%d", rand.Intn(100000)))
	case reflect.Float64, reflect.Float32:
		field.SetFloat(field.Float() + rand.Float64()*10)
	case reflect.Bool:
		field.SetBool(!field.Bool())
	case reflect.Slice:
		if field.Len() == 0 {
			field.Set(reflect.MakeSlice(field.Type(), 1, 1))
		}
		return mc.generateRandomReflectValue(field.Index(rand.Intn(field.Len())))
	case reflect.Array:
		index := rand.Intn(field.Len())
		return mc.generateRandomReflectValue(field.Index(index))
	case reflect.Map:
		mapKeys := field.MapKeys()
		var key reflect.Value
		if len(mapKeys) == 0 {
			field.Set(reflect.MakeMap(field.Type()))
			key = reflect.New(field.Type().Key()).Elem()
			if err := mc.generateRandomReflectValue(key); err != nil {
				return err
			}
		} else {
			index := rand.Intn(len(mapKeys))
			key = mapKeys[index]
		}
		val := reflect.New(field.Type().Elem()).Elem()
		if err := mc.generateRandomReflectValue(val); err != nil {
			return err
		}
		field.SetMapIndex(key, val)
	case reflect.Ptr:
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return mc.generateRandomReflectValue(field.Elem()) // modify underlying value
	case reflect.Struct:
		// if we are dealing with a struct, we need to go through all fields and generate random values for them
		// if the field is another struct, we will deal with it recursively.
		// at the end of the recursion, we must encounter a primitive type, which we can generate a random value for, otherwise an error is returned.
		for i := 0; i < field.NumField(); i++ {
			structField := field.Field(i)
			err := mc.generateRandomReflectValue(structField)
			if err != nil {
				return fmt.Errorf("cannot generate random value for struct field: %s", field.Type().String())
			}
		}
	case reflect.Interface:
		generatedValue := reflect.ValueOf(generateInterfaceFlowValue(field)) // it's always a pointer
		if !generatedValue.IsValid() {
			return fmt.Errorf("cannot generate random value for interface: %s", field.Type().String())
		}
		field.Set(generatedValue)
	default:
		return fmt.Errorf("cannot generate random value, unsupported type: %s", field.Kind().String())
	}
	return nil
}

// generateInterfaceFlowValue generates a random value for the field of the struct that is an interface.
// This can be extended for types that are broadly used in the code base.
func generateInterfaceFlowValue(field reflect.Value) any {
	if field.Type().Implements(reflect.TypeOf((*crypto.PublicKey)(nil)).Elem()) {
		return KeyFixture(crypto.ECDSAP256).PublicKey()
	}
	return nil
}
