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

package execute

import (
	"example.com/cadence-initial/runtime/cmd"
)

// Execute parses the given filename and prints any syntax errors.
// If there are no syntax errors, the program is interpreted.
// If after the interpretation a global function `main` is defined, it will be called.
// The program may call the function `log` to print a value.
func Execute(args []string) {

	if len(args) < 1 {
		cmd.ExitWithError("no input file")
	}

	inter, _, must := cmd.PrepareInterpreter(args[0])

	if _, hasMain := inter.Globals["main"]; !hasMain {
		return
	}

	_, err := inter.Invoke("main")
	must(err)
}
