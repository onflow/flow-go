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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/integration/internal/emulator"
)

func TestRuntimeLogs(t *testing.T) {

	t.Parallel()

	b, err := emulator.New()
	require.NoError(t, err)

	script := []byte(`
		access(all) fun main() {
			log("elephant ears")
		}
	`)

	result, err := b.ExecuteScript(script, nil)
	assert.NoError(t, err)
	assert.Equal(t, []string{`"elephant ears"`}, result.Logs)
}
