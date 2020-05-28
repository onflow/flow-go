package virtualmachine

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru"
	"github.com/onflow/cadence/runtime/ast"
)

// ASTCache is an interface to a cache for parsed program ASTs.
type ASTCache interface {
	GetProgram(ast.Location) (*ast.Program, error)
	SetProgram(ast.Location, *ast.Program) error
}

// LRUASTCache implements a program AST cache with a LRU cache.
type LRUASTCache struct {
	lru *lru.Cache
}

// NewLRUASTCache creates a new LRU cache that implements the ASTCache interface.
func NewLRUASTCache(size int) (*LRUASTCache, error) {
	lru, err := lru.New(size)
	if err != nil {
		return nil, fmt.Errorf("failed to create lru cache, %w", err)
	}
	return &LRUASTCache{
		lru: lru,
	}, nil
}

// GetProgram retrieves a program AST from the LRU cache.
func (cache *LRUASTCache) GetProgram(location ast.Location) (*ast.Program, error) {
	program, found := cache.lru.Get(location.ID())
	if found {
		cachedProgram, ok := program.(*ast.Program)
		if !ok {
			return nil, fmt.Errorf("could not convert cached value to ast.Program")
		}
		// Return a new program to clear importedPrograms.
		// This will avoid a concurrent map write when attempting to
		// resolveImports.
		return &ast.Program{Declarations: cachedProgram.Declarations}, nil
	}
	return nil, nil
}

// SetProgram adds a program AST to the LRU cache.
func (cache *LRUASTCache) SetProgram(location ast.Location, program *ast.Program) error {
	_ = cache.lru.Add(location.ID(), program)
	return nil
}
