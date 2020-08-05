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

package fuzz

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/cadence"
)

const crashersDir = "../../../crashers"

func TestCrashers(t *testing.T) {

	t.Parallel()

	f, err := os.Open(crashersDir)
	if err != nil {
		t.Skip()
	}

	files, err := f.Readdir(-1)
	_ = f.Close()

	for _, file := range files {

		name := file.Name()
		if path.Ext(name) != "" {
			continue
		}

		t.Run(name, func(t *testing.T) {

			var data []byte
			data, err = ioutil.ReadFile(path.Join(crashersDir, name))
			if err != nil {
				t.Fatal(err)
			}

			assert.NotPanics(t,
				func() {
					cadence.Fuzz(data)
				},
				string(data),
			)
		})

	}
}
