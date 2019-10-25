package ast

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgram_ResolveImports(t *testing.T) {

	makeImportingProgram := func(imported string) *Program {
		return &Program{
			Declarations: []Declaration{
				&ImportDeclaration{
					Location: StringImportLocation(imported),
				},
			},
		}
	}

	a := makeImportingProgram("b")
	b := makeImportingProgram("c")
	c := &Program{}

	err := a.ResolveImports(func(location ImportLocation) (*Program, error) {
		switch location {
		case StringImportLocation("b"):
			return b, nil
		case StringImportLocation("c"):
			return c, nil
		default:
			return nil, fmt.Errorf("tried to resolve unknown import location: %s", location)
		}
	})

	assert.Nil(t, err)

	importsA := a.ImportedPrograms()

	actual := importsA[StringImportLocation("b").ID()]
	if actual != b {
		assert.Fail(t, "not b", actual)
	}

	importsB := b.ImportedPrograms()

	actual = importsB[StringImportLocation("c").ID()]
	if actual != c {
		assert.Fail(t, "not c", actual)
	}
}

func TestProgram_ResolveImportsCycle(t *testing.T) {

	makeImportingProgram := func(imported string) *Program {
		return &Program{
			Declarations: []Declaration{
				&ImportDeclaration{
					Location: StringImportLocation(imported),
				},
			},
		}
	}

	a := makeImportingProgram("b")
	b := makeImportingProgram("c")
	c := makeImportingProgram("a")

	err := a.ResolveImports(func(location ImportLocation) (*Program, error) {
		switch location {
		case StringImportLocation("a"):
			return a, nil
		case StringImportLocation("b"):
			return b, nil
		case StringImportLocation("c"):
			return c, nil
		default:
			return nil, fmt.Errorf("tried to resolve unknown import location: %s", location)
		}
	})

	assert.Equal(t,
		err,
		CyclicImportsError{
			Location: StringImportLocation("b"),
		},
	)
}
