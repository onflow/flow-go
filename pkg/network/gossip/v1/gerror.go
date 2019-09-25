package gnode

import "fmt"

// gossipError is the error type used when sending multiple requests and awaiting
// multiple errors
type gossipError []error

func (g *gossipError) Collect(e error) {
	if e != nil {
		*g = append(*g, e)
	}
}

func (g *gossipError) Error() string {
	err := "gossip errors:\n"
	for i, e := range *g {
		err += fmt.Sprintf("\terror %d: %s\n", i, e.Error())
	}

	return err
}
