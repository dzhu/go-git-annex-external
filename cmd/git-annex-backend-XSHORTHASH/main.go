// Command git-annex-backend-XSHORTHASH is an external backend for git-annex that computes keys
// using a short prefix of the SHA512 hash of a file. It is meant as a demonstration of the usage of
// the backend package; in practice, git-annex's native backends should be used instead.
package main

import (
	"crypto/sha512"
	"encoding/hex"
	"io"
	"os"

	"github.com/dzhu/go-git-annex-external/backend"
)

type shortHashBackend struct{}

func (h *shortHashBackend) IsStable(a backend.Annex) bool {
	return true
}

func (h *shortHashBackend) GenKey(a backend.Annex, file string) (string, bool, error) {
	hasher := sha512.New()
	f, err := os.Open(file)
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	_, err = io.Copy(hasher, f)
	if err != nil {
		return "", false, err
	}

	return hex.EncodeToString(hasher.Sum(nil)[:4]), true, nil
}

func (h *shortHashBackend) VerifyKeyContent(a backend.Annex, key, file string) bool {
	key2, _, err := h.GenKey(a, file)
	return err == nil && key == key2
}

func (h *shortHashBackend) IsCryptographicallySecure(a backend.Annex) bool {
	return false
}

// Statically ensure that the backend correctly implements the desired optional interfaces.
var (
	_ backend.HasVerifyKeyContent = (*shortHashBackend)(nil)
)

func main() {
	backend.Run(&shortHashBackend{})
}
