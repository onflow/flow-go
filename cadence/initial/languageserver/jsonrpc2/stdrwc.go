/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
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

package jsonrpc2

import "os"

// stdrwc implements an io.ReadWriter and io.Closer around STDIN and STDOUT.
type stdrwc struct{}

// Read reads from STDIN.
func (stdrwc) Read(p []byte) (int, error) {
	return os.Stdin.Read(p)
}

// Write writes to STDOUT.
func (stdrwc) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}

// Close closes STDIN and STDOUT.
func (stdrwc) Close() error {
	if err := os.Stdin.Close(); err != nil {
		return err
	}
	return os.Stdout.Close()
}
