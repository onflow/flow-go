package unittest

import (
	"github.com/dapperlabs/flow-go/model/flow"
	module "github.com/dapperlabs/flow-go/module/mock"
)

// CreateNParticipantsWithMyRole creates a list of identities from given roles
func CreateNParticipantsWithMyRole(myRole flow.Role, otherRoles ...flow.Role) (
	flow.IdentityList, flow.Identifier, *module.Local) {
	// initialize the paramaters
	// participants := IdentityFixture(myRole)
	participants := make(flow.IdentityList, 0)
	myIdentity := IdentityFixture(WithRole(myRole))
	myID := myIdentity.ID()
	participants = append(participants, myIdentity)
	for _, role := range otherRoles {
		id := IdentityFixture(WithRole(role))
		participants = append(participants, id)
	}

	// set up local module mock
	me := &module.Local{}
	me.On("NodeID").Return(
		func() flow.Identifier {
			return myID
		},
	)
	return participants, myID, me
}
