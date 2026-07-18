// Package fingerprint computes deterministic content hashes of directory
// trees, used to detect when checked-in generated code has drifted from what
// was recorded at generation time.
package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// HashDir returns a deterministic "sha256:<hex>" digest over every file
// under root, or ("", nil) if root does not exist. The digest covers each
// file's root-relative slash path and content, so renames and edits both
// change the result, while the order files were created in does not.
func HashDir(root string) (string, error) {
	if _, err := os.Stat(root); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}

	var relPaths []string
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relPaths = append(relPaths, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		return "", err
	}
	sort.Strings(relPaths)

	h := sha256.New()
	for _, rel := range relPaths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		writeField(h, rel, data)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// writeField length-prefixes each field so concatenation is unambiguous.
// Mirrors internal/spec.Spec.Hash's writeField.
func writeField(h interface{ Write([]byte) (int, error) }, name string, data []byte) {
	var lenbuf [8]byte
	putUint64(lenbuf[:], uint64(len(name)))
	h.Write(lenbuf[:])
	h.Write([]byte(name))
	putUint64(lenbuf[:], uint64(len(data)))
	h.Write(lenbuf[:])
	h.Write(data)
}

func putUint64(b []byte, v uint64) {
	for i := 0; i < 8; i++ {
		b[i] = byte(v >> (8 * i))
	}
}
