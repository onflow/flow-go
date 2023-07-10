package convert

import (
	"errors"
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

var ErrEmptyMessage = errors.New("protobuf message is empty")
var ValidChainIds = map[string]bool{
	flow.Mainnet.String():           true,
	flow.Testnet.String():           true,
	flow.Sandboxnet.String():        true,
	flow.Benchnet.String():          true,
	flow.Localnet.String():          true,
	flow.Emulator.String():          true,
	flow.BftTestnet.String():        true,
	flow.MonotonicEmulator.String(): true,
}

// MessageToChainId converts the chainID from a protobuf message to a flow.ChainID
// It returns an error if the value is not a valid chainId
func MessageToChainId(m string) (*flow.ChainID, error) {
	if !ValidChainIds[m] {
		return nil, fmt.Errorf("invalid chainId %s: ", m)
	}
	chainId := flow.ChainID(m)
	return &chainId, nil
}

// AggregatedSignaturesToMessages converts a slice of AggregatedSignature structs to a corresponding
// slice of protobuf messages
func AggregatedSignaturesToMessages(a []flow.AggregatedSignature) []*entities.AggregatedSignature {
	parsedMessages := make([]*entities.AggregatedSignature, len(a))
	for i, sig := range a {
		parsedMessages[i] = &entities.AggregatedSignature{
			SignerIds:          IdentifiersToMessages(sig.SignerIDs),
			VerifierSignatures: SignaturesToMessages(sig.VerifierSignatures),
		}
	}
	return parsedMessages
}

// MessagesToAggregatedSignatures converts a slice of protobuf messages to their corresponding
// AggregatedSignature structs
func MessagesToAggregatedSignatures(m []*entities.AggregatedSignature) []flow.AggregatedSignature {
	parsedSignatures := make([]flow.AggregatedSignature, len(m))
	for i, message := range m {
		parsedSignatures[i] = flow.AggregatedSignature{
			SignerIDs:          MessagesToIdentifiers(message.SignerIds),
			VerifierSignatures: MessagesToSignatures(message.VerifierSignatures),
		}
	}
	return parsedSignatures
}

// SignatureToMessage converts a crypto.Signature to a byte slice for inclusion in a protobuf message
func SignatureToMessage(s crypto.Signature) []byte {
	return s[:]
}

// MessageToSignature converts a byte slice from a protobuf message to a crypto.Signature
func MessageToSignature(m []byte) crypto.Signature {
	return m[:]
}

// SignaturesToMessages converts a slice of crypto.Signatures to a slice of byte slices for inclusion in a protobuf message
func SignaturesToMessages(s []crypto.Signature) [][]byte {
	messages := make([][]byte, len(s))
	for i, sig := range s {
		messages[i] = SignatureToMessage(sig)
	}
	return messages
}

// MessagesToSignatures converts a slice of byte slices from a protobuf message to a slice of crypto.Signatures
func MessagesToSignatures(m [][]byte) []crypto.Signature {
	signatures := make([]crypto.Signature, len(m))
	for i, message := range m {
		signatures[i] = MessageToSignature(message)
	}
	return signatures
}

// IdentifierToMessage converts a flow.Identifier to a byte slice for inclusion in a protobuf message
func IdentifierToMessage(i flow.Identifier) []byte {
	return i[:]
}

// MessageToIdentifier converts a byte slice from a protobuf message to a flow.Identifier
func MessageToIdentifier(b []byte) flow.Identifier {
	return flow.HashToID(b)
}

// IdentifiersToMessages converts a slice of flow.Identifiers to a slice of byte slices for inclusion in a protobuf message
func IdentifiersToMessages(l []flow.Identifier) [][]byte {
	results := make([][]byte, len(l))
	for i, item := range l {
		results[i] = IdentifierToMessage(item)
	}
	return results
}

// MessagesToIdentifiers converts a slice of byte slices from a protobuf message to a slice of flow.Identifiers
func MessagesToIdentifiers(l [][]byte) []flow.Identifier {
	results := make([]flow.Identifier, len(l))
	for i, item := range l {
		results[i] = MessageToIdentifier(item)
	}
	return results
}

// StateCommitmentToMessage converts a flow.StateCommitment to a byte slice for inclusion in a protobuf message
func StateCommitmentToMessage(s flow.StateCommitment) []byte {
	return s[:]
}

// MessageToStateCommitment converts a byte slice from a protobuf message to a flow.StateCommitment
func MessageToStateCommitment(bytes []byte) (sc flow.StateCommitment, err error) {
	if len(bytes) != len(sc) {
		return sc, fmt.Errorf("invalid state commitment length. got %d expected %d", len(bytes), len(sc))
	}
	copy(sc[:], bytes)
	return
}
