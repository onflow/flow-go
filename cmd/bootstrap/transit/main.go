package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"flag"
	"io/ioutil"

	"golang.org/x/crypto/nacl/box"
)

func main() {

	var ciphertext, keyfile, keyInFile string
	var mode string

	plaintextFile := "random-beacon-key"

	flag.StringVar(&keyInFile, "key", "transit-key", "The name of the keypair files generated without the pub/priv suffix")
	flag.StringVar(&ciphertext, "encrypted-file", "random-beacon-key.enc", "Wrapped random beacon key input file")
	flag.StringVar(&keyfile, "create-key", "transit-key", "The name of the keypair files to generate")
	flag.StringVar(&mode, "mode", "decrypt", "One of [decrypt, gen-keys]")
	flag.Parse()

	var err error = nil
	switch mode {
	case "decrypt":
		err = unwrapFile(ciphertext, keyInFile)
	case "gen-keys":
		err = generateKeys(keyfile)
	case "encrypt":
		fmt.Println("What are you trying to encrypt? RTFM dude.")
		os.Exit(13)
		return
	case "wrap-file":
		err = wrapFile(plaintextFile, keyInFile, ciphertext)
	default:
		flag.Usage()
		return
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

}

func generateKeys(filename string) error {

	fmt.Fprintf(os.Stderr, "Generating keypair %s\n", filename)

	// Generate the keypair
	priv, pub, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("Failed to create keys: %w", err)
	}

	// Write private key file
	privateFile, err := os.Create(filename + ".priv")
	if err != nil {
		return fmt.Errorf("Failed to open %s.priv for writing: %w", filename, err)
	}
	defer privateFile.Close()

	_, err = privateFile.Write(priv[:])
	if err != nil {
		return fmt.Errorf("Failed to write public key file: %w", err)
	}

	// Write public key file
	publicFile, err := os.Create(filename + ".pub")
	if err != nil {
		return fmt.Errorf("Failed to open %s.pub for writing: %w", filename, err)
	}
	defer publicFile.Close()

	_, err = publicFile.Write(pub[:])
	if err != nil {
		return fmt.Errorf("Failed to write public key file: %w", err)
	}

	return nil
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
