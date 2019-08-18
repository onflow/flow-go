package ast

import "github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"

// StructureDeclaration

type StructureDeclaration struct {
	Identifier    string
	Conformances  []*Conformance
	Fields        []*FieldDeclaration
	Initializer   *InitializerDeclaration
	Functions     []*FunctionDeclaration
	IdentifierPos Position
	StartPos      Position
	EndPos        Position
}

func (s *StructureDeclaration) StartPosition() Position {
	return s.StartPos
}

func (s *StructureDeclaration) EndPosition() Position {
	return s.EndPos
}

func (s *StructureDeclaration) IdentifierPosition() Position {
	return s.IdentifierPos
}

func (s *StructureDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitStructureDeclaration(s)
}

func (*StructureDeclaration) isDeclaration() {}

// NOTE: statement, so it can be represented in the AST,
// but will be rejected in semantic analysis
//
func (*StructureDeclaration) isStatement() {}

func (s *StructureDeclaration) DeclarationName() string {
	return s.Identifier
}

func (s *StructureDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindStructure
}

// Conformance

type Conformance struct {
	Identifier string
	Pos        Position
}

func (c *Conformance) Accept(visitor Visitor) Repr {
	return visitor.VisitConformance(c)
}

func (c *Conformance) StartPosition() Position {
	return c.Pos
}

func (c *Conformance) EndPosition() Position {
	return c.Pos.Shifted(len(c.Identifier) - 1)
}

// FieldDeclaration

type FieldDeclaration struct {
	Access        Access
	VariableKind  VariableKind
	Identifier    string
	Type          Type
	StartPos      Position
	EndPos        Position
	IdentifierPos Position
}

func (f *FieldDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitFieldDeclaration(f)
}

func (f *FieldDeclaration) StartPosition() Position {
	return f.StartPos
}

func (f *FieldDeclaration) EndPosition() Position {
	return f.EndPos
}

func (f *FieldDeclaration) IdentifierPosition() Position {
	return f.IdentifierPos
}

func (*FieldDeclaration) isDeclaration() {}

func (f *FieldDeclaration) DeclarationName() string {
	return f.Identifier
}

func (f *FieldDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindField
}

// InitializerDeclaration

type InitializerDeclaration struct {
	Identifier    string
	Parameters    []*Parameter
	FunctionBlock *FunctionBlock
	StartPos      Position
}

func (i *InitializerDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitInitializerDeclaration(i)
}

func (i *InitializerDeclaration) StartPosition() Position {
	return i.StartPos
}

func (i *InitializerDeclaration) EndPosition() Position {
	return i.FunctionBlock.EndPos
}

func (i *InitializerDeclaration) IdentifierPosition() Position {
	return i.StartPos
}

func (*InitializerDeclaration) isDeclaration() {}

func (i *InitializerDeclaration) DeclarationName() string {
	return "init"
}

func (i *InitializerDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindInitializer
}

func (i *InitializerDeclaration) ToFunctionExpression() *FunctionExpression {
	return &FunctionExpression{
		Parameters:    i.Parameters,
		FunctionBlock: i.FunctionBlock,
		StartPos:      i.StartPos,
	}
}
