package ast

import "github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"

// StructureDeclaration

type StructureDeclaration struct {
	Identifier    string
	Fields        []*FieldDeclaration
	Initializer   *InitializerDeclaration
	Functions     []*FunctionDeclaration
	IdentifierPos *Position
	StartPos      *Position
	EndPos        *Position
}

func (s *StructureDeclaration) StartPosition() *Position {
	return s.StartPos
}

func (s *StructureDeclaration) EndPosition() *Position {
	return s.EndPos
}

func (s *StructureDeclaration) IdentifierPosition() *Position {
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

// FieldDeclaration

type FieldDeclaration struct {
	Access        Access
	IsConstant    bool
	Identifier    string
	Type          Type
	StartPos      *Position
	EndPos        *Position
	IdentifierPos *Position
}

func (f *FieldDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitFieldDeclaration(f)
}

func (f *FieldDeclaration) StartPosition() *Position {
	return f.StartPos
}

func (f *FieldDeclaration) EndPosition() *Position {
	return f.EndPos
}

func (f *FieldDeclaration) IdentifierPosition() *Position {
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
	Identifier string
	Parameters []*Parameter
	Block      *Block
	StartPos   *Position
	EndPos     *Position
}

func (i *InitializerDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitInitializerDeclaration(i)
}

func (i *InitializerDeclaration) StartPosition() *Position {
	return i.StartPos
}

func (i *InitializerDeclaration) EndPosition() *Position {
	return i.EndPos
}

func (i *InitializerDeclaration) IdentifierPosition() *Position {
	return nil
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
		Parameters: i.Parameters,
		Block:      i.Block,
		StartPos:   i.StartPos,
	}
}
