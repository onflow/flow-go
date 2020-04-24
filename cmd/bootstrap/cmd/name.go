package cmd

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"crypto/rand"
)

// shhh ;)
var names []string = {
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

getNameID() (flow.Identifier, bool) {
	if len(names) == 0 {
		return flow.Identifier{0}, false
	}

	name := names[0]
	names = names[1:]

	offset := len(name)
	id := make([]byte, 32)

	copy(id, []byte(name))
	id[offset] = 0
	offset++

	rand.Read(id[offset:])

	var identifier Identifier
	copy(identifier[:], id)
	return identifier, true
}
