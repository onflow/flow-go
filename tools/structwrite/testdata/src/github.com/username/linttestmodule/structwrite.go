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
	w.A = 1 // want "write to NonWritable field outside constructor"
}

func NonEmbedsWritableConstructLiteral() {
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
