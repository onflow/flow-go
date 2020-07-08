package ledger

// TODO Here
// Proof holds all the data needed to verify a payload at specific StateCommitment
type Proof interface {
	// Verify validate if a proof is valid or not
	Verify() (bool, error)

	// returns the payload
	Payload()

	// returns the state commitment of this proof
	StateCommitment()

	// Encode the proof into a byte slice
	Encode() []byte
}
