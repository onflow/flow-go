package cmd

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/nacl/box"

	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	ioutils "github.com/onflow/flow-go/utils/io"
)

const fileMode = os.FileMode(0644)

var (

	// FilenameTransitKeyPub transit key pub file name that consensus nodes will upload (to securely transport DKG in phase 2)
	FilenameTransitKeyPub  = "transit-key.pub.%v"
	FilenameTransitKeyPriv = "transit-key.priv.%v"

	// FilenameRandomBeaconCipher consensus node additionally gets the random beacon file
	FilenameRandomBeaconCipher = bootstrap.FilenameRandomBeaconPriv + ".%v.enc"

	// default folder to download for all role type
	folderToDownload = bootstrap.DirnamePublicBootstrap

	// consensus node additionally gets the random beacon file
	filesToDownloadConsensus = FilenameRandomBeaconCipher

	// commit and semver vars
	commit = build.Commit()
	semver = build.Semver()
)

// readNodeID reads the NodeID file
func readNodeID() (string, error) {
	path := filepath.Join(flagBootDir, bootstrap.PathNodeID)

	data, err := ioutils.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("error reading file %s: %w", path, err)
	}

	return strings.TrimSpace(string(data)), nil
}

func getAdditionalFilesToDownload(role flow.Role, nodeID string) []string {
	switch role {
	case flow.RoleConsensus:
		return []string{fmt.Sprintf(filesToDownloadConsensus, nodeID)}
	}
	return make([]string, 0)
}

func getFileSHA256(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// moveFile moves a file from source to destination where src and dst are full paths including the filename
func moveFile(src, dst string) error {

	// check if source file exist
	if !ioutils.FileExists(src) {
		return fmt.Errorf("file not found: %s", src)
	}

	// create the destination dir if it does not exist
	destinationDir := filepath.Dir(dst)
	err := os.MkdirAll(destinationDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destinationDir, err)
	}

	// first, try renaming the file
	err = os.Rename(src, dst)
	if err == nil {
		// if renaming works, we are done
		return nil
	}

	// renaming may fail if the destination dir is on a different disk, in that case we do a copy followed by remove
	// open the source file
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", src, err)
	}

	// create the destination file
	destination, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", dst, err)
	}
	defer destination.Close()

	// copy the source file to the destination file
	_, err = io.Copy(destination, source)
	if err != nil {
		errorStr := err.Error()
		closeErr := source.Close()
		if closeErr != nil {
			errorStr = fmt.Sprintf("%s, %s", errorStr, closeErr)
		}
		return fmt.Errorf("failed to copy file %s to %s: %s", src, dst, errorStr)
	}

	// close the source file
	err = source.Close()
	if err != nil {
		return fmt.Errorf("failed to close source file %s: %w", src, err)
	}

	// flush the destination file
	err = destination.Sync()
	if err != nil {
		return fmt.Errorf("failed to copy file %s to %s: %w", src, dst, err)
	}

	// read the source file permissions
	si, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to get file information %s: %w", src, err)
	}

	// set the same permissions on the destination file
	err = os.Chmod(dst, si.Mode())
	if err != nil {
		return fmt.Errorf("failed to set permisson on file %s: %w", dst, err)
	}

	// delete the source file
	err = os.Remove(src)
	if err != nil {
		return fmt.Errorf("failed removing original file: %s", err)
	}

	return nil
}

func unWrapFile(bootDir string, nodeID string) error {

	log.Info().Msg("decrypting Random Beacon key")

	pubKeyPath := filepath.Join(bootDir, fmt.Sprintf(FilenameTransitKeyPub, nodeID))
	privKeyPath := filepath.Join(bootDir, fmt.Sprintf(FilenameTransitKeyPriv, nodeID))
	ciphertextPath := filepath.Join(bootDir, fmt.Sprintf(FilenameRandomBeaconCipher, nodeID))
	plaintextPath := filepath.Join(bootDir, fmt.Sprintf(bootstrap.PathRandomBeaconPriv, nodeID))

	ciphertext, err := ioutils.ReadFile(ciphertextPath)
	if err != nil {
		return fmt.Errorf("failed to open ciphertext file %s: %w", ciphertextPath, err)
	}

	publicKey, err := ioutils.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("failed to open public keyfile %s: %w", pubKeyPath, err)
	}

	privateKey, err := ioutils.ReadFile(privKeyPath)
	if err != nil {
		return fmt.Errorf("failed to open private keyfile %s: %w", privKeyPath, err)
	}

	// NaCl is picky and wants its type to be exactly a [32]byte, but readfile reads a slice
	var pubKeyBytes, privKeyBytes [32]byte
	copy(pubKeyBytes[:], publicKey)
	copy(privKeyBytes[:], privateKey)

	plaintext := make([]byte, 0, len(ciphertext)-box.AnonymousOverhead)
	plaintext, ok := box.OpenAnonymous(plaintext, ciphertext, &pubKeyBytes, &privKeyBytes)
	if !ok {
		return fmt.Errorf("failed to decrypt random beacon key using private key from file: %s", privKeyPath)
	}

	err = ioutil.WriteFile(plaintextPath, plaintext, fileMode)
	if err != nil {
		return fmt.Errorf("failed to write the decrypted file %s: %w", plaintextPath, err)
	}

	return nil
}

func wrapFile(bootDir string, nodeID string) error {
	pubKeyPath := filepath.Join(bootDir, fmt.Sprintf(FilenameTransitKeyPub, nodeID))
	plaintextPath := filepath.Join(bootDir, fmt.Sprintf(bootstrap.PathRandomBeaconPriv, nodeID))
	ciphertextPath := filepath.Join(bootDir, fmt.Sprintf(FilenameRandomBeaconCipher, nodeID))

	plaintext, err := ioutils.ReadFile(plaintextPath)
	if err != nil {
		return fmt.Errorf("failed to open plaintext file %s: %w", plaintextPath, err)
	}

	publicKey, err := ioutils.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("faield to open public keyfile %s: %w", pubKeyPath, err)
	}

	var pubKeyBytes [32]byte
	copy(pubKeyBytes[:], publicKey)

	ciphertext := make([]byte, 0, len(plaintext)+box.AnonymousOverhead)

	ciphertext, err = box.SealAnonymous(ciphertext, plaintext, &pubKeyBytes, rand.Reader)
	if err != nil {
		return fmt.Errorf("could not encrypt file: %w", err)
	}

	err = ioutil.WriteFile(ciphertextPath, ciphertext, fileMode)
	if err != nil {
		return fmt.Errorf("error writing ciphertext: %w", err)
	}

	return nil
}

// generateKeys creates the transit keypair and writes them to disk for later
func generateKeys(bootDir string, nodeID string) error {

	privPath := filepath.Join(bootDir, fmt.Sprintf(FilenameTransitKeyPriv, nodeID))
	pubPath := filepath.Join(bootDir, fmt.Sprintf(FilenameTransitKeyPub, nodeID))

	if ioutils.FileExists(privPath) && ioutils.FileExists(pubPath) {
		log.Warn().Msg("transit-key-path priv & pub both exist, skipping key generation")
		return nil
	}

	log.Info().Msg("generating key pair")

	// Generate the keypair
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to create keys: %w", err)
	}

	// Write private key file
	err = ioutil.WriteFile(privPath, priv[:], fileMode)
	if err != nil {
		return fmt.Errorf("failed to write pivate key file: %w", err)
	}

	// Write public key file
	err = ioutil.WriteFile(pubPath, pub[:], fileMode)
	if err != nil {
		return fmt.Errorf("failed to write public key file: %w", err)
	}

	return nil
}
