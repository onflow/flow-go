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

package common

import (
	"fmt"
	"strings"
)

func EnumerateWords(words []string, conjunction string) string {
	count := len(words)
	switch count {
	case 0:
		return ""

	case 1:
		return words[0]

	case 2:
		return fmt.Sprintf("%s %s %s", words[0], conjunction, words[1])

	default:
		lastIndex := count - 1
		commaSeparatedExceptLastWord := strings.Join(words[:lastIndex], ", ")
		lastWord := words[lastIndex]
		return fmt.Sprintf("%s, %s %s", commaSeparatedExceptLastWord, conjunction, lastWord)
	}
}
