package oldcrypto

import (
	"fmt"
)

// InvalidSeed indicates that a the supplied seed could not be used to generate a master extended key.
type InvalidSeed struct {
	seed string
}

func (e *InvalidSeed) Error() string {
	return fmt.Sprintf("Invalid seed: %s", e.seed)
}

// InvalidMnemonic indicates that the supplied mnemonic could not be used to generate an HD wallet.
type InvalidMnemonic struct {
	mnemonic string
}

func (e *InvalidMnemonic) Error() string {
	return fmt.Sprintf("Invalid mnemonic: %s", e.mnemonic)
}

// InvalidDerivationPath indicates that the supplied derivation path was invalid (see BIP-0044).
type InvalidDerivationPath struct {
	path string
}

func (e *InvalidDerivationPath) Error() string {
	return fmt.Sprintf("Invalid derivation path: %s", e.path)
}

// FileDoesNotExistError indicates that the file does not exist.
type FileDoesNotExistError struct {
	filename string
}

func (e *FileDoesNotExistError) Error() string {
	return fmt.Sprintf("%s does not exist", e.filename)
}
