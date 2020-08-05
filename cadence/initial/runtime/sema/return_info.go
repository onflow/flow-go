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

package sema

type ReturnInfo struct {
	MaybeReturned      bool
	DefinitelyReturned bool
	DefinitelyHalted   bool
}

func (ri *ReturnInfo) MergeBranches(thenReturnInfo *ReturnInfo, elseReturnInfo *ReturnInfo) {
	ri.MaybeReturned = ri.MaybeReturned ||
		thenReturnInfo.MaybeReturned ||
		elseReturnInfo.MaybeReturned

	ri.DefinitelyReturned = ri.DefinitelyReturned ||
		(thenReturnInfo.DefinitelyReturned &&
			elseReturnInfo.DefinitelyReturned)

	ri.DefinitelyHalted = ri.DefinitelyHalted ||
		(thenReturnInfo.DefinitelyHalted &&
			elseReturnInfo.DefinitelyHalted)
}

func (ri *ReturnInfo) Clone() *ReturnInfo {
	result := &ReturnInfo{}
	*result = *ri
	return result
}
