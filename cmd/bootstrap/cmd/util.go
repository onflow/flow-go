package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/ioutil"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

func generateRandomSeeds(n int) [][]byte {
	seeds := make([][]byte, 0, n)

	for i := 0; i < n; i++ {
		seeds = append(seeds, make([]byte, 64))
		if _, err := rand.Read(seeds[i]); err != nil {
			log.Fatal().Err(err).Msg("cannot generate random seeds")
		}
	}

	return seeds
}

func readYaml(filename string, target interface{}) {
	dat, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot read file")
	}
	err = yaml.Unmarshal(dat, target)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot unmarshal yaml in file")
	}
}

func writeYaml(filename string, data interface{}) {
	bz, err := yaml.Marshal(data)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot marshal yaml")
	}

	err = ioutil.WriteFile(filename, bz, 0644)

	log.Info().Msgf("wrote yaml to file %v", filename)
}

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

func pubKeyToBytes(key crypto.PublicKey) []byte {
	enc, err := key.Encode()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot encode public key")
	}
	return enc
}

func pubKeyToString(key crypto.PublicKey) string {
	return fmt.Sprintf("%x", pubKeyToBytes(key))
}

func privKeyToBytes(key crypto.PrivateKey) []byte {
	enc, err := key.Encode()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot encode private key")
	}
	return enc
}

func privKeyToString(key crypto.PrivateKey) string {
	return fmt.Sprintf("%x", privKeyToBytes(key))
}

func unmarshalToBytes(unmarshal func(interface{}) error) ([]byte, error) {
	var s string
	err := unmarshal(&s)
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(s)
}
