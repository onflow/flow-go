package cmd

import (
	"encoding/json"

	"github.com/dapperlabs/flow-go/crypto"
)

// EncodableNetworkPubKey wraps a public key and allows it to be JSON encoded and decoded. It is not defined in the
// crypto package since the crypto package should not know about the different key types.
type EncodableNetworkPubKey struct {
	crypto.PublicKey
}

func (pub EncodableNetworkPubKey) MarshalJSON() ([]byte, error) {
	if pub.PublicKey == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(pubKeyToBytes(pub))
}

func (pub *EncodableNetworkPubKey) UnmarshalJSON(b []byte) error {
	var bz []byte
	if err := json.Unmarshal(b, &bz); err != nil {
		return err
	}
	if len(bz) == 0 {
		return nil
	}
	var err error
	pub.PublicKey, err = crypto.DecodePublicKey(crypto.ECDSA_SECp256k1, bz)
	return err
}

// EncodableNetworkPrivKey wraps a private key and allows it to be JSON encoded and decoded. It is not defined in the
// crypto package since the crypto package should not know about the different key types. More importantly, private
// keys should not be automatically encodable/serializable to prevent accidental secret sharing. The bootstrapping
// package is an exception, since it generates private keys that need to be serialized.
type EncodableNetworkPrivKey struct {
	crypto.PrivateKey
}

func (priv EncodableNetworkPrivKey) MarshalJSON() ([]byte, error) {
	if priv.PrivateKey == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(privKeyToBytes(priv))
}

func (priv *EncodableNetworkPrivKey) UnmarshalJSON(b []byte) error {
	var bz []byte
	if err := json.Unmarshal(b, &bz); err != nil {
		return err
	}
	if len(bz) == 0 {
		return nil
	}
	var err error
	priv.PrivateKey, err = crypto.DecodePrivateKey(crypto.ECDSA_SECp256k1, bz)
	return err
}

// EncodableStakingPubKey wraps a public key and allows it to be JSON encoded and decoded. It is not defined in the
// crypto package since the crypto package should not know about the different key types.
type EncodableStakingPubKey struct {
	crypto.PublicKey
}

func (pub EncodableStakingPubKey) MarshalJSON() ([]byte, error) {
	if pub.PublicKey == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(pubKeyToBytes(pub))
}

func (pub *EncodableStakingPubKey) UnmarshalJSON(b []byte) error {
	var bz []byte
	if err := json.Unmarshal(b, &bz); err != nil {
		return err
	}
	if len(bz) == 0 {
		return nil
	}
	var err error
	pub.PublicKey, err = crypto.DecodePublicKey(crypto.BLS_BLS12381, bz)
	return err
}

// EncodableStakingPrivKey wraps a private key and allows it to be JSON encoded and decoded. It is not defined in the
// crypto package since the crypto package should not know about the different key types. More importantly, private
// keys should not be automatically encodable/serializable to prevent accidental secret sharing. The bootstrapping
// package is an exception, since it generates private keys that need to be serialized.
type EncodableStakingPrivKey struct {
	crypto.PrivateKey
}

func (priv EncodableStakingPrivKey) MarshalJSON() ([]byte, error) {
	if priv.PrivateKey == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(privKeyToBytes(priv))
}

func (priv *EncodableStakingPrivKey) UnmarshalJSON(b []byte) error {
	var bz []byte
	if err := json.Unmarshal(b, &bz); err != nil {
		return err
	}
	if len(bz) == 0 {
		return nil
	}
	var err error
	priv.PrivateKey, err = crypto.DecodePrivateKey(crypto.BLS_BLS12381, bz)
	return err
}

// EncodableRandomBeaconPubKey wraps a public key and allows it to be JSON encoded and decoded. It is not defined in the
// crypto package since the crypto package should not know about the different key types.
type EncodableRandomBeaconPubKey struct {
	crypto.PublicKey
}

func (pub EncodableRandomBeaconPubKey) MarshalJSON() ([]byte, error) {
	if pub.PublicKey == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(pubKeyToBytes(pub))
}

func (pub *EncodableRandomBeaconPubKey) UnmarshalJSON(b []byte) error {
	var bz []byte
	if err := json.Unmarshal(b, &bz); err != nil {
		return err
	}
	if len(bz) == 0 {
		return nil
	}
	var err error
	pub.PublicKey, err = crypto.DecodePublicKey(crypto.BLS_BLS12381, bz)
	return err
}

// EncodableRandomBeaconPrivKey wraps a private key and allows it to be JSON encoded and decoded. It is not defined in
// the crypto package since the crypto package should not know about the different key types. More importantly, private
// keys should not be automatically encodable/serializable to prevent accidental secret sharing. The bootstrapping
// package is an exception, since it generates private keys that need to be serialized.
type EncodableRandomBeaconPrivKey struct {
	crypto.PrivateKey
}

func (priv EncodableRandomBeaconPrivKey) MarshalJSON() ([]byte, error) {
	if priv.PrivateKey == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(privKeyToBytes(priv))
}

func (priv *EncodableRandomBeaconPrivKey) UnmarshalJSON(b []byte) error {
	var bz []byte
	if err := json.Unmarshal(b, &bz); err != nil {
		return err
	}
	if len(bz) == 0 {
		return nil
	}
	var err error
	priv.PrivateKey, err = crypto.DecodePrivateKey(crypto.BLS_BLS12381, bz)
	return err
}
