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

package tests_test

import (
	"testing"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go-sdk/test"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/integration/emulator"
	flowgo "github.com/onflow/flow-go/model/flow"
)

func TestVm(t *testing.T) {

	t.Parallel()

	test := func(eventEncodingVersion entities.EventEncodingVersion) {
		t.Run(eventEncodingVersion.String(), func(t *testing.T) {
			t.Parallel()
			t.Run("should be able to convert", func(t *testing.T) {

				t.Parallel()

				idGenerator := test.IdentifierGenerator()

				eventGenerator := test.EventGenerator(eventEncodingVersion)
				event1, err := emulator.SDKEventToFlow(eventGenerator.New())
				assert.NoError(t, err)

				event2, err := emulator.SDKEventToFlow(eventGenerator.New())
				assert.NoError(t, err)

				txnId := flowgo.Identifier(idGenerator.New())
				output := fvm.ProcedureOutput{
					Logs:            []string{"TestLog1", "TestLog2"},
					Events:          []flowgo.Event{event1, event2},
					ComputationUsed: 5,
					MemoryEstimate:  1211,
					Err:             nil,
				}

				tr, err := emulator.VMTransactionResultToEmulator(txnId, output)
				assert.NoError(t, err)

				assert.Equal(t, txnId, flowgo.Identifier(tr.TransactionID))
				assert.Equal(t, output.Logs, tr.Logs)

				flowEvents, err := emulator.FlowEventsToSDK(output.Events)
				assert.NoError(t, err)
				assert.Equal(t, flowEvents, tr.Events)

				assert.Equal(t, output.ComputationUsed, tr.ComputationUsed)
				assert.Equal(t, output.MemoryEstimate, tr.MemoryEstimate)
				assert.Equal(t, output.Err, tr.Error)
			})
		})
	}

	test(entities.EventEncodingVersion_JSON_CDC_V0)
	test(entities.EventEncodingVersion_CCF_V0)
}
