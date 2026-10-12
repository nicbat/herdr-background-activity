// Package buildinfo identifies the running reporter, including rebuilds of the
// same release. The digest is calculated once, before any upgrade replaces it.
package buildinfo

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"sync"
)

var Version = "0.5.0"

var Digest = sync.OnceValues(func() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
})
