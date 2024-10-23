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

package unittest

import (
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go-sdk/test"

	"github.com/onflow/flow-go/integration/emulator"
	flowgo "github.com/onflow/flow-go/model/flow"
)

func TransactionFixture() flowgo.TransactionBody {
	return *emulator.SDKTransactionToFlow(*test.TransactionGenerator().New())
}

func StorableTransactionResultFixture(eventEncodingVersion entities.EventEncodingVersion) emulator.StorableTransactionResult {
	events := test.EventGenerator(eventEncodingVersion)

	eventA, _ := emulator.SDKEventToFlow(events.New())
	eventB, _ := emulator.SDKEventToFlow(events.New())

	return emulator.StorableTransactionResult{
		ErrorCode:    42,
		ErrorMessage: "foo",
		Logs:         []string{"a", "b", "c"},
		Events: []flowgo.Event{
			eventA,
			eventB,
		},
	}
}

func FullCollectionFixture(n int) flowgo.Collection {
	transactions := make([]*flowgo.TransactionBody, n)
	for i := 0; i < n; i++ {
		tx := TransactionFixture()
		transactions[i] = &tx
	}

	return flowgo.Collection{
		Transactions: transactions,
	}
}
