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

package emulator

import (
	"time"
)

type BlocksTicker struct {
	emulator Emulator
	ticker   *time.Ticker
	done     chan bool
}

func NewBlocksTicker(
	emulator Emulator,
	blockTime time.Duration,
) *BlocksTicker {
	return &BlocksTicker{
		emulator: emulator,
		ticker:   time.NewTicker(blockTime),
		done:     make(chan bool, 1),
	}
}

func (t *BlocksTicker) Start() error {
	for {
		select {
		case <-t.ticker.C:
			_, _ = t.emulator.ExecuteBlock()
			_, _ = t.emulator.CommitBlock()
		case <-t.done:
			return nil
		}
	}
}

func (t *BlocksTicker) Stop() {
	t.done <- true
}
