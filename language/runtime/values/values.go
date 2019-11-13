package values

type Value interface{}

type String string

type Composite struct {
	Fields []Value
}

type Dictionary map[Value]Value
