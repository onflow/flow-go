package environment

// Export functions from account_public_key_util.go for testing.
var (
	GetAccountPublicKey0                    = getAccountPublicKey0
	SetAccountPublicKey0                    = setAccountPublicKey0
	RevokeAccountPublicKey0                 = revokeAccountPublicKey0
	GetAccountPublicKeySequenceNumber       = getAccountPublicKeySequenceNumber
	IncrementAccountPublicKeySequenceNumber = incrementAccountPublicKeySequenceNumber
	GetStoredPublicKey                      = getStoredPublicKey
	AppendStoredKey                         = appendStoredKey
	EncodeBatchedPublicKey                  = encodeBatchedPublicKey
)
