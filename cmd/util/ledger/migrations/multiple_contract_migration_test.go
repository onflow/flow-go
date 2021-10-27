package migrations_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
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
			t.Run(fmt.Sprintf("%d. key %s", i, k.String()), func(t *testing.T) {
				_, err := migrations.MultipleContractMigration([]ledger.Payload{{
					Key:   k,
					Value: nil,
				}})
				require.Error(t, err)
			})
		}
	})

	t.Run("Non code registers are not migrated", func(t *testing.T) {
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

		migrated, err := migrations.MultipleContractMigration([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 1)
		require.Equal(t, payload, migrated[0])
	})

	t.Run("Non address registers are not migrated", func(t *testing.T) {
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

		migrated, err := migrations.MultipleContractMigration([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 1)
		require.Equal(t, payload, migrated[0])
	})

	t.Run("Empty code registers are removed", func(t *testing.T) {
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

		migrated, err := migrations.MultipleContractMigration([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 0)
	})

	t.Run("If code cannot be parsed return error", func(t *testing.T) {

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

		_, err := migrations.MultipleContractMigration([]ledger.Payload{payload})
		require.Error(t, err)
	})

	t.Run("If there are multiple errors they are aggregated", func(t *testing.T) {

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

		_, err := migrations.MultipleContractMigration([]ledger.Payload{payload1, payload2})
		require.Error(t, err)
		require.Len(t, err.(*migrations.MultipleContractMigrationError).Errors, 2)
	})

	t.Run("If code has only one contract, migrate it to the new key", func(t *testing.T) {

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

		migrated, err := migrations.MultipleContractMigration([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 2)
		require.Equal(t, "code.Test", string(migrated[0].Key.KeyParts[2].Value))
		require.Equal(t, contract, string(migrated[0].Value))
	})

	// this case was not allowed before, so it shouldn't really happen
	t.Run("If code has only one interface, migrate it to the new key", func(t *testing.T) {
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

		migrated, err := migrations.MultipleContractMigration([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 2)
		require.Equal(t, "code.Test", string(migrated[0].Key.KeyParts[2].Value))
		require.Equal(t, contract, string(migrated[0].Value))
	})

	// this case was not allowed before, so it shouldn't really happen
	t.Run("If code has more than two declarations, return error", func(t *testing.T) {
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

		_, err := migrations.MultipleContractMigration([]ledger.Payload{payload})
		require.Error(t, err)
	})

	t.Run("If code has no declarations, return error", func(t *testing.T) {
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

		_, err := migrations.MultipleContractMigration([]ledger.Payload{payload})
		require.Error(t, err)
	})

	t.Run("If code has two declarations, correctly split them (interface first)", func(t *testing.T) {
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

		migrated, err := migrations.MultipleContractMigration([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 3)
		require.Equal(t, "code.ITest", string(migrated[0].Key.KeyParts[2].Value))
		require.Equal(t, expectedInterfaceCode, string(migrated[0].Value))
		require.Equal(t, "code.Test", string(migrated[1].Key.KeyParts[2].Value))
		require.Equal(t, expectedContractCode, string(migrated[1].Value))
	})
	t.Run("If code has two declarations, correctly split them (ugly formatting)", func(t *testing.T) {
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

		migrated, err := migrations.MultipleContractMigration([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 3)

		require.Equal(t, "code.ITest", string(migrated[0].Key.KeyParts[2].Value))
		require.Equal(t, expectedInterfaceCode, string(migrated[0].Value))

		require.Equal(t, "code.Test", string(migrated[1].Key.KeyParts[2].Value))
		require.Equal(t, expectedContractCode, string(migrated[1].Value))
	})

	t.Run("If code has two declarations, correctly split them (interface last)", func(t *testing.T) {
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

		migrated, err := migrations.MultipleContractMigration([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 3)
		require.Equal(t, "code.ITest", string(migrated[0].Key.KeyParts[2].Value))
		require.Equal(t, expectedInterfaceCode, string(migrated[0].Value))

		require.Equal(t, "code.Test", string(migrated[1].Key.KeyParts[2].Value))
		require.Equal(t, expectedContractCode, string(migrated[1].Value))
	})

	t.Run("If code has two declarations, correctly split them (imports)", func(t *testing.T) {
		address := flow.HexToAddress("01")

		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, address.Bytes()),
				ledger.NewKeyPart(state.KeyPartController, []byte("")),
				ledger.NewKeyPart(state.KeyPartKey, []byte("code")),
			},
		}
		contract := `
			import NonFungibleToken from 0x631e88ae7f1d7c20
			import NonFungibleToken from 0x631e88ae7f1d7c20
			
			// some comments about Test
			pub contract Test {
				// some code 
			}

			import NonFungibleToken from 0x631e88ae7f1d7c20

			// some comments about ITest
			pub contract interface ITest {
				// some code 
			}


			import NonFungibleToken from 0x631e88ae7f1d7c20
			// some unrelated comments
		`
		payload := ledger.Payload{
			Key:   key,
			Value: []byte(contract),
		}

		expectedInterfaceCode := `import NonFungibleToken from 0x631e88ae7f1d7c20
import NonFungibleToken from 0x631e88ae7f1d7c20
import NonFungibleToken from 0x631e88ae7f1d7c20
import NonFungibleToken from 0x631e88ae7f1d7c20


			

			// some comments about ITest
			pub contract interface ITest {
				// some code 
			}


			
			// some unrelated comments
		`

		expectedContractCode := fmt.Sprintf(`import NonFungibleToken from 0x631e88ae7f1d7c20
import NonFungibleToken from 0x631e88ae7f1d7c20
import NonFungibleToken from 0x631e88ae7f1d7c20
import NonFungibleToken from 0x631e88ae7f1d7c20
import ITest from %s

			
			
			
			// some comments about Test
			pub contract Test {
				// some code 
			}`, address.HexWithPrefix())

		migrated, err := migrations.MultipleContractMigration([]ledger.Payload{payload})
		require.NoError(t, err)
		require.Len(t, migrated, 3)

		require.Equal(t, "code.Test", string(migrated[1].Key.KeyParts[2].Value))
		require.Equal(t, expectedContractCode, string(migrated[1].Value))

		require.Equal(t, "code.ITest", string(migrated[0].Key.KeyParts[2].Value))
		require.Equal(t, expectedInterfaceCode, string(migrated[0].Value))
	})

	t.Run("If code has two declarations of the same type return error", func(t *testing.T) {
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

		_, err := migrations.MultipleContractMigration([]ledger.Payload{payload})
		require.Error(t, err)
	})

	t.Run("If exceptional migration for owner exists, use it", func(t *testing.T) {
		address := flow.HexToAddress("01")
		called := false

		migrations.MultipleContractsSpecialMigrations[string(address.Bytes())] = func(payload ledger.Payload) ([]ledger.Payload, error) {
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

		_, err := migrations.MultipleContractMigration([]ledger.Payload{payload})
		require.NoError(t, err)
		require.True(t, called)
	})
	t.Run("If exceptional migration for a different owner exists, dont use it", func(t *testing.T) {
		address := flow.HexToAddress("01")
		called := false

		migrations.MultipleContractsSpecialMigrations[string(address.Bytes())] = func(payload ledger.Payload) ([]ledger.Payload, error) {
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

		_, err := migrations.MultipleContractMigration([]ledger.Payload{payload})
		require.NoError(t, err)
		require.False(t, called)
	})

	t.Run("contract object", func(t *testing.T) {
		if os.Getenv("TEST_CADENCE_STORAGE_FORMAT") != "v4" {
			t.Skip("this test assumes Cadence storage format pre-v4")
		}

		payload1 := ledger.Payload{
			Key: ledger.Key{
				KeyParts: []ledger.KeyPart{
					ledger.NewKeyPart(state.KeyPartOwner, flow.HexToAddress("01").Bytes()),
					ledger.NewKeyPart(state.KeyPartController, []byte("")),
					ledger.NewKeyPart(state.KeyPartKey, []byte("contract")),
				},
			},
			Value: []byte{
				// tag
				0xd8, 0x84,
				// map, 4 pairs of items follow
				0xa4,
				// key 0
				0x0,
				// tag
				0xd8, 0xC0,
				// byte sequence, length 1
				0x41,
				// positive integer 1
				0x1,
				// key 1
				0x1,
				// UTF-8 string, length 20
				0x74,
				// A.0000000000000001.C
				0x41,
				0x2E, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x31,
				0x2E, 0x43,
				// key 2
				0x2,
				// positive integer 1
				0x3,
				// key 3
				0x3,
				// map, 0 pairs of items follow
				0xa0,
			},
		}

		payload2 := ledger.Payload{
			Key: ledger.Key{
				KeyParts: []ledger.KeyPart{
					ledger.NewKeyPart(state.KeyPartOwner, flow.HexToAddress("01").Bytes()),
					ledger.NewKeyPart(state.KeyPartController, []byte("")),
					ledger.NewKeyPart(state.KeyPartKey, []byte("contract\x1Fnodes\x1Fv\x1F1")),
				},
			},
			Value: []byte{
				// tag
				0xd8, 0x84,
				// map, 4 pairs of items follow
				0xa4,
				// key 0
				0x0,
				// tag
				0xd8, 0xC0,
				// byte sequence, length 1
				0x41,
				// positive integer 1
				0x1,
				// key 1
				0x1,
				// UTF-8 string, length 20
				0x74,
				// A.0000000000000001.S
				0x41,
				0x2E, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x31,
				0x2E, 0x53,
				// key 2
				0x2,
				// positive integer 1
				0x1,
				// key 3
				0x3,
				// map, 0 pairs of items follow
				0xa0,
			},
		}

		migrated, err := migrations.MultipleContractMigration([]ledger.Payload{
			payload1,
			payload2,
		})
		require.NoError(t, err)
		require.Len(t, migrated, 2)

		migrated1 := migrated[0]
		require.Equal(t, "contract\x1fC", string(migrated1.Key.KeyParts[2].Value))
		require.Equal(t, payload1.Value, migrated1.Value)

		migrated2 := migrated[1]
		require.Equal(t, "contract\x1fC\x1Fnodes\x1Fv\x1F1", string(migrated2.Key.KeyParts[2].Value))
		require.Equal(t, payload2.Value, migrated2.Value)
	})
}
