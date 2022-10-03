package errors

// The only purpose of this file is to enable flow-go integration test to
// build.
//
// TODO(patrick): update emulator error classification
//
// TODO(patrick): remove this file once emulator is updated.

// DO NOT USE.
type InvalidProposalSignatureError struct{ CodedError }

// DO NOT USE.
type AccountNotFoundError struct{ CodedError }

// DO NOT USE.
type InvalidAddressError struct{ CodedError }

// DO NOT USE.
type InvalidEnvelopeSignatureError struct{ CodedError }

// DO NOT USE.
//type InvalidProposalSeqNumberError struct{ CodedError }

// DO NOT USE.
type InvalidPayloadSignatureError struct{ CodedError }

// DO NOT USE.
type AccountAuthorizationError struct{ CodedError }

// DO NOT USE.
type AccountPublicKeyNotFoundError struct{ CodedError }
