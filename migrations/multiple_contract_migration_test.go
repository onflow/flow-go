package migrations_test

import (
	"fmt"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/migrations"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMultipleContractMigration(t *testing.T) {

	t.Run("Failing to parse register key returns error", func(t *testing.T) {

		keys := []ledger.Key{
			{},
			{
				KeyParts: []ledger.KeyPart{
					ledger.NewKeyPart(state.KeyPartOwner, []byte("")),
					ledger.NewKeyPart(state.KeyPartController, []byte("")),
					ledger.NewKeyPart(state.KeyPartKey, []byte("")),
					ledger.NewKeyPart(state.KeyPartKey, []byte("")),
				},
			},
			{
				KeyParts: []ledger.KeyPart{
					ledger.NewKeyPart(state.KeyPartOwner, []byte("")),
					ledger.NewKeyPart(state.KeyPartController, []byte("")),
				},
			},
			{
				KeyParts: []ledger.KeyPart{
					ledger.NewKeyPart(state.KeyPartOwner, []byte("")),
					ledger.NewKeyPart(state.KeyPartController, []byte("")),
					ledger.NewKeyPart(state.KeyPartOwner, []byte("")),
				},
			},
		}
		for i, k := range keys {
			m := migrations.NewMultipleContractMigration(zerolog.Logger{})
			t.Run(fmt.Sprintf("%d. key %s", i, k.String()), func(t *testing.T) {
				_, err := m.Migrate([]ledger.Payload{{
					Key:   k,
					Value: nil,
				}})
				require.Error(t, err)
			})
		}
	})

	t.Run("Non code registers are not migrated", func(t *testing.T) {
		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, []byte("")),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("not code")),
			},
		}
		payload := ledger.Payload{
			Key:   key,
			Value: []byte("some value 1234"),
		}

		migrated, err := m.Migrate([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 1)
		require.Equal(t, migrated[0], payload)
	})

	t.Run("Non address registers are not migrated", func(t *testing.T) {
		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, []byte("234")),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		payload := ledger.Payload{
			Key:   key,
			Value: []byte("some value 1234"),
		}

		migrated, err := m.Migrate([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 1)
		require.Equal(t, migrated[0], payload)
	})

	t.Run("Empty code registers are removed", func(t *testing.T) {
		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, flow.HexToAddress("01").Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		payload := ledger.Payload{
			Key:   key,
			Value: []byte{},
		}

		migrated, err := m.Migrate([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 0)
	})

	t.Run("Registers are migrated only once", func(t *testing.T) {
		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, flow.HexToAddress("01").Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		payload := ledger.Payload{
			Key:   key,
			Value: []byte{},
		}

		migrated, err := m.Migrate([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 0)

		payload = ledger.Payload{
			Key:   key,
			Value: []byte("123"),
		}

		migrated, err = m.Migrate([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 1)
		// because code was already migrated the migration is skipped just returning the register
		// this is a way to test the cache works
		// in real scenarios there wouldn't be two code registers on one address
		require.Equal(t, migrated[0], payload)
	})

	t.Run("Registers are migrated only once", func(t *testing.T) {
		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, flow.HexToAddress("01").Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		payload := ledger.Payload{
			Key:   key,
			Value: []byte{},
		}

		migrated, err := m.Migrate([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 0)

		payload = ledger.Payload{
			Key:   key,
			Value: []byte("123"),
		}

		migrated, err = m.Migrate([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 1)
		// because code was already migrated the migration is skipped just returning the register
		// this is a way to test the cache works
		// in real scenarios there wouldn't be two code registers on one address
		require.Equal(t, migrated[0], payload)
	})

	t.Run("If code cannot be parsed return error", func(t *testing.T) {

		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, flow.HexToAddress("01").Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		payload := ledger.Payload{
			Key:   key,
			Value: []byte("{!@#$%234985yhtgv3yr9b  dvomngifn ..."),
		}

		_, err := m.Migrate([]ledger.Payload{payload})
		require.Error(t, err)
	})

	t.Run("If there are multiple errors they are aggregated", func(t *testing.T) {

		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		key1 := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, flow.HexToAddress("01").Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		key2 := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, flow.HexToAddress("02").Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		payload1 := ledger.Payload{
			Key:   key1,
			Value: []byte("{!@#$%234985yhtgv3yr9b  dvomngifn ..."),
		}
		payload2 := ledger.Payload{
			Key:   key2,
			Value: []byte("{!@#$%234985yhtgv3yr9b  dvomngifn ..."),
		}

		_, err := m.Migrate([]ledger.Payload{payload1, payload2})
		require.Error(t, err)
		require.Len(t, err.(*migrations.MultipleContractMigrationError).Errors, 2)
	})

	t.Run("If code has only one contract, migrate it to the new key", func(t *testing.T) {

		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, flow.HexToAddress("01").Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		contract := `pub contract Test{}`
		payload := ledger.Payload{
			Key:   key,
			Value: []byte(contract),
		}

		migrated, err := m.Migrate([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 1)
		require.Equal(t, string(migrated[0].Key.KeyParts[2].Value), "code.Test")
		require.Equal(t, string(migrated[0].Value), contract)
	})

	// this case was not allowed before, so it shouldn't really happen
	t.Run("If code has only one interface, migrate it to the new key", func(t *testing.T) {
		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, flow.HexToAddress("01").Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		contract := `pub contract interface Test{}`
		payload := ledger.Payload{
			Key:   key,
			Value: []byte(contract),
		}

		migrated, err := m.Migrate([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 1)
		require.Equal(t, string(migrated[0].Key.KeyParts[2].Value), "code.Test")
		require.Equal(t, string(migrated[0].Value), contract)
	})

	// this case was not allowed before, so it shouldn't really happen
	t.Run("If code has more than two declarations, return error", func(t *testing.T) {
		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, flow.HexToAddress("01").Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		contract := `
			pub contract interface Test1{}
			pub contract interface Test2{}
			pub contract interface Test3{}
		`
		payload := ledger.Payload{
			Key:   key,
			Value: []byte(contract),
		}

		_, err := m.Migrate([]ledger.Payload{payload})
		require.Error(t, err)
	})

	t.Run("If code has no declarations, return error", func(t *testing.T) {
		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, flow.HexToAddress("01").Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		contract := `
			// pub contract interface Test1{}
		`
		payload := ledger.Payload{
			Key:   key,
			Value: []byte(contract),
		}

		_, err := m.Migrate([]ledger.Payload{payload})
		require.Error(t, err)
	})

	t.Run("If code has two declarations, correctly split them (interface first)", func(t *testing.T) {
		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		address := flow.HexToAddress("01")

		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, address.Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		contract := `
			// some comments about ITest
			pub contract interface ITest {
				// some code 
			}

			// some comments about Test
			pub contract Test {
				// some code 
			}

			// some unrelated comments
		`
		payload := ledger.Payload{
			Key:   key,
			Value: []byte(contract),
		}

		expectedInterfaceCode := `
			// some comments about ITest
			pub contract interface ITest {
				// some code 
			}`

		expectedContractCode := fmt.Sprintf(`import ITest from %s


			// some comments about Test
			pub contract Test {
				// some code 
			}

			// some unrelated comments
		`, address.HexWithPrefix())

		migrated, err := m.Migrate([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 2)
		require.Equal(t, string(migrated[0].Key.KeyParts[2].Value), "code.ITest")
		require.Equal(t, string(migrated[0].Value), expectedInterfaceCode)
		require.Equal(t, string(migrated[1].Key.KeyParts[2].Value), "code.Test")
		require.Equal(t, string(migrated[1].Value), expectedContractCode)
	})
	t.Run("If code has two declarations, correctly split them (ugly formatting)", func(t *testing.T) {
		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		address := flow.HexToAddress("01")

		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, address.Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		contract := `// some comments about ITest
			pub contract interface ITest {
				// some code 
			}// some comments about Test
			pub contract Test {
				// some code 
			}// some unrelated comments`
		payload := ledger.Payload{
			Key:   key,
			Value: []byte(contract),
		}

		expectedInterfaceCode := `// some comments about ITest
			pub contract interface ITest {
				// some code 
			}`

		expectedContractCode := fmt.Sprintf(`import ITest from %s
// some comments about Test
			pub contract Test {
				// some code 
			}// some unrelated comments`, address.HexWithPrefix())

		migrated, err := m.Migrate([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 2)
		require.Equal(t, string(migrated[0].Key.KeyParts[2].Value), "code.ITest")
		require.Equal(t, string(migrated[0].Value), expectedInterfaceCode)
		require.Equal(t, string(migrated[1].Key.KeyParts[2].Value), "code.Test")
		require.Equal(t, string(migrated[1].Value), expectedContractCode)
	})

	t.Run("If code has two declarations, correctly split them (interface last)", func(t *testing.T) {
		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		address := flow.HexToAddress("01")

		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, address.Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		contract := `
			// some comments about Test
			pub contract Test {
				// some code 
			}

			// some comments about ITest
			pub contract interface ITest {
				// some code 
			}

			// some unrelated comments
		`
		payload := ledger.Payload{
			Key:   key,
			Value: []byte(contract),
		}

		expectedInterfaceCode := `

			// some comments about ITest
			pub contract interface ITest {
				// some code 
			}

			// some unrelated comments
		`

		expectedContractCode := fmt.Sprintf(`import ITest from %s

			// some comments about Test
			pub contract Test {
				// some code 
			}`, address.HexWithPrefix())

		migrated, err := m.Migrate([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 2)
		require.Equal(t, string(migrated[0].Key.KeyParts[2].Value), "code.ITest")
		require.Equal(t, string(migrated[0].Value), expectedInterfaceCode)
		require.Equal(t, string(migrated[1].Key.KeyParts[2].Value), "code.Test")
		require.Equal(t, string(migrated[1].Value), expectedContractCode)
	})

	t.Run("If code has two declarations of the same type return error", func(t *testing.T) {
		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		address := flow.HexToAddress("01")

		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, address.Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		contract := `
			// some comments about Test
			pub contract Test {
				// some code 
			}

			// some comments about ITest
			pub contract Test2 {
				// some code 
			}

			// some unrelated comments
		`
		payload := ledger.Payload{
			Key:   key,
			Value: []byte(contract),
		}

		_, err := m.Migrate([]ledger.Payload{payload})
		require.Error(t, err)
	})

	t.Run("If exceptional migration for owner exists, use it", func(t *testing.T) {
		address := flow.HexToAddress("01")
		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		called := false

		m.ExceptionalMigrations[string(address.Bytes())] = func(payload ledger.Payload, logger zerolog.Logger) ([]ledger.Payload, error) {
			called = true
			return []ledger.Payload{payload}, nil
		}
		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, address.Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		payload := ledger.Payload{
			Key:   key,
			Value: []byte{},
		}

		_, err := m.Migrate([]ledger.Payload{payload})
		require.NoError(t, err)
		require.True(t, called)
	})
	t.Run("If exceptional migration for a different owner exists, dont use it", func(t *testing.T) {
		address := flow.HexToAddress("01")
		m := migrations.NewMultipleContractMigration(zerolog.Logger{})
		called := false

		m.ExceptionalMigrations[string(address.Bytes())] = func(payload ledger.Payload, logger zerolog.Logger) ([]ledger.Payload, error) {
			called = true
			return []ledger.Payload{payload}, nil
		}
		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, flow.HexToAddress("02").Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		payload := ledger.Payload{
			Key:   key,
			Value: []byte{},
		}

		_, err := m.Migrate([]ledger.Payload{payload})
		require.NoError(t, err)
		require.False(t, called)
	})
}
