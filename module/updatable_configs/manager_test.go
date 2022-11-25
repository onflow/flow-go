package updatable_configs_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/updatable_configs"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRegisterDuplicateConfig(t *testing.T) {
	mgr := updatable_configs.NewManager()

	// should be able to register a config the first time
	err := mgr.RegisterBoolConfig("field",
		func() bool { return true },
		func(_ bool) error { return nil })
	require.NoError(t, err)

	// should fail to register the same field name again, regardless of type
	err = mgr.RegisterUintConfig("field",
		func() uint { return 0 },
		func(_ uint) error { return nil })
	assert.ErrorIs(t, err, updatable_configs.ErrAlreadyRegistered)
}

func TestManager_RegisterBoolConfig(t *testing.T) {
	mgr := updatable_configs.NewManager()

	// should be able to register config
	fieldSet := make(chan struct{}) // closed when field is successfully set
	err := mgr.RegisterBoolConfig("field",
		func() bool { return true },
		func(_ bool) error { close(fieldSet); return nil })
	require.NoError(t, err)

	// should be able to get the field
	field, ok := mgr.GetField("field")
	assert.True(t, ok)
	// field must be parseable by structpb (otherwise admin server will error)
	_, err = structpb.NewValue(field.Get())
	require.NoError(t, err)

	// should fail to set incorrect type
	err = field.Set(struct{}{})
	assert.Error(t, err)
	assert.True(t, updatable_configs.IsValidationError(err))

	// should succeed setting correct type
	err = field.Set(true)
	assert.NoError(t, err)
	assert.True(t, util.CheckClosed(fieldSet))
}

func TestManager_RegisterUintConfig(t *testing.T) {
	mgr := updatable_configs.NewManager()

	// should be able to register config
	fieldSet := make(chan struct{}) // closed when field is successfully set
	err := mgr.RegisterUintConfig("field",
		func() uint { return 0 },
		func(_ uint) error { close(fieldSet); return nil })
	require.NoError(t, err)

	// should be able to get the field
	field, ok := mgr.GetField("field")
	assert.True(t, ok)
	// field must be parseable by structpb (otherwise admin server will error)
	_, err = structpb.NewValue(field.Get())
	require.NoError(t, err)

	// should fail to set incorrect type
	err = field.Set(struct{}{})
	assert.Error(t, err)
	assert.True(t, updatable_configs.IsValidationError(err))

	// should succeed setting correct type
	err = field.Set(float64(1)) // JSON uints parse to float64
	assert.NoError(t, err)
	assert.True(t, util.CheckClosed(fieldSet))
}

func TestManager_RegisterDurationConfig(t *testing.T) {
	mgr := updatable_configs.NewManager()

	// should be able to register config
	fieldSet := make(chan struct{}) // closed when field is successfully set
	err := mgr.RegisterDurationConfig("field",
		func() time.Duration { return time.Second },
		func(_ time.Duration) error { close(fieldSet); return nil })
	require.NoError(t, err)

	// should be able to get the field
	field, ok := mgr.GetField("field")
	assert.True(t, ok)
	// field must be parseable by structpb (otherwise admin server will error)
	_, err = structpb.NewValue(field.Get())
	require.NoError(t, err)

	// should fail to set incorrect type
	err = field.Set(struct{}{})
	assert.Error(t, err)
	assert.True(t, updatable_configs.IsValidationError(err))
	// should fail to set with correct type, but unparseable
	err = field.Set("not a parseable duration string")
	assert.Error(t, err)
	assert.True(t, updatable_configs.IsValidationError(err))

	// should succeed setting correct type
	err = field.Set("1h")
	assert.NoError(t, err)
	assert.True(t, util.CheckClosed(fieldSet))
}

func TestManager_RegisterIdentifierListConfig(t *testing.T) {
	mgr := updatable_configs.NewManager()

	// should be able to register config
	fieldSet := make(chan struct{}) // closed when field is successfully set
	err := mgr.RegisterIdentifierListConfig("field",
		func() flow.IdentifierList { return nil },
		func(_ flow.IdentifierList) error { close(fieldSet); return nil })
	require.NoError(t, err)

	// should be able to get the field
	field, ok := mgr.GetField("field")
	assert.True(t, ok)
	// field must be parseable by structpb (otherwise admin server will error)
	_, err = structpb.NewValue(field.Get())
	require.NoError(t, err)

	// should fail to set incorrect type
	err = field.Set(struct{}{})
	assert.Error(t, err)
	assert.True(t, updatable_configs.IsValidationError(err))
	// should fail to set with correct type, but un-parseable
	err = field.Set([]string{"un-parseable"})
	assert.Error(t, err)
	assert.True(t, updatable_configs.IsValidationError(err))
	err = field.Set([]string{unittest.IdentifierFixture().String(), "un-parseable"})
	assert.Error(t, err)
	assert.True(t, updatable_configs.IsValidationError(err))

	// should succeed setting correct type
	err = field.Set(util.DetypeSlice(unittest.IdentifierListFixture(5).Strings()))
	assert.NoError(t, err)
	assert.True(t, util.CheckClosed(fieldSet))
}
