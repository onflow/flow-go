package unittest

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/onflow/crypto"
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
	for range n {
		list = append(list, MockEntityFixture())
	}
	return list
}

func MockEntityFixture() *MockEntity {
	return &MockEntity{Identifier: IdentifierFixture()}
}

// RequireEntityNonMalleable is a sanity check that the entity is not malleable with regards to the ID() function.
// Non-malleability in this sense means that it is computationally hard to build a different entity with the same ID.
// Hence, changing *any* field of a non-malleable entity should change the ID, which we check here.
// Note that this is sanity check of non-malleability and that passing this test does not guarantee non-malleability.
// Non-malleability is a required property for any entity that implements the [flow.IDEntity] interface. This is especially
// important for entities that contain signatures and are transmitted over the network.
// ID is used by the protocol to insure entity integrity when transmitted over the network. ID must therefore be a binding cryptographic commitment to an entity.
// This function consumes the entity and modifies its fields randomly to ensure that the ID changes after each modification.
// Generally speaking each type that implements [flow.IDEntity] method should be tested with this function.
// ATTENTION: We put only one requirement for data types, that is all fields have to be exported so we can modify them.
func RequireEntityNonMalleable(t *testing.T, entity flow.IDEntity, ops ...MalleabilityCheckerOpt) {
	err := NewMalleabilityChecker(ops...).Check(entity)
	require.NoError(t, err)
}

// MalleabilityChecker is a customizable checker to test whether an entity is malleable. If a structure implements [flow.IDEntity]
// interface, *any* change to the data structure has to change the ID of the entity as well.
// The MalleabilityChecker performs a recursive check of all fields of the entity and ensures that changing any field will change
// the ID of the entity. By default, the MalleabilityChecker uses pre-defined generators for each basic golang type, which return
// a random value, to modify the entity's field values. However, the MalleabilityChecker can be customized, by providing custom
// types and their generators.
//
// The caller must provide a properly instantiated entity struct, which serves a template for further modification.
// Input entities must have all non-nil and non-empty slice/map fields, otherwise `Check` will return an error.
// In rare cases, a type may have a different ID computation depending on whether a field is nil.
// In such cases, we can use the `malleability:"optional"` struct tag to skip malleability checks when the field is nil.
//
// This checker heavily relies on generation of random values for the fields based on their type. All types are split into three categories:
//  1. structures (generateCustomFlowValue)
//  2. interfaces (generateInterfaceFlowValue)
//  3. primitives, slices, arrays, maps (generateRandomReflectValue)
//
// Checker knows how to deal with each of the categories and generate random values for them.
// There are two ways to handle types not natively recognized byt he MalleabilityChecker:
//  1. User can provide a custom type generator for the type using WithCustomType option.
//  2. User can extend the checker with new type handling.
//
// It is recommended to use the second option if type is used in multiple places and general enough. For other cases
// it is better to use the first option. Mind that the first option overrides any type handling that is embedded in the checker.
// This is very useful for cases where the field is context-sensitive, and we cannot generate a completely random value.
type MalleabilityChecker struct {
	customTypes map[reflect.Type]func() reflect.Value
}

// MalleabilityCheckerOpt is a functional option for the MalleabilityChecker which allows to modify behavior of the checker.
type MalleabilityCheckerOpt func(*MalleabilityChecker)

// WithCustomType allows to override the default behavior of the checker for the given type, meaning if a field of the given type
// is encountered, the MalleabilityChecker will use the provided generator instead of a random value.
// ATTENTION: In order for the MalleabilityChecker to work properly, two calls of the generator should produce two different values.
func WithCustomType[T any](generator func() T) MalleabilityCheckerOpt {
	return func(mc *MalleabilityChecker) {
		mc.customTypes[reflect.TypeOf((*T)(nil)).Elem()] = func() reflect.Value {
			return reflect.ValueOf(generator())
		}
	}
}

// NewMalleabilityChecker creates a new instance of the MalleabilityChecker with the given options.
func NewMalleabilityChecker(ops ...MalleabilityCheckerOpt) *MalleabilityChecker {
	checker := &MalleabilityChecker{
		customTypes: make(map[reflect.Type]func() reflect.Value),
	}
	for _, op := range ops {
		op(checker)
	}
	return checker
}

// Check is a method that performs the malleability check on the entity.
// The malleability check is recursively applied to all fields of the entity.
// Inputs must have all non-nil and non-empty slice/map fields, otherwise Check will return an error.
// In rare cases, a type may have a different ID computation depending on whether a field is nil.
// In such cases, we can use the `malleability:"optional"` struct tag to skip malleability checks when the field is nil.
// It returns an error if the entity is malleable, otherwise it returns nil.
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
	return mc.isEntityMalleable(v, entity.ID)
}

