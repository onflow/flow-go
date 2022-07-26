package channels

// Topic is the internal type of Libp2p which corresponds to the Channel in the network level.
// It is a virtual medium enabling nodes to subscribe and communicate over epidemic dissemination.
type Topic string

func (t Topic) String() string {
	return string(t)
}
