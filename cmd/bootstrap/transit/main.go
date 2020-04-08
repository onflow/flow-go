package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"flag"
	"io/ioutil"

	"golang.org/x/crypto/nacl/box"
	"github.com/dapperlabs/flow-go/model/bootstrap"
)

const (
	FilenameTransitKeyPub = "%v.transit-key.pub"
	FilenameTransitKeyPriv = "%v.transit-key.priv"
)

func main() {

	var bootdir, keydir string
	var pull, push bool

	flag.StringVar(&bootdir, "d", "~/bootstrap", "The bootstrap directory containing your node-info files")
	flag.StringVar(&keydir, "k", "", "Key provided by the Flow team to access the transit server")
	flag.BoolVar(&pull, "pull", false, "Fetch keys and metadata from the transit server")
	flag.BoolVar(&push, "push", false, "Upload public keys to the transit server")
	flag.Parse()

	var err error = nil
	if pull && push {
		fmt.Fprintf(stderr, "Only one of -pull or -push may be specified\n")
		flag.Usage()
		os.Exit(2)
	}

	if !(pull || push) {
		fmt.Fprintf(stderr, "One of -pull or -push must be specified\n")
		flag.Usage()
		os.Exit(2)
	}

	if keydir == "" {
		fmt.Fprintf(stderr, "Access key required\n")
		flag.Usage()
		os.Exit(2)
	}

	nodeId, err := fetchNodeId(bootdir)
	if err != nil {
		fmt.Fprintf(stderr, "Could not determine node ID: %s\n", err)
		os.Exit(1)
	}

	if push {
		return runPush()
	}

	if pull {
		return runPull()
	}
}

func fetchNodeId(bootdir string) (string, error) {
	path := filepath.Join(bootdir, bootstrap.FilenameNodeId)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Error reading file %s: %w", path, err)
	}

	return string(data), nil
}

func runPush(bootdir, token) {
	generateKeys(bootdir)
}

// generateKeys creates the transit keypair, and also writes them to disk for later
func generateKeys(bootdir string) (transitPub, transitPriv [32]byte, err error) {

	fmt.Fprintf(os.Stderr, "Generating keypair %s\n", filename)

	// Generate the keypair
	priv, pub, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create keys: %w", err)
	}

	// Write private key file
	privateFile, err := os.Create(filename + ".priv")
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to open %s.priv for writing: %w", filename, err)
	}
	defer privateFile.Close()

	_, err = privateFile.Write(priv[:])
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to write public key file: %w", err)
	}

	// Write public key file
	publicFile, err := os.Create(filename + ".pub")
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to open %s.pub for writing: %w", filename, err)
	}
	defer publicFile.Close()

	_, err = publicFile.Write(pub[:])
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to write public key file: %w", err)
	}

	return pub, priv, nil
}

func unwrapFile(filename, keyfile string) error {

	ciphertext, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("Failed to open ciphertext file %s: %w", filename, err)
	}

	publicKey, err := ioutil.ReadFile(keyfile+".pub",)
	if err != nil {
		return fmt.Errorf("Faield to open public keyfile %s.pub: %w", keyfile, err)
	}

	privateKey, err := ioutil.ReadFile(keyfile+".priv",)
	if err != nil {
		return fmt.Errorf("Faield to open private keyfile %s.priv: %w", keyfile, err)
	}

	outputFile, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Could not open output file for writing %s: %w", filename, err)
	}
	defer outputFile.Close()


	// NaCl is picky and wants its type to be exactly a [32]byte, but readfile reads a slice
	var pubKeyBytes, privKeyBytes [32]byte
	copy(pubKeyBytes[:], publicKey)
	copy(privKeyBytes[:], privateKey)

	plaintext := make([]byte, 0, len(ciphertext) - box.AnonymousOverhead)
	plaintext, ok := box.OpenAnonymous(plaintext, ciphertext, &pubKeyBytes, &privKeyBytes)
	if !ok {
		return fmt.Errorf("Failed to decrypt ciphertext: unknown error")
	}

	_, err = outputFile.Write(plaintext)
	if err != nil {
		return fmt.Errorf("Failed to write the decrypted file: %w", err)
	}

	return nil
}

func wrapFile(inputFile, keyfile, outputFile string) error {
	plaintext, err := ioutil.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("Failed to open plaintext file %s: %w", inputFile, err)
	}

	publicKey, err := ioutil.ReadFile(keyfile)
	if err != nil {
		return fmt.Errorf("Faield to open public keyfile %s: %w", keyfile, err)
	}

	ciphertextFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("Could not open output file for writing %s: %w", outputFile, err)
	}
	defer ciphertextFile.Close()

	var pubKeyBytes [32]byte
	copy(pubKeyBytes[:], publicKey)

	ciphertext := make([]byte, 0, len(plaintext) + box.AnonymousOverhead)

	ciphertext, err = box.SealAnonymous(ciphertext, plaintext, &pubKeyBytes, rand.Reader)
	if err != nil {
		return fmt.Errorf("Could not encrypt file: %w", err)
	}

	return nil
}
