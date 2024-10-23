/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tests

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/integration/emulator"
	"github.com/onflow/flow-go/model/flow"
	flowgo "github.com/onflow/flow-go/model/flow"
)

func TestMemstore(t *testing.T) {

	t.Parallel()

	const blockHeight = 0
	key := flow.NewRegisterID(flowgo.EmptyAddress, "foo")
	value := []byte("bar")
	store := emulator.NewMemoryStore()

	err := store.InsertExecutionSnapshot(
		blockHeight,
		&snapshot.ExecutionSnapshot{
			WriteSet: map[flowgo.RegisterID]flowgo.RegisterValue{
				key: value,
			},
		},
	)
	require.NoError(t, err)

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			snapshot, err := store.LedgerByHeight(
				context.Background(),
				blockHeight)
			require.NoError(t, err)
			actualValue, err := snapshot.Get(key)

			require.NoError(t, err)
			assert.Equal(t, value, actualValue)
		}()
	}

	wg.Wait()
}

func TestMemstoreSetValueToNil(t *testing.T) {

	t.Parallel()

	store := emulator.NewMemoryStore()
	key := flow.NewRegisterID(flowgo.EmptyAddress, "foo")
	value := []byte("bar")
	var nilByte []byte
	nilValue := nilByte

	// set initial value
	err := store.InsertExecutionSnapshot(
		0,
		&snapshot.ExecutionSnapshot{
			WriteSet: map[flowgo.RegisterID]flowgo.RegisterValue{
				key: value,
			},
		})
	require.NoError(t, err)

	// check initial value
	ledger, err := store.LedgerByHeight(context.Background(), 0)
	require.NoError(t, err)
	register, err := ledger.Get(key)
	require.NoError(t, err)
	require.Equal(t, string(value), string(register))

	// set value to nil
	err = store.InsertExecutionSnapshot(
		1,
		&snapshot.ExecutionSnapshot{
			WriteSet: map[flowgo.RegisterID]flowgo.RegisterValue{
				key: nilValue,
			},
		})
	require.NoError(t, err)

	// check value is nil
	ledger, err = store.LedgerByHeight(context.Background(), 1)
	require.NoError(t, err)
	register, err = ledger.Get(key)
	require.NoError(t, err)
	require.Equal(t, string(nilValue), string(register))
}
