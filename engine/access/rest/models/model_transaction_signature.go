/*
 * Access API
 *
 * No description provided (generated by Swagger Codegen https://github.com/swagger-api/swagger-codegen)
 *
 * API version: 1.0.0
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package models

type TransactionSignature struct {
	Address string `json:"address"`

	KeyIndex string `json:"key_index"`

	Signature string `json:"signature"`
}
