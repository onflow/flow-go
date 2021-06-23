package swagger

import (
	"encoding/hex"
	"strconv"
)

func convertTransaction(transaction *transactionproto.Transaction) *EntitiesTransaction {
	// TODO
	// message Transaction {
	// 	message ProposalKey {
	// 	  bytes address = 1;
	// 	  uint32 key_id = 2;
	// 	  uint64 sequence_number = 3;
	// 	}

	// 	message Signature {
	// 	  bytes address = 1;
	// 	  uint32 key_id = 2;
	// 	  bytes signature = 3;
	// 	}

	// 	bytes script = 1;
	// 	repeated bytes arguments = 2;
	// 	bytes reference_block_id = 3;
	// 	uint64 gas_limit = 4;
	// 	ProposalKey proposal_key = 5;
	// 	bytes payer = 6;
	// 	repeated bytes authorizers = 7;
	// 	repeated Signature payload_signatures = 8;
	// 	repeated Signature envelope_signatures = 9;
	//   }
	// }

	// type Transaction struct {
	// 	Script               []byte                   `protobuf:"bytes,1,opt,name=script,proto3" json:"script,omitempty"`
	// 	Arguments            [][]byte                 `protobuf:"bytes,2,rep,name=arguments,proto3" json:"arguments,omitempty"`
	// 	ReferenceBlockId     []byte                   `protobuf:"bytes,3,opt,name=reference_block_id,json=referenceBlockId,proto3" json:"reference_block_id,omitempty"`
	// 	GasLimit             uint64                   `protobuf:"varint,4,opt,name=gas_limit,json=gasLimit,proto3" json:"gas_limit,omitempty"`
	// 	ProposalKey          *Transaction_ProposalKey `protobuf:"bytes,5,opt,name=proposal_key,json=proposalKey,proto3" json:"proposal_key,omitempty"`
	// 	Payer                []byte                   `protobuf:"bytes,6,opt,name=payer,proto3" json:"payer,omitempty"`
	// 	Authorizers          [][]byte                 `protobuf:"bytes,7,rep,name=authorizers,proto3" json:"authorizers,omitempty"`
	// 	PayloadSignatures    []*Transaction_Signature `protobuf:"bytes,8,rep,name=payload_signatures,json=payloadSignatures,proto3" json:"payload_signatures,omitempty"`
	// 	EnvelopeSignatures   []*Transaction_Signature `protobuf:"bytes,9,rep,name=envelope_signatures,json=envelopeSignatures,proto3" json:"envelope_signatures,omitempty"`
	// }

	// to

	// type EntitiesTransaction struct {
	// 	Script string `json:"script,omitempty"`
	// 	Arguments []string `json:"arguments,omitempty"`
	// 	ReferenceBlockId string `json:"referenceBlockId,omitempty"`
	// 	GasLimit string `json:"gasLimit,omitempty"`
	// 	ProposalKey *TransactionProposalKey `json:"proposalKey,omitempty"`
	// 	Payer string `json:"payer,omitempty"`
	// 	Authorizers []string `json:"authorizers,omitempty"`
	// 	PayloadSignatures []TransactionSignature `json:"payloadSignatures,omitempty"`
	// 	EnvelopeSignatures []TransactionSignature `json:"envelopeSignatures,omitempty"`
	// }

	// type Transaction_Signature struct {
	// 	Address              []byte   `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	// 	KeyId                uint32   `protobuf:"varint,2,opt,name=key_id,json=keyId,proto3" json:"key_id,omitempty"`
	// 	Signature            []byte   `protobuf:"bytes,3,opt,name=signature,proto3" json:"signature,omitempty"`
	// }

	// type Transaction_ProposalKey struct {
	// 	Address              []byte   `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	// 	KeyId                uint32   `protobuf:"varint,2,opt,name=key_id,json=keyId,proto3" json:"key_id,omitempty"`
	// 	SequenceNumber       uint64   `protobuf:"varint,3,opt,name=sequence_number,json=sequenceNumber,proto3" json:"sequence_number,omitempty"`
	// }

	// TODO: these functions below could throw error on invalid input!
	return &EntitiesTransaction{
		Script: hex.encodeToString(transaction.Script),
		// Arguments
		ReferenceBlockId: hex.encodeToString(transaction.ReferenceBlockId),
		GasLimit:         strconv.FormatUint(transaction.GasLimit, 10),
		ProposalKey: ProposalKey{
			Address: hex.DecodeString(transaction.ProposalKey.Address),
			// KeyId:
			SequenceNumber: strconv.FormatUint(transaction.ProposalKey.SequenceNumber, 10),
		},
		// Payer:
		// Authorizers:
		PayloadSignatures: 

	}
}
