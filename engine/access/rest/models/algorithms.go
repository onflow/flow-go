package models

type SigningAlgorithmResponse string

// List of SigningAlgorithm
const (
	BLSBLS12381     SigningAlgorithmResponse = "BLSBLS12381"
	ECDSAP256       SigningAlgorithmResponse = "ECDSAP256"
	ECDSA_SECP256K1 SigningAlgorithmResponse = "ECDSASecp256k1"
)

type HashingAlgorithmResponse string

// List of HashingAlgorithm
const (
	SHA2_256 HashingAlgorithmResponse = "SHA2_256"
	SHA2_384 HashingAlgorithmResponse = "SHA2_384"
	SHA3_256 HashingAlgorithmResponse = "SHA3_256"
	SHA3_384 HashingAlgorithmResponse = "SHA3_384"
	KMAC128  HashingAlgorithmResponse = "KMAC128"
)
