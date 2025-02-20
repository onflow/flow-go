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

// RequireEntityNonMalleable is a sanity check that the entity is not malleable with regards to the ID() function.
// Non-malleability in this sense means that it is computationally hard to build a different entity with the same ID. This implies that in a non-malleable entity, changing any field should change the ID, which is checked by this function. Note that this is sanity check of non-malleability and that passing this test does not guarantee non-malleability.
// Non-malleability is a required property for any entity that implements the [flow.IDEntity] interface. This is especially
// important for entities that contain signatures and are transmitted over the network.
// ID is used by the protocol to insure entity integrity when transmitted over the network. ID must therefore be a binding cryptographic commitment to an entity.
// This function consumes the entity and modifies its fields randomly to ensure that the ID changes after each modification.
// Generally speaking each type that implements [flow.IDEntity] method should be tested with this function.
// ATTENTION: We put only one requirement for data types, that is all fields have to be exported so we can modify them.
func RequireEntityNonMalleable(t *testing.T, entity flow.IDEntity, ops ...MalleabilityCheckerOpt) {
	NewMalleabilityChecker(t, ops...).Check(entity)
}

// MalleabilityChecker is a structure that holds additional information about the context of malleability check.
// It allows to customize the behavior of the check by providing custom types and their generators.
// All underlying checks are implemented as methods of this structure.
// This structure is used to check if the entity is malleable. Strictly speaking if a structure implements [flow.IDEntity] interface
// any change to the data structure has to change the ID of the entity as well.
// This structure performs a recursive check of all fields of the entity and ensures that changing any field will change the ID of the entity.
// The idea behind implementation is that user provides a structure which serves a layout(scheme) for the entity and
// the checker will generate random values for the fields that are not empty or nil. This way it supports structures where some fields
// are omitted in some scenarios but the structure has to be non-malleable.
// This checker heavily relies on generation of random values for the fields based on their type. All types are split into three categories:
// 1) structures (generateCustomFlowValue)
// 2) interfaces (generateInterfaceFlowValue)
// 3) primitives, slices, arrays, maps (generateRandomReflectValue)
// Checker knows how to deal with each of the categories and generate random values for them but not for all types.
// If the type is not recognized there are two ways:
// 1) User can provide a custom type generator for the type using WithCustomType option.
// 2) User can extend the checker with new type handling.
// It is recommended to use the second option if type is used in multiple places and general enough. For other cases
// it is better to use the first option. Mind that the first option overrides any type handling that is embedded in the checker.
// This is very useful for cases where the field is context-sensitive, and we cannot generate a completely random value.
type MalleabilityChecker struct {
	*testing.T
	customTypes map[reflect.Type]func() reflect.Value
}

// MalleabilityCheckerOpt is a functional option for the MalleabilityChecker which allows to modify behavior of the checker.
type MalleabilityCheckerOpt func(*MalleabilityChecker)

// WithCustomType allows to override the default behavior of the checker for the given type, meaning if a field of the given type is encountered
// it will use generator instead of generating a random value.
func WithCustomType[T any](tType any, generator func() T) MalleabilityCheckerOpt {
	return func(t *MalleabilityChecker) {
		t.customTypes[reflect.TypeOf(tType)] = func() reflect.Value {
			return reflect.ValueOf(generator())
		}
	}
}

// NewMalleabilityChecker creates a new instance of the MalleabilityChecker with the given options.
func NewMalleabilityChecker(t *testing.T, ops ...MalleabilityCheckerOpt) *MalleabilityChecker {
	checker := &MalleabilityChecker{
		T:           t,
		customTypes: make(map[reflect.Type]func() reflect.Value),
	}
	for _, op := range ops {
		op(checker)
	}
	return checker
}

// Check is a method that performs the malleability check on the entity.
func (t *MalleabilityChecker) Check(entity flow.IDEntity) {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Ptr {
		require.False(t, v.IsNil(), "entity is nil, nothing to check")
		v = v.Elem()
	}
	isMalleable := t.isEntityMalleable(v, entity.ID)
	require.False(t, isMalleable, "entity is malleable")
}

// isEntityMalleable is a helper function to recursively check fields of the entity. Every time we change a field we check if ID of the entity has changed.
// If ID has not changed then entity is malleable.
// This function returns boolean value so we can add extra traces to the test output in case of recursive calls to structure fields.
func (t *MalleabilityChecker) isEntityMalleable(v reflect.Value, idFunc func() flow.Identifier) bool {
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return false // there is nothing to check for this field if it's nil
	}

	tType := v.Type()
	// if we have a custom type function we should use it to generate a random value for the field.
	customTypeGenerator, hasCustomTypeOverride := t.customTypes[tType]
	if hasCustomTypeOverride {
		origID := idFunc()
		v.Set(customTypeGenerator())
		return origID == idFunc()
	}

	if v.Kind() == reflect.Struct {
		// in case of a struct we would like to ensure that changing any field will change the ID of the entity.
		// we will recursively check all fields of the struct and if any of the fields is malleable then the whole entity is malleable.
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			require.Truef(t, field.CanSet(), "field %s is not settable", tType.Field(i).Name)
			require.False(t, t.isEntityMalleable(field, idFunc), "field %s is malleable", tType.Field(i).Name)
		}
		return false
	} else {
		// when dealing with non-composite type we can generate random values for it and check if ID has changed.
		origID := idFunc()
		expectChange := generateRandomReflectValue(t.T, v)
		newID := idFunc()
		return expectChange && origID == newID
	}
}

// generateRandomReflectValue uses reflection to switch on the field type and generate a random value for it.
func generateRandomReflectValue(t *testing.T, field reflect.Value) bool {
	generated := true
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
		if field.Len() > 0 {
			index := rand.Intn(field.Len())
			generateRandomReflectValue(t, field.Index(index))
		} else {
			generated = false // empty slice is ignored
		}
	case reflect.Array:
		index := rand.Intn(field.Len())
		generateRandomReflectValue(t, field.Index(index))
	case reflect.Map:
		if mapKeys := field.MapKeys(); len(mapKeys) > 0 {
			for _, key := range mapKeys {
				oldVal := field.MapIndex(key)
				newVal := reflect.New(oldVal.Type()).Elem()
				generateRandomReflectValue(t, newVal)
				field.SetMapIndex(key, newVal)
				break
			}
		} else {
			generated = false // empty map is ignored
		}
	case reflect.Ptr:
		require.False(t, field.IsNil(), "expect non-nil pointer")
		generateRandomReflectValue(t, field.Elem()) // modify underlying value
	case reflect.Struct:
		generatedValue := reflect.ValueOf(generateCustomFlowValue(field))
		require.Truef(t, generatedValue.IsValid(), "cannot generate random value for struct: %s", field.Type().String())
		field.Set(generatedValue)
	case reflect.Interface:
		generatedValue := reflect.ValueOf(generateInterfaceFlowValue(field)) // it's always a pointer
		require.Truef(t, generatedValue.IsValid(), "cannot generate random value for interface: %s", field.Type().String())
		field.Set(generatedValue)
	default:
		require.FailNowf(t, "cannot generate random value", "unsupported type: %s", field.Kind().String())
	}
	return generated
}

// generateCustomFlowValue generates a random value for the field of the struct that is not a primitive type.
// This can be extended for types that are broadly used in the code base.
func generateCustomFlowValue(field reflect.Value) any {
	switch field.Type() {
	case reflect.TypeOf(flow.Identity{}):
		return *IdentityFixture()
	case reflect.TypeOf(flow.IdentitySkeleton{}):
		return IdentityFixture().IdentitySkeleton
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
