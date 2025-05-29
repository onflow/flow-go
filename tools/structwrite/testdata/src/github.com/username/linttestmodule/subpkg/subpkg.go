package subpkg

// NonWritable has the same type name as NonWritable in the root package.
// The test configuration should specify only the root type.
// This type exists to check that types are checked by fully qualified name.
//
//structwrite:immutable
type NonWritable struct {
	A int
}

func NewNonWritable() NonWritable {
	return NonWritable{
		A: 1,
	}
}

func (nw *NonWritable) SetA() {
	nw.A = 1
}

// NonWritableInSubpackage is configured for linting.
type NonWritableInSubpackage struct {
	A int
}

func NewNonWritableInSubpackage() NonWritableInSubpackage {
	return NonWritableInSubpackage{
		A: 1,
	}
}

func (nw *NonWritableInSubpackage) SetA() {
	nw.A = 1 // want "write to NonWritable field outside constructor"
}
