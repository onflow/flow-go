package structwrite

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin("structwrite", New)
}

// Settings defines the configuration schema for the plugin.
type Settings struct {
	// Structs is a list of fully-qualified struct type names for which immutability is enforced.
	// TODO: define in godoc instead?
	Structs []string `json:"structs"`

	// ConstructorRegex is a regex pattern (optional) to identify allowed constructor function names.
	// TODO: unused - current just use New.*
	ConstructorRegex string `json:"constructorRegex"`
}

// PluginStructWrite implements the LinterPlugin interface for the structwrite linter.
type PluginStructWrite struct {
	structs map[string]bool
}

// New creates a new instance of the PluginStructWrite plugin.
func New(cfg any) (register.LinterPlugin, error) {
	s, err := register.DecodeSettings[Settings](cfg)
	if err != nil {
		return nil, err
	}

	structMap := make(map[string]bool)
	for _, name := range s.Structs {
		structMap[name] = true
	}

	return &PluginStructWrite{structs: structMap}, nil
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
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				p.handleAssignStmt(node, pass, file)
			case *ast.CompositeLit:
				p.handleCompositeLit(node, pass, file)
			case *ast.CallExpr:
				p.handleCallExpr(node, pass, file)
			}
			return true
		})
	}
	return nil, nil
}

// handleAssignStmt checks for disallowed writes to tracked struct fields in assignments.
// It handles pointer and literal types, and writes to fields promoted through embedding.
func (p *PluginStructWrite) handleAssignStmt(assign *ast.AssignStmt, pass *analysis.Pass, file *ast.File) {
	for i, lhs := range assign.Lhs {
		selExpr, ok := lhs.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		found, named := p.containsTrackedStruct(selExpr, pass)
		if !found {
			continue
		}

		funcDecl := findEnclosingFunc(file, assign.Pos())
		if funcDecl == nil || !strings.HasPrefix(funcDecl.Name.Name, "New") {
			pass.Reportf(assign.Lhs[i].Pos(),
				"write to %s field outside constructor: func=%s, named=%s",
				named.Obj().Name(), funcNameOrEmpty(funcDecl), named.String())
		}
	}
}

// handleCompositeLit checks for disallowed construction of tracked structs using literals.
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
	if !p.structs[fullyQualified] {
		return
	}

	funcDecl := findEnclosingFunc(file, lit.Pos())
	if funcDecl == nil || !strings.HasPrefix(funcDecl.Name.Name, "New") {
		pass.Reportf(lit.Pos(),
			"construction of %s outside constructor", named.Obj().Name())
	}
}

// handleCallExpr checks for disallowed construction of tracked structs using new().
// TODO: is this desirable? We should do one of the following:
//   - consistently allow construction of empty instances of structs
//   - prevent construction of empty instances in ALL cases
//     (the main one not currently handled is instantiating a type where struct is a field of the type
//     and the field is not explicitly set)
func (p *PluginStructWrite) handleCallExpr(call *ast.CallExpr, pass *analysis.Pass, file *ast.File) {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "new" || len(call.Args) != 1 {
		return
	}

	typ := pass.TypesInfo.Types[call.Args[0]].Type
	if typ == nil {
		return
	}
	typ = deref(typ)

	named, ok := typ.(*types.Named)
	if !ok {
		return
	}

	fullyQualified := named.String()
	if !p.structs[fullyQualified] {
		return
	}

	funcDecl := findEnclosingFunc(file, call.Pos())
	if funcDecl == nil || !strings.HasPrefix(funcDecl.Name.Name, "New") {
		pass.Reportf(call.Pos(),
			"construction of %s using new() outside constructor function: func=%s",
			fullyQualified, funcNameOrEmpty(funcDecl))
	}
}

// containsTrackedStruct checks whether the field accessed via selector expression belongs to a tracked struct,
// either directly or via embedding.
func (p *PluginStructWrite) containsTrackedStruct(selExpr *ast.SelectorExpr, pass *analysis.Pass) (bool, *types.Named) {
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
				if p.structs[fullyQualified] {
					return true, named
				}
			}
		}
	}

	// Fallback: direct access (non-promoted)
	tv, ok := pass.TypesInfo.Types[selExpr.X]
	if !ok {
		return false, nil
	}

	typ := deref(tv.Type)
	named, ok := typ.(*types.Named)
	if !ok {
		return false, nil
	}
	fullyQualified := named.String()
	if p.structs[fullyQualified] {
		return true, named
	}

	return false, nil
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

// deref removes pointer indirection from a type.
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
