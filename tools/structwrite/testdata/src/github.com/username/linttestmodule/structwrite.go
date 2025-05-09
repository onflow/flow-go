// Package linttestmodule is a collection of code representing test cases for the linter.
// See package golang.org/x/tools/go/analysis/analysistest for details.
package linttestmodule

// NonWritable is configured for linting.
//
//structwrite:immutable
type NonWritable struct {
	A int
}

func NewNonWritable() *NonWritable {
	return &NonWritable{
		A: 1,
	}
}

func NotConstructorButContainsNewString() *NonWritable {
	return &NonWritable{A: 1} // want "construction of NonWritable outside constructor"
}

func (nw *NonWritable) SetA() {
	nw.A = 1 // want "write to NonWritable field outside constructor"
}

func NonWritableConstructLiteral() {
	nw := NonWritable{}      // want "construction of NonWritable outside constructor"
	nw = NonWritable{A: 1}   // want "construction of NonWritable outside constructor"
	nwp := &NonWritable{}    // want "construction of NonWritable outside constructor"
	nwp = &NonWritable{A: 1} // want "construction of NonWritable outside constructor"
	nwp = new(NonWritable)
	var nwnw NonWritable
	_ = nw
	_ = nwp
	_ = nwnw
}

func NonWritableSetALiteral() {
	nw := NewNonWritable()
	nw.A = 1               // want "write to NonWritable field outside constructor"
	(*nw).A = 1            // want "write to NonWritable field outside constructor"
	NewNonWritable().A = 1 // want "write to NonWritable field outside constructor"
}

func NonWritableSetADoublePtr() {
	nw := NewNonWritable()
	nwp := &nw
	(*nwp).A = 1 // want "write to NonWritable field outside constructor"
}

func NonWritableSetWithinSlice() {
	ptr := []*NonWritable{NewNonWritable()}
	ptr[0].A = 1 // want "write to NonWritable field outside constructor"
	lit := []NonWritable{*NewNonWritable()}
	lit[0].A = 1 // want "write to NonWritable field outside constructor"
}

func NonWritableSetWithinArray() {
	ptr := [1]*NonWritable{NewNonWritable()}
	ptr[0].A = 1 // want "write to NonWritable field outside constructor"
	lit := [1]NonWritable{*NewNonWritable()}
	lit[0].A = 1 // want "write to NonWritable field outside constructor"
}

func NonWritableSetWithinMap() {
	ptr := map[int]*NonWritable{1: NewNonWritable()}
	ptr[1].A = 1 // want "write to NonWritable field outside constructor"
	// writing to a non-pointer value in a map is a compile error
}

func NonWritableSetWithinAnonymousFunc() {
	nw := NewNonWritable()
	fn := func() {
		nw.A = 1 // want "write to NonWritable field outside constructor"
	}
	fn()
}

// Writable is not configured for linting.
type Writable struct {
	A int
}

func NewWritable() Writable {
	return Writable{
		A: 1,
	}
}

func (w Writable) SetA() {
	w.A = 1
}

// EmbedsNonWritable is not configured for linting, but embeds a type that is.
type EmbedsNonWritable struct {
	NonWritable
	B int
}

func NewEmbedsNonWritable() EmbedsNonWritable {
	return EmbedsNonWritable{
		NonWritable: *NewNonWritable(),
	}
}

func (w *EmbedsNonWritable) SetA() {
	// disallowed because A is promoted by embedding from a mutation-protected type
	w.A = 1 // want "write to NonWritable field outside constructor"
}

func (w *EmbedsNonWritable) SetB() {
	// allowed because B is not promoted from the embedded mutation-protected type
	w.B = 1
}

func EmbedsWritableConstructLiteral() {
	nw := EmbedsNonWritable{}
	nw = EmbedsNonWritable{B: 2}
	nw = EmbedsNonWritable{NonWritable: NonWritable{A: 1}}       // want "construction of NonWritable outside constructor"
	nw = EmbedsNonWritable{B: 2, NonWritable: NonWritable{A: 1}} // want "construction of NonWritable outside constructor"
	nwp := &EmbedsNonWritable{}
	nwp = &EmbedsNonWritable{B: 2}
	nwp = &EmbedsNonWritable{NonWritable: NonWritable{A: 1}} // want "construction of NonWritable outside constructor"
	_ = nw
	_ = nwp
}

