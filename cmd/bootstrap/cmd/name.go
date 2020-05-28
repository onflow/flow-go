package cmd

import (
	"crypto/rand"

	"github.com/dapperlabs/flow-go/model/flow"
)

// shhh ;)
var names = []string{
	"", // blanks to buffer single-node gen
	"",
	"Alex Hentschel",
	"Andrew Burian",
	"Bastian Muller",
	"Benjamin Van Meter",
	"Casey Arrington",
	"Dieter Shirley",
	"James Hunter",
	"Jeffery Doyle",
	"Jordan Schalm",
	"Josh Hannan",
	"Kan Zhang",
	"Layne Lafrance",
	"Leo Zhang",
	"Mackenzie Kieran",
	"Maks Pawlak",
	"Max Wolter",
	"Peter Siemens",
	"Philip Stanislaus",
	"Ramtin Seraj",
	"Tarak Ben Youssef",
	"Tim Nan",
	"Vishal Changrani",
	"Yahya Hassanzadeh",
}

func getNameID() (flow.Identifier, bool) {
	if len(names) == 0 {
		return flow.Identifier{0}, false
	}

	name := names[0]
	names = names[1:]

	if name == "" {
		return flow.Identifier{0}, false
	}

	offset := len(name)
	id := make([]byte, 32)

	copy(id, []byte(name))
	id[offset] = 0
	offset++

	_, _ = rand.Read(id[offset:])

	var identifier flow.Identifier
	copy(identifier[:], id)
	return identifier, true
}
