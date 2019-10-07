package gnode

import "fmt"

// gossipError models the aggregated execution error of a gossip message at different recipients
type gossipError []error

// Append takes in an error and adds it to the gossipError array
func (g *gossipError) Append(e error) {
	if e != nil {
		*g = append(*g, e)
	}
}

// Error returns an error string made from all the errors in the list
func (g *gossipError) Error() string {
	err := "gossip errors:\n"
	for i, e := range *g {
		err += fmt.Sprintf("\terror %d: %s\n", i, e.Error())
	}
	return err
}

