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

// generateRandomReflectValue uses reflection to switch on the field type and generate a random value for it.
func generateRandomReflectValue(t *testing.T, field reflect.Value) bool {
	generated := true
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
			generated = false
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
			generated = false
		}
	case reflect.Ptr:
		require.False(t, field.IsNil(), "expect non-nil pointer")
		generateRandomReflectValue(t, field.Elem()) // modify underlying value
	case reflect.Struct:
		generatedValue := generateCustomFlowValue(field)
		require.Truef(t, generatedValue.IsValid(), "cannot generate random value for struct: %s", field.Type().String())
		field.Set(generatedValue)
	case reflect.Interface:
		generatedValue := generateInterfaceFlowValue(field) // it's always a pointer
		require.Truef(t, generatedValue.IsValid(), "cannot generate random value for interface: %s", field.Type().String())
		field.Set(generatedValue)
	default:
		require.FailNowf(t, "cannot generate random value", "unsupported type: %s", field.Kind().String())
	}
	return generated
}

func generateCustomFlowValue(field reflect.Value) reflect.Value {
	switch field.Type() {
	case reflect.TypeOf(flow.IdentitySkeleton{}):
		return reflect.ValueOf(IdentityFixture().IdentitySkeleton)
	case reflect.TypeOf(flow.DynamicIdentityEntry{}):
		return reflect.ValueOf(flow.DynamicIdentityEntry{
			NodeID:  IdentifierFixture(),
			Ejected: rand.Intn(2) == 1,
		})
	case reflect.TypeOf(flow.EpochExtension{}):
		return reflect.ValueOf(flow.EpochExtension{
			FirstView: uint64(0),
			FinalView: uint64(rand.Uint32() + 1000),
		})
	case reflect.TypeOf(flow.ClusterQCVoteData{}):
		return reflect.ValueOf(flow.ClusterQCVoteDataFromQC(QuorumCertificateWithSignerIDsFixture()))
	case reflect.TypeOf(flow.QuorumCertificate{}):
		return reflect.ValueOf(*QuorumCertificateFixture())
	case reflect.TypeOf(flow.TimeoutCertificate{}):
		return reflect.ValueOf(flow.TimeoutCertificate{
			View:          rand.Uint64(),
			NewestQCViews: []uint64{rand.Uint64()},
			NewestQC:      QuorumCertificateFixture(),
			SignerIndices: SignerIndicesFixture(2),
			SigData:       SignatureFixture(),
		})
	}
	return reflect.Value{}
}

func generateInterfaceFlowValue(field reflect.Value) reflect.Value {
	if field.Type().Implements(reflect.TypeOf((*crypto.PublicKey)(nil)).Elem()) {
		return reflect.ValueOf(KeyFixture(crypto.ECDSAP256).PublicKey())
	}
	return reflect.Value{}
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
	NewMalleabilityChecker(t).Check(entity)
}

type MalleabilityChecker struct {
	*testing.T
	customTypes map[reflect.Type]func() any
}

type MalleabilityCheckerOpt func(*MalleabilityChecker)

func WithCustomType(tType any, generator func() any) MalleabilityCheckerOpt {
	return func(t *MalleabilityChecker) {
		t.customTypes[reflect.TypeOf(tType)] = generator
	}
}

func NewMalleabilityChecker(t *testing.T, ops ...MalleabilityCheckerOpt) *MalleabilityChecker {
	checker := &MalleabilityChecker{
		T:           t,
		customTypes: make(map[reflect.Type]func() any),
	}
	for _, op := range ops {
		op(checker)
	}
	return checker
}

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
		v.Set(reflect.ValueOf(customTypeGenerator()))
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
		return expectChange && origID == idFunc()
	}
}
