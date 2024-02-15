package network

import (
	"github.com/onflow/flow-go/model/flow"
)

// DisallowListedCause is a type representing the cause of disallow listing. A remote node may be disallow-listed by the
// current node for a variety of reasons. This type is used to represent the reason for disallow-listing, so that if
// a node is disallow-listed for reasons X and Y, allow-listing it back for reason X does not automatically allow-list
// it for reason Y.
type DisallowListedCause string

func (c DisallowListedCause) String() string {
	return string(c)
}

const (
	// DisallowListedCauseAdmin is the cause of disallow-listing a node by an admin command.
	DisallowListedCauseAdmin DisallowListedCause = "disallow-listed-admin"
	// DisallowListedCauseAlsp is the cause of disallow-listing a node by the ALSP (Application Layer Spam Prevention).
	DisallowListedCauseAlsp DisallowListedCause = "disallow-listed-alsp"
)

// DisallowListingUpdate is a notification of a new disallow list update, it contains a list of Flow identities that
// are now disallow listed for a specific reason.
type DisallowListingUpdate struct {
	FlowIds flow.IdentifierList
	Cause   DisallowListedCause
}

// AllowListingUpdate is a notification of a new allow list update, it contains a list of Flow identities that
// are now allow listed for a specific reason, i.e., their disallow list entry for that reason is removed.
type AllowListingUpdate struct {
	FlowIds flow.IdentifierList
	Cause   DisallowListedCause
}

// DisallowListNotificationConsumer is an interface for consuming disallow/allow list update notifications.
type DisallowListNotificationConsumer interface {
	// OnDisallowListNotification is called when a new disallow list update notification is distributed.
	// Any error on consuming an event must be handled internally.
	// The implementation must be concurrency safe.
	OnDisallowListNotification(*DisallowListingUpdate)

	// OnAllowListNotification is called when a new allow list update notification is distributed.
	// Any error on consuming an event must be handled internally.
	// The implementation must be concurrency safe.
	OnAllowListNotification(*AllowListingUpdate)
}
