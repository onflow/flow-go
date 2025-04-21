package structwrite

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin("structwrite", New)
}

// Settings defines the configuration schema for the plugin.
type Settings struct {
	// ConstructorRegex is a regex pattern (optional) to identify allowed constructor function names.
	ConstructorRegex string `json:"constructorRegex"`
}

// PluginStructWrite implements the LinterPlugin interface for the structwrite linter.
// This linter prevents mutations and non-empty construction of struct types marked as immutable.
// Mutation and construction of these types are only allowed in constructor functions.
// Re-assignment of a variable with a type marked immutable is allowed.
//
//	x := NewImmutableType(1)
//	x.SomeField = 2         // not allowed
//	x = NewImmutableType(2) // allowed
//
// A struct type is marked as immutable by adding a directive comment of the form: `//structwrite:immutable .*`.
// The directive comment must appear in the godoc for the type being marked immutable.
//
// See handleAssignStmt and handleCompositeLit in this file for examples of what operations are allowed.
// See also the Go files under ./testdata, which represent the test cases for the linter.
//
// This linter does not guarantee that structs marked immutable cannot be mutated, but it does
// warn for the majority of possible mutation situations. Below are a list of scenarios which will
// mutate a mutation-protected struct without the linter noticing:
//
//  1. Reflection (for example passing a pointer to a struct type into json.Unmarshal)
//  2. Use of unsafe.Pointer
//  3. Re-assignment after reference escape.
//
// Example of (3.):
//
//	type Y struct {
//	  B int
//	}
//	func NewY(b int) Y { return Y{B: b} }
//	type X struct {
//	  Y *Y
//	}
//	func NewX(y *Y) X { return X{Y:y} }
//
//	y := NewY(1)
//	x := NewX(&y)
//	// x.Y.B == 1
//	y = NewY(2)
//	// x.Y.B == 2: x has been mutated due to the shared reference
type PluginStructWrite struct {
	// Set of mutation-protected types, stored as fully qualified type names
	mutationProtected map[string]bool
	// Regex of constructor function names, where mutation is allowed.
	constructorRegex *regexp.Regexp
}

// New creates a new instance of the PluginStructWrite plugin.
func New(cfg any) (register.LinterPlugin, error) {
	s, err := register.DecodeSettings[Settings](cfg)
	if err != nil {
		return nil, err
	}

	// Default to New*
	if s.ConstructorRegex == "" {
		s.ConstructorRegex = "^New.*"
	}
	re, err := regexp.Compile(s.ConstructorRegex)
	if err != nil {
		return nil, err
	}

	return &PluginStructWrite{
		mutationProtected: make(map[string]bool),
		constructorRegex:  re,
	}, nil
}

func (p *PluginStructWrite) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	a := &analysis.Analyzer{
		Name: "structwrite",
		Doc:  "flags writes to specified struct fields or construction of structs outside constructor functions",
		Run:  p.run,
	}
	return []*analysis.Analyzer{a}, nil
}

func (p *PluginStructWrite) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

// run is the main analysis function.
func (p *PluginStructWrite) run(pass *analysis.Pass) (interface{}, error) {
	p.gatherMutationProtectedTypes(pass)

	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				p.handleAssignStmt(node, pass, file)
			case *ast.CompositeLit:
				p.handleCompositeLit(node, pass, file)
			}
			return true
		})
	}
	return nil, nil
}

// gatherMutationProtectedTypes populates the set of mutation-protected types before the linter runs.
// A struct type is marked as immutable by adding a directive comment of the form: `//structwrite:immutable .*`.
// The directive comment must appear in the godoc for the type being marked immutable.
func (p *PluginStructWrite) gatherMutationProtectedTypes(pass *analysis.Pass) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				if genDecl.Doc != nil {
					for _, comment := range genDecl.Doc.List {
						if strings.HasPrefix(comment.Text, "//structwrite:immutable") {
							typeObj := pass.TypesInfo.Defs[typeSpec.Name]
							if named, ok := typeObj.Type().(*types.Named); ok {
								fullyQualified := named.String()
								p.mutationProtected[fullyQualified] = true
							}
							break
						}
					}
				}
			}
		}
	}
}

