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

package intervalst

import "fmt"

type Position interface {
	Compare(other Position) int
}

type Interval struct {
	Min, Max Position
}

func NewInterval(min, max Position) Interval {
	if min.Compare(max) > 0 {
		panic("illegal interval: min > max")
	}
	return Interval{min, max}
}

func (i Interval) Intersects(other Interval) bool {
	return !(other.Max.Compare(i.Min) == -1 ||
		i.Max.Compare(other.Min) == -1)
}

func (i Interval) Contains(x Position) bool {
	return i.Min.Compare(x) <= 0 &&
		x.Compare(i.Max) <= 0
}

func (i Interval) Compare(other Interval) int {
	mins := i.Min.Compare(other.Min)
	maxs := i.Max.Compare(other.Max)
	if mins < 0 {
		return -1
	} else if mins > 0 {
		return 1
	} else if maxs < 0 {
		return -1
	} else if maxs > 0 {
		return 1
	} else {
		return 0
	}
}

func (i Interval) String() string {
	return fmt.Sprintf("[%s, %s]", i.Min, i.Max)
}
