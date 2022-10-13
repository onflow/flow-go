package updatable_configs

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrAlreadyRegistered is returned when a config field is registered with a name
// conflicting with an already registered config field.
var ErrAlreadyRegistered = fmt.Errorf("config name already registered")

// ValidationError is returned by a config setter function (Set*ConfigFunc) when
// the provided config field is invalid, and was not applied.
type ValidationError struct {
	Err error
}

func (err ValidationError) Error() string {
	return err.Err.Error()
}

func NewValidationErrorf(msg string, args ...any) ValidationError {
	return ValidationError{
		Err: fmt.Errorf(msg, args...),
	}
}

func IsValidationError(err error) bool {
	return errors.As(err, &ValidationError{})
}

type (
	SetAnyConfigFunc func(any) error
	GetAnyConfigFunc func() any
)

// The below are typed setter and getter functions for different config types.
// If you need to add a new configurable config field with a type not below,
// add a new setter and getter type below, and a Register*Config method to the
// Registerer interface and Manager implementation below.

type (
	// Set*ConfigFunc is a setter function for a single updatable config field.
	// Returns ValidationError if the new config value is invalid.
	SetUintConfigFunc     func(uint) error
	SetBoolConfigFunc     func(bool) error
	SetDurationConfigFunc func(time.Duration) error

	// Get*ConfigFunc is a getter function for a single updatable config field.
	GetUintConfigFunc     func() uint
	GetBoolConfigFunc     func() bool
	GetDurationConfigFunc func() time.Duration
)

// Field represents one dynamically configurable field.
type Field struct {
	// Name is the name of the config field, must be globally unique.
	Name string
	// Set is the setter function for the config field. It enforces validation rules
	// and applies the new config value.
	// Returns ValidationError if the new config value is invalid.
	Set SetAnyConfigFunc
	// Get is the getter function for the config field. It returns the current value
	// for the config field.
	Get GetAnyConfigFunc
}

// Manager manages setter and getter for updatable configs, across all components.
// Components register updatable config fields with the manager at startup, then
// the Manager exposes the ability to dynamically update these configs while the
// node is running, for example, via admin commands.
//
// The Manager maintains a list of type-agnostic updatable config fields. The
// typed registration function (Register*Config) is responsible for type conversion.
type Manager struct {
	mu     sync.Mutex
	fields map[string]Field
}

func NewManager() *Manager {
	return &Manager{
		fields: make(map[string]Field),
	}
}

// GetField returns the updatable config field with the given name, if one exists.
func (m *Manager) GetField(name string) (Field, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	field, ok := m.fields[name]
	return field, ok
}

var _ Registerer = (*Manager)(nil)

// Registerer provides an interface for registering config fields which can be
// dynamically updated while the node is running.
type Registerer interface {
	// RegisterBoolConfig registers a new bool config.
	// Returns ErrAlreadyRegistered if a config is already registered with name.
	RegisterBoolConfig(name string, get GetBoolConfigFunc, set SetBoolConfigFunc) error
	// RegisterUintConfig registers a new uint config.
	// Returns ErrAlreadyRegistered if a config is already registered with name.
	RegisterUintConfig(name string, get GetUintConfigFunc, set SetUintConfigFunc) error
	// RegisterDurationConfig registers a new duration config.
	// Returns ErrAlreadyRegistered if a config is already registered with name.
	RegisterDurationConfig(name string, get GetDurationConfigFunc, set SetDurationConfigFunc) error
}

func (m *Manager) RegisterBoolConfig(name string, get GetBoolConfigFunc, set SetBoolConfigFunc) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.fields[name]; exists {
		return fmt.Errorf("can't register config %s: %w", name, ErrAlreadyRegistered)
	}

	field := Field{
		Name: name,
		Get: func() any {
			return get()
		},
		Set: func(val any) error {
			bval, ok := val.(bool)
			if !ok {
				return fmt.Errorf("invalid type for bool config: %T", val)
			}
			return set(bval)
		},
	}
	m.fields[field.Name] = field
	return nil
}

func (m *Manager) RegisterUintConfig(name string, get GetUintConfigFunc, set SetUintConfigFunc) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.fields[name]; exists {
		return fmt.Errorf("can't register config %s: %w", name, ErrAlreadyRegistered)
	}

	field := Field{
		Name: name,
		Get: func() any {
			return get()
		},
		Set: func(val any) error {
			fval, ok := val.(float64) // JSON numbers always parse to float64
			if !ok {
				return fmt.Errorf("invalid type for bool config: %T", val)
			}
			return set(uint(fval))
		},
	}
	m.fields[field.Name] = field
	return nil
}

func (m *Manager) RegisterDurationConfig(name string, get GetDurationConfigFunc, set SetDurationConfigFunc) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.fields[name]; exists {
		return fmt.Errorf("can't register config %s: %w", name, ErrAlreadyRegistered)
	}

	field := Field{
		Name: name,
		Get: func() any {
			return get()
		},
		Set: func(val any) error {
			sval, ok := val.(string)
			if !ok {
				return fmt.Errorf("invalid type for duration config: %T", val)
			}
			dval, err := time.ParseDuration(sval)
			if err != nil {
				return fmt.Errorf("unparseable duration: %s: %w", sval, err)
			}
			return set(dval)
		},
	}
	m.fields[field.Name] = field
	return nil
}
