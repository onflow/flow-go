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

package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/cadence/runtime/sema"
)

func TestCheckAnyStruct(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      let a: AnyStruct = 1
      let b: AnyStruct = true
    `)

	assert.NoError(t, err)
}

func TestCheckInvalidAnyStructResourceType(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource R {}

      let a: AnyStruct = <-create R()
      let b: AnyStruct = [<-create R()]
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
}

func TestCheckAnyResource(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource R {}

      let a: @AnyResource <- create R()
      let b: @AnyResource <- [<-create R()]
    `)

	assert.NoError(t, err)
}

func TestCheckInvalidAnyResourceNonResourceType(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource R {}

      let a: AnyStruct <- create R()
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[1])
}
