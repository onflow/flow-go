package subpkg

type NonWritable struct {
	A int
}

func NewNonWritable() NonWritable {
	return NonWritable{
		A: 1,
	}
}

func (nw *NonWritable) SetA() {
	nw.A = 1 // want "write to NonWritable field outside constructor"
}
