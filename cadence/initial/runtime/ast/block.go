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

package ast

type Block struct {
	Statements []Statement
	Range
}

func (b *Block) Accept(visitor Visitor) Repr {
	return visitor.VisitBlock(b)
}

// FunctionBlock

type FunctionBlock struct {
	*Block
	PreConditions  *Conditions
	PostConditions *Conditions
}

func (b *FunctionBlock) Accept(visitor Visitor) Repr {
	return visitor.VisitFunctionBlock(b)
}

// Condition

type Condition struct {
	Kind    ConditionKind
	Test    Expression
	Message Expression
}

func (c *Condition) Accept(visitor Visitor) Repr {
	return visitor.VisitCondition(c)
}

type Conditions []*Condition

func (c *Conditions) Append(conditions Conditions) {
	if c == nil {
		return
	}
	*c = append(*c, conditions...)
}
