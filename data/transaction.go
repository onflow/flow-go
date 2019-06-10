package data

type Transaction struct {
	TxHash 			Hash
	Status 			Status
	ToAddress 		Address
	TxData 			[]byte
	Nonce 			uint64
	ComputeUsed		uint64
	PayerSignature	[]byte
}