// isEntityMalleable is a helper function to recursively check fields of the entity.
// Every time we change a field we check if ID of the entity has changed. If the ID has not changed then entity is malleable.
// This function returns error if the entity is malleable, otherwise it returns nil.
func (mc *MalleabilityChecker) isEntityMalleable(v reflect.Value, idFunc func() flow.Identifier) error {
	if err := ensureFieldNotEmpty(v); err != nil {
		return err
	}

	tType := v.Type()
	// if we have a custom type function we should use it to generate a random value for the field.
	customTypeGenerator, hasCustomTypeOverride := mc.customTypes[tType]
	if hasCustomTypeOverride {
		origID := idFunc()
		v.Set(customTypeGenerator())
		newID := idFunc()
		if origID != newID {
			return nil
		}
		return fmt.Errorf("ID did not change after changing %s value", tType.String())
	}

	if v.Kind() == reflect.Struct {
		// in case we are dealing with struct we have two options:
		// 1) if it's a type that we know how to generate a random value for, we replace the whole field with such random value
		// 2) if it's an unknown struct type, we check if the field is malleable by checking all of its fields recursively
		if generatedValue := reflect.ValueOf(generateCustomFlowValue(v)); generatedValue.IsValid() {
			origID := idFunc()
			v.Set(generatedValue)
			newID := idFunc()
			if origID != newID {
				return nil
			}
			return fmt.Errorf("ID did not change after changing %s value", tType.String())
		} else {
			for i := 0; i < v.NumField(); i++ {
				field := v.Field(i)
				if !field.CanSet() {
					return fmt.Errorf("field %s is not settable", tType.Field(i).Name)
				}
				if err := ensureFieldNotEmpty(field); err != nil {
					// if the field is empty and has a tag malleability:"optional" we omit it from the check
					// and consider it as non-malleable.
					if tType.Field(i).Tag.Get("malleability") == "optional" {
						continue
					}
					return err
				}
				if err := mc.isEntityMalleable(field, idFunc); err != nil {
					return fmt.Errorf("field %s is malleable: %w", tType.Field(i).Name, err)
				}
			}
			return nil
		}
	} else {
		// when dealing with non-composite type we can generate random values for it and check if ID has changed.
		origID := idFunc()
		err := generateRandomReflectValue(v)
		if err != nil {
			return fmt.Errorf("failed to generate random value for %s: %w", tType.String(), err)
		}
		newID := idFunc()
		if origID != newID {
			return nil
		}
		return fmt.Errorf("ID did not change after changing %s value", tType.String())
	}
}

// ensureFieldNotEmpty is a helper function to ensure that the field is not empty.
// An empty field is considered a nil or an empty map/slice.
func ensureFieldNotEmpty(v reflect.Value) error {
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return fmt.Errorf("invalid entity, field is nil")
	} else if v.Kind() == reflect.Map || v.Kind() == reflect.Slice {
		if v.Len() == 0 {
			return fmt.Errorf("invalid entity, map/slice is empty")
		}
	}
	return nil
}

// generateRandomReflectValue uses reflection to switch on the field type and generate a random value for it.
// This function mutates the input [reflect.Value]. If it cannot mutate the input, an error is returned and the malleability check should be considered failed.
// In rare cases, a type may have a different ID computation depending on whether a field is nil.
// In such cases, we can use the `malleability:"optional"` struct tag to skip malleability checks when the field is nil.
func generateRandomReflectValue(field reflect.Value) error {
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
		sliceLen := field.Len()
		if sliceLen <= 0 {
			return fmt.Errorf("cannot generate random value for empty slice")
		}
		index := rand.Intn(sliceLen)
		return generateRandomReflectValue(field.Index(index))
	case reflect.Array:
		index := rand.Intn(field.Len())
		return generateRandomReflectValue(field.Index(index))
	case reflect.Map:
		mapKeys := field.MapKeys()
		if len(mapKeys) == 0 {
			return fmt.Errorf("cannot generate random value for empty map")
		}
		index := rand.Intn(len(mapKeys))
		key := mapKeys[index]
		oldVal := field.MapIndex(key)
		newVal := reflect.New(oldVal.Type()).Elem()
		if err := generateRandomReflectValue(newVal); err != nil {
			return err
		}
		field.SetMapIndex(key, newVal)
	case reflect.Ptr:
		if field.IsNil() {
			return fmt.Errorf("cannot generate random value for nil pointer")
		}
		return generateRandomReflectValue(field.Elem()) // modify underlying value
	case reflect.Struct:
		generatedValue := reflect.ValueOf(generateCustomFlowValue(field))
		if !generatedValue.IsValid() {
			return fmt.Errorf("cannot generate random value for struct: %s", field.Type().String())
		}
		field.Set(generatedValue)
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

// generateCustomFlowValue generates a random value for the field of the struct that is not a primitive type.
// This can be extended for types that are broadly used in the code base.
func generateCustomFlowValue(field reflect.Value) any {
	switch field.Type() {
	case reflect.TypeOf(flow.Identity{}):
		return *IdentityFixture()
	case reflect.TypeOf(flow.IdentitySkeleton{}):
		return IdentityFixture().IdentitySkeleton
	case reflect.TypeOf(flow.DynamicIdentityEntry{}):
		return flow.DynamicIdentityEntry{
			NodeID:  IdentifierFixture(),
			Ejected: rand.Intn(2) == 1,
		}
	case reflect.TypeOf(flow.ClusterQCVoteData{}):
		return flow.ClusterQCVoteDataFromQC(QuorumCertificateWithSignerIDsFixture())
	case reflect.TypeOf(flow.QuorumCertificate{}):
		return *QuorumCertificateFixture()
	case reflect.TypeOf(flow.TimeoutCertificate{}):
		return flow.TimeoutCertificate{
			View:          rand.Uint64(),
			NewestQCViews: []uint64{rand.Uint64()},
			NewestQC:      QuorumCertificateFixture(),
			SignerIndices: SignerIndicesFixture(2),
			SigData:       SignatureFixture(),
		}
	case reflect.TypeOf(time.Time{}):
		return time.Now()
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
