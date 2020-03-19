package cmd

import "github.com/dapperlabs/flow-go/crypto"

type NetworkPubKey struct {
	crypto.PublicKey
}

func (pub NetworkPubKey) MarshalYAML() (interface{}, error) {
	return pubKeyToString(pub), nil
}

func (pub *NetworkPubKey) UnmarshalYAML(unmarshal func(interface{}) error) error {
	bz, err := unmarshalToBytes(unmarshal)
	if err != nil {
		return err
	}
	pub.PublicKey, err = crypto.DecodePublicKey(crypto.ECDSA_SECp256k1, bz)
	return err
}

type NetworkPrivKey struct {
	crypto.PrivateKey
}

func (priv NetworkPrivKey) MarshalYAML() (interface{}, error) {
	return privKeyToString(priv), nil
}

func (priv *NetworkPrivKey) UnmarshalYAML(unmarshal func(interface{}) error) error {
	bz, err := unmarshalToBytes(unmarshal)
	if err != nil {
		return err
	}
	priv.PrivateKey, err = crypto.DecodePrivateKey(crypto.ECDSA_SECp256k1, bz)
	return err
}

type StakingPubKey struct {
	crypto.PublicKey
}

func (pub StakingPubKey) MarshalYAML() (interface{}, error) {
	return pubKeyToString(pub), nil
}

func (pub *StakingPubKey) UnmarshalYAML(unmarshal func(interface{}) error) error {
	bz, err := unmarshalToBytes(unmarshal)
	if err != nil {
		return err
	}
	pub.PublicKey, err = crypto.DecodePublicKey(crypto.BLS_BLS12381, bz)
	return err
}

type StakingPrivKey struct {
	crypto.PrivateKey
}

func (priv StakingPrivKey) MarshalYAML() (interface{}, error) {
	return privKeyToString(priv), nil
}

func (priv *StakingPrivKey) UnmarshalYAML(unmarshal func(interface{}) error) error {
	bz, err := unmarshalToBytes(unmarshal)
	if err != nil {
		return err
	}
	priv.PrivateKey, err = crypto.DecodePrivateKey(crypto.BLS_BLS12381, bz)
	return err
}

type RandomBeaconPubKey struct {
	crypto.PublicKey
}

func (pub RandomBeaconPubKey) MarshalYAML() (interface{}, error) {
	return pubKeyToString(pub), nil
}

func (pub *RandomBeaconPubKey) UnmarshalYAML(unmarshal func(interface{}) error) error {
	bz, err := unmarshalToBytes(unmarshal)
	if err != nil {
		return err
	}
	pub.PublicKey, err = crypto.DecodePublicKey(crypto.BLS_BLS12381, bz)
	return err
}

type RandomBeaconPrivKey struct {
	crypto.PrivateKey
}

func (priv RandomBeaconPrivKey) MarshalYAML() (interface{}, error) {
	return privKeyToString(priv), nil
}

func (priv *RandomBeaconPrivKey) UnmarshalYAML(unmarshal func(interface{}) error) error {
	bz, err := unmarshalToBytes(unmarshal)
	if err != nil {
		return err
	}
	priv.PrivateKey, err = crypto.DecodePrivateKey(crypto.BLS_BLS12381, bz)
	return err
}