// handleAssignStmt checks for disallowed writes to tracked struct fields in assignments.
// It handles pointer and literal types, and writes to fields promoted through embedding.
//
// In the examples below, suppose A is mutation-protected and B is not mutation-protected.
// Suppose C is a struct which embeds A, and which is not mutation-protected.
//
//	type A struct {
//	  FieldA int
//	}
//	type C struct {
//	  A
//	  FieldC int
//	}
//
// You can only write to fields of non-mutation-protected structs:
//
//	A.FieldA = 1     // not allowed
//	B.SomeField = 1  // allowed
//
// When a mutation-protected struct is embedded, you can write to fields of the outer struct.
// You cannot write to fields defined on the inner struct that are promoted through embedding.
//
//	C.FieldC = 1     // allowed
//	C.FieldA = 1     // not allowed
func (p *PluginStructWrite) handleAssignStmt(assign *ast.AssignStmt, pass *analysis.Pass, file *ast.File) {
	for i, lhs := range assign.Lhs {
		// we are only concerned about assignments
		selExpr, ok := lhs.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		named, found := p.containsTrackedStruct(selExpr, pass)
		if !found {
			continue
		}

		funcDecl := findEnclosingFunc(file, assign.Pos())
		if funcDecl == nil || !p.constructorRegex.MatchString(funcDecl.Name.Name) {
			pass.Reportf(assign.Lhs[i].Pos(), "write to %s field outside constructor: func=%s, named=%s",
				named.Obj().Name(), funcNameOrEmpty(funcDecl), named.String())
		}
	}
}

// handleCompositeLit checks for disallowed literal construction of mutation-protected structs.
//
// In the examples below, suppose A is mutation-protected and B is not mutation-protected.
// In general, construction of empty instances of mutation-protected structs is allowed:
//
//	x := new(A)            // allowed
//	var x A                // allowed
//
// However, for simplicity, literal construction is disallowed even when no fields are specified:
//
//	x := A{}               // not allowed
//
// Additional examples:
//
//	x := A{SomeField: 1}   // not allowed
//	x := B{}               // allowed
//	x := B{SomeField: 1}   // allowed
//	x := B{SomeField: A{}} // not allowed
func (p *PluginStructWrite) handleCompositeLit(lit *ast.CompositeLit, pass *analysis.Pass, file *ast.File) {
	typ := pass.TypesInfo.Types[lit].Type
	if typ == nil {
		return
	}
	typ = deref(typ)

	named, ok := typ.(*types.Named)
	if !ok {
		return
	}

	fullyQualified := named.String()
	if !p.mutationProtected[fullyQualified] {
		return
	}

	funcDecl := findEnclosingFunc(file, lit.Pos())
	if funcDecl == nil || !p.constructorRegex.MatchString(funcDecl.Name.Name) {
		pass.Reportf(lit.Pos(), "construction of %s outside constructor", named.Obj().Name())
	}
}

// containsTrackedStruct checks whether the field accessed via selector expression belongs to a tracked struct,
// either directly or via embedding.
func (p *PluginStructWrite) containsTrackedStruct(selExpr *ast.SelectorExpr, pass *analysis.Pass) (*types.Named, bool) {
	// Handle promoted fields (embedding)
	if sel := pass.TypesInfo.Selections[selExpr]; sel != nil && sel.Kind() == types.FieldVal {
		typ := sel.Recv()
		for _, idx := range sel.Index() {
			structType, ok := deref(typ).Underlying().(*types.Struct)
			if !ok || idx >= structType.NumFields() {
				break
			}
			field := structType.Field(idx)
			typ = field.Type()

			if named, ok := deref(typ).(*types.Named); ok {
				fullyQualified := named.String()
				if p.mutationProtected[fullyQualified] {
					return named, true
				}
			}
		}
	}

	// Fallback: direct access (non-promoted)
	tv, ok := pass.TypesInfo.Types[selExpr.X]
	if !ok {
		return nil, false
	}

	typ := deref(tv.Type)
	named, ok := typ.(*types.Named)
	if !ok {
		return nil, false
	}
	fullyQualified := named.String()
	if p.mutationProtected[fullyQualified] {
		return named, true
	}

	return nil, false
}

// findEnclosingFunc returns the enclosing function declaration for a given position.
func findEnclosingFunc(file *ast.File, pos token.Pos) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			if fn.Body.Pos() <= pos && pos <= fn.Body.End() {
				return fn
			}
		}
	}
	return nil
}

// deref removes pointer indirection from a type if it is a pointer.
// Otherwise returns the input unchanged.
func deref(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

// funcNameOrEmpty returns the function name or a fallback if nil.
func funcNameOrEmpty(fn *ast.FuncDecl) string {
	if fn != nil {
		return fn.Name.Name
	}
	return "(unknown)"
}
