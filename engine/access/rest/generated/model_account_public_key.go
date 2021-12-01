/*
 * Access API
 *
 * No description provided (generated by Swagger Codegen https://github.com/swagger-api/swagger-codegen)
 *
 * API version: 1.0.0
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package generated

type AccountPublicKey struct {
	Index int32 `json:"index,omitempty"`

	PublicKey string `json:"public_key,omitempty"`

	SigningAlgorithm *SigningAlgorithm `json:"signing_algorithm,omitempty"`

	HashingAlgorithm *HashingAlgorithm `json:"hashing_algorithm,omitempty"`

	SequenceNumber int32 `json:"sequence_number,omitempty"`

	Weight int32 `json:"weight,omitempty"`

	Revoked bool `json:"revoked,omitempty"`
}
