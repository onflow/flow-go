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

	"github.com/onflow/flow-go/integration/internal/emulator"
	flowgo "github.com/onflow/flow-go/model/flow"
)

// NewTransactionBodyFixture generates a complete and random flowgo.TransactionBody object.
func NewTransactionBodyFixture() flowgo.TransactionBody {
	sdkTx := test.TransactionGenerator().New()
	flowTx, err := emulator.SDKTransactionToFlow(*sdkTx)
	if err != nil {
		panic(err)
	}
	return *flowTx
}

// NewStorableTransactionResultFixture generates a complete and random emulator.StorableTransactionResult object.
func NewStorableTransactionResultFixture(eventEncodingVersion entities.EventEncodingVersion) emulator.StorableTransactionResult {
	events := test.EventGenerator(eventEncodingVersion)

	sdkEventA := events.New()
	eventA, err := emulator.SDKEventToFlow(sdkEventA)
	if err != nil {
		panic(err)
	}

	sdkEventB := events.New()
	eventB, err := emulator.SDKEventToFlow(sdkEventB)
	if err != nil {
		panic(err)
	}

	return emulator.StorableTransactionResult{
		ErrorCode:    uint32(test.RandomNumber(0, 100)),
		ErrorMessage: test.RandomString(20),
		Logs:         []string{test.RandomString(10), test.RandomString(15), test.RandomString(12)},
		Events: []flowgo.Event{
			*eventA,
			*eventB,
		},
	}
}

// NewBlockHeaderFixture generates a complete and random flowgo.BlockHeader object.
func NewBlockHeaderFixture() flowgo.BlockHeader {
	sdkHeader := test.BlockGenerator().NewHeader()
	flowHeader, err := emulator.SDKBlockHeaderToFlow(sdkHeader)
	if err != nil {
		panic(err)
	}
	return *flowHeader
}

// NewBlockFixture generates a complete and random flowgo.Block object.
func NewBlockFixture() flowgo.Block {
	sdkBlock := test.BlockGenerator().New()
	flowBlock, err := emulator.SDKBlockToFlow(sdkBlock)
	if err != nil {
		panic(err)
	}
	return *flowBlock
}

// NewCollectionFixture generates a flowgo.Collection with n random transactions.
func NewCollectionFixture(n int) flowgo.Collection {
	transactions := make([]*flowgo.TransactionBody, n)
	for i := 0; i < n; i++ {
		tx := NewTransactionBodyFixture()
		transactions[i] = &tx
	}

	return flowgo.Collection{
		Transactions: transactions,
	}
}