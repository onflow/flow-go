package unicastmgr_test

//func unicastManagerConfigFixture(t *testing.T) *unicastmgr.Manager {
//
//}

// TODO tests: 1. Manager tries streamFactory x times for connection and y times for stream.
// TODO test: 2. when there is a no protocol issue, it does not retry it.
// TODO test: 3. After each unsuccessful attempt (failing all x times), it reduces the backoff time by one.
// TODO test: 4. When backoff time is 0, it does not back it off.
// TODO test: 5. When connection is successful, it resets the backoff times only when it passes a grace period.
// TODO test: 6. When stream time is 0, it does not back it off.
// TODO test: 7. When stream is successful, it resets the backoff time only when it passes a grace period.
// TODO test: 8. Manager exactly acts based on the dial config of the node.