// ContainsNonWritableField is not configured for linting, but has a field which is mutation-protected.
type ContainsNonWritableField struct {
	NonWritableField NonWritable
	B                int
}

func (w *ContainsNonWritableField) SetA() {
	// disallowed because we are writing the mutation-protected type
	w.NonWritableField.A = 1 // want "write to NonWritable field outside constructor"
}

// SetNonWritableField sets a field which has a mutation-protected type, but is a field
// of a non-mutation-protected type. This is allowed.
func (w *ContainsNonWritableField) SetNonWritableField() {
	// allowed because we are not mutating the mutation-protected type (we are mutating ContainsNonWritableField)
	w.NonWritableField = *NewNonWritable()
}

func (w *ContainsNonWritableField) SetB() {
	// allowed because B is not promoted from the embedded mutation-protected type
	w.B = 1
}

func ContainsNonWritableFieldConstructLiteral() {
	nw := ContainsNonWritableField{}
	nw = ContainsNonWritableField{B: 2}
	nw = ContainsNonWritableField{NonWritableField: NonWritable{A: 1}}       // want "construction of NonWritable outside constructor"
	nw = ContainsNonWritableField{B: 2, NonWritableField: NonWritable{A: 1}} // want "construction of NonWritable outside constructor"
	nwp := &ContainsNonWritableField{}
	nwp = &ContainsNonWritableField{B: 2}
	nwp = &ContainsNonWritableField{NonWritableField: NonWritable{A: 1}} // want "construction of NonWritable outside constructor"
	_ = nw
	_ = nwp
}

// ContainsDeeplyNestedNonWritableField is not configured for linting, but has a (deeply nested) field which is mutation-protected.
type ContainsDeeplyNestedNonWritableField struct {
	DeeplyNestedNonWritableField struct {
		L1 struct {
			NonWritableField NonWritable
		}
	}
	B int
}

// SetNonWritableField sets a field which has a mutation-protected type, but is a field
// of a non-mutation-protected type. This is allowed.
func (w *ContainsDeeplyNestedNonWritableField) SetNonWritableField() {
	// allowed because we are not mutating the mutation-protected type (we are mutating ContainsNonWritableField)
	w.DeeplyNestedNonWritableField.L1.NonWritableField = *NewNonWritable()
	// can set the middle nested layer for the same reason
	w.DeeplyNestedNonWritableField.L1 = struct {
		NonWritableField NonWritable
	}{
		NonWritableField: *NewNonWritable(),
	}
}

func (w *ContainsDeeplyNestedNonWritableField) SetB() {
	// allowed because B is not promoted from the embedded mutation-protected type
	w.B = 1
}

func ContainsDeeplyNestedNonWritableFieldConstructLiteral() {
	nw := ContainsDeeplyNestedNonWritableField{}
	nw = ContainsDeeplyNestedNonWritableField{B: 2}
	nw = ContainsDeeplyNestedNonWritableField{
		DeeplyNestedNonWritableField: struct {
			L1 struct{ NonWritableField NonWritable }
		}{L1: struct {
			NonWritableField NonWritable
		}{
			NonWritableField: NonWritable{A: 1}, // want "construction of NonWritable outside constructor"
		}},
	}
	nwp := &ContainsDeeplyNestedNonWritableField{}
	nwp = &ContainsDeeplyNestedNonWritableField{B: 2}
	nwp = &ContainsDeeplyNestedNonWritableField{
		DeeplyNestedNonWritableField: struct {
			L1 struct{ NonWritableField NonWritable }
		}{L1: struct {
			NonWritableField NonWritable
		}{
			NonWritableField: NonWritable{A: 1}, // want "construction of NonWritable outside constructor"
		}},
	}
	_ = nw
	_ = nwp
}
