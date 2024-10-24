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
	"errors"
	"testing"

	"github.com/onflow/cadence"
	"github.com/stretchr/testify/assert"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/test"

	emulator "github.com/onflow/flow-go/integration/internal/emulator"
	"github.com/onflow/flow-go/model/flow"
)

func TestResult(t *testing.T) {

	t.Parallel()

	t.Run("should return correct boolean", func(t *testing.T) {

		t.Parallel()

		idGenerator := test.IdentifierGenerator()

		trSucceed := &emulator.TransactionResult{
			TransactionID:   idGenerator.New(),
			ComputationUsed: 20,
			MemoryEstimate:  2048,
			Error:           nil,
			Logs:            []string{},
			Events:          []flowsdk.Event{},
		}
		assert.True(t, trSucceed.Succeeded())
		assert.False(t, trSucceed.Reverted())

		trReverted := &emulator.TransactionResult{
			TransactionID:   idGenerator.New(),
			ComputationUsed: 20,
			MemoryEstimate:  2048,
			Error:           errors.New("transaction execution error"),
			Logs:            []string{},
			Events:          []flowsdk.Event{},
		}
		assert.True(t, trReverted.Reverted())
		assert.False(t, trReverted.Succeeded())

		srSucceed := &emulator.ScriptResult{
			ScriptID: emulator.SDKIdentifierToFlow(idGenerator.New()),
			Value:    cadence.Value(cadence.NewInt(1)),
			Error:    nil,
			Logs:     []string{},
			Events:   []flow.Event{},
		}
		assert.True(t, srSucceed.Succeeded())
		assert.False(t, srSucceed.Reverted())

		srReverted := &emulator.ScriptResult{
			ScriptID: emulator.SDKIdentifierToFlow(idGenerator.New()),
			Value:    cadence.Value(cadence.NewInt(1)),
			Error:    errors.New("transaction execution error"),
			Logs:     []string{},
			Events:   []flow.Event{},
		}
		assert.True(t, srReverted.Reverted())
		assert.False(t, srReverted.Succeeded())
	})
}
